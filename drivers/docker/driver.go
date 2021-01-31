package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/consul-template/signals"
	hclog "github.com/hashicorp/go-hclog"
	multierror "github.com/hashicorp/go-multierror"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/drivers/docker/docklog"
	"github.com/hashicorp/nomad/drivers/shared/eventer"
	"github.com/hashicorp/nomad/drivers/shared/resolvconf"
	nstructs "github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	pstructs "github.com/hashicorp/nomad/plugins/shared/structs"
)

var (
	// createClientsLock is a lock that protects reading/writing global client
	// variables
	createClientsLock sync.Mutex

	// client is a docker client with a timeout of 5 minutes. This is for doing
	// all operations with the docker daemon besides which are not long running
	// such as creating, killing containers, etc.
	client *docker.Client

	// waitClient is a docker client with no timeouts. This is used for long
	// running operations such as waiting on containers and collect stats
	waitClient *docker.Client

	dockerTransientErrs = []string{
		"Client.Timeout exceeded while awaiting headers",
		"EOF",
		"API error (500)",
	}

	// recoverableErrTimeouts returns a recoverable error if the error was due
	// to timeouts
	recoverableErrTimeouts = func(err error) error {
		r := false
		if strings.Contains(err.Error(), "Client.Timeout exceeded while awaiting headers") ||
			strings.Contains(err.Error(), "EOF") {
			r = true
		}
		return nstructs.NewRecoverableError(err, r)
	}

	// taskHandleVersion is the version of task handle which this driver sets
	// and understands how to decode driver state
	taskHandleVersion = 1

	// Nvidia-container-runtime environment variable names
	nvidiaVisibleDevices = "NVIDIA_VISIBLE_DEVICES"
)

const (
	dockerLabelAllocID = "com.hashicorp.nomad.alloc_id"
)

type Driver struct {
	// eventer is used to handle multiplexing of TaskEvents calls such that an
	// event can be broadcast to all callers
	eventer *eventer.Eventer

	// config contains the runtime configuration for the driver set by the
	// SetConfig RPC
	config *DriverConfig

	// clientConfig contains a driver specific subset of the Nomad client
	// configuration
	clientConfig *base.ClientDriverConfig

	// ctx is the context for the driver. It is passed to other subsystems to
	// coordinate shutdown
	ctx context.Context

	// tasks is the in memory datastore mapping taskIDs to taskHandles
	tasks *taskStore

	// coordinator is what tracks multiple image pulls against the same docker image
	coordinator *dockerCoordinator

	// logger will log to the Nomad agent
	logger hclog.Logger

	// gpuRuntime indicates nvidia-docker runtime availability
	gpuRuntime bool

	// A tri-state boolean to know if the fingerprinting has happened and
	// whether it has been successful
	fingerprintSuccess *bool
	fingerprintLock    sync.RWMutex

	// A boolean to know if the docker driver has ever been correctly detected
	// for use during fingerprinting.
	detected     bool
	detectedLock sync.RWMutex

	reconciler *containerReconciler
}

// NewDockerDriver returns a docker implementation of a driver plugin
func NewDockerDriver(ctx context.Context, logger hclog.Logger) drivers.DriverPlugin {
	logger = logger.Named(pluginName)
	return &Driver{
		eventer: eventer.NewEventer(ctx, logger),
		config:  &DriverConfig{},
		tasks:   newTaskStore(),
		ctx:     ctx,
		logger:  logger,
	}
}

func (d *Driver) reattachToDockerLogger(reattachConfig *pstructs.ReattachConfig) (docklog.DockerLogger, *plugin.Client, error) {
	reattach, err := pstructs.ReattachConfigToGoPlugin(reattachConfig)
	if err != nil {
		return nil, nil, err
	}

	dlogger, dloggerPluginClient, err := docklog.ReattachDockerLogger(reattach)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to reattach to docker logger process: %v", err)
	}

	return dlogger, dloggerPluginClient, nil
}

func (d *Driver) setupNewDockerLogger(container *docker.Container, cfg *drivers.TaskConfig, startTime time.Time) (docklog.DockerLogger, *plugin.Client, error) {
	dlogger, pluginClient, err := docklog.LaunchDockerLogger(d.logger)
	if err != nil {
		if pluginClient != nil {
			pluginClient.Kill()
		}
		return nil, nil, fmt.Errorf("failed to launch docker logger plugin: %v", err)
	}

	if err := dlogger.Start(&docklog.StartOpts{
		Endpoint:    d.config.Endpoint,
		ContainerID: container.ID,
		TTY:         container.Config.Tty,
		Stdout:      cfg.StdoutPath,
		Stderr:      cfg.StderrPath,
		TLSCert:     d.config.TLS.Cert,
		TLSKey:      d.config.TLS.Key,
		TLSCA:       d.config.TLS.CA,
		StartTime:   startTime.Unix(),
	}); err != nil {
		pluginClient.Kill()
		return nil, nil, fmt.Errorf("failed to launch docker logger process %s: %v", container.ID, err)
	}

	return dlogger, pluginClient, nil
}

func (d *Driver) RecoverTask(handle *drivers.TaskHandle) error {
	if _, ok := d.tasks.Get(handle.Config.ID); ok {
		return nil
	}

	// COMPAT(0.10): pre 0.9 upgrade path check
	if handle.Version == 0 {
		return d.recoverPre09Task(handle)
	}

	var handleState taskHandleState
	if err := handle.GetDriverState(&handleState); err != nil {
		return fmt.Errorf("failed to decode driver task state: %v", err)
	}

	client, _, err := d.dockerClients()
	if err != nil {
		return fmt.Errorf("failed to get docker client: %v", err)
	}

	container, err := client.InspectContainerWithOptions(docker.InspectContainerOptions{
		ID: handleState.ContainerID,
	})
	if err != nil {
		return fmt.Errorf("failed to inspect container for id %q: %v", handleState.ContainerID, err)
	}

	h := &taskHandle{
		client:                client,
		waitClient:            waitClient,
		logger:                d.logger.With("container_id", container.ID),
		task:                  handle.Config,
		containerID:           container.ID,
		containerImage:        container.Image,
		doneCh:                make(chan bool),
		waitCh:                make(chan struct{}),
		removeContainerOnExit: d.config.GC.Container,
		net:                   handleState.DriverNetwork,
	}

	if !d.config.DisableLogCollection {
		h.dlogger, h.dloggerPluginClient, err = d.reattachToDockerLogger(handleState.ReattachConfig)
		if err != nil {
			d.logger.Warn("failed to reattach to docker logger process", "error", err)

			h.dlogger, h.dloggerPluginClient, err = d.setupNewDockerLogger(container, handle.Config, time.Now())
			if err != nil {
				if err := client.StopContainer(handleState.ContainerID, 0); err != nil {
					d.logger.Warn("failed to stop container during cleanup", "container_id", handleState.ContainerID, "error", err)
				}
				return fmt.Errorf("failed to setup replacement docker logger: %v", err)
			}

			if err := handle.SetDriverState(h.buildState()); err != nil {
				if err := client.StopContainer(handleState.ContainerID, 0); err != nil {
					d.logger.Warn("failed to stop container during cleanup", "container_id", handleState.ContainerID, "error", err)
				}
				return fmt.Errorf("failed to store driver state: %v", err)
			}
		}
	}

	d.tasks.Set(handle.Config.ID, h)
	go h.run()

	return nil
}

