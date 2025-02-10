// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	containerapi "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	networkapi "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/hashicorp/consul-template/signals"
	hclog "github.com/hashicorp/go-hclog"
	multierror "github.com/hashicorp/go-multierror"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/go-set/v3"
	"github.com/hashicorp/nomad/client/lib/cgroupslib"
	"github.com/hashicorp/nomad/client/lib/cpustats"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/drivers/docker/docklog"
	"github.com/hashicorp/nomad/drivers/shared/capabilities"
	"github.com/hashicorp/nomad/drivers/shared/eventer"
	"github.com/hashicorp/nomad/drivers/shared/hostnames"
	"github.com/hashicorp/nomad/drivers/shared/resolvconf"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/pointer"
	nstructs "github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	pstructs "github.com/hashicorp/nomad/plugins/shared/structs"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/ryanuber/go-glob"
	"golang.org/x/mod/semver"
)

var (
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

	// We support "process" and "hyper-v" isolation modes on windows
	windowsIsolationModes = []string{windowsIsolationModeProcess, windowsIsolationModeHyperV}
)

const (
	dockerLabelAllocID          = "com.hashicorp.nomad.alloc_id"
	dockerLabelJobName          = "com.hashicorp.nomad.job_name"
	dockerLabelJobID            = "com.hashicorp.nomad.job_id"
	dockerLabelTaskGroupName    = "com.hashicorp.nomad.task_group_name"
	dockerLabelTaskName         = "com.hashicorp.nomad.task_name"
	dockerLabelNamespace        = "com.hashicorp.nomad.namespace"
	dockerLabelNodeName         = "com.hashicorp.nomad.node_name"
	dockerLabelNodeID           = "com.hashicorp.nomad.node_id"
	dockerLabelParentJobID      = "com.hashicorp.nomad.parent_job_id"
	windowsIsolationModeProcess = "process"
	windowsIsolationModeHyperV  = "hyperv"
)

type pauseContainerStore struct {
	lock         sync.Mutex
	containerIDs *set.Set[string]
}

func newPauseContainerStore() *pauseContainerStore {
	return &pauseContainerStore{
		containerIDs: set.New[string](10),
	}
}

func (s *pauseContainerStore) add(id string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.containerIDs.Insert(id)
}

func (s *pauseContainerStore) remove(id string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.containerIDs.Remove(id)
}

func (s *pauseContainerStore) union(other *set.Set[string]) set.Collection[string] {
	s.lock.Lock()
	defer s.lock.Unlock()
	return other.Union(s.containerIDs)
}

type createContainerOptions struct {
	Name       string
	Config     *containerapi.Config
	Host       *containerapi.HostConfig
	Networking *networkapi.NetworkingConfig
}

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

	// pauseContainers keeps track of pause container IDs in use by allocations
	pauseContainers *pauseContainerStore

	// coordinator is what tracks multiple image pulls against the same docker image
	coordinator *dockerCoordinator

	// logger will log to the Nomad agent
	logger hclog.Logger

	// gpuRuntime indicates nvidia-docker runtime availability
	gpuRuntime bool

	// compute contains information about the available cpu compute
	compute cpustats.Compute

	// A tri-state boolean to know if the fingerprinting has happened and
	// whether it has been successful
	fingerprintSuccess *bool
	fingerprintLock    sync.RWMutex

	// A boolean to know if the docker driver has ever been correctly detected
	// for use during fingerprinting.
	detected     bool
	detectedLock sync.RWMutex

	dockerClientLock sync.Mutex
	dockerClient     *client.Client // for most docker api calls (use getDockerClient())
	infinityClient   *client.Client // for wait and stop calls (use getInfinityClient())

	danglingReconciler *containerReconciler
}

