package driver

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	docker "github.com/fsouza/go-dockerclient"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	cstructs "github.com/hashicorp/nomad/client/driver/structs"
	"github.com/hashicorp/nomad/client/fingerprint"
	"github.com/hashicorp/nomad/helper/args"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/mapstructure"
)

// We store the client globally to cache the connection to the docker daemon.
var createClient sync.Once
var client *docker.Client

type DockerDriver struct {
	DriverContext
	fingerprint.StaticFingerprinter
}

type DockerDriverAuth struct {
	Username      string `mapstructure:"username"`       // username for the registry
	Password      string `mapstructure:"password"`       // password to access the registry
	Email         string `mapstructure:"email"`          // email address of the user who is allowed to access the registry
	ServerAddress string `mapstructure:"server_address"` // server address of the registry
}

type DockerDriverConfig struct {
	ImageName        string              `mapstructure:"image"`              // Container's Image Name
	Command          string              `mapstructure:"command"`            // The Command/Entrypoint to run when the container starts up
	Args             []string            `mapstructure:"args"`               // The arguments to the Command/Entrypoint
	IpcMode          string              `mapstructure:"ipc_mode"`           // The IPC mode of the container - host and none
	NetworkMode      string              `mapstructure:"network_mode"`       // The network mode of the container - host, net and none
	PidMode          string              `mapstructure:"pid_mode"`           // The PID mode of the container - host and none
	UTSMode          string              `mapstructure:"uts_mode"`           // The UTS mode of the container - host and none
	PortMapRaw       []map[string]int    `mapstructure:"port_map"`           //
	PortMap          map[string]int      `mapstructure:"-"`                  // A map of host port labels and the ports exposed on the container
	Privileged       bool                `mapstructure:"privileged"`         // Flag to run the container in priviledged mode
	DNSServers       []string            `mapstructure:"dns_servers"`        // DNS Server for containers
	DNSSearchDomains []string            `mapstructure:"dns_search_domains"` // DNS Search domains for containers
	Hostname         string              `mapstructure:"hostname"`           // Hostname for containers
	LabelsRaw        []map[string]string `mapstructure:"labels"`             //
	Labels           map[string]string   `mapstructure:"-"`                  // Labels to set when the container starts up
	Auth             []DockerDriverAuth  `mapstructure:"auth"`               // Authentication credentials for a private Docker registry
}

func (c *DockerDriverConfig) Validate() error {
	if c.ImageName == "" {
		return fmt.Errorf("Docker Driver needs an image name")
	}

	c.PortMap = mapMergeStrInt(c.PortMapRaw...)
	c.Labels = mapMergeStrStr(c.LabelsRaw...)

	return nil
}

type dockerPID struct {
	ImageID     string
	ContainerID string
	KillTimeout time.Duration
}

type DockerHandle struct {
	client           *docker.Client
	logger           *log.Logger
	cleanupContainer bool
	cleanupImage     bool
	imageID          string
	containerID      string
	killTimeout      time.Duration
	waitCh           chan *cstructs.WaitResult
	doneCh           chan struct{}
}

func NewDockerDriver(ctx *DriverContext) Driver {
	return &DockerDriver{DriverContext: *ctx}
}

// dockerClient creates *docker.Client. In test / dev mode we can use ENV vars
// to connect to the docker daemon. In production mode we will read
// docker.endpoint from the config file.
func (d *DockerDriver) dockerClient() (*docker.Client, error) {
	if client != nil {
		return client, nil
	}

	var err error
	createClient.Do(func() {
		// Default to using whatever is configured in docker.endpoint. If this is
		// not specified we'll fall back on NewClientFromEnv which reads config from
		// the DOCKER_* environment variables DOCKER_HOST, DOCKER_TLS_VERIFY, and
		// DOCKER_CERT_PATH. This allows us to lock down the config in production
		// but also accept the standard ENV configs for dev and test.
		dockerEndpoint := d.config.Read("docker.endpoint")
		if dockerEndpoint != "" {
			cert := d.config.Read("docker.tls.cert")
			key := d.config.Read("docker.tls.key")
			ca := d.config.Read("docker.tls.ca")

			if cert+key+ca != "" {
				d.logger.Printf("[DEBUG] driver.docker: using TLS client connection to %s", dockerEndpoint)
				client, err = docker.NewTLSClient(dockerEndpoint, cert, key, ca)
			} else {
				d.logger.Printf("[DEBUG] driver.docker: using standard client connection to %s", dockerEndpoint)
				client, err = docker.NewClient(dockerEndpoint)
			}
			return
		}

		d.logger.Println("[DEBUG] driver.docker: using client connection initialized from environment")
		client, err = docker.NewClientFromEnv()
	})
	return client, err
}