func (d *Driver) StartTask(cfg *drivers.TaskConfig) (*drivers.TaskHandle, *drivers.DriverNetwork, error) {
	if _, ok := d.tasks.Get(cfg.ID); ok {
		return nil, nil, fmt.Errorf("task with ID %q already started", cfg.ID)
	}

	var driverConfig TaskConfig

	if err := cfg.DecodeDriverConfig(&driverConfig); err != nil {
		return nil, nil, fmt.Errorf("failed to decode driver config: %v", err)
	}

	if driverConfig.Image == "" {
		return nil, nil, fmt.Errorf("image name required for docker driver")
	}

	driverConfig.Image = strings.TrimPrefix(driverConfig.Image, "https://")

	handle := drivers.NewTaskHandle(taskHandleVersion)
	handle.Config = cfg

	// Initialize docker API clients
	client, _, err := d.dockerClients()
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to connect to docker daemon: %s", err)
	}

	id, err := d.createImage(cfg, &driverConfig, client)
	if err != nil {
		return nil, nil, err
	}

	if runtime.GOOS == "windows" {
		err = d.convertAllocPathsForWindowsLCOW(cfg, driverConfig.Image)
		if err != nil {
			return nil, nil, err
		}
	}

	containerCfg, err := d.createContainerConfig(cfg, &driverConfig, driverConfig.Image)
	if err != nil {
		d.logger.Error("failed to create container configuration", "image_name", driverConfig.Image,
			"image_id", id, "error", err)
		return nil, nil, fmt.Errorf("Failed to create container configuration for image %q (%q): %v", driverConfig.Image, id, err)
	}

	startAttempts := 0
CREATE:
	container, err := d.createContainer(client, containerCfg, driverConfig.Image)
	if err != nil {
		d.logger.Error("failed to create container", "error", err)
		client.RemoveContainer(docker.RemoveContainerOptions{
			ID:    containerCfg.Name,
			Force: true,
		})
		return nil, nil, nstructs.WrapRecoverable(fmt.Sprintf("failed to create container: %v", err), err)
	}

	d.logger.Info("created container", "container_id", container.ID)

	// We don't need to start the container if the container is already running
	// since we don't create containers which are already present on the host
	// and are running
	if !container.State.Running {
		// Start the container
		if err := d.startContainer(container); err != nil {
			d.logger.Error("failed to start container", "container_id", container.ID, "error", err)
			client.RemoveContainer(docker.RemoveContainerOptions{
				ID:    container.ID,
				Force: true,
			})
			// Some sort of docker race bug, recreating the container usually works
			if strings.Contains(err.Error(), "OCI runtime create failed: container with id exists:") && startAttempts < 5 {
				startAttempts++
				d.logger.Debug("reattempting container create/start sequence", "attempt", startAttempts, "container_id", id)
				goto CREATE
			}
			return nil, nil, nstructs.WrapRecoverable(fmt.Sprintf("Failed to start container %s: %s", container.ID, err), err)
		}

		// Inspect container to get all of the container metadata as much of the
		// metadata (eg networking) isn't populated until the container is started
		runningContainer, err := client.InspectContainerWithOptions(docker.InspectContainerOptions{
			ID: container.ID,
		})
		if err != nil {
			client.RemoveContainer(docker.RemoveContainerOptions{
				ID:    container.ID,
				Force: true,
			})
			msg := "failed to inspect started container"
			d.logger.Error(msg, "error", err)
			client.RemoveContainer(docker.RemoveContainerOptions{
				ID:    container.ID,
				Force: true,
			})
			return nil, nil, nstructs.NewRecoverableError(fmt.Errorf("%s %s: %s", msg, container.ID, err), true)
		}
		container = runningContainer
		d.logger.Info("started container", "container_id", container.ID)
	} else {
		d.logger.Debug("re-attaching to container", "container_id",
			container.ID, "container_state", container.State.String())
	}

	collectingLogs := !d.config.DisableLogCollection

	var dlogger docklog.DockerLogger
	var pluginClient *plugin.Client

	if collectingLogs {
		dlogger, pluginClient, err = d.setupNewDockerLogger(container, cfg, time.Unix(0, 0))
		if err != nil {
			d.logger.Error("an error occurred after container startup, terminating container", "container_id", container.ID)
			client.RemoveContainer(docker.RemoveContainerOptions{ID: container.ID, Force: true})
			return nil, nil, err
		}
	}

	// Detect container address
	ip, autoUse := d.detectIP(container, &driverConfig)

	net := &drivers.DriverNetwork{
		PortMap:       driverConfig.PortMap,
		IP:            ip,
		AutoAdvertise: autoUse,
	}

	// Return a driver handle
	h := &taskHandle{
		client:                client,
		waitClient:            waitClient,
		dlogger:               dlogger,
		dloggerPluginClient:   pluginClient,
		logger:                d.logger.With("container_id", container.ID),
		task:                  cfg,
		containerID:           container.ID,
		containerImage:        container.Image,
		doneCh:                make(chan bool),
		waitCh:                make(chan struct{}),
		removeContainerOnExit: d.config.GC.Container,
		net:                   net,
	}

	if err := handle.SetDriverState(h.buildState()); err != nil {
		d.logger.Error("error encoding container occurred after startup, terminating container", "container_id", container.ID, "error", err)
		if collectingLogs {
			dlogger.Stop()
			pluginClient.Kill()
		}
		client.RemoveContainer(docker.RemoveContainerOptions{ID: container.ID, Force: true})
		return nil, nil, err
	}

	d.tasks.Set(cfg.ID, h)
	go h.run()

	return handle, net, nil
}

// createContainerClient is the subset of Docker Client methods used by the
// createContainer method to ease testing subtle error conditions.
type createContainerClient interface {
	CreateContainer(docker.CreateContainerOptions) (*docker.Container, error)
	InspectContainer(id string) (*docker.Container, error)
	ListContainers(docker.ListContainersOptions) ([]docker.APIContainers, error)
	RemoveContainer(opts docker.RemoveContainerOptions) error
}

// createContainer creates the container given the passed configuration. It
// attempts to handle any transient Docker errors.
func (d *Driver) createContainer(client createContainerClient, config docker.CreateContainerOptions,
	image string) (*docker.Container, error) {
	// Create a container
	attempted := 0
CREATE:
	container, createErr := client.CreateContainer(config)
	if createErr == nil {
		return container, nil
	}

	d.logger.Debug("failed to create container", "container_name",
		config.Name, "image_name", image, "image_id", config.Config.Image,
		"attempt", attempted+1, "error", createErr)

	// Volume management tools like Portworx may not have detached a volume
	// from a previous node before Nomad started a task replacement task.
	// Treat these errors as recoverable so we retry.
	if strings.Contains(strings.ToLower(createErr.Error()), "volume is attached on another node") {
		return nil, nstructs.NewRecoverableError(createErr, true)
	}

	// If the container already exists determine whether it's already
	// running or if it's dead and needs to be recreated.
	if strings.Contains(strings.ToLower(createErr.Error()), "container already exists") {

		container, err := d.containerByName(config.Name)
		if err != nil {
			return nil, err
		}

		if container != nil && container.State.Running {
			return container, nil
		}

		// Purge conflicting container if found.
		// If container is nil here, the conflicting container was
		// deleted in our check here, so retry again.
		if container != nil {
			// Delete matching containers
			err = client.RemoveContainer(docker.RemoveContainerOptions{
				ID:    container.ID,
				Force: true,
			})
			if err != nil {
				d.logger.Error("failed to purge container", "container_id", container.ID)
				return nil, recoverableErrTimeouts(fmt.Errorf("Failed to purge container %s: %s", container.ID, err))
			} else {
				d.logger.Info("purged container", "container_id", container.ID)
			}
		}

		if attempted < 5 {
			attempted++
			time.Sleep(nextBackoff(attempted))
			goto CREATE
		}
	} else if strings.Contains(strings.ToLower(createErr.Error()), "no such image") {
		// There is still a very small chance this is possible even with the
		// coordinator so retry.
		return nil, nstructs.NewRecoverableError(createErr, true)
	} else if isDockerTransientError(createErr) && attempted < 5 {
		attempted++
		time.Sleep(nextBackoff(attempted))
		goto CREATE
	}

	return nil, recoverableErrTimeouts(createErr)
}

