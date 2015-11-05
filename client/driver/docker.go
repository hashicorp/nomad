package driver

import (
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"strings"

	docker "github.com/fsouza/go-dockerclient"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver/args"
	"github.com/hashicorp/nomad/nomad/structs"
)

type DockerDriver struct {
	DriverContext
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
	waitCh           chan error
	doneCh           chan struct{}
}

func NewDockerDriver(ctx *DriverContext) Driver {
	return &DockerDriver{*ctx}
}

// dockerClient creates *docker.Client. In test / dev mode we can use ENV vars
// to connect to the docker daemon. In production mode we will read
// docker.endpoint from the config file.
func (d *DockerDriver) dockerClient() (*docker.Client, error) {
	// In dev mode, read DOCKER_* environment variables DOCKER_HOST,
	// DOCKER_TLS_VERIFY, and DOCKER_CERT_PATH. This allows you to run tests and
	// demo against boot2docker or a VM on OSX and Windows. This falls back on
	// the default unix socket on linux if tests are run on linux.
	//
	// Also note that we need to turn on DevMode in the test configs.
	if d.config.DevMode {
		return docker.NewClientFromEnv()
	}

	// In prod mode we'll read the docker.endpoint configuration and fall back
	// on the host-specific default. We do not read from the environment.
	defaultEndpoint, err := docker.DefaultDockerHost()
	if err != nil {
		return nil, fmt.Errorf("Unable to determine default docker endpoint: %s", err)
	}
	dockerEndpoint := d.config.ReadDefault("docker.endpoint", defaultEndpoint)

	return docker.NewClient(dockerEndpoint)
}

