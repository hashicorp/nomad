package driver

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"path/filepath"
	"strconv"
	"strings"

	docker "github.com/fsouza/go-dockerclient"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver/args"
	cstructs "github.com/hashicorp/nomad/client/driver/structs"
	"github.com/hashicorp/nomad/client/fingerprint"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/mapstructure"
)

type DockerDriver struct {
	DriverContext
	fingerprint.StaticFingerprinter
}

type DockerAuthConfig struct {
	UserName      string `mapstructure:"auth.username"`       // user name of the registry
	Password      string `mapstructure:"auth.password"`       // password to access the registry
	Email         string `mapstructure:"auth.email"`          // email address of the user who is allowed to access the registry
	ServerAddress string `mapstructure:"auth.server_address"` // server address of the registry

}

type DockerDriverConfig struct {
	DockerAuthConfig
	ImageName     string              `mapstructure:"image"`          // Container's Image Name
	Command       string              `mapstructure:"command"`        // The Command/Entrypoint to run when the container starts up
	Args          string              `mapstructure:"args"`           // The arguments to the Command/Entrypoint
	NetworkMode   string              `mapstructure:"network_mode"`   // The network mode of the container - host, net and none
	PortMap       []map[string]int    `mapstructure:"port_map"`       // A map of host port labels and the ports exposed on the container
	Privileged    bool                `mapstructure:"privileged"`     // Flag to run the container in priviledged mode
	DNS           string              `mapstructure:"dns_server"`     // DNS Server for containers
	SearchDomains string              `mapstructure:"search_domains"` // DNS Search domains for containers
	Hostname      string              `mapstructure:"hostname"`       // Hostname for containers
	Labels        []map[string]string `mapstructure:"labels"`         // Labels to set when the container starts up
}

func (c *DockerDriverConfig) Validate() error {
	if c.ImageName == "" {
		return fmt.Errorf("Docker Driver needs an image name")
	}

	if len(c.PortMap) > 1 {
		return fmt.Errorf("Only one port_map block is allowed in the docker driver config")
	}

	if len(c.Labels) > 1 {
		return fmt.Errorf("Only one labels block is allowed in the docker driver config")
	}
	return nil
}

type dockerPID struct {
	ImageID     string
	ContainerID string
}

type dockerHandle struct {
	client           *docker.Client
	logger           *log.Logger
	cleanupContainer bool
	cleanupImage     bool
	imageID          string
	containerID      string
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
	// Default to using whatever is configured in docker.endpoint. If this is
	// not specified we'll fall back on NewClientFromEnv which reads config from
	// the DOCKER_* environment variables DOCKER_HOST, DOCKER_TLS_VERIFY, and
	// DOCKER_CERT_PATH. This allows us to lock down the config in production
	// but also accept the standard ENV configs for dev and test.
	dockerEndpoint := d.config.Read("docker.endpoint")
	if dockerEndpoint != "" {
		return docker.NewClient(dockerEndpoint)
	}

	return docker.NewClientFromEnv()
}