// startContainer starts the passed container. It attempts to handle any
// transient Docker errors.
func (d *Driver) startContainer(c *docker.Container) error {
	// Start a container
	attempted := 0
START:
	startErr := client.StartContainer(c.ID, c.HostConfig)
	if startErr == nil || strings.Contains(startErr.Error(), "Container already running") {
		return nil
	}

	d.logger.Debug("failed to start container", "container_id", c.ID, "attempt", attempted+1, "error", startErr)

	if isDockerTransientError(startErr) {
		if attempted < 5 {
			attempted++
			time.Sleep(nextBackoff(attempted))
			goto START
		}
		return nstructs.NewRecoverableError(startErr, true)
	}

	return recoverableErrTimeouts(startErr)
}

// nextBackoff returns appropriate docker backoff durations after attempted attempts.
func nextBackoff(attempted int) time.Duration {
	// attempts in 200ms, 800ms, 3.2s, 12.8s, 51.2s
	// TODO: add randomization factor and extract to a helper
	return 1 << (2 * uint64(attempted)) * 50 * time.Millisecond
}

// createImage creates a docker image either by pulling it from a registry or by
// loading it from the file system
func (d *Driver) createImage(task *drivers.TaskConfig, driverConfig *TaskConfig, client *docker.Client) (string, error) {
	image := driverConfig.Image
	repo, tag := parseDockerImage(image)

	// We're going to check whether the image is already downloaded. If the tag
	// is "latest", or ForcePull is set, we have to check for a new version every time so we don't
	// bother to check and cache the id here. We'll download first, then cache.
	if driverConfig.ForcePull {
		d.logger.Debug("force pulling image instead of inspecting local", "image_ref", dockerImageRef(repo, tag))
	} else if tag != "latest" {
		if dockerImage, _ := client.InspectImage(image); dockerImage != nil {
			// Image exists so just increment its reference count
			d.coordinator.IncrementImageReference(dockerImage.ID, image, task.ID)
			return dockerImage.ID, nil
		}
	}

	// Load the image if specified
	if driverConfig.LoadImage != "" {
		return d.loadImage(task, driverConfig, client)
	}

	// Download the image
	return d.pullImage(task, driverConfig, client, repo, tag)
}

// pullImage creates an image by pulling it from a docker registry
func (d *Driver) pullImage(task *drivers.TaskConfig, driverConfig *TaskConfig, client *docker.Client, repo, tag string) (id string, err error) {
	authOptions, err := d.resolveRegistryAuthentication(driverConfig, repo)
	if err != nil {
		if driverConfig.AuthSoftFail {
			d.logger.Warn("Failed to find docker repo auth", "repo", repo, "error", err)
		} else {
			return "", fmt.Errorf("Failed to find docker auth for repo %q: %v", repo, err)
		}
	}

	if authIsEmpty(authOptions) {
		d.logger.Debug("did not find docker auth for repo", "repo", repo)
	}

	d.eventer.EmitEvent(&drivers.TaskEvent{
		TaskID:    task.ID,
		AllocID:   task.AllocID,
		TaskName:  task.Name,
		Timestamp: time.Now(),
		Message:   "Downloading image",
		Annotations: map[string]string{
			"image": dockerImageRef(repo, tag),
		},
	})

	pullDur, err := time.ParseDuration(driverConfig.ImagePullTimeout)
	if err != nil {
		return "", fmt.Errorf("Failed to parse image_pull_timeout: %v", err)
	}

	return d.coordinator.PullImage(driverConfig.Image, authOptions, task.ID, d.emitEventFunc(task), pullDur, d.config.pullActivityTimeoutDuration)
}

func (d *Driver) emitEventFunc(task *drivers.TaskConfig) LogEventFn {
	return func(msg string, annotations map[string]string) {
		d.eventer.EmitEvent(&drivers.TaskEvent{
			TaskID:      task.ID,
			AllocID:     task.AllocID,
			TaskName:    task.Name,
			Timestamp:   time.Now(),
			Message:     msg,
			Annotations: annotations,
		})
	}
}

// authBackend encapsulates a function that resolves registry credentials.
type authBackend func(string) (*docker.AuthConfiguration, error)

// resolveRegistryAuthentication attempts to retrieve auth credentials for the
// repo, trying all authentication-backends possible.
func (d *Driver) resolveRegistryAuthentication(driverConfig *TaskConfig, repo string) (*docker.AuthConfiguration, error) {
	return firstValidAuth(repo, []authBackend{
		authFromTaskConfig(driverConfig),
		authFromDockerConfig(d.config.Auth.Config),
		authFromHelper(d.config.Auth.Helper),
	})
}

// loadImage creates an image by loading it from the file system
func (d *Driver) loadImage(task *drivers.TaskConfig, driverConfig *TaskConfig, client *docker.Client) (id string, err error) {

	archive := filepath.Join(task.TaskDir().LocalDir, driverConfig.LoadImage)
	d.logger.Debug("loading image from disk", "archive", archive)

	f, err := os.Open(archive)
	if err != nil {
		return "", fmt.Errorf("unable to open image archive: %v", err)
	}

	if err := client.LoadImage(docker.LoadImageOptions{InputStream: f}); err != nil {
		return "", err
	}
	f.Close()

	dockerImage, err := client.InspectImage(driverConfig.Image)
	if err != nil {
		return "", recoverableErrTimeouts(err)
	}

	d.coordinator.IncrementImageReference(dockerImage.ID, driverConfig.Image, task.ID)
	return dockerImage.ID, nil
}

func (d *Driver) convertAllocPathsForWindowsLCOW(task *drivers.TaskConfig, image string) error {
	imageConfig, err := client.InspectImage(image)
	if err != nil {
		return fmt.Errorf("the image does not exist: %v", err)
	}
	// LCOW If we are running a Linux Container on Windows, we need to mount it correctly, as c:\ does not exist on unix
	if imageConfig.OS == "linux" {
		a := []rune(task.Env[taskenv.AllocDir])
		task.Env[taskenv.AllocDir] = strings.ReplaceAll(string(a[2:]), "\\", "/")
		l := []rune(task.Env[taskenv.TaskLocalDir])
		task.Env[taskenv.TaskLocalDir] = strings.ReplaceAll(string(l[2:]), "\\", "/")
		s := []rune(task.Env[taskenv.SecretsDir])
		task.Env[taskenv.SecretsDir] = strings.ReplaceAll(string(s[2:]), "\\", "/")
	}
	return nil
}