// NewDockerDriver returns a docker implementation of a driver plugin
func NewDockerDriver(ctx context.Context, logger hclog.Logger) drivers.DriverPlugin {
	logger = logger.Named(pluginName)
	driver := &Driver{
		eventer:         eventer.NewEventer(ctx, logger),
		tasks:           newTaskStore(),
		config:          new(DriverConfig),
		pauseContainers: newPauseContainerStore(),
		ctx:             ctx,
		logger:          logger,
	}
	return driver
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

func (d *Driver) setupNewDockerLogger(container types.ContainerJSON, cfg *drivers.TaskConfig, startTime time.Time) (docklog.DockerLogger, *plugin.Client, error) {
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

	var handleState taskHandleState
	if err := handle.GetDriverState(&handleState); err != nil {
		return fmt.Errorf("failed to decode driver task state: %v", err)
	}

	dockerClient, err := d.getDockerClient()
	if err != nil {
		return fmt.Errorf("failed to get docker client: %w", err)
	}

	dockerInfo, err := dockerClient.Info(d.ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch docker daemon info: %v", err)
	}

	infinityClient, err := d.getInfinityClient()
	if err != nil {
		return fmt.Errorf("failed to get docker long operations client: %w", err)
	}

	container, err := dockerClient.ContainerInspect(d.ctx, handleState.ContainerID)
	if err != nil {
		return fmt.Errorf("failed to inspect container for id %q: %v", handleState.ContainerID, err)
	}

	h := &taskHandle{
		dockerClient:            dockerClient,
		dockerCGroupDriver:      dockerInfo.CgroupDriver,
		infinityClient:          infinityClient,
		logger:                  d.logger.With("container_id", container.ID),
		task:                    handle.Config,
		containerID:             container.ID,
		containerCgroup:         string(container.HostConfig.Cgroup),
		containerImage:          container.Image,
		doneCh:                  make(chan bool),
		waitCh:                  make(chan struct{}),
		removeContainerOnExit:   d.config.GC.Container,
		net:                     handleState.DriverNetwork,
		disableCpusetManagement: d.config.disableCpusetManagement,
	}

	if loggingIsEnabled(d.config, handle.Config) {
		h.dlogger, h.dloggerPluginClient, err = d.reattachToDockerLogger(handleState.ReattachConfig)
		if err != nil {
			d.logger.Warn("failed to reattach to docker logger process", "error", err)

			h.dlogger, h.dloggerPluginClient, err = d.setupNewDockerLogger(container, handle.Config, time.Now())
			if err != nil {
				if err := dockerClient.ContainerStop(d.ctx, handleState.ContainerID, stopWithZeroTimeout()); err != nil {
					d.logger.Warn("failed to stop container during cleanup", "container_id", handleState.ContainerID, "error", err)
				}
				return fmt.Errorf("failed to setup replacement docker logger: %v", err)
			}

			if err := handle.SetDriverState(h.buildState()); err != nil {
				if err := dockerClient.ContainerStop(d.ctx, handleState.ContainerID, stopWithZeroTimeout()); err != nil {
					d.logger.Warn("failed to stop container during cleanup", "container_id", handleState.ContainerID, "error", err)
				}
				return fmt.Errorf("failed to store driver state: %v", err)
			}
		}
	}

	d.tasks.Set(handle.Config.ID, h)

	// find a pause container?

	go h.run()

	return nil
}

func loggingIsEnabled(driverCfg *DriverConfig, taskCfg *drivers.TaskConfig) bool {
	if driverCfg.DisableLogCollection {
		return false
	}
	if taskCfg.StderrPath == os.DevNull && taskCfg.StdoutPath == os.DevNull {
		return false
	}
	return true
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

	// we'll need the normal docker client
	dockerClient, err := d.getDockerClient()
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to create docker client: %v", err)
	}

	dockerInfo, err := dockerClient.Info(d.ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch docker daemon info: %v", err)
	}

	// and also the long operations client
	infinityClient, err := d.getInfinityClient()
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to create long operations docker client: %v", err)
	}

	id, user, err := d.createImage(cfg, &driverConfig, dockerClient)
	if err != nil {
		return nil, nil, err
	}

	// validate the image user (windows only)
	if err := validateImageUser(user, cfg.User, &driverConfig, d.config); err != nil {
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
	container, err := d.createContainer(dockerClient, containerCfg, driverConfig.Image)
	if err != nil {
		d.logger.Error("failed to create container", "error", err)
		if container != nil {
			removeErr := dockerClient.ContainerRemove(d.ctx, container.ID, containerapi.RemoveOptions{Force: true})
			if removeErr != nil {
				return nil, nil, fmt.Errorf("failed to remove container %s: %v", container.ID, removeErr)
			}
		}
		return nil, nil, nstructs.WrapRecoverable(fmt.Sprintf("failed to create container: %v", err), err)
	}

	d.logger.Info("created container", "container_id", container.ID)

	if !container.State.Running {
		// Start the container
		if err := d.startContainer(*container); err != nil {
			d.logger.Error("failed to start container", "container_id", container.ID, "error", err)
			dockerClient.ContainerRemove(d.ctx, container.ID, containerapi.RemoveOptions{Force: true})
			// Some sort of docker race bug, recreating the container usually works
			if errdefs.IsConflict(err) && startAttempts < 5 {
				startAttempts++
				d.logger.Debug("reattempting container create/start sequence", "attempt", startAttempts, "container_id", id)
				goto CREATE
			}
			return nil, nil, nstructs.WrapRecoverable(fmt.Sprintf("Failed to start container %s: %s", container.ID, err), err)
		}

		// Inspect container to get all of the container metadata as much of the
		// metadata (eg networking) isn't populated until the container is started
		runningContainer, err := dockerClient.ContainerInspect(d.ctx, container.ID)
		if err != nil {
			dockerClient.ContainerRemove(d.ctx, container.ID, containerapi.RemoveOptions{Force: true})
			msg := "failed to inspect started container"
			d.logger.Error(msg, "error", err)
			dockerClient.ContainerRemove(d.ctx, container.ID, containerapi.RemoveOptions{Force: true})
			return nil, nil, nstructs.NewRecoverableError(fmt.Errorf("%s %s: %s", msg, container.ID, err), true)
		}
		container = &runningContainer
		d.logger.Info("started container", "container_id", container.ID)
	} else {
		d.logger.Debug("re-attaching to container", "container_id",
			container.ID, "container_state", container.State.Status)
	}

	collectingLogs := loggingIsEnabled(d.config, cfg)

	var dlogger docklog.DockerLogger
	var pluginClient *plugin.Client

	if collectingLogs {
		dlogger, pluginClient, err = d.setupNewDockerLogger(*container, cfg, time.Unix(0, 0))
		if err != nil {
			d.logger.Error("an error occurred after container startup, terminating container", "container_id", container.ID)
			dockerClient.ContainerRemove(d.ctx, container.ID, containerapi.RemoveOptions{Force: true})
			return nil, nil, err
		}
	}

	// Detect container address
	ip, autoUse := d.detectIP(*container, &driverConfig)

	net := &drivers.DriverNetwork{
		PortMap:       driverConfig.PortMap,
		IP:            ip,
		AutoAdvertise: autoUse,
	}

	// Return a driver handle
	h := &taskHandle{
		dockerClient:            dockerClient,
		dockerCGroupDriver:      dockerInfo.CgroupDriver,
		infinityClient:          infinityClient,
		dlogger:                 dlogger,
		dloggerPluginClient:     pluginClient,
		logger:                  d.logger.With("container_id", container.ID),
		task:                    cfg,
		containerID:             container.ID,
		containerImage:          container.Image,
		doneCh:                  make(chan bool),
		waitCh:                  make(chan struct{}),
		removeContainerOnExit:   d.config.GC.Container,
		net:                     net,
		disableCpusetManagement: d.config.disableCpusetManagement,
	}

	if err := handle.SetDriverState(h.buildState()); err != nil {
		d.logger.Error("error encoding container occurred after startup, terminating container", "container_id", container.ID, "error", err)
		if collectingLogs {
			dlogger.Stop()
			pluginClient.Kill()
		}
		dockerClient.ContainerRemove(d.ctx, container.ID, containerapi.RemoveOptions{Force: true})
		return nil, nil, err
	}

	d.tasks.Set(cfg.ID, h)
	go h.run()

	return handle, net, nil
}