func (d *DockerDriver) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	// Initialize docker API client
	client, err := d.dockerClient()
	if err != nil {
		d.logger.Printf("[INFO] driver.docker: failed to initialize client: %s", err)
		return false, nil
	}

	privileged := d.config.ReadBoolDefault("docker.privileged.enabled", false)
	if privileged {
		d.logger.Println("[DEBUG] driver.docker: privileged containers are enabled")
		node.Attributes["docker.privileged.enabled"] = "1"
	} else {
		d.logger.Println("[DEBUG] driver.docker: privileged containers are disabled")
	}

	// This is the first operation taken on the client so we'll try to
	// establish a connection to the Docker daemon. If this fails it means
	// Docker isn't available so we'll simply disable the docker driver.
	env, err := client.Version()
	if err != nil {
		d.logger.Printf("[DEBUG] driver.docker: could not connect to docker daemon at %s: %s", client.Endpoint(), err)
		return false, nil
	}
	node.Attributes["driver.docker"] = "1"
	node.Attributes["driver.docker.version"] = env.Get("Version")

	return true, nil
}

func (d *DockerDriver) containerBinds(alloc *allocdir.AllocDir, task *structs.Task) ([]string, error) {
	shared := alloc.SharedDir
	local, ok := alloc.TaskDirs[task.Name]
	if !ok {
		return nil, fmt.Errorf("Failed to find task local directory: %v", task.Name)
	}

	return []string{
		// "z" and "Z" option is to allocate directory with SELinux label.
		fmt.Sprintf("%s:/%s:rw,z", shared, allocdir.SharedAllocName),
		// capital "Z" will label with Multi-Category Security (MCS) labels
		fmt.Sprintf("%s:/%s:rw,Z", local, allocdir.TaskLocal),
	}, nil
}