func (d *Driver) containerBinds(task *drivers.TaskConfig, driverConfig *TaskConfig) ([]string, error) {
	allocDirBind := fmt.Sprintf("%s:%s", task.TaskDir().SharedAllocDir, task.Env[taskenv.AllocDir])
	taskLocalBind := fmt.Sprintf("%s:%s", task.TaskDir().LocalDir, task.Env[taskenv.TaskLocalDir])
	secretDirBind := fmt.Sprintf("%s:%s", task.TaskDir().SecretsDir, task.Env[taskenv.SecretsDir])
	binds := []string{allocDirBind, taskLocalBind, secretDirBind}

	taskLocalBindVolume := driverConfig.VolumeDriver == ""

	if !d.config.Volumes.Enabled && !taskLocalBindVolume {
		return nil, fmt.Errorf("volumes are not enabled; cannot use volume driver %q", driverConfig.VolumeDriver)
	}

	for _, userbind := range driverConfig.Volumes {
		// This assumes host OS = docker container OS.
		// Not true, when we support Linux containers on Windows
		src, dst, mode, err := parseVolumeSpec(userbind, runtime.GOOS)
		if err != nil {
			return nil, fmt.Errorf("invalid docker volume %q: %v", userbind, err)
		}

		// Paths inside task dir are always allowed when using the default driver,
		// Relative paths are always allowed as they mount within a container
		// When a VolumeDriver is set, we assume we receive a binding in the format
		// volume-name:container-dest
		// Otherwise, we assume we receive a relative path binding in the format
		// relative/to/task:/also/in/container
		if taskLocalBindVolume {
			src = expandPath(task.TaskDir().Dir, src)
		} else {
			// Resolve dotted path segments
			src = filepath.Clean(src)
		}

		if !d.config.Volumes.Enabled && !isParentPath(task.AllocDir, src) {
			return nil, fmt.Errorf("volumes are not enabled; cannot mount host paths: %+q", userbind)
		}

		bind := src + ":" + dst
		if mode != "" {
			bind += ":" + mode
		}
		binds = append(binds, bind)
	}

	if selinuxLabel := d.config.Volumes.SelinuxLabel; selinuxLabel != "" {
		// Apply SELinux Label to each volume
		for i := range binds {
			binds[i] = fmt.Sprintf("%s:%s", binds[i], selinuxLabel)
		}
	}

	return binds, nil
}

var userMountToUnixMount = map[string]string{
	// Empty string maps to `rprivate` for backwards compatibility in restored
	// older tasks, where mount propagation will not be present.
	"":                                     "rprivate",
	nstructs.VolumeMountPropagationPrivate: "rprivate",
	nstructs.VolumeMountPropagationHostToTask:    "rslave",
	nstructs.VolumeMountPropagationBidirectional: "rshared",
}

// takes a local seccomp daemon, reads the file contents for sending to the daemon
// this code modified slightly from the docker CLI code
// https://github.com/docker/cli/blob/8ef8547eb6934b28497d309d21e280bcd25145f5/cli/command/container/opts.go#L840
func parseSecurityOpts(securityOpts []string) ([]string, error) {
	for key, opt := range securityOpts {
		con := strings.SplitN(opt, "=", 2)
		if len(con) == 1 && con[0] != "no-new-privileges" {
			if strings.Contains(opt, ":") {
				con = strings.SplitN(opt, ":", 2)
			} else {
				return securityOpts, fmt.Errorf("invalid security_opt: %q", opt)
			}
		}
		if con[0] == "seccomp" && con[1] != "unconfined" {
			f, err := ioutil.ReadFile(con[1])
			if err != nil {
				return securityOpts, fmt.Errorf("opening seccomp profile (%s) failed: %v", con[1], err)
			}
			b := bytes.NewBuffer(nil)
			if err := json.Compact(b, f); err != nil {
				return securityOpts, fmt.Errorf("compacting json for seccomp profile (%s) failed: %v", con[1], err)
			}
			securityOpts[key] = fmt.Sprintf("seccomp=%s", b.Bytes())
		}
	}

	return securityOpts, nil
}

// memoryLimits computes the memory and memory_reservation values passed along to
// the docker host config. These fields represent hard and soft memory limits from
// docker's perspective, respectively.
//
// The memory field on the task configuration can be interpreted as a hard or soft
// limit. Before Nomad v0.11.3, it was always a hard limit. Now, it is interpreted
// as a soft limit if the memory_hard_limit value is configured on the docker
// task driver configuration. When memory_hard_limit is set, the docker host
// config is configured such that the memory field is equal to memory_hard_limit
// value, and the memory_reservation field is set to the task driver memory value.
//
// If memory_hard_limit is not set (i.e. zero value), then the memory field of
// the task resource config is interpreted as a hard limit. In this case both the
// memory is set to the task resource memory value and memory_reservation is left
// unset.
//
// Returns (memory (hard), memory_reservation (soft)) values in bytes.
func (_ *Driver) memoryLimits(driverHardLimitMB, taskMemoryLimitBytes int64) (int64, int64) {
	if driverHardLimitMB <= 0 {
		return taskMemoryLimitBytes, 0
	}
	return driverHardLimitMB * 1024 * 1024, taskMemoryLimitBytes
}