func (d *DockerDriver) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	// Initialize docker API client
	client, err := d.dockerClient()
	if err != nil {
		d.logger.Printf("[DEBUG] driver.docker: could not connect to docker daemon: %v", err)
		return false, nil
	}

	_, err = strconv.ParseBool(d.config.ReadDefault("docker.cleanup.container", "true"))
	if err != nil {
		return false, fmt.Errorf("Unable to parse docker.cleanup.container: %s", err)
	}
	_, err = strconv.ParseBool(d.config.ReadDefault("docker.cleanup.image", "true"))
	if err != nil {
		return false, fmt.Errorf("Unable to parse docker.cleanup.image: %s", err)
	}

	env, err := client.Version()
	if err != nil {
		d.logger.Printf("[DEBUG] driver.docker: could not read version from daemon: %v", err)
		// Check the "no such file" error if the unix file is missing
		if strings.Contains(err.Error(), "no such file") {
			return false, nil
		}

		// We connected to the daemon but couldn't read the version so something
		// is broken.
		return false, err
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
func (d *DockerDriver) createContainer(ctx *ExecContext, task *structs.Task) (docker.CreateContainerOptions, error) {
	var c docker.CreateContainerOptions
	if task.Resources == nil {
		d.logger.Printf("[ERR] driver.docker: task.Resources is empty")
		return c, fmt.Errorf("task.Resources is nil and we can't constrain resource usage. We shouldn't have been able to schedule this in the first place.")
	}

	binds, err := d.containerBinds(ctx.AllocDir, task)
	if err != nil {
		return c, err
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

	mode, ok := task.Config["network_mode"]
	if !ok || mode == "" {
		// docker default
		d.logger.Printf("[WARN] driver.docker: no mode specified for networking, defaulting to bridge")
		mode = "bridge"
	}

	// Ignore the container mode for now
	switch mode {
	case "default", "bridge", "none", "host":
		d.logger.Printf("[DEBUG] driver.docker: using %s as network mode", mode)
	default:
		d.logger.Printf("[ERR] driver.docker: invalid setting for network mode: %s", mode)
		return c, fmt.Errorf("Invalid setting for network mode: %s", mode)
	}
	hostConfig.NetworkMode = mode

	// Handle the privileged flag
	privileged, ok := task.Config["privileged"]
	if !ok || privileged == "" {
		d.logger.Printf("[WARN] driver.docker: privileged flag not set, defaulting to non-privileged")
		privileged = "false"
	}

	parsed_privileged, err := strconv.ParseBool(privileged)
	if err != nil {
		d.logger.Printf("[ERR] driver.docker: invalid value found for privileged flag: %t", parsed_privileged)
		return c, fmt.Errorf("invalid value found for privileged flag: %t", parsed_privileged)
	}
	hostConfig.Privileged = parsed_privileged

	// Setup port mapping (equivalent to -p on docker CLI). Ports must already be
	// exposed in the container.
	if len(task.Resources.Networks) == 0 {
		d.logger.Print("[WARN] driver.docker: No networks are available for port mapping")
	} else {
		network := task.Resources.Networks[0]
		dockerPorts := map[docker.Port][]docker.PortBinding{}

		for _, port := range network.ListStaticPorts() {
			dockerPorts[docker.Port(strconv.Itoa(port)+"/tcp")] = []docker.PortBinding{docker.PortBinding{HostIP: network.IP, HostPort: strconv.Itoa(port)}}
			dockerPorts[docker.Port(strconv.Itoa(port)+"/udp")] = []docker.PortBinding{docker.PortBinding{HostIP: network.IP, HostPort: strconv.Itoa(port)}}
			d.logger.Printf("[DEBUG] driver.docker: allocated port %s:%d -> %d (static)\n", network.IP, port, port)
		}

		for label, port := range network.MapDynamicPorts() {
			// If the label is numeric we expect that there is a service
			// listening on that port inside the container. In this case we'll
			// setup a mapping from our random host port to the label port.
			//
			// Otherwise we'll setup a direct 1:1 mapping from the host port to
			// the container, and assume that the process inside will read the
			// environment variable and bind to the correct port.
			if _, err := strconv.Atoi(label); err == nil {
				dockerPorts[docker.Port(label+"/tcp")] = []docker.PortBinding{docker.PortBinding{HostIP: network.IP, HostPort: strconv.Itoa(port)}}
				dockerPorts[docker.Port(label+"/udp")] = []docker.PortBinding{docker.PortBinding{HostIP: network.IP, HostPort: strconv.Itoa(port)}}
				d.logger.Printf("[DEBUG] driver.docker: allocated port %s:%d -> %s (mapped)", network.IP, port, label)
			} else {
				dockerPorts[docker.Port(strconv.Itoa(port)+"/tcp")] = []docker.PortBinding{docker.PortBinding{HostIP: network.IP, HostPort: strconv.Itoa(port)}}
				dockerPorts[docker.Port(strconv.Itoa(port)+"/udp")] = []docker.PortBinding{docker.PortBinding{HostIP: network.IP, HostPort: strconv.Itoa(port)}}
				d.logger.Printf("[DEBUG] driver.docker: allocated port %s:%d -> %d for label %s\n", network.IP, port, port, label)
			}
		}
		hostConfig.PortBindings = dockerPorts
	}

	// Create environment variables.
	env := TaskEnvironmentVariables(ctx, task)
	env.SetAllocDir(filepath.Join("/", allocdir.SharedAllocName))
	env.SetTaskLocalDir(filepath.Join("/", allocdir.TaskLocal))

	config := &docker.Config{
		Env:   env.List(),
		Image: task.Config["image"],
	}

	rawArgs, hasArgs := task.Config["args"]
	parsedArgs, err := args.ParseAndReplace(rawArgs, env.Map())
	if err != nil {
		return c, err
	}

	// If the user specified a custom command to run, we'll inject it here.
	if command, ok := task.Config["command"]; ok {
		cmd := []string{command}
		if hasArgs {
			cmd = append(cmd, parsedArgs...)
		}
		config.Cmd = cmd
	} else if hasArgs {
		d.logger.Println("[DEBUG] driver.docker: ignoring args because command not specified")
	}

	return docker.CreateContainerOptions{
		Config:     config,
		HostConfig: hostConfig,
	}, nil
}

func (d *DockerDriver) Start(ctx *ExecContext, task *structs.Task) (DriverHandle, error) {
	// Get the image from config
	image, ok := task.Config["image"]
	if !ok || image == "" {
		return nil, fmt.Errorf("Image not specified")
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

	cleanupContainer, err := strconv.ParseBool(d.config.ReadDefault("docker.cleanup.container", "true"))
	if err != nil {
		return nil, fmt.Errorf("Unable to parse docker.cleanup.container: %s", err)
	}
	cleanupImage, err := strconv.ParseBool(d.config.ReadDefault("docker.cleanup.image", "true"))
	if err != nil {
		return nil, fmt.Errorf("Unable to parse docker.cleanup.image: %s", err)
	}

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
		// TODO add auth configuration for private repos
		authOptions := docker.AuthConfiguration{}
		err = client.PullImage(pullOptions, authOptions)
		if err != nil {
			d.logger.Printf("[ERR] driver.docker: pulling container %s", err)
			return nil, fmt.Errorf("Failed to pull `%s`: %s", image, err)
		}
		d.logger.Printf("[DEBUG] driver.docker: docker pull %s:%s succeeded", repo, tag)

		// Now that we have the image we can get the image id
		dockerImage, err = client.InspectImage(image)
		if err != nil {
			d.logger.Printf("[ERR] driver.docker: getting image id for %s", image)
			return nil, fmt.Errorf("Failed to determine image id for `%s`: %s", image, err)
		}
	}
	d.logger.Printf("[DEBUG] driver.docker: using image %s", dockerImage.ID)
	d.logger.Printf("[INFO] driver.docker: identified image %s as %s", image, dockerImage.ID)

	config, err := d.createContainer(ctx, task)
	if err != nil {
		d.logger.Printf("[ERR] driver.docker: %s", err)
		return nil, fmt.Errorf("Failed to create container config for image %s", image)
	}
	// Create a container
	container, err := client.CreateContainer(config)
	if err != nil {
		d.logger.Printf("[ERR] driver.docker: %s", err)
		return nil, fmt.Errorf("Failed to create container from image %s", image)
	}
	d.logger.Printf("[INFO] driver.docker: created container %s", container.ID)

	// Start the container
	err = client.StartContainer(container.ID, container.HostConfig)
	if err != nil {
		d.logger.Printf("[ERR] driver.docker: starting container %s", container.ID)
		return nil, fmt.Errorf("Failed to start container %s", container.ID)
	}
	d.logger.Printf("[INFO] driver.docker: started container %s", container.ID)

	// Return a driver handle
	h := &dockerHandle{
		client:           client,
		cleanupContainer: cleanupContainer,
		cleanupImage:     cleanupImage,
		logger:           d.logger,
		imageID:          dockerImage.ID,
		containerID:      container.ID,
		doneCh:           make(chan struct{}),
		waitCh:           make(chan error, 1),
	}
	go h.run()
	return h, nil
}

func (d *DockerDriver) Open(ctx *ExecContext, handleID string) (DriverHandle, error) {
	cleanupContainer, err := strconv.ParseBool(d.config.ReadDefault("docker.cleanup.container", "true"))
	if err != nil {
		return nil, fmt.Errorf("Unable to parse docker.cleanup.container: %s", err)
	}
	cleanupImage, err := strconv.ParseBool(d.config.ReadDefault("docker.cleanup.image", "true"))
	if err != nil {
		return nil, fmt.Errorf("Unable to parse docker.cleanup.image: %s", err)
	}

	// Split the handle
	pidBytes := []byte(strings.TrimPrefix(handleID, "DOCKER:"))
	pid := &dockerPID{}
	err = json.Unmarshal(pidBytes, pid)
	if err != nil {
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
	h := &dockerHandle{
		client:           client,
		cleanupContainer: cleanupContainer,
		cleanupImage:     cleanupImage,
		logger:           d.logger,
		imageID:          pid.ImageID,
		containerID:      pid.ContainerID,
		doneCh:           make(chan struct{}),
		waitCh:           make(chan error, 1),
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
		h.logger.Printf("[ERR] driver.docker: failed to marshal docker PID to JSON: %s", err)
	}
	return fmt.Sprintf("DOCKER:%s", string(data))
}

func (h *dockerHandle) WaitCh() chan error {
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
		log.Printf("[ERR] driver.docker: failed stopping container %s", h.containerID)
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
			log.Printf("[ERR] driver.docker: removing container %s", h.containerID)
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
				log.Printf("[INFO] driver.docker: image %s is still in use by %d containers", h.imageID, inUse)
			} else {
				return fmt.Errorf("Failed to remove image %s", h.imageID)
			}
		} else {
			log.Printf("[INFO] driver.docker: removed image %s", h.imageID)
		}
	}
	return nil
}

func (h *dockerHandle) run() {
	// Wait for it...
	exitCode, err := h.client.WaitContainer(h.containerID)
	if err != nil {
		h.logger.Printf("[ERR] driver.docker: unable to wait for %s; container already terminated", h.containerID)
	}

	if exitCode != 0 {
		err = fmt.Errorf("Docker container exited with non-zero exit code: %d", exitCode)
	}

	close(h.doneCh)
	if err != nil {
		h.waitCh <- err
	}
	close(h.waitCh)
}