// createContainer initializes a struct needed to call docker.client.CreateContainer()
func (d *DockerDriver) createContainer(ctx *ExecContext, task *structs.Task, driverConfig *DockerDriverConfig) (docker.CreateContainerOptions, error) {
	var c docker.CreateContainerOptions
	if task.Resources == nil {
		// Guard against missing resources. We should never have been able to
		// schedule a job without specifying this.
		d.logger.Println("[ERR] driver.docker: task.Resources is empty")
		return c, fmt.Errorf("task.Resources is empty")
	}

	binds, err := d.containerBinds(ctx.AllocDir, task)
	if err != nil {
		return c, err
	}

	// Create environment variables.
	env := TaskEnvironmentVariables(ctx, task)
	env.SetAllocDir(filepath.Join("/", allocdir.SharedAllocName))
	env.SetTaskLocalDir(filepath.Join("/", allocdir.TaskLocal))

	config := &docker.Config{
		Image:    driverConfig.ImageName,
		Hostname: driverConfig.Hostname,
	}

	hostConfig := &docker.HostConfig{
		// Convert MB to bytes. This is an absolute value.
		//
		// This value represents the total amount of memory a process can use.
		// Swap is added to total memory and is managed by the OS, not docker.
		// Since this may cause other processes to swap and cause system
		// instability, we will simply not use swap.
		//
		// See: https://www.kernel.org/doc/Documentation/cgroups/memory.txt
		Memory:     int64(task.Resources.MemoryMB) * 1024 * 1024,
		MemorySwap: -1,
		// Convert Mhz to shares. This is a relative value.
		//
		// There are two types of CPU limiters available: Shares and Quotas. A
		// Share allows a particular process to have a proportion of CPU time
		// relative to other processes; 1024 by default. A CPU Quota is enforced
		// over a Period of time and is a HARD limit on the amount of CPU time a
		// process can use. Processes with quotas cannot burst, while processes
		// with shares can, so we'll use shares.
		//
		// The simplest scale is 1 share to 1 MHz so 1024 = 1GHz. This means any
		// given process will have at least that amount of resources, but likely
		// more since it is (probably) rare that the machine will run at 100%
		// CPU. This scale will cease to work if a node is overprovisioned.
		//
		// See:
		//  - https://www.kernel.org/doc/Documentation/scheduler/sched-bwc.txt
		//  - https://www.kernel.org/doc/Documentation/scheduler/sched-design-CFS.txt
		CPUShares: int64(task.Resources.CPU),

		// Binds are used to mount a host volume into the container. We mount a
		// local directory for storage and a shared alloc directory that can be
		// used to share data between different tasks in the same task group.
		Binds: binds,
	}

	d.logger.Printf("[DEBUG] driver.docker: using %d bytes memory for %s", hostConfig.Memory, task.Config["image"])
	d.logger.Printf("[DEBUG] driver.docker: using %d cpu shares for %s", hostConfig.CPUShares, task.Config["image"])
	d.logger.Printf("[DEBUG] driver.docker: binding directories %#v for %s", hostConfig.Binds, task.Config["image"])

	//  set privileged mode
	hostPrivileged := d.config.ReadBoolDefault("docker.privileged.enabled", false)
	if driverConfig.Privileged && !hostPrivileged {
		return c, fmt.Errorf(`Docker privileged mode is disabled on this Nomad agent`)
	}
	hostConfig.Privileged = hostPrivileged

	// set DNS servers
	for _, ip := range driverConfig.DNSServers {
		if net.ParseIP(ip) != nil {
			hostConfig.DNS = append(hostConfig.DNS, ip)
		} else {
			d.logger.Printf("[ERR] driver.docker: invalid ip address for container dns server: %s", ip)
		}
	}

	// set DNS search domains
	for _, domain := range driverConfig.DNSSearchDomains {
		hostConfig.DNSSearch = append(hostConfig.DNSSearch, domain)
	}

	if driverConfig.IpcMode != "" {
		if !hostPrivileged {
			return c, fmt.Errorf(`Docker privileged mode is disabled on this Nomad agent, setting ipc mode not allowed`)
		}
		d.logger.Printf("[DEBUG] driver.docker: setting ipc mode to %s", driverConfig.IpcMode)
	}
	hostConfig.IpcMode = driverConfig.IpcMode

	if driverConfig.PidMode != "" {
		if !hostPrivileged {
			return c, fmt.Errorf(`Docker privileged mode is disabled on this Nomad agent, setting pid mode not allowed`)
		}
		d.logger.Printf("[DEBUG] driver.docker: setting pid mode to %s", driverConfig.PidMode)
	}
	hostConfig.PidMode = driverConfig.PidMode

	if driverConfig.UTSMode != "" {
		if !hostPrivileged {
			return c, fmt.Errorf(`Docker privileged mode is disabled on this Nomad agent, setting UTS mode not allowed`)
		}
		d.logger.Printf("[DEBUG] driver.docker: setting UTS mode to %s", driverConfig.UTSMode)
	}
	hostConfig.UTSMode = driverConfig.UTSMode

	hostConfig.NetworkMode = driverConfig.NetworkMode
	if hostConfig.NetworkMode == "" {
		// docker default
		d.logger.Println("[DEBUG] driver.docker: networking mode not specified; defaulting to bridge")
		hostConfig.NetworkMode = "bridge"
	}

	// Setup port mapping and exposed ports
	if len(task.Resources.Networks) == 0 {
		d.logger.Println("[DEBUG] driver.docker: No network interfaces are available")
		if len(driverConfig.PortMap) > 0 {
			return c, fmt.Errorf("Trying to map ports but no network interface is available")
		}
	} else {
		// TODO add support for more than one network
		network := task.Resources.Networks[0]
		publishedPorts := map[docker.Port][]docker.PortBinding{}
		exposedPorts := map[docker.Port]struct{}{}

		for _, port := range network.ReservedPorts {
			// By default we will map the allocated port 1:1 to the container
			containerPortInt := port.Value

			// If the user has mapped a port using port_map we'll change it here
			if mapped, ok := driverConfig.PortMap[port.Label]; ok {
				containerPortInt = mapped
			}

			hostPortStr := strconv.Itoa(port.Value)
			containerPort := docker.Port(strconv.Itoa(containerPortInt))

			publishedPorts[containerPort+"/tcp"] = []docker.PortBinding{docker.PortBinding{HostIP: network.IP, HostPort: hostPortStr}}
			publishedPorts[containerPort+"/udp"] = []docker.PortBinding{docker.PortBinding{HostIP: network.IP, HostPort: hostPortStr}}
			d.logger.Printf("[DEBUG] driver.docker: allocated port %s:%d -> %d (static)", network.IP, port.Value, port.Value)

			exposedPorts[containerPort+"/tcp"] = struct{}{}
			exposedPorts[containerPort+"/udp"] = struct{}{}
			d.logger.Printf("[DEBUG] driver.docker: exposed port %d", port.Value)
		}

		for _, port := range network.DynamicPorts {
			// By default we will map the allocated port 1:1 to the container
			containerPortInt := port.Value

			// If the user has mapped a port using port_map we'll change it here
			if mapped, ok := driverConfig.PortMap[port.Label]; ok {
				containerPortInt = mapped
			}

			hostPortStr := strconv.Itoa(port.Value)
			containerPort := docker.Port(strconv.Itoa(containerPortInt))

			publishedPorts[containerPort+"/tcp"] = []docker.PortBinding{docker.PortBinding{HostIP: network.IP, HostPort: hostPortStr}}
			publishedPorts[containerPort+"/udp"] = []docker.PortBinding{docker.PortBinding{HostIP: network.IP, HostPort: hostPortStr}}
			d.logger.Printf("[DEBUG] driver.docker: allocated port %s:%d -> %d (mapped)", network.IP, port.Value, containerPortInt)

			exposedPorts[containerPort+"/tcp"] = struct{}{}
			exposedPorts[containerPort+"/udp"] = struct{}{}
			d.logger.Printf("[DEBUG] driver.docker: exposed port %s", containerPort)
		}

		// This was set above in a call to TaskEnvironmentVariables but if we
		// have mapped any ports we will need to override them.
		//
		// TODO refactor the implementation in TaskEnvironmentVariables to match
		// the 0.2 ports world view. Docker seems to be the only place where
		// this is actually needed, but this is kinda hacky.
		if len(driverConfig.PortMap) > 0 {
			env.SetPorts(network.MapLabelToValues(driverConfig.PortMap))
		}
		hostConfig.PortBindings = publishedPorts
		config.ExposedPorts = exposedPorts
	}

	parsedArgs := args.ParseAndReplace(driverConfig.Args, env.Map())

	// If the user specified a custom command to run as their entrypoint, we'll
	// inject it here.
	if driverConfig.Command != "" {
		cmd := []string{driverConfig.Command}
		if len(driverConfig.Args) != 0 {
			cmd = append(cmd, parsedArgs...)
		}
		d.logger.Printf("[DEBUG] driver.docker: setting container startup command to: %s", strings.Join(cmd, " "))
		config.Cmd = cmd
	} else if len(driverConfig.Args) != 0 {
		d.logger.Println("[DEBUG] driver.docker: ignoring command arguments because command is not specified")
	}

	if len(driverConfig.Labels) > 0 {
		config.Labels = driverConfig.Labels
		d.logger.Printf("[DEBUG] driver.docker: applied labels on the container: %+v", config.Labels)
	}

	config.Env = env.List()

	containerName := fmt.Sprintf("%s-%s", task.Name, ctx.AllocID)
	d.logger.Printf("[DEBUG] driver.docker: setting container name to: %s", containerName)

	return docker.CreateContainerOptions{
		Name:       containerName,
		Config:     config,
		HostConfig: hostConfig,
	}, nil
}