func (d *Driver) createContainerConfig(task *drivers.TaskConfig, driverConfig *TaskConfig,
	imageID string) (docker.CreateContainerOptions, error) {

	// ensure that PortMap variables are populated early on
	task.Env = taskenv.SetPortMapEnvs(task.Env, driverConfig.PortMap)

	logger := d.logger.With("task_name", task.Name)
	var c docker.CreateContainerOptions
	if task.Resources == nil {
		// Guard against missing resources. We should never have been able to
		// schedule a job without specifying this.
		logger.Error("task.Resources is empty")
		return c, fmt.Errorf("task.Resources is empty")
	}
	binds, err := d.containerBinds(task, driverConfig)
	if err != nil {
		return c, err
	}
	logger.Trace("binding volumes", "volumes", binds)

	// create the config block that will later be consumed by go-dockerclient
	config := &docker.Config{
		Image:      imageID,
		Entrypoint: driverConfig.Entrypoint,
		Hostname:   driverConfig.Hostname,
		User:       task.User,
		Tty:        driverConfig.TTY,
		OpenStdin:  driverConfig.Interactive,
	}

	if driverConfig.WorkDir != "" {
		config.WorkingDir = driverConfig.WorkDir
	}

	containerRuntime := driverConfig.Runtime
	if _, ok := task.DeviceEnv[nvidiaVisibleDevices]; ok {
		if !d.gpuRuntime {
			return c, fmt.Errorf("requested docker runtime %q was not found", d.config.GPURuntimeName)
		}
		if containerRuntime != "" && containerRuntime != d.config.GPURuntimeName {
			return c, fmt.Errorf("conflicting runtime requests: gpu runtime %q conflicts with task runtime %q", d.config.GPURuntimeName, containerRuntime)
		}
		containerRuntime = d.config.GPURuntimeName
	}
	if _, ok := d.config.allowRuntimes[containerRuntime]; !ok && containerRuntime != "" {
		return c, fmt.Errorf("requested runtime %q is not allowed", containerRuntime)
	}

	memory, memoryReservation := d.memoryLimits(driverConfig.MemoryHardLimit, task.Resources.LinuxResources.MemoryLimitBytes)

	hostConfig := &docker.HostConfig{
		Memory:            memory,            // hard limit
		MemoryReservation: memoryReservation, // soft limit

		CPUShares: task.Resources.LinuxResources.CPUShares,

		// Binds are used to mount a host volume into the container. We mount a
		// local directory for storage and a shared alloc directory that can be
		// used to share data between different tasks in the same task group.
		Binds: binds,

		StorageOpt:   driverConfig.StorageOpt,
		VolumeDriver: driverConfig.VolumeDriver,

		PidsLimit: &driverConfig.PidsLimit,

		Runtime: containerRuntime,
	}

	// This translates to docker create/run --cpuset-cpus option.
	// --cpuset-cpus limit the specific CPUs or cores a container can use.
	if driverConfig.CPUSetCPUs != "" {
		hostConfig.CPUSetCPUs = driverConfig.CPUSetCPUs
	}

	// Calculate CPU Quota
	// cfs_quota_us is the time per core, so we must
	// multiply the time by the number of cores available
	// See https://access.redhat.com/documentation/en-us/red_hat_enterprise_linux/6/html/resource_management_guide/sec-cpu
	if driverConfig.CPUHardLimit {
		numCores := runtime.NumCPU()
		if driverConfig.CPUCFSPeriod < 0 || driverConfig.CPUCFSPeriod > 1000000 {
			return c, fmt.Errorf("invalid value for cpu_cfs_period")
		}
		if driverConfig.CPUCFSPeriod == 0 {
			driverConfig.CPUCFSPeriod = task.Resources.LinuxResources.CPUPeriod
		}
		hostConfig.CPUPeriod = driverConfig.CPUCFSPeriod
		hostConfig.CPUQuota = int64(task.Resources.LinuxResources.PercentTicks*float64(driverConfig.CPUCFSPeriod)) * int64(numCores)
	}

	// Windows does not support MemorySwap/MemorySwappiness #2193
	if runtime.GOOS == "windows" {
		hostConfig.MemorySwap = 0
		hostConfig.MemorySwappiness = nil
	} else {
		hostConfig.MemorySwap = memory

		// disable swap explicitly in non-Windows environments
		var swapiness int64 = 0
		hostConfig.MemorySwappiness = &swapiness

	}

	loggingDriver := driverConfig.Logging.Type
	if loggingDriver == "" {
		loggingDriver = driverConfig.Logging.Driver
	}

	hostConfig.LogConfig = docker.LogConfig{
		Type:   loggingDriver,
		Config: driverConfig.Logging.Config,
	}

	if hostConfig.LogConfig.Type == "" && hostConfig.LogConfig.Config == nil {
		logger.Trace("no docker log driver provided, defaulting to json-file")
		hostConfig.LogConfig.Type = "json-file"
		hostConfig.LogConfig.Config = map[string]string{
			"max-file": "2",
			"max-size": "2m",
		}
	}

	logger.Debug("configured resources",
		"memory", hostConfig.Memory, "memory_reservation", hostConfig.MemoryReservation,
		"cpu_shares", hostConfig.CPUShares, "cpu_quota", hostConfig.CPUQuota,
		"cpu_period", hostConfig.CPUPeriod)

	logger.Debug("binding directories", "binds", hclog.Fmt("%#v", hostConfig.Binds))

	//  set privileged mode
	if driverConfig.Privileged && !d.config.AllowPrivileged {
		return c, fmt.Errorf(`Docker privileged mode is disabled on this Nomad agent`)
	}
	hostConfig.Privileged = driverConfig.Privileged

	// set capabilities
	hostCapsWhitelistConfig := d.config.AllowCaps
	hostCapsWhitelist := make(map[string]struct{})
	for _, cap := range hostCapsWhitelistConfig {
		cap = strings.ToLower(strings.TrimSpace(cap))
		hostCapsWhitelist[cap] = struct{}{}
	}

	if _, ok := hostCapsWhitelist["all"]; !ok {
		effectiveCaps, err := tweakCapabilities(
			strings.Split(dockerBasicCaps, ","),
			driverConfig.CapAdd,
			driverConfig.CapDrop,
		)
		if err != nil {
			return c, err
		}
		var missingCaps []string
		for _, cap := range effectiveCaps {
			cap = strings.ToLower(cap)
			if _, ok := hostCapsWhitelist[cap]; !ok {
				missingCaps = append(missingCaps, cap)
			}
		}
		if len(missingCaps) > 0 {
			return c, fmt.Errorf("Docker driver doesn't have the following caps allowlisted on this Nomad agent: %s", missingCaps)
		}
	}

	hostConfig.CapAdd = driverConfig.CapAdd
	hostConfig.CapDrop = driverConfig.CapDrop

	// set SHM size
	if driverConfig.ShmSize != 0 {
		hostConfig.ShmSize = driverConfig.ShmSize
	}

	// Setup devices
	for _, device := range driverConfig.Devices {
		dd, err := device.toDockerDevice()
		if err != nil {
			return c, err
		}
		hostConfig.Devices = append(hostConfig.Devices, dd)
	}
	for _, device := range task.Devices {
		hostConfig.Devices = append(hostConfig.Devices, docker.Device{
			PathOnHost:        device.HostPath,
			PathInContainer:   device.TaskPath,
			CgroupPermissions: device.Permissions,
		})
	}

	// Setup mounts
	for _, m := range driverConfig.Mounts {
		hm, err := d.toDockerMount(&m, task)
		if err != nil {
			return c, err
		}
		hostConfig.Mounts = append(hostConfig.Mounts, *hm)
	}
	for _, m := range driverConfig.MountsList {
		hm, err := d.toDockerMount(&m, task)
		if err != nil {
			return c, err
		}
		hostConfig.Mounts = append(hostConfig.Mounts, *hm)
	}

	// Setup DNS
	// If task DNS options are configured Nomad will manage the resolv.conf file
	// Docker driver dns options are not compatible with task dns options
	if task.DNS != nil {
		dnsMount, err := resolvconf.GenerateDNSMount(task.TaskDir().Dir, task.DNS)
		if err != nil {
			return c, fmt.Errorf("failed to build mount for resolv.conf: %v", err)
		}
		hostConfig.Mounts = append(hostConfig.Mounts, docker.HostMount{
			Target:   dnsMount.TaskPath,
			Source:   dnsMount.HostPath,
			Type:     "bind",
			ReadOnly: dnsMount.Readonly,
			BindOptions: &docker.BindOptions{
				Propagation: dnsMount.PropagationMode,
			},
		})
	} else {
		if len(driverConfig.DNSSearchDomains) > 0 {
			hostConfig.DNSSearch = driverConfig.DNSSearchDomains
		}
		if len(driverConfig.DNSOptions) > 0 {
			hostConfig.DNSOptions = driverConfig.DNSOptions
		}
		// set DNS servers
		for _, ip := range driverConfig.DNSServers {
			if net.ParseIP(ip) != nil {
				hostConfig.DNS = append(hostConfig.DNS, ip)
			} else {
				logger.Error("invalid ip address for container dns server", "ip", ip)
			}
		}
	}

	for _, m := range task.Mounts {
		hm := docker.HostMount{
			Type:     "bind",
			Target:   m.TaskPath,
			Source:   m.HostPath,
			ReadOnly: m.Readonly,
		}

		// MountPropagation is only supported by Docker on Linux:
		// https://docs.docker.com/storage/bind-mounts/#configure-bind-propagation
		if runtime.GOOS == "linux" {
			hm.BindOptions = &docker.BindOptions{
				Propagation: userMountToUnixMount[m.PropagationMode],
			}
		}

		hostConfig.Mounts = append(hostConfig.Mounts, hm)
	}

	hostConfig.ExtraHosts = driverConfig.ExtraHosts

	hostConfig.IpcMode = driverConfig.IPCMode
	hostConfig.PidMode = driverConfig.PidMode
	hostConfig.UTSMode = driverConfig.UTSMode
	hostConfig.UsernsMode = driverConfig.UsernsMode
	hostConfig.SecurityOpt = driverConfig.SecurityOpt
	hostConfig.Sysctls = driverConfig.Sysctl

	hostConfig.SecurityOpt, err = parseSecurityOpts(driverConfig.SecurityOpt)
	if err != nil {
		return c, fmt.Errorf("failed to parse security_opt configuration: %v", err)
	}

	ulimits, err := sliceMergeUlimit(driverConfig.Ulimit)
	if err != nil {
		return c, fmt.Errorf("failed to parse ulimit configuration: %v", err)
	}
	hostConfig.Ulimits = ulimits

	hostConfig.ReadonlyRootfs = driverConfig.ReadonlyRootfs

	// set the docker network mode
	hostConfig.NetworkMode = driverConfig.NetworkMode

	// if the driver config does not specify a network mode then try to use the
	// shared alloc network
	if hostConfig.NetworkMode == "" {
		if task.NetworkIsolation != nil && task.NetworkIsolation.Path != "" {
			// find the previously created parent container to join networks with
			netMode := fmt.Sprintf("container:%s", task.NetworkIsolation.Labels[dockerNetSpecLabelKey])
			logger.Debug("configuring network mode for task group", "network_mode", netMode)
			hostConfig.NetworkMode = netMode
		} else {
			// docker default
			logger.Debug("networking mode not specified; using default")
			hostConfig.NetworkMode = "default"
		}
	}

	// Setup port mapping and exposed ports
	ports := newPublishedPorts(logger)
	switch {
	case task.Resources.Ports != nil && len(driverConfig.Ports) > 0:
		// Do not set up docker port mapping if shared alloc networking is used
		if strings.HasPrefix(hostConfig.NetworkMode, "container:") {
			break
		}

		for _, port := range driverConfig.Ports {
			if mapping, ok := task.Resources.Ports.Get(port); ok {
				ports.add(mapping.Label, mapping.HostIP, mapping.Value, mapping.To)
			} else {
				return c, fmt.Errorf("Port %q not found, check network stanza", port)
			}
		}
	case len(task.Resources.NomadResources.Networks) > 0:
		network := task.Resources.NomadResources.Networks[0]

		for _, port := range network.ReservedPorts {
			ports.addMapped(port.Label, network.IP, port.Value, driverConfig.PortMap)
		}

		for _, port := range network.DynamicPorts {
			ports.addMapped(port.Label, network.IP, port.Value, driverConfig.PortMap)
		}

	default:
		if len(driverConfig.PortMap) > 0 {
			if task.Resources.Ports != nil {
				return c, fmt.Errorf("'port_map' cannot map group network ports, use 'ports' instead")
			}
			return c, fmt.Errorf("Trying to map ports but no network interface is available")
		}
	}
	hostConfig.PortBindings = ports.publishedPorts
	config.ExposedPorts = ports.exposedPorts

	// If the user specified a custom command to run, we'll inject it here.
	if driverConfig.Command != "" {
		// Validate command
		if err := validateCommand(driverConfig.Command, "args"); err != nil {
			return c, err
		}

		cmd := []string{driverConfig.Command}
		if len(driverConfig.Args) != 0 {
			cmd = append(cmd, driverConfig.Args...)
		}
		logger.Debug("setting container startup command", "command", strings.Join(cmd, " "))
		config.Cmd = cmd
	} else if len(driverConfig.Args) != 0 {
		config.Cmd = driverConfig.Args
	}

	if len(driverConfig.Labels) > 0 {
		config.Labels = driverConfig.Labels
	}

	labels := make(map[string]string, len(driverConfig.Labels)+1)
	for k, v := range driverConfig.Labels {
		labels[k] = v
	}
	labels[dockerLabelAllocID] = task.AllocID
	config.Labels = labels
	logger.Debug("applied labels on the container", "labels", config.Labels)

	config.Env = task.EnvList()

	containerName := fmt.Sprintf("%s-%s", strings.Replace(task.Name, "/", "_", -1), task.AllocID)
	logger.Debug("setting container name", "container_name", containerName)

	var networkingConfig *docker.NetworkingConfig
	if len(driverConfig.NetworkAliases) > 0 || driverConfig.IPv4Address != "" || driverConfig.IPv6Address != "" {
		networkingConfig = &docker.NetworkingConfig{
			EndpointsConfig: map[string]*docker.EndpointConfig{
				hostConfig.NetworkMode: {},
			},
		}
	}

	if len(driverConfig.NetworkAliases) > 0 {
		networkingConfig.EndpointsConfig[hostConfig.NetworkMode].Aliases = driverConfig.NetworkAliases
		logger.Debug("setting container network aliases", "network_mode", hostConfig.NetworkMode,
			"network_aliases", strings.Join(driverConfig.NetworkAliases, ", "))
	}

	if driverConfig.IPv4Address != "" || driverConfig.IPv6Address != "" {
		networkingConfig.EndpointsConfig[hostConfig.NetworkMode].IPAMConfig = &docker.EndpointIPAMConfig{
			IPv4Address: driverConfig.IPv4Address,
			IPv6Address: driverConfig.IPv6Address,
		}
		logger.Debug("setting container network configuration", "network_mode", hostConfig.NetworkMode,
			"ipv4_address", driverConfig.IPv4Address, "ipv6_address", driverConfig.IPv6Address)
	}

	if driverConfig.MacAddress != "" {
		config.MacAddress = driverConfig.MacAddress
		logger.Debug("setting container mac address", "mac_address", config.MacAddress)
	}

	return docker.CreateContainerOptions{
		Name:             containerName,
		Config:           config,
		HostConfig:       hostConfig,
		NetworkingConfig: networkingConfig,
	}, nil
}