func (d *DockerDriver) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	// Initialize docker API client
	client, err := d.dockerClient()
	if err != nil {
		d.logger.Printf("[INFO] driver.docker: failed to initialize client: %s\n", err)
		return false, nil
	}

	privileged := d.config.ReadBoolDefault("docker.privileged.enabled", false)
	if privileged {
		d.logger.Println("[INFO] driver.docker: privileged containers are enabled")
		node.Attributes["docker.privileged.enabled"] = "1"
	} else {
		d.logger.Println("[INFO] driver.docker: privileged containers are disabled")
	}

	// This is the first operation taken on the client so we'll try to
	// establish a connection to the Docker daemon. If this fails it means
	// Docker isn't available so we'll simply disable the docker driver.
	env, err := client.Version()
	if err != nil {
		d.logger.Printf("[INFO] driver.docker: could not connect to docker daemon at %s: %s\n", client.Endpoint(), err)
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

	d.logger.Printf("[DEBUG] driver.docker: using %d bytes memory for %s\n", hostConfig.Memory, task.Config["image"])
	d.logger.Printf("[DEBUG] driver.docker: using %d cpu shares for %s\n", hostConfig.CPUShares, task.Config["image"])
	d.logger.Printf("[DEBUG] driver.docker: binding directories %#v for %s\n", hostConfig.Binds, task.Config["image"])

	//  set privileged mode
	hostPrivileged := d.config.ReadBoolDefault("docker.privileged.enabled", false)
	if driverConfig.Privileged && !hostPrivileged {
		return c, fmt.Errorf(`Unable to set privileged flag since "docker.privileged.enabled" is false`)
	}
	hostConfig.Privileged = hostPrivileged

	// set DNS servers
	if driverConfig.DNS != "" {
		for _, v := range strings.Split(driverConfig.DNS, ",") {
			ip := strings.TrimSpace(v)
			if net.ParseIP(ip) != nil {
				hostConfig.DNS = append(hostConfig.DNS, ip)
			} else {
				d.logger.Printf("[ERR] driver.docker: invalid ip address for container dns server: %s\n", ip)
			}
		}
	}

	// set DNS search domains
	if driverConfig.SearchDomains != "" {
		for _, v := range strings.Split(driverConfig.SearchDomains, ",") {
			hostConfig.DNSSearch = append(hostConfig.DNSSearch, strings.TrimSpace(v))
		}
	}

	mode := driverConfig.NetworkMode
	if mode == "" {
		// docker default
		d.logger.Println("[DEBUG] driver.docker: no mode specified for networking, defaulting to bridge")
		mode = "bridge"
	}

	// Ignore the container mode for now
	switch mode {
	case "default", "bridge", "none", "host":
		d.logger.Printf("[DEBUG] driver.docker: using %s as network mode\n", mode)
	default:
		d.logger.Printf("[ERR] driver.docker: invalid setting for network mode: %s\n", mode)
		return c, fmt.Errorf("Invalid setting for network mode: %s", mode)
	}
	hostConfig.NetworkMode = mode

	// Setup port mapping and exposed ports
	if len(task.Resources.Networks) == 0 {
		d.logger.Println("[DEBUG] driver.docker: No network interfaces are available")
		if len(driverConfig.PortMap[0]) > 0 {
			return c, fmt.Errorf("Trying to map ports but no network interface is available")
		}
	} else {
		// TODO add support for more than one network
		network := task.Resources.Networks[0]
		publishedPorts := map[docker.Port][]docker.PortBinding{}
		exposedPorts := map[docker.Port]struct{}{}

		for _, port := range network.ReservedPorts {
			hostPortStr := strconv.Itoa(port.Value)
			dockerPort := docker.Port(hostPortStr)

			publishedPorts[dockerPort+"/tcp"] = []docker.PortBinding{docker.PortBinding{HostIP: network.IP, HostPort: hostPortStr}}
			publishedPorts[dockerPort+"/udp"] = []docker.PortBinding{docker.PortBinding{HostIP: network.IP, HostPort: hostPortStr}}
			d.logger.Printf("[DEBUG] driver.docker: allocated port %s:%d -> %d (static)\n", network.IP, port.Value, port.Value)

			exposedPorts[dockerPort+"/tcp"] = struct{}{}
			exposedPorts[dockerPort+"/udp"] = struct{}{}
			d.logger.Printf("[DEBUG] driver.docker: exposed port %d\n", port.Value)
		}

		containerToHostPortMap := make(map[string]int)
		for _, port := range network.DynamicPorts {
			containerPort, ok := driverConfig.PortMap[0][port.Label]
			if !ok {
				containerPort = port.Value
			}

			containerPortStr := docker.Port(strconv.Itoa(containerPort))
			hostPortStr := strconv.Itoa(port.Value)

			publishedPorts[containerPortStr+"/tcp"] = []docker.PortBinding{docker.PortBinding{HostIP: network.IP, HostPort: hostPortStr}}
			publishedPorts[containerPortStr+"/udp"] = []docker.PortBinding{docker.PortBinding{HostIP: network.IP, HostPort: hostPortStr}}
			d.logger.Printf("[DEBUG] driver.docker: allocated port %s:%d -> %d (mapped)\n", network.IP, port.Value, containerPort)

			exposedPorts[containerPortStr+"/tcp"] = struct{}{}
			exposedPorts[containerPortStr+"/udp"] = struct{}{}
			d.logger.Printf("[DEBUG] driver.docker: exposed port %s\n", hostPortStr)

			containerToHostPortMap[string(containerPortStr)] = port.Value
		}

		env.SetPorts(containerToHostPortMap)
		hostConfig.PortBindings = publishedPorts
		config.ExposedPorts = exposedPorts
	}

	parsedArgs, err := args.ParseAndReplace(driverConfig.Args, env.Map())
	if err != nil {
		return c, err
	}

	// If the user specified a custom command to run as their entrypoint, we'll
	// inject it here.
	if driverConfig.Command != "" {
		cmd := []string{driverConfig.Command}
		if driverConfig.Args != "" {
			cmd = append(cmd, parsedArgs...)
		}
		d.logger.Printf("[DEBUG] driver.docker: setting container startup command to: %s\n", strings.Join(cmd, " "))
		config.Cmd = cmd
	} else if driverConfig.Args != "" {
		d.logger.Println("[DEBUG] driver.docker: ignoring command arguments because command is not specified")
	}

	if len(driverConfig.Labels) == 1 {
		config.Labels = driverConfig.Labels[0]
		d.logger.Println("[DEBUG] driver.docker: applied labels on the container")
	}

	config.Env = env.List()
	return docker.CreateContainerOptions{
		Name:       fmt.Sprintf("%s-%s", task.Name, ctx.AllocID),
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

		authOptions := docker.AuthConfiguration{
			Username:      driverConfig.UserName,
			Password:      driverConfig.Password,
			Email:         driverConfig.Email,
			ServerAddress: driverConfig.ServerAddress,
		}

		err = client.PullImage(pullOptions, authOptions)
		if err != nil {
			d.logger.Printf("[ERR] driver.docker: failed pulling container %s:%s: %s\n", repo, tag, err)
			return nil, fmt.Errorf("Failed to pull `%s`: %s", image, err)
		}
		d.logger.Printf("[DEBUG] driver.docker: docker pull %s:%s succeeded\n", repo, tag)

		// Now that we have the image we can get the image id
		dockerImage, err = client.InspectImage(image)
		if err != nil {
			d.logger.Printf("[ERR] driver.docker: failed getting image id for %s\n", image)
			return nil, fmt.Errorf("Failed to determine image id for `%s`: %s", image, err)
		}
	}
	d.logger.Printf("[DEBUG] driver.docker: identified image %s as %s\n", image, dockerImage.ID)

	config, err := d.createContainer(ctx, task, &driverConfig)
	if err != nil {
		d.logger.Printf("[ERR] driver.docker: failed to create container configuration for image %s: %s\n", image, err)
		return nil, fmt.Errorf("Failed to create container configuration for image %s: %s", image, err)
	}
	// Create a container
	container, err := client.CreateContainer(config)
	if err != nil {
		d.logger.Printf("[ERR] driver.docker: failed to create container from image %s: %s\n", image, err)
		return nil, fmt.Errorf("Failed to create container from image %s", image)
	}
	d.logger.Printf("[INFO] driver.docker: created container %s\n", container.ID)

	// Start the container
	err = client.StartContainer(container.ID, container.HostConfig)
	if err != nil {
		d.logger.Printf("[ERR] driver.docker: starting container %s\n", container.ID)
		return nil, fmt.Errorf("Failed to start container %s", container.ID)
	}
	d.logger.Printf("[INFO] driver.docker: started container %s\n", container.ID)

	// Return a driver handle
	h := &dockerHandle{
		client:           client,
		cleanupContainer: cleanupContainer,
		cleanupImage:     cleanupImage,
		logger:           d.logger,
		imageID:          dockerImage.ID,
		containerID:      container.ID,
		doneCh:           make(chan struct{}),
		waitCh:           make(chan *cstructs.WaitResult, 1),
	}
	go h.run()
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
	d.logger.Printf("[INFO] driver.docker: re-attaching to docker process: %s\n", handleID)

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
	h := &dockerHandle{
		client:           client,
		cleanupContainer: cleanupContainer,
		cleanupImage:     cleanupImage,
		logger:           d.logger,
		imageID:          pid.ImageID,
		containerID:      pid.ContainerID,
		doneCh:           make(chan struct{}),
		waitCh:           make(chan *cstructs.WaitResult, 1),
	}
	go h.run()
	return h, nil
}