// createContainerClient is the subset of Docker Client methods used by the
// createContainer method to ease testing subtle error conditions.
type createContainerClient interface {
	ContainerCreate(context.Context, *containerapi.Config, *containerapi.HostConfig, *networkapi.NetworkingConfig, *ocispec.Platform, string) (containerapi.CreateResponse, error)
	ContainerInspect(context.Context, string) (types.ContainerJSON, error)
	ContainerList(context.Context, containerapi.ListOptions) ([]types.Container, error)
	ContainerRemove(context.Context, string, containerapi.RemoveOptions) error
}

// createContainer creates the container given the passed configuration. It
// attempts to handle any transient Docker errors.
func (d *Driver) createContainer(client createContainerClient, config createContainerOptions, image string) (*types.ContainerJSON, error) {
	// Create a container
	var attempted uint64
	var backoff time.Duration

CREATE:
	_, createErr := client.ContainerCreate(d.ctx, config.Config, config.Host, config.Networking, nil, config.Name)
	if createErr == nil {
		containerJSON, err := d.containerByName(config.Name)
		if err != nil {
			return nil, err
		}
		return containerJSON, nil
	}

	d.logger.Debug("failed to create container", "container_name",
		config.Name, "image_name", image, "image_id", config.Config.Image,
		"attempt", attempted+1, "error", createErr)

	// Volume management tools like Portworx may not have detached a volume
	// from a previous node before Nomad started a task replacement task.
	// Treat these errors as recoverable so we retry.
	if strings.Contains(strings.ToLower(createErr.Error()), "duplicate mount point") {
		return nil, nstructs.NewRecoverableError(createErr, true)
	}

	// If the container already exists determine whether it's already
	// running or if it's dead and needs to be recreated.
	if errdefs.IsConflict(createErr) {

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
			err = client.ContainerRemove(d.ctx, container.ID, containerapi.RemoveOptions{Force: true})
			if err != nil {
				d.logger.Error("failed to purge container", "container_id", container.ID)
				return nil, recoverableErrTimeouts(fmt.Errorf("Failed to purge container %s: %s", container.ID, err))
			} else {
				d.logger.Info("purged container", "container_id", container.ID)
			}
		}

		if attempted < d.config.ContainerExistsAttempts {
			attempted++
			backoff = helper.Backoff(50*time.Millisecond, time.Minute, attempted)
			time.Sleep(backoff)
			goto CREATE
		}

	} else if errdefs.IsNotFound(createErr) {
		// There is still a very small chance this is possible even with the
		// coordinator so retry.
		return nil, nstructs.NewRecoverableError(createErr, true)
	} else if isDockerTransientError(createErr) && attempted < 5 {
		attempted++
		backoff = helper.Backoff(50*time.Millisecond, time.Minute, attempted)
		time.Sleep(backoff)
		goto CREATE
	}

	return nil, recoverableErrTimeouts(createErr)
}

// startContainer starts the passed container. It attempts to handle any
// transient Docker errors.
func (d *Driver) startContainer(c types.ContainerJSON) error {
	dockerClient, err := d.getDockerClient()
	if err != nil {
		return err
	}

	var attempted uint64
	var backoff time.Duration

START:
	startErr := dockerClient.ContainerStart(d.ctx, c.ID, containerapi.StartOptions{})
	if startErr == nil || errdefs.IsConflict(err) {
		return nil
	}

	d.logger.Debug("failed to start container", "container_id", c.ID, "attempt", attempted+1, "error", startErr)

	if isDockerTransientError(startErr) {
		if attempted < 5 {
			attempted++
			backoff = helper.Backoff(50*time.Millisecond, time.Minute, attempted)
			time.Sleep(backoff)
			goto START
		}
		return nstructs.NewRecoverableError(startErr, true)
	}

	return recoverableErrTimeouts(startErr)
}

// createImage creates a docker image either by pulling it from a registry or by
// loading it from the file system
func (d *Driver) createImage(task *drivers.TaskConfig, driverConfig *TaskConfig, client *client.Client) (string, string, error) {
	image := driverConfig.Image
	repo, tag := parseDockerImage(image)

	// We're going to check whether the image is already downloaded. If the tag
	// is "latest", or ForcePull is set, we have to check for a new version every time so we don't
	// bother to check and cache the id here. We'll download first, then cache.
	if driverConfig.ForcePull {
		d.logger.Debug("force pulling image instead of inspecting local", "image_ref", dockerImageRef(repo, tag))
	} else if tag != "latest" {
		if dockerImage, _, _ := client.ImageInspectWithRaw(d.ctx, image); dockerImage.ID != "" {
			// Image exists so just increment its reference count
			d.coordinator.IncrementImageReference(dockerImage.ID, image, task.ID)
			var user string
			if dockerImage.Config != nil {
				user = dockerImage.Config.User
			}
			return dockerImage.ID, user, nil
		}
	}

	// Load the image if specified
	if driverConfig.LoadImage != "" {
		return d.loadImage(task, driverConfig, client)
	}

	// Download the image
	return d.pullImage(task, driverConfig, repo, tag)
}