func (d *Driver) toDockerMount(m *DockerMount, task *drivers.TaskConfig) (*docker.HostMount, error) {
	hm, err := m.toDockerHostMount()
	if err != nil {
		return nil, err
	}

	switch hm.Type {
	case "bind":
		hm.Source = expandPath(task.TaskDir().Dir, hm.Source)

		// paths inside alloc dir are always allowed as they mount within
		// a container, and treated as relative to task dir
		if !d.config.Volumes.Enabled && !isParentPath(task.AllocDir, hm.Source) {
			return nil, fmt.Errorf(
				"volumes are not enabled; cannot mount host path: %q %q",
				hm.Source, task.AllocDir)
		}
	case "tmpfs":
		// no source, so no sandbox check required
	default: // "volume", but also any new thing that comes along
		if !d.config.Volumes.Enabled {
			return nil, fmt.Errorf(
				"volumes are not enabled; cannot mount volume: %q", hm.Source)
		}
	}

	return &hm, nil
}

// detectIP of Docker container. Returns the first IP found as well as true if
// the IP should be advertised (bridge network IPs return false). Returns an
// empty string and false if no IP could be found.
func (d *Driver) detectIP(c *docker.Container, driverConfig *TaskConfig) (string, bool) {
	if c.NetworkSettings == nil {
		// This should only happen if there's been a coding error (such
		// as not calling InspectContainer after CreateContainer). Code
		// defensively in case the Docker API changes subtly.
		d.logger.Error("no network settings for container", "container_id", c.ID)
		return "", false
	}

	ip, ipName := "", ""
	auto := false
	for name, net := range c.NetworkSettings.Networks {
		if net.IPAddress == "" {
			// Ignore networks without an IP address
			continue
		}

		ip = net.IPAddress
		if driverConfig.AdvertiseIPv6Addr {
			ip = net.GlobalIPv6Address
			auto = true
		}
		ipName = name

		// Don't auto-advertise IPs for default networks (bridge on
		// Linux, nat on Windows)
		if name != "bridge" && name != "nat" {
			auto = true
		}

		break
	}

	if n := len(c.NetworkSettings.Networks); n > 1 {
		d.logger.Warn("multiple Docker networks for container found but Nomad only supports 1",
			"total_networks", n,
			"container_id", c.ID,
			"container_network", ipName)
	}

	return ip, auto
}