func (d *DockerDriver) Start(ctx *ExecContext, task *structs.Task) (DriverHandle, error) {
	var driverConfig DockerDriverConfig
	if err := mapstructure.WeakDecode(task.Config, &driverConfig); err != nil {
		return nil, err
	}
	image := driverConfig.ImageName

	if err := driverConfig.Validate(); err != nil {
		return nil, err
	}
	if task.Resources == nil {
		return nil, fmt.Errorf("Resources are not specified")
	}
	if task.Resources.MemoryMB == 0 {
		return nil, fmt.Errorf("Memory limit cannot be zero")
	}
	if task.Resources.CPU == 0 {
		return nil, fmt.Errorf("CPU limit cannot be zero")
	}

	cleanupContainer := d.config.ReadBoolDefault("docker.cleanup.container", true)
	cleanupImage := d.config.ReadBoolDefault("docker.cleanup.image", true)

	// Initialize docker API client
	client, err := d.dockerClient()
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to docker daemon: %s", err)
	}

	repo, tag := docker.ParseRepositoryTag(image)
	// Make sure tag is always explicitly set. We'll default to "latest" if it
	// isn't, which is the expected behavior.
	if tag == "" {
		tag = "latest"
	}

	var dockerImage *docker.Image
	// We're going to check whether the image is already downloaded. If the tag
	// is "latest" we have to check for a new version every time so we don't
	// bother to check and cache the id here. We'll download first, then cache.
	if tag != "latest" {
		dockerImage, err = client.InspectImage(image)
	}

	// Download the image
	if dockerImage == nil {
		pullOptions := docker.PullImageOptions{
			Repository: repo,
			Tag:        tag,
		}

		authOptions := docker.AuthConfiguration{}
		if len(driverConfig.Auth) != 0 {
			authOptions = docker.AuthConfiguration{
				Username:      driverConfig.Auth[0].Username,
				Password:      driverConfig.Auth[0].Password,
				Email:         driverConfig.Auth[0].Email,
				ServerAddress: driverConfig.Auth[0].ServerAddress,
			}
		}

		err = client.PullImage(pullOptions, authOptions)
		if err != nil {
			d.logger.Printf("[ERR] driver.docker: failed pulling container %s:%s: %s", repo, tag, err)
			return nil, fmt.Errorf("Failed to pull `%s`: %s", image, err)
		}
		d.logger.Printf("[DEBUG] driver.docker: docker pull %s:%s succeeded", repo, tag)

		// Now that we have the image we can get the image id
		dockerImage, err = client.InspectImage(image)
		if err != nil {
			d.logger.Printf("[ERR] driver.docker: failed getting image id for %s: %s", image, err)
			return nil, fmt.Errorf("Failed to determine image id for `%s`: %s", image, err)
		}
	}
	d.logger.Printf("[DEBUG] driver.docker: identified image %s as %s", image, dockerImage.ID)

	config, err := d.createContainer(ctx, task, &driverConfig)
	if err != nil {
		d.logger.Printf("[ERR] driver.docker: failed to create container configuration for image %s: %s", image, err)
		return nil, fmt.Errorf("Failed to create container configuration for image %s: %s", image, err)
	}
	// Create a container
	container, err := client.CreateContainer(config)
	if err != nil {
		// If the container already exists because of a previous failure we'll
		// try to purge it and re-create it.
		if strings.Contains(err.Error(), "container already exists") {
			// Get the ID of the existing container so we can delete it
			containers, err := client.ListContainers(docker.ListContainersOptions{
				// The image might be in use by a stopped container, so check everything
				All: true,
				Filters: map[string][]string{
					"name": []string{config.Name},
				},
			})
			if err != nil {
				d.logger.Printf("[ERR] driver.docker: failed to query list of containers matching name:%s", config.Name)
				return nil, fmt.Errorf("Failed to query list of containers: %s", err)
			}

			if len(containers) != 1 {
				d.logger.Printf("[ERR] driver.docker: failed to get id for container %s", config.Name)
				return nil, fmt.Errorf("Failed to get id for container %s", config.Name)
			}

			d.logger.Printf("[INFO] driver.docker: a container with the name %s already exists; will attempt to purge and re-create", config.Name)
			err = client.RemoveContainer(docker.RemoveContainerOptions{
				ID: containers[0].ID,
			})
			if err != nil {
				d.logger.Printf("[ERR] driver.docker: failed to purge container %s", config.Name)
				return nil, fmt.Errorf("Failed to purge container %s: %s", config.Name, err)
			}
			d.logger.Printf("[INFO] driver.docker: purged container %s", config.Name)
			container, err = client.CreateContainer(config)
			if err != nil {
				d.logger.Printf("[ERR] driver.docker: failed to re-create container %s; aborting", config.Name)
				return nil, fmt.Errorf("Failed to re-create container %s; aborting", config.Name)
			}
		} else {
			// We failed to create the container for some other reason.
			d.logger.Printf("[ERR] driver.docker: failed to create container from image %s: %s", image, err)
			return nil, fmt.Errorf("Failed to create container from image %s: %s", image, err)
		}
	}
	d.logger.Printf("[INFO] driver.docker: created container %s", container.ID)

	// Start the container
	err = client.StartContainer(container.ID, container.HostConfig)
	if err != nil {
		d.logger.Printf("[ERR] driver.docker: failed to start container %s: %s", container.ID, err)
		return nil, fmt.Errorf("Failed to start container %s: %s", container.ID, err)
	}
	d.logger.Printf("[INFO] driver.docker: started container %s", container.ID)

	// Return a driver handle
	h := &DockerHandle{
		client:           client,
		cleanupContainer: cleanupContainer,
		cleanupImage:     cleanupImage,
		logger:           d.logger,
		imageID:          dockerImage.ID,
		containerID:      container.ID,
		killTimeout:      d.DriverContext.KillTimeout(task),
		doneCh:           make(chan struct{}),
		waitCh:           make(chan *cstructs.WaitResult, 1),
	}
	go h.Wait()
	return h, nil
}