// pullImage creates an image by pulling it from a docker registry
func (d *Driver) pullImage(task *drivers.TaskConfig, driverConfig *TaskConfig, repo, tag string) (id, user string, err error) {
	authOptions, err := d.resolveRegistryAuthentication(driverConfig, repo)
	if err != nil {
		if driverConfig.AuthSoftFail {
			d.logger.Warn("Failed to find docker repo auth", "repo", repo, "error", err)
		} else {
			return "", "", fmt.Errorf("Failed to find docker auth for repo %q: %v", repo, err)
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
		return "", "", fmt.Errorf("Failed to parse image_pull_timeout: %v", err)
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
type authBackend func(string) (*registry.AuthConfig, error)

// resolveRegistryAuthentication attempts to retrieve auth credentials for the
// repo, trying all authentication-backends possible.
func (d *Driver) resolveRegistryAuthentication(driverConfig *TaskConfig, repo string) (*registry.AuthConfig, error) {
	return firstValidAuth(repo, []authBackend{
		authFromTaskConfig(driverConfig),
		authFromDockerConfig(d.config.Auth.Config),
		authFromHelper(d.config.Auth.Helper),
	})
}

// loadImage creates an image by loading it from the file system
func (d *Driver) loadImage(task *drivers.TaskConfig, driverConfig *TaskConfig, client *client.Client) (id string, user string, err error) {

	archive := filepath.Join(task.TaskDir().LocalDir, driverConfig.LoadImage)
	d.logger.Debug("loading image from disk", "archive", archive)

	f, err := os.Open(archive)
	if err != nil {
		return "", "", fmt.Errorf("unable to open image archive: %v", err)
	}

	if _, err := client.ImageLoad(d.ctx, f, true); err != nil {
		return "", "", err
	}
	f.Close()

	dockerImage, _, err := client.ImageInspectWithRaw(d.ctx, driverConfig.Image)
	if err != nil {
		return "", "", recoverableErrTimeouts(err)
	}

	d.coordinator.IncrementImageReference(dockerImage.ID, driverConfig.Image, task.ID)
	var imageUser string
	if dockerImage.Config != nil {
		imageUser = dockerImage.Config.User
	}
	return dockerImage.ID, imageUser, nil
}

func (d *Driver) convertAllocPathsForWindowsLCOW(task *drivers.TaskConfig, image string) error {
	dockerClient, err := d.getDockerClient()
	if err != nil {
		return err
	}

	imageConfig, _, err := dockerClient.ImageInspectWithRaw(d.ctx, image)
	if err != nil {
		return fmt.Errorf("the image does not exist: %v", err)
	}
	// LCOW If we are running a Linux Container on Windows, we need to mount it correctly, as c:\ does not exist on unix
	if imageConfig.Os == "linux" {
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
	taskLocalBindVolume := driverConfig.VolumeDriver == ""
	if !d.config.Volumes.Enabled && !taskLocalBindVolume {
		return nil, fmt.Errorf("volumes are not enabled; cannot use volume driver %q", driverConfig.VolumeDriver)
	}

	allocDirBind := fmt.Sprintf("%s:%s", task.TaskDir().SharedAllocDir, task.Env[taskenv.AllocDir])
	taskLocalBind := fmt.Sprintf("%s:%s", task.TaskDir().LocalDir, task.Env[taskenv.TaskLocalDir])
	secretDirBind := fmt.Sprintf("%s:%s", task.TaskDir().SecretsDir, task.Env[taskenv.SecretsDir])
	binds := []string{allocDirBind, taskLocalBind, secretDirBind}

	selinuxLabel := d.config.Volumes.SelinuxLabel
	if selinuxLabel != "" {
		// Apply SELinux Label to each built-in bind
		for i := range binds {
			binds[i] = fmt.Sprintf("%s:%s", binds[i], selinuxLabel)
		}
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
		opts := mode
		if opts != "" {
			if selinuxLabel != "" {
				opts += "," + selinuxLabel
			}
		} else {
			opts = selinuxLabel
		}
		if opts != "" {
			bind += ":" + opts
		}
		binds = append(binds, bind)
	}

	return binds, nil
}

func (d *Driver) findPauseContainer(allocID string) (string, error) {

	dockerClient, err := d.getDockerClient()
	if err != nil {
		return "", err
	}

	containers, listErr := dockerClient.ContainerList(context.Background(), containerapi.ListOptions{
		All:     false, // running only
		Filters: filters.NewArgs(filters.KeyValuePair{Key: "label", Value: dockerLabelAllocID}),
	})
	if listErr != nil {
		d.logger.Error("failed to list pause containers for recovery", "error", listErr)
		return "", listErr
	}

	for _, c := range containers {
		if !slices.ContainsFunc(c.Names, func(s string) bool {
			return strings.HasPrefix(s, "/nomad_init_")
		}) {
			continue
		}
		if c.Labels[dockerLabelAllocID] == allocID {
			return c.ID, nil
		}
	}

	return "", nil
}

// recoverPauseContainers gets called when we start up the plugin. On client
// restarts we need to rebuild the set of pause containers we are
// tracking. Basically just scan all containers and pull the ID from anything
// that has the Nomad Label and has Name with prefix "/nomad_init_".
func (d *Driver) recoverPauseContainers(ctx context.Context) {
	dockerClient, err := d.getDockerClient()
	if err != nil {
		d.logger.Error("failed to recover pause containers", "error", err)
		return
	}

	containers, listErr := dockerClient.ContainerList(ctx, containerapi.ListOptions{
		All:     false, // running only
		Filters: filters.NewArgs(filters.KeyValuePair{Key: "label", Value: dockerLabelAllocID}),
	})
	if listErr != nil && !errors.Is(listErr, ctx.Err()) {
		d.logger.Error("failed to list pause containers for recovery", "error", listErr)
		return
	}

CONTAINER:
	for _, c := range containers {
		for _, name := range c.Names {
			if strings.HasPrefix(name, "/nomad_init_") {
				d.pauseContainers.add(c.ID)
				continue CONTAINER
			}
		}
	}
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
			f, err := os.ReadFile(con[1])
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
// the docker host config. These fields represent hard and soft/reserved memory
// limits from docker's perspective, respectively.
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
func memoryLimits(driverHardLimitMB int64, taskMemory drivers.MemoryResources) (memory, reserve int64) {
	softBytes := taskMemory.MemoryMB * 1024 * 1024

	hard := driverHardLimitMB
	if taskMemory.MemoryMaxMB > hard {
		hard = taskMemory.MemoryMaxMB
	}

	if hard <= 0 {
		return softBytes, 0
	}
	return hard * 1024 * 1024, softBytes
}

func (d *Driver) createContainerConfig(task *drivers.TaskConfig, driverConfig *TaskConfig,
	imageID string) (createContainerOptions, error) {

	// ensure that PortMap variables are populated early on
	task.Env = taskenv.SetPortMapEnvs(task.Env, driverConfig.PortMap)

	logger := d.logger.With("task_name", task.Name)
	var c createContainerOptions
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
	config := &containerapi.Config{
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

	// Validate isolation modes on windows
	if runtime.GOOS != "windows" {
		if driverConfig.Isolation != "" {
			return c, fmt.Errorf("Failed to create container configuration, cannot use isolation mode \"%s\" on %s", driverConfig.Isolation, runtime.GOOS)
		}
	} else {
		if driverConfig.Isolation == "" {
			driverConfig.Isolation = windowsIsolationModeHyperV
		}
		if !slices.Contains(windowsIsolationModes, driverConfig.Isolation) {
			return c, fmt.Errorf("Unsupported isolation mode \"%s\"", driverConfig.Isolation)
		}
	}

	memory, memoryReservation := memoryLimits(driverConfig.MemoryHardLimit, task.Resources.NomadResources.Memory)

	var pidsLimit int64

	// Pids limit defined in Nomad plugin config. Defaults to 0 (Unlimited).
	if d.config.PidsLimit > 0 {
		pidsLimit = d.config.PidsLimit
	}

	// Override Nomad plugin config pids limit, by user defined pids limit.
	if driverConfig.PidsLimit > 0 {
		if d.config.PidsLimit > 0 && driverConfig.PidsLimit > d.config.PidsLimit {
			return c, fmt.Errorf("pids_limit cannot be greater than nomad plugin config pids_limit: %d", d.config.PidsLimit)
		}
		pidsLimit = driverConfig.PidsLimit
	}

	hostConfig := &containerapi.HostConfig{
		// do not set cgroup parent anymore

		OomScoreAdj: driverConfig.OOMScoreAdj, // ignored on platforms other than linux

		// Binds are used to mount a host volume into the container. We mount a
		// local directory for storage and a shared alloc directory that can be
		// used to share data between different tasks in the same task group.
		Binds: binds,

		Isolation:    containerapi.Isolation(driverConfig.Isolation),
		StorageOpt:   driverConfig.StorageOpt,
		VolumeDriver: driverConfig.VolumeDriver,

		Runtime:  containerRuntime,
		GroupAdd: driverConfig.GroupAdd,
	}

	hostConfig.Resources = containerapi.Resources{
		Memory:            memory,            // hard limit
		MemoryReservation: memoryReservation, // soft limit
		CPUShares:         task.Resources.LinuxResources.CPUShares,
		CpusetCpus:        task.Resources.LinuxResources.CpusetCpus,
		PidsLimit:         &pidsLimit,
	}

	// Setting cpuset_cpus in driver config is no longer supported (it has
	// not worked correctly since Nomad 0.12)
	if driverConfig.CPUSetCPUs != "" {
		d.logger.Warn("cpuset_cpus is no longer supported")
	}

	// Enable tini (docker-init) init system.
	if driverConfig.Init {
		hostConfig.Init = &driverConfig.Init
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
		if cgroupslib.MaybeDisableMemorySwappiness() != nil {
			hostConfig.MemorySwappiness = pointer.Of(int64(*(cgroupslib.MaybeDisableMemorySwappiness())))
		} else {
			hostConfig.MemorySwappiness = nil
		}
	}

	loggingDriver := driverConfig.Logging.Type
	if loggingDriver == "" {
		loggingDriver = driverConfig.Logging.Driver
	}

	hostConfig.LogConfig = containerapi.LogConfig{
		Type:   loggingDriver,
		Config: driverConfig.Logging.Config,
	}

	if hostConfig.LogConfig.Type == "" && hostConfig.LogConfig.Config == nil {
		logger.Trace("no docker log driver provided, defaulting to plugin config")
		hostConfig.LogConfig.Type = d.config.Logging.Type
		hostConfig.LogConfig.Config = d.config.Logging.Config
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

	// get docker client info (we need to know the runtime to adjust
	// OS-specific capabilities)
	client, err := d.getDockerClient()
	if err != nil {
		return c, err
	}
	ver, err := client.ServerVersion(d.ctx)
	if err != nil {
		return c, err
	}

	// set add/drop capabilities
	if hostConfig.CapAdd, hostConfig.CapDrop, err = capabilities.Delta(
		capabilities.DockerDefaults(ver), d.config.AllowCaps, driverConfig.CapAdd, driverConfig.CapDrop,
	); err != nil {
		return c, err
	}

	// set SHM size
	if driverConfig.ShmSize != 0 {
		hostConfig.ShmSize = driverConfig.ShmSize
	}

	// Setup devices from Docker-specific config
	for _, device := range driverConfig.Devices {
		dd, err := device.toDockerDevice()
		if err != nil {
			return c, err
		}
		hostConfig.Devices = append(hostConfig.Devices, dd)
	}

	// Setup devices from Nomad device plugins
	for _, device := range task.Devices {
		hostConfig.Devices = append(hostConfig.Devices, containerapi.DeviceMapping{
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

	// Setup /etc/hosts
	// If the task's network_mode is unset our hostname and IP will come from
	// the Nomad-owned network (if in use), so we need to generate an
	// /etc/hosts file that matches the network rather than the default one
	// that comes from the pause container
	if task.NetworkIsolation != nil && driverConfig.NetworkMode == "" {
		etcHostMount, err := hostnames.GenerateEtcHostsMount(
			task.AllocDir, task.NetworkIsolation, driverConfig.ExtraHosts)
		if err != nil {
			return c, fmt.Errorf("failed to build mount for /etc/hosts: %v", err)
		}
		if etcHostMount != nil {
			// erase the extra_hosts field if we have a mount so we don't get
			// conflicting options error from dockerd
			driverConfig.ExtraHosts = nil
			hostConfig.Mounts = append(hostConfig.Mounts, mount.Mount{
				Target:   etcHostMount.TaskPath,
				Source:   etcHostMount.HostPath,
				Type:     "bind",
				ReadOnly: etcHostMount.Readonly,
				BindOptions: &mount.BindOptions{
					Propagation: mount.Propagation(etcHostMount.PropagationMode),
				},
			})
		}
	}

	// Setup DNS
	// If task DNS options are configured Nomad will manage the resolv.conf file
	// Docker driver dns options are not compatible with task dns options
	if task.DNS != nil {
		dnsMount, err := resolvconf.GenerateDNSMount(task.TaskDir().Dir, task.DNS)
		if err != nil {
			return c, fmt.Errorf("failed to build mount for resolv.conf: %v", err)
		}
		hostConfig.Mounts = append(hostConfig.Mounts, mount.Mount{
			Target:   dnsMount.TaskPath,
			Source:   dnsMount.HostPath,
			Type:     "bind",
			ReadOnly: dnsMount.Readonly,
			BindOptions: &mount.BindOptions{
				Propagation: mount.Propagation(dnsMount.PropagationMode),
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
		hm := mount.Mount{
			Type:     "bind",
			Target:   m.TaskPath,
			Source:   m.HostPath,
			ReadOnly: m.Readonly,
		}

		// MountPropagation is only supported by Docker on Linux:
		// https://docs.docker.com/storage/bind-mounts/#configure-bind-propagation
		if runtime.GOOS == "linux" {
			hm.BindOptions = &mount.BindOptions{
				Propagation: mount.Propagation(userMountToUnixMount[m.PropagationMode]),
			}
		}

		hostConfig.Mounts = append(hostConfig.Mounts, hm)
	}

	hostConfig.ExtraHosts = driverConfig.ExtraHosts

	hostConfig.IpcMode = containerapi.IpcMode(driverConfig.IPCMode)
	hostConfig.PidMode = containerapi.PidMode(driverConfig.PidMode)
	hostConfig.UTSMode = containerapi.UTSMode(driverConfig.UTSMode)
	hostConfig.UsernsMode = containerapi.UsernsMode(driverConfig.UsernsMode)
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
	hostConfig.NetworkMode = containerapi.NetworkMode(driverConfig.NetworkMode)

	// if the driver config does not specify a network mode then try to use the
	// shared alloc network
	if hostConfig.NetworkMode == "" {
		if task.NetworkIsolation != nil && task.NetworkIsolation.Path != "" {
			// find the previously created parent container to join networks with
			netMode := fmt.Sprintf("container:%s", task.NetworkIsolation.Labels[dockerNetSpecLabelKey])
			logger.Debug("configuring network mode for task group", "network_mode", netMode)
			hostConfig.NetworkMode = containerapi.NetworkMode(netMode)
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
		if hostConfig.NetworkMode.IsContainer() {
			break
		}

		for _, port := range driverConfig.Ports {
			if mapping, ok := task.Resources.Ports.Get(port); ok {
				ports.add(mapping.Label, mapping.HostIP, mapping.Value, mapping.To)
			} else {
				return c, fmt.Errorf("Port %q not found, check network block", port)
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
	// main mandatory label
	labels[dockerLabelAllocID] = task.AllocID

	//optional labels, as configured in plugin configuration
	for _, configurationExtraLabel := range d.config.ExtraLabels {
		if glob.Glob(configurationExtraLabel, "job_name") {
			labels[dockerLabelJobName] = task.JobName
		}
		if glob.Glob(configurationExtraLabel, "job_id") {
			labels[dockerLabelJobID] = task.JobID
		}
		if glob.Glob(configurationExtraLabel, "parent_job_id") && len(task.ParentJobID) > 0 {
			labels[dockerLabelParentJobID] = task.ParentJobID
		}
		if glob.Glob(configurationExtraLabel, "task_group_name") {
			labels[dockerLabelTaskGroupName] = task.TaskGroupName
		}
		if glob.Glob(configurationExtraLabel, "task_name") {
			labels[dockerLabelTaskName] = task.Name
		}
		if glob.Glob(configurationExtraLabel, "namespace") {
			labels[dockerLabelNamespace] = task.Namespace
		}
		if glob.Glob(configurationExtraLabel, "node_name") {
			labels[dockerLabelNodeName] = task.NodeName
		}
		if glob.Glob(configurationExtraLabel, "node_id") {
			labels[dockerLabelNodeID] = task.NodeID
		}
	}

	config.Labels = labels
	logger.Debug("applied labels on the container", "labels", config.Labels)

	config.Env = task.EnvList()

	containerName := fmt.Sprintf("%s-%s", strings.ReplaceAll(task.Name, "/", "_"), task.AllocID)
	logger.Debug("setting container name", "container_name", containerName)

	var networkingConfig *networkapi.NetworkingConfig
	if len(driverConfig.NetworkAliases) > 0 || driverConfig.IPv4Address != "" || driverConfig.IPv6Address != "" {
		networkingConfig = &networkapi.NetworkingConfig{
			EndpointsConfig: map[string]*networkapi.EndpointSettings{
				string(hostConfig.NetworkMode): {},
			},
		}
	}

	if len(driverConfig.NetworkAliases) > 0 {
		networkingConfig.EndpointsConfig[string(hostConfig.NetworkMode)].Aliases = driverConfig.NetworkAliases
		logger.Debug("setting container network aliases", "network_mode", hostConfig.NetworkMode,
			"network_aliases", strings.Join(driverConfig.NetworkAliases, ", "))
	}

	if driverConfig.IPv4Address != "" || driverConfig.IPv6Address != "" {
		networkingConfig.EndpointsConfig[string(hostConfig.NetworkMode)].IPAMConfig = &networkapi.EndpointIPAMConfig{
			IPv4Address: driverConfig.IPv4Address,
			IPv6Address: driverConfig.IPv6Address,
		}
		logger.Debug("setting container network configuration", "network_mode", hostConfig.NetworkMode,
			"ipv4_address", driverConfig.IPv4Address, "ipv6_address", driverConfig.IPv6Address)
	}

	if driverConfig.MacAddress != "" {
		config.MacAddress = driverConfig.MacAddress

		// newer docker versions obsolete the config.MacAddress field
		isTooNew := semver.Compare(fmt.Sprintf("v%s", ver.APIVersion), "v1.44")
		if isTooNew >= 0 {
			if networkingConfig == nil {
				networkingConfig = &networkapi.NetworkingConfig{
					EndpointsConfig: map[string]*networkapi.EndpointSettings{
						string(hostConfig.NetworkMode): {},
					},
				}
			}
			networkingConfig.EndpointsConfig[string(hostConfig.NetworkMode)].MacAddress = driverConfig.MacAddress
		}

		logger.Debug("setting container mac address", "mac_address", config.MacAddress)
	}

	if driverConfig.Healthchecks.Disabled() {
		// Override any image-supplied health-check with disable sentinel.
		// https://github.com/docker/engine-api/blob/master/types/container/config.go#L16
		config.Healthcheck = &containerapi.HealthConfig{Test: []string{"NONE"}}
		logger.Debug("setting container healthchecks to be disabled")
	}

	return createContainerOptions{
		Name:       containerName,
		Config:     config,
		Host:       hostConfig,
		Networking: networkingConfig,
	}, nil
}

func (d *Driver) toDockerMount(m *DockerMount, task *drivers.TaskConfig) (*mount.Mount, error) {
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
func (d *Driver) detectIP(c types.ContainerJSON, driverConfig *TaskConfig) (string, bool) {
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
func (d *Driver) containerByName(name string) (*types.ContainerJSON, error) {

	dockerClient, err := d.getDockerClient()
	if err != nil {
		return nil, err
	}
	containers, err := dockerClient.ContainerList(d.ctx, containerapi.ListOptions{All: true})
	if err != nil {
		d.logger.Error("failed to query list of containers matching name",
			"container_name", name)
		return nil, recoverableErrTimeouts(
			fmt.Errorf("Failed to query list of containers: %s", err))
	}

	// container names with a / pre-pended to the Nomad generated container names
	containerName := "/" + name
	var (
		shimContainer types.Container
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

	container, err := dockerClient.ContainerInspect(d.ctx, shimContainer.ID)
	if err != nil {
		err = fmt.Errorf("Failed to inspect container %s: %s", shimContainer.ID, err)

		// This error is always recoverable as it could
		// be caused by races between listing
		// containers and this container being removed.
		// See #2802
		return nil, nstructs.NewRecoverableError(err, true)
	}
	return &container, nil
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

func (d *Driver) StopTask(taskID string, timeout time.Duration, signal string) error {
	h, ok := d.tasks.Get(taskID)
	if !ok {
		return drivers.ErrTaskNotFound
	}

	return h.Kill(timeout, signal)
}

func (d *Driver) DestroyTask(taskID string, force bool) error {
	h, ok := d.tasks.Get(taskID)
	if !ok {
		return drivers.ErrTaskNotFound
	}

	dockerClient, err := d.getDockerClient()
	if err != nil {
		return err
	}

	c, err := dockerClient.ContainerInspect(d.ctx, h.containerID)
	if err != nil {
		if _, ok := err.(errdefs.ErrNotFound); ok {
			h.logger.Info("container was removed out of band, will proceed with DestroyTask",
				"error", err)
		} else {
			return fmt.Errorf("failed to inspect container state: %v", err)
		}
	} else {
		if c.State.Running {
			if !force {
				return fmt.Errorf("must call StopTask for the given task before Destroy or set force to true")
			}
			if err := dockerClient.ContainerStop(d.ctx, h.containerID, containerapi.StopOptions{Timeout: pointer.Of(0)}); err != nil {
				h.logger.Warn("failed to stop container during destroy", "error", err)
			}
		}

		if h.removeContainerOnExit {
			if err := dockerClient.ContainerRemove(d.ctx, h.containerID, containerapi.RemoveOptions{RemoveVolumes: true, Force: true}); err != nil {
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

	dockerClient, err := d.getDockerClient()
	if err != nil {
		return nil, err
	}

	container, err := dockerClient.ContainerInspect(d.ctx, h.containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container %q: %v", h.containerID, err)
	}

	started, _ := time.Parse(time.RFC3339, container.State.StartedAt)
	completed, _ := time.Parse(time.RFC3339, container.State.FinishedAt)

	status := &drivers.TaskStatus{
		ID:          h.task.ID,
		Name:        h.task.Name,
		StartedAt:   started,
		CompletedAt: completed,
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

	return h.Stats(ctx, interval, d.compute)
}

func (d *Driver) TaskEvents(ctx context.Context) (<-chan *drivers.TaskEvent, error) {
	return d.eventer.TaskEvents(ctx)
}

func (d *Driver) SignalTask(taskID string, signal string) error {
	h, ok := d.tasks.Get(taskID)
	if !ok {
		return drivers.ErrTaskNotFound
	}

	_, err := signals.Parse(signal)
	if err != nil {
		return fmt.Errorf("failed to parse signal: %v", err)
	}

	// TODO: review whether we can timeout in this and other Docker API
	// calls without breaking the expected client behavior.
	// see https://github.com/hashicorp/nomad/issues/9503
	return h.dockerClient.ContainerKill(d.ctx, h.containerID, signal)
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

	createExecOpts := containerapi.ExecOptions{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          opts.Tty,
		Cmd:          opts.Command,
	}

	dockerClient, err := d.getDockerClient()
	if err != nil {
		return nil, err
	}

	exec, err := dockerClient.ContainerExecCreate(d.ctx, h.containerID, createExecOpts)
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
				dockerClient.ContainerExecResize(d.ctx, exec.ID, containerapi.ResizeOptions{
					Height: uint(s.Height),
					Width:  uint(s.Width),
				})
			}
		}
	}()

	resp, err := dockerClient.ContainerExecAttach(ctx, exec.ID, containerapi.ExecAttachOptions{Tty: opts.Tty})
	if err != nil {
		return nil, fmt.Errorf("failed to attach to exec: %v", err)
	}
	defer func() {
		opts.Stdin.Close() // close stdin
		resp.CloseWrite()  // close hijacked write connection
		resp.Close()       // close read connection
	}()

	go func() {
		if !opts.Tty {
			_, _ = stdcopy.StdCopy(opts.Stdout, opts.Stderr, resp.Reader)
		} else {
			_, _ = io.Copy(opts.Stdout, resp.Reader)
		}
	}()

	go func() {
		_, _ = io.Copy(resp.Conn, opts.Stdin)
		_ = resp.CloseWrite()
	}()

	exitCode := 999
	for {
		inspect, err := dockerClient.ContainerExecInspect(ctx, exec.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to inspect exec: %v", err)
		}

		running := inspect.Running
		if running {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		exitCode = inspect.ExitCode
		break
	}

	return &drivers.ExitResult{
		ExitCode: exitCode,
	}, nil
}

func (d *Driver) getOrCreateClient(timeout time.Duration) (*client.Client, error) {
	var (
		client *client.Client
		err    error
	)

	helper.WithLock(&d.dockerClientLock, func() {
		if timeout == 0 {
			if d.infinityClient == nil {
				d.infinityClient, err = d.newDockerClient(0)
			}
			client = d.infinityClient
		} else {
			if d.dockerClient == nil {
				d.dockerClient, err = d.newDockerClient(timeout)
			}
			client = d.dockerClient
		}
	})

	return client, err
}

// getInfinityClient creates a docker API client with no timeout.
func (d *Driver) getInfinityClient() (*client.Client, error) {
	return d.getOrCreateClient(0)
}

// getDockerClient creates a docker API client with a hard-coded timeout.
func (d *Driver) getDockerClient() (*client.Client, error) {
	return d.getOrCreateClient(dockerTimeout)
}

// newDockerClient creates a new *client.Client with a configurable timeout
func (d *Driver) newDockerClient(timeout time.Duration) (*client.Client, error) {
	var err error
	var merr multierror.Error
	var newClient *client.Client

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
			newClient, err = client.NewClientWithOpts(
				client.WithHost(dockerEndpoint),
				client.WithTLSClientConfig(ca, cert, key),
				client.WithAPIVersionNegotiation(),
			)
			if err != nil {
				merr.Errors = append(merr.Errors, err)
			}
		} else {
			d.logger.Debug("using standard client connection", "endpoint", dockerEndpoint)
			newClient, err = client.NewClientWithOpts(
				client.WithHost(dockerEndpoint),
				client.WithAPIVersionNegotiation(),
			)
			if err != nil {
				merr.Errors = append(merr.Errors, err)
			}
		}
	} else {
		d.logger.Debug("using client connection initialized from environment")
		newClient, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			merr.Errors = append(merr.Errors, err)
		}
	}

	if timeout != 0 && newClient != nil {
		newClient.HTTPClient().Timeout = timeout
	}
	return newClient, merr.ErrorOrNil()
}

func sliceMergeUlimit(ulimitsRaw map[string]string) ([]*containerapi.Ulimit, error) {
	var ulimits []*containerapi.Ulimit

	for name, ulimitRaw := range ulimitsRaw {
		if len(ulimitRaw) == 0 {
			return []*containerapi.Ulimit{}, fmt.Errorf("Malformed ulimit specification %v: %q, cannot be empty", name, ulimitRaw)
		}
		// hard limit is optional
		if !strings.Contains(ulimitRaw, ":") {
			ulimitRaw = ulimitRaw + ":" + ulimitRaw
		}

		splitted := strings.SplitN(ulimitRaw, ":", 2)
		if len(splitted) < 2 {
			return []*containerapi.Ulimit{}, fmt.Errorf("Malformed ulimit specification %v: %v", name, ulimitRaw)
		}
		soft, err := strconv.Atoi(splitted[0])
		if err != nil {
			return []*containerapi.Ulimit{}, fmt.Errorf("Malformed soft ulimit %v: %v", name, ulimitRaw)
		}
		hard, err := strconv.Atoi(splitted[1])
		if err != nil {
			return []*containerapi.Ulimit{}, fmt.Errorf("Malformed hard ulimit %v: %v", name, ulimitRaw)
		}

		ulimit := &containerapi.Ulimit{
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

func stopWithZeroTimeout() containerapi.StopOptions {
	return containerapi.StopOptions{Timeout: pointer.Of(0)}
}