func (h *dockerHandle) ID() string {
	// Return a handle to the PID
	pid := dockerPID{
		ImageID:     h.imageID,
		ContainerID: h.containerID,
	}
	data, err := json.Marshal(pid)
	if err != nil {
		h.logger.Printf("[ERR] driver.docker: failed to marshal docker PID to JSON: %s\n", err)
	}
	return fmt.Sprintf("DOCKER:%s", string(data))
}

func (h *dockerHandle) WaitCh() chan *cstructs.WaitResult {
	return h.waitCh
}

func (h *dockerHandle) Update(task *structs.Task) error {
	// Update is not possible
	return nil
}

// Kill is used to terminate the task. This uses docker stop -t 5
func (h *dockerHandle) Kill() error {
	// Stop the container
	err := h.client.StopContainer(h.containerID, 5)
	if err != nil {
		log.Printf("[ERR] driver.docker: failed to stop container %s", h.containerID)
		return fmt.Errorf("Failed to stop container %s: %s", h.containerID, err)
	}
	log.Printf("[INFO] driver.docker: stopped container %s", h.containerID)

	// Cleanup container
	if h.cleanupContainer {
		err = h.client.RemoveContainer(docker.RemoveContainerOptions{
			ID:            h.containerID,
			RemoveVolumes: true,
		})
		if err != nil {
			log.Printf("[ERR] driver.docker: failed to remove container %s", h.containerID)
			return fmt.Errorf("Failed to remove container %s: %s", h.containerID, err)
		}
		log.Printf("[INFO] driver.docker: removed container %s", h.containerID)
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
				return fmt.Errorf("Unable to query list of containers: %s", err)
			}
			inUse := len(containers)
			if inUse > 0 {
				log.Printf("[INFO] driver.docker: image %s is still in use by %d containers\n", h.imageID, inUse)
			} else {
				return fmt.Errorf("Failed to remove image %s", h.imageID)
			}
		} else {
			log.Printf("[INFO] driver.docker: removed image %s\n", h.imageID)
		}
	}
	return nil
}

func (h *dockerHandle) run() {
	// Wait for it...
	exitCode, err := h.client.WaitContainer(h.containerID)
	if err != nil {
		h.logger.Printf("[ERR] driver.docker: unable to wait for %s; container already terminated\n", h.containerID)
	}

	if exitCode != 0 {
		err = fmt.Errorf("Docker container exited with non-zero exit code: %d", exitCode)
	}

	close(h.doneCh)
	h.waitCh <- cstructs.NewWaitResult(exitCode, 0, err)
	close(h.waitCh)
}