func (d *DockerDriver) Open(ctx *ExecContext, handleID string) (DriverHandle, error) {
	cleanupContainer := d.config.ReadBoolDefault("docker.cleanup.container", true)
	cleanupImage := d.config.ReadBoolDefault("docker.cleanup.image", true)

	// Split the handle
	pidBytes := []byte(strings.TrimPrefix(handleID, "DOCKER:"))
	pid := &dockerPID{}
	if err := json.Unmarshal(pidBytes, pid); err != nil {
		return nil, fmt.Errorf("Failed to parse handle '%s': %v", handleID, err)
	}
	d.logger.Printf("[INFO] driver.docker: re-attaching to docker process: %s", handleID)

	// Initialize docker API client
	client, err := d.dockerClient()
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to docker daemon: %s", err)
	}

	// Look for a running container with this ID
	containers, err := client.ListContainers(docker.ListContainersOptions{
		Filters: map[string][]string{
			"id": []string{pid.ContainerID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to query for container %s: %v", pid.ContainerID, err)
	}

	found := false
	for _, container := range containers {
		if container.ID == pid.ContainerID {
			found = true
		}
	}
	if !found {
		return nil, fmt.Errorf("Failed to find container %s: %v", pid.ContainerID, err)
	}

	// Return a driver handle
	h := &DockerHandle{
		client:           client,
		cleanupContainer: cleanupContainer,
		cleanupImage:     cleanupImage,
		logger:           d.logger,
		imageID:          pid.ImageID,
		containerID:      pid.ContainerID,
		killTimeout:      pid.KillTimeout,
		doneCh:           make(chan struct{}),
		waitCh:           make(chan *cstructs.WaitResult, 1),
	}
	return h, nil
}

func (h *DockerHandle) ID() string {
	// Return a handle to the PID
	pid := dockerPID{
		ImageID:     h.imageID,
		ContainerID: h.containerID,
		KillTimeout: h.killTimeout,
	}
	data, err := json.Marshal(pid)
	if err != nil {
		h.logger.Printf("[ERR] driver.docker: failed to marshal docker PID to JSON: %s", err)
	}
	return fmt.Sprintf("DOCKER:%s", string(data))
}

func (h *DockerHandle) ContainerID() string {
	return h.containerID
}

func (h *DockerHandle) WaitCh() chan *cstructs.WaitResult {
	return h.waitCh
}

func (h *DockerHandle) Update(task *structs.Task) error {
	// Update is not possible
	return nil
}

// Kill is used to terminate the task. This uses `docker stop -t killTimeout`
func (h *DockerHandle) Kill() error {
	// Stop the container
	err := h.client.StopContainer(h.containerID, uint(h.killTimeout.Seconds()))
	if err != nil {
		h.logger.Printf("[ERR] driver.docker: failed to stop container %s", h.containerID)
		return fmt.Errorf("Failed to stop container %s: %s", h.containerID, err)
	}
	h.logger.Printf("[INFO] driver.docker: stopped container %s", h.containerID)

	// Cleanup container
	if h.cleanupContainer {
		err = h.client.RemoveContainer(docker.RemoveContainerOptions{
			ID:            h.containerID,
			RemoveVolumes: true,
		})
		if err != nil {
			h.logger.Printf("[ERR] driver.docker: failed to remove container %s", h.containerID)
			return fmt.Errorf("Failed to remove container %s: %s", h.containerID, err)
		}
		h.logger.Printf("[INFO] driver.docker: removed container %s", h.containerID)
	}

	// Cleanup image. This operation may fail if the image is in use by another
	// job. That is OK. Will we log a message but continue.
	if h.cleanupImage {
		err = h.client.RemoveImage(h.imageID)
		if err != nil {
			containers, err := h.client.ListContainers(docker.ListContainersOptions{
				// The image might be in use by a stopped container, so check everything
				All: true,
				Filters: map[string][]string{
					"image": []string{h.imageID},
				},
			})
			if err != nil {
				h.logger.Printf("[ERR] driver.docker: failed to query list of containers matching image:%s", h.imageID)
				return fmt.Errorf("Failed to query list of containers: %s", err)
			}
			inUse := len(containers)
			if inUse > 0 {
				h.logger.Printf("[INFO] driver.docker: image %s is still in use by %d container(s)", h.imageID, inUse)
			} else {
				return fmt.Errorf("Failed to remove image %s", h.imageID)
			}
		} else {
			h.logger.Printf("[INFO] driver.docker: removed image %s", h.imageID)
		}
	}
	return nil
}

func (h *DockerHandle) Wait() {
	// Wait for it...
	exitCode, err := h.client.WaitContainer(h.containerID)
	if err != nil {
		h.logger.Printf("[ERR] driver.docker: failed to wait for %s; container already terminated", h.containerID)
	}

	if exitCode != 0 {
		err = fmt.Errorf("Docker container exited with non-zero exit code: %d", exitCode)
	}

	close(h.doneCh)
	h.waitCh <- cstructs.NewWaitResult(exitCode, 0, err)
	close(h.waitCh)
}

func (h *DockerHandle) Logs(w io.Writer, follow bool, stdout bool, stderr bool, lines int64) error {
	err := h.client.Logs(docker.LogsOptions{
		Container:    h.containerID,
		OutputStream: w,
		ErrorStream:  w,
		Stdout:       stdout,
		Stderr:       stderr,
		Follow:       follow,
		Tail:         strconv.Itoa(int(lines)),
	})

	if err != nil {
		h.logger.Printf("[ERROR] client: error fetching logs from container: %v, err: %v", h.containerID, err)
	}
	return nil
}