// containerByName finds a running container by name, and returns an error
// if the container is dead or can't be found.
func (d *Driver) containerByName(name string) (*docker.Container, error) {

	client, _, err := d.dockerClients()
	if err != nil {
		return nil, err
	}
	containers, err := client.ListContainers(docker.ListContainersOptions{
		All: true,
	})
	if err != nil {
		d.logger.Error("failed to query list of containers matching name",
			"container_name", name)
		return nil, recoverableErrTimeouts(
			fmt.Errorf("Failed to query list of containers: %s", err))
	}

	// container names with a / pre-pended to the Nomad generated container names
	containerName := "/" + name
	var (
		shimContainer docker.APIContainers
		found         bool
	)
OUTER:
	for _, shimContainer = range containers {
		d.logger.Trace("listed container", "names", hclog.Fmt("%+v", shimContainer.Names))
		for _, name := range shimContainer.Names {
			if name == containerName {
				d.logger.Trace("Found container",
					"container_name", containerName, "container_id", shimContainer.ID)
				found = true
				break OUTER
			}
		}
	}
	if !found {
		return nil, nil
	}

	container, err := client.InspectContainerWithOptions(docker.InspectContainerOptions{
		ID: shimContainer.ID,
	})
	if err != nil {
		err = fmt.Errorf("Failed to inspect container %s: %s", shimContainer.ID, err)

		// This error is always recoverable as it could
		// be caused by races between listing
		// containers and this container being removed.
		// See #2802
		return nil, nstructs.NewRecoverableError(err, true)
	}
	return container, nil
}

// validateCommand validates that the command only has a single value and
// returns a user friendly error message telling them to use the passed
// argField.
func validateCommand(command, argField string) error {
	trimmed := strings.TrimSpace(command)
	if len(trimmed) == 0 {
		return fmt.Errorf("command empty: %q", command)
	}

	if len(trimmed) != len(command) {
		return fmt.Errorf("command contains extra white space: %q", command)
	}

	return nil
}

func (d *Driver) WaitTask(ctx context.Context, taskID string) (<-chan *drivers.ExitResult, error) {
	h, ok := d.tasks.Get(taskID)
	if !ok {
		return nil, drivers.ErrTaskNotFound
	}
	ch := make(chan *drivers.ExitResult)
	go d.handleWait(ctx, ch, h)
	return ch, nil
}

func (d *Driver) handleWait(ctx context.Context, ch chan *drivers.ExitResult, h *taskHandle) {
	defer close(ch)
	select {
	case <-h.waitCh:
		ch <- h.ExitResult()
	case <-ctx.Done():
		ch <- &drivers.ExitResult{
			Err: ctx.Err(),
		}
	}
}

// parseSignal interprets the signal name into an os.Signal. If no name is
// provided, the docker driver defaults to SIGTERM. If the OS is Windows and
// SIGINT is provided, the signal is converted to SIGTERM.
func (d *Driver) parseSignal(os, signal string) (os.Signal, error) {
	// Unlike other drivers, docker defaults to SIGTERM, aiming for consistency
	// with the 'docker stop' command.
	// https://docs.docker.com/engine/reference/commandline/stop/#extended-description
	if signal == "" {
		signal = "SIGTERM"
	}

	// Windows Docker daemon does not support SIGINT, SIGTERM is the semantic equivalent that
	// allows for graceful shutdown before being followed up by a SIGKILL.
	// Supported signals:
	//   https://github.com/moby/moby/blob/0111ee70874a4947d93f64b672f66a2a35071ee2/pkg/signal/signal_windows.go#L17-L26
	if os == "windows" && signal == "SIGINT" {
		signal = "SIGTERM"
	}

	return signals.Parse(signal)
}

func (d *Driver) StopTask(taskID string, timeout time.Duration, signal string) error {
	h, ok := d.tasks.Get(taskID)
	if !ok {
		return drivers.ErrTaskNotFound
	}

	sig, err := d.parseSignal(runtime.GOOS, signal)
	if err != nil {
		return fmt.Errorf("failed to parse signal: %v", err)
	}

	return h.Kill(timeout, sig)
}

func (d *Driver) DestroyTask(taskID string, force bool) error {
	h, ok := d.tasks.Get(taskID)
	if !ok {
		return drivers.ErrTaskNotFound
	}

	c, err := client.InspectContainerWithOptions(docker.InspectContainerOptions{
		ID: h.containerID,
	})
	if err != nil {
		switch err.(type) {
		case *docker.NoSuchContainer:
			h.logger.Info("container was removed out of band, will proceed with DestroyTask",
				"error", err)
		default:
			return fmt.Errorf("failed to inspect container state: %v", err)
		}
	} else {
		if c.State.Running {
			if !force {
				return fmt.Errorf("must call StopTask for the given task before Destroy or set force to true")
			}
			if err := h.client.StopContainer(h.containerID, 0); err != nil {
				h.logger.Warn("failed to stop container during destroy", "error", err)
			}
		}

		if h.removeContainerOnExit {
			if err := h.client.RemoveContainer(docker.RemoveContainerOptions{ID: h.containerID, RemoveVolumes: true, Force: true}); err != nil {
				h.logger.Error("error removing container", "error", err)
			}
		} else {
			h.logger.Debug("not removing container due to config")
		}
	}

	if err := d.cleanupImage(h); err != nil {
		h.logger.Error("failed to cleanup image after destroying container",
			"error", err)
	}

	d.tasks.Delete(taskID)
	return nil
}

// cleanupImage removes a Docker image. No error is returned if the image
// doesn't exist or is still in use. Requires the global client to already be
// initialized.
func (d *Driver) cleanupImage(handle *taskHandle) error {
	if !d.config.GC.Image {
		return nil
	}

	d.coordinator.RemoveImage(handle.containerImage, handle.task.ID)

	return nil
}

func (d *Driver) InspectTask(taskID string) (*drivers.TaskStatus, error) {
	h, ok := d.tasks.Get(taskID)
	if !ok {
		return nil, drivers.ErrTaskNotFound
	}

	container, err := client.InspectContainerWithOptions(docker.InspectContainerOptions{
		ID: h.containerID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container %q: %v", h.containerID, err)
	}
	status := &drivers.TaskStatus{
		ID:          h.task.ID,
		Name:        h.task.Name,
		StartedAt:   container.State.StartedAt,
		CompletedAt: container.State.FinishedAt,
		DriverAttributes: map[string]string{
			"container_id": container.ID,
		},
		NetworkOverride: h.net,
		ExitResult:      h.ExitResult(),
	}

	status.State = drivers.TaskStateUnknown
	if container.State.Running {
		status.State = drivers.TaskStateRunning
	}
	if container.State.Dead {
		status.State = drivers.TaskStateExited
	}

	return status, nil
}

func (d *Driver) TaskStats(ctx context.Context, taskID string, interval time.Duration) (<-chan *drivers.TaskResourceUsage, error) {
	h, ok := d.tasks.Get(taskID)
	if !ok {
		return nil, drivers.ErrTaskNotFound
	}

	return h.Stats(ctx, interval)
}

func (d *Driver) TaskEvents(ctx context.Context) (<-chan *drivers.TaskEvent, error) {
	return d.eventer.TaskEvents(ctx)
}

func (d *Driver) SignalTask(taskID string, signal string) error {
	h, ok := d.tasks.Get(taskID)
	if !ok {
		return drivers.ErrTaskNotFound
	}

	sig, err := signals.Parse(signal)
	if err != nil {
		return fmt.Errorf("failed to parse signal: %v", err)
	}

	// TODO: review whether we can timeout in this and other Docker API
	// calls without breaking the expected client behavior.
	// see https://github.com/hashicorp/nomad/issues/9503
	return h.Signal(context.Background(), sig)
}

func (d *Driver) ExecTask(taskID string, cmd []string, timeout time.Duration) (*drivers.ExecTaskResult, error) {
	h, ok := d.tasks.Get(taskID)
	if !ok {
		return nil, drivers.ErrTaskNotFound
	}

	if len(cmd) == 0 {
		return nil, fmt.Errorf("cmd is required, but was empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return h.Exec(ctx, cmd[0], cmd[1:])
}

var _ drivers.ExecTaskStreamingDriver = (*Driver)(nil)

func (d *Driver) ExecTaskStreaming(ctx context.Context, taskID string, opts *drivers.ExecOptions) (*drivers.ExitResult, error) {
	defer opts.Stdout.Close()
	defer opts.Stderr.Close()

	done := make(chan interface{})
	defer close(done)

	h, ok := d.tasks.Get(taskID)
	if !ok {
		return nil, drivers.ErrTaskNotFound
	}

	if len(opts.Command) == 0 {
		return nil, fmt.Errorf("command is required but was empty")
	}

	createExecOpts := docker.CreateExecOptions{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          opts.Tty,
		Cmd:          opts.Command,
		Container:    h.containerID,
		Context:      ctx,
	}
	exec, err := h.client.CreateExec(createExecOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create exec object: %v", err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case s, ok := <-opts.ResizeCh:
				if !ok {
					return
				}
				client.ResizeExecTTY(exec.ID, s.Height, s.Width)
			}
		}
	}()

	startOpts := docker.StartExecOptions{
		Detach: false,

		// When running in TTY, we must use a raw terminal.
		// If not, we set RawTerminal to false to allow docker client
		// to interpret special stdout/stderr messages
		Tty:         opts.Tty,
		RawTerminal: opts.Tty,

		InputStream:  opts.Stdin,
		OutputStream: opts.Stdout,
		ErrorStream:  opts.Stderr,
		Context:      ctx,
	}
	if err := client.StartExec(exec.ID, startOpts); err != nil {
		return nil, fmt.Errorf("failed to start exec: %v", err)
	}

	// StartExec returns after process completes, but InspectExec seems to have a delay
	// get in getting status code

	const execTerminatingTimeout = 3 * time.Second
	start := time.Now()
	var res *docker.ExecInspect
	for (res == nil || res.Running) && time.Since(start) <= execTerminatingTimeout {
		res, err = client.InspectExec(exec.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to inspect exec result: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	if res == nil || res.Running {
		return nil, fmt.Errorf("failed to retrieve exec result")
	}

	return &drivers.ExitResult{
		ExitCode: res.ExitCode,
	}, nil
}

// dockerClients creates two *docker.Client, one for long running operations and
// the other for shorter operations. In test / dev mode we can use ENV vars to
// connect to the docker daemon. In production mode we will read docker.endpoint
// from the config file.
func (d *Driver) dockerClients() (*docker.Client, *docker.Client, error) {
	createClientsLock.Lock()
	defer createClientsLock.Unlock()

	if client != nil && waitClient != nil {
		return client, waitClient, nil
	}

	var err error

	// Only initialize the client if it hasn't yet been done
	if client == nil {
		client, err = d.newDockerClient(dockerTimeout)
		if err != nil {
			return nil, nil, err
		}
	}

	// Only initialize the waitClient if it hasn't yet been done
	if waitClient == nil {
		waitClient, err = d.newDockerClient(0 * time.Minute)
		if err != nil {
			return nil, nil, err
		}
	}

	return client, waitClient, nil
}

// newDockerClient creates a new *docker.Client with a configurable timeout
func (d *Driver) newDockerClient(timeout time.Duration) (*docker.Client, error) {
	var err error
	var merr multierror.Error
	var newClient *docker.Client

	// Default to using whatever is configured in docker.endpoint. If this is
	// not specified we'll fall back on NewClientFromEnv which reads config from
	// the DOCKER_* environment variables DOCKER_HOST, DOCKER_TLS_VERIFY, and
	// DOCKER_CERT_PATH. This allows us to lock down the config in production
	// but also accept the standard ENV configs for dev and test.
	dockerEndpoint := d.config.Endpoint
	if dockerEndpoint != "" {
		cert := d.config.TLS.Cert
		key := d.config.TLS.Key
		ca := d.config.TLS.CA

		if cert+key+ca != "" {
			d.logger.Debug("using TLS client connection", "endpoint", dockerEndpoint)
			newClient, err = docker.NewTLSClient(dockerEndpoint, cert, key, ca)
			if err != nil {
				merr.Errors = append(merr.Errors, err)
			}
		} else {
			d.logger.Debug("using standard client connection", "endpoint", dockerEndpoint)
			newClient, err = docker.NewClient(dockerEndpoint)
			if err != nil {
				merr.Errors = append(merr.Errors, err)
			}
		}
	} else {
		d.logger.Debug("using client connection initialized from environment")
		newClient, err = docker.NewClientFromEnv()
		if err != nil {
			merr.Errors = append(merr.Errors, err)
		}
	}

	if timeout != 0 && newClient != nil {
		newClient.SetTimeout(timeout)
	}
	return newClient, merr.ErrorOrNil()
}

func sliceMergeUlimit(ulimitsRaw map[string]string) ([]docker.ULimit, error) {
	var ulimits []docker.ULimit

	for name, ulimitRaw := range ulimitsRaw {
		if len(ulimitRaw) == 0 {
			return []docker.ULimit{}, fmt.Errorf("Malformed ulimit specification %v: %q, cannot be empty", name, ulimitRaw)
		}
		// hard limit is optional
		if !strings.Contains(ulimitRaw, ":") {
			ulimitRaw = ulimitRaw + ":" + ulimitRaw
		}

		splitted := strings.SplitN(ulimitRaw, ":", 2)
		if len(splitted) < 2 {
			return []docker.ULimit{}, fmt.Errorf("Malformed ulimit specification %v: %v", name, ulimitRaw)
		}
		soft, err := strconv.Atoi(splitted[0])
		if err != nil {
			return []docker.ULimit{}, fmt.Errorf("Malformed soft ulimit %v: %v", name, ulimitRaw)
		}
		hard, err := strconv.Atoi(splitted[1])
		if err != nil {
			return []docker.ULimit{}, fmt.Errorf("Malformed hard ulimit %v: %v", name, ulimitRaw)
		}

		ulimit := docker.ULimit{
			Name: name,
			Soft: int64(soft),
			Hard: int64(hard),
		}
		ulimits = append(ulimits, ulimit)
	}
	return ulimits, nil
}

func isDockerTransientError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := err.Error()
	for _, te := range dockerTransientErrs {
		if strings.Contains(errMsg, te) {
			return true
		}
	}

	return false
}
