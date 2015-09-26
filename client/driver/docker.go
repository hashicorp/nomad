package driver

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	docker "github.com/fsouza/go-dockerclient"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

var (
	reDockerVersion = regexp.MustCompile("Docker version ([\\d\\.]+),.+")
	reDockerSha     = regexp.MustCompile("^[a-f0-9]{64}$")
	reNumeric       = regexp.MustCompile("^[0-9]+$")
)

type DockerDriver struct {
	DriverContext
}

type dockerPID struct {
	ImageID     string
	ContainerID string
}

type dockerHandle struct {
	logger      *log.Logger
	imageID     string
	containerID string
	waitCh      chan error
	doneCh      chan struct{}
}

func NewDockerDriver(ctx *DriverContext) Driver {
	return &DockerDriver{*ctx}
}

func (d *DockerDriver) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	outBytes, err := exec.Command("docker", "-v").Output()
	out := strings.TrimSpace(string(outBytes))
	if err != nil {
		return false, nil
	}

	matches := reDockerVersion.FindStringSubmatch(out)
	if len(matches) != 2 {
		return false, fmt.Errorf("Unable to parse docker version string: %#v", matches)
	}

	node.Attributes["driver.docker"] = "true"
	node.Attributes["driver.docker.version"] = matches[1]

	return true, nil
}

// We have to call this when we create the container AND when we start it so
// we'll make a function.
func createHostConfig(task *structs.Task) *docker.HostConfig {
	// hostConfig holds options for the docker container that are unique to this
	// machine, such as resource limits and port mappings
	return &docker.HostConfig{
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
	}
}

// createContainer initializes a struct needed to call docker.client.CreateContainer()
func createContainer(ctx *ExecContext, task *structs.Task, logger *log.Logger) docker.CreateContainerOptions {
	if task.Resources == nil {
		panic("task.Resources is nil and we can't constrain resource usage. We shouldn't have been able to schedule this in the first place.")
	}

	hostConfig := createHostConfig(task)
	logger.Printf("[DEBUG] driver.docker: using %d bytes memory for %s", hostConfig.Memory, task.Config["image"])
	logger.Printf("[DEBUG] driver.docker: using %d cpu shares for %s", hostConfig.CPUShares, task.Config["image"])

	// Setup port mapping (equivalent to -p on docker CLI). Ports must already be
	// exposed in the container.
	if len(task.Resources.Networks) == 0 {
		logger.Print("[WARN] driver.docker: No networks are available for port mapping")
	} else {
		network := task.Resources.Networks[0]
		dockerPorts := map[docker.Port][]docker.PortBinding{}

		for _, port := range network.ListStaticPorts() {
			dockerPorts[docker.Port(strconv.Itoa(port)+"/tcp")] = []docker.PortBinding{docker.PortBinding{HostIP: network.IP, HostPort: strconv.Itoa(port)}}
			dockerPorts[docker.Port(strconv.Itoa(port)+"/udp")] = []docker.PortBinding{docker.PortBinding{HostIP: network.IP, HostPort: strconv.Itoa(port)}}
			logger.Printf("[DEBUG] driver.docker: allocated port %s:%d -> %d (static) %s\n", network.IP, port, port)
		}

		for label, port := range network.MapDynamicPorts() {
			// If the label is numeric we expect that there is a service
			// listening on that port inside the container. In this case we'll
			// setup a mapping from our random host port to the label port.
			//
			// Otherwise we'll setup a direct 1:1 mapping from the host port to
			// the container, and assume that the process inside will read the
			// environment variable and bind to the correct port.
			if reNumeric.MatchString(label) {
				dockerPorts[docker.Port(label+"/tcp")] = []docker.PortBinding{docker.PortBinding{HostIP: network.IP, HostPort: strconv.Itoa(port)}}
				dockerPorts[docker.Port(label+"/udp")] = []docker.PortBinding{docker.PortBinding{HostIP: network.IP, HostPort: strconv.Itoa(port)}}
				logger.Printf("[DEBUG] driver.docker: allocated port %s:%d -> %s (mapped)", network.IP, port, label)
			} else {
				dockerPorts[docker.Port(strconv.Itoa(port)+"/tcp")] = []docker.PortBinding{docker.PortBinding{HostIP: network.IP, HostPort: strconv.Itoa(port)}}
				dockerPorts[docker.Port(strconv.Itoa(port)+"/udp")] = []docker.PortBinding{docker.PortBinding{HostIP: network.IP, HostPort: strconv.Itoa(port)}}
				logger.Printf("[DEBUG] driver.docker: allocated port %s:%d -> %d for label %s\n", network.IP, port, port, label)
			}
		}
		hostConfig.PortBindings = dockerPorts
	}

	config := &docker.Config{
		Env:   PopulateEnvironment(ctx, task),
		Image: task.Config["image"],
	}

	// If the user specified a custom command to run, we'll inject it here.
	if command, ok := task.Config["command"]; ok {
		config.Cmd = strings.Split(command, " ")
	}

	return docker.CreateContainerOptions{
		Config:     config,
		HostConfig: hostConfig,
	}
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

	// Initialize docker API client
	dockerEndpoint := d.config.ReadDefault("docker.endpoint", "unix:///var/run/docker.sock")
	client, err := docker.NewClient(dockerEndpoint)
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to docker.endpoint (%s): %s", dockerEndpoint, err)
	}

	repo, tag := docker.ParseRepositoryTag(image)
	// Make sure tag is always explicitly set. We'll default to "latest" if it
	// isn't, which is the expected behavior.
	if tag == "" {
		tag = "latest"
	}

	var dockerImage *docker.Image
	// We're going to check whether the image is already downloaded. If the tag
	// is "latest" we have to check for a new version every time.
	if tag != "latest" {
		dockerImage, err = client.InspectImage(image)
	}

	// Download the image
	if dockerImage == nil {
		pullOptions := docker.PullImageOptions{
			Repository: repo,
			Tag:        tag,
		}
		// TODO add auth configuration
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

	// Sanity check
	if !reDockerSha.MatchString(dockerImage.ID) {
		return nil, fmt.Errorf("Image id not in expected format (sha256); found %s", dockerImage.ID)
	}

	d.logger.Printf("[DEBUG] driver.docker: using image %s", dockerImage.ID)
	d.logger.Printf("[INFO] driver.docker: identified image %s as %s", image, dockerImage.ID)

	// Create a container
	container, err := client.CreateContainer(createContainer(ctx, task, d.logger))
	if err != nil {
		d.logger.Printf("[ERR] driver.docker: %s", err)
		return nil, fmt.Errorf("Failed to create container from image %s", image)
	}
	// Sanity check
	if !reDockerSha.MatchString(container.ID) {
		return nil, fmt.Errorf("Container id not in expected format (sha256); found %s", container.ID)
	}
	d.logger.Printf("[INFO] driver.docker: created container %s", container.ID)

	// Start the container
	err = client.StartContainer(container.ID, createHostConfig(task))
	if err != nil {
		d.logger.Printf("[ERR] driver.docker: starting container %s", container.ID)
		return nil, fmt.Errorf("Failed to start container %s", container.ID)
	}
	d.logger.Printf("[INFO] driver.docker: started container %s", container.ID)

	// Return a driver handle
	h := &dockerHandle{
		logger:      d.logger,
		imageID:     dockerImage.ID,
		containerID: container.ID,
		doneCh:      make(chan struct{}),
		waitCh:      make(chan error, 1),
	}
	go h.run()
	return h, nil
}

func (d *DockerDriver) Open(ctx *ExecContext, handleID string) (DriverHandle, error) {
	// Split the handle
	pidBytes := []byte(strings.TrimPrefix(handleID, "DOCKER:"))
	pid := &dockerPID{}
	err := json.Unmarshal(pidBytes, pid)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse handle '%s': %v", handleID, err)
	}
	d.logger.Printf("[INFO] driver.docker: re-attaching to docker process: %s", handleID)

	// Look for a running container with this ID
	// docker ps does not return an exit code if there are no matching processes
	// so we have to read the output and compare it to our known containerID
	psBytes, err := exec.Command("docker", "ps", "-q", "--no-trunc",
		fmt.Sprintf("-f=id=%s", pid.ContainerID)).Output()
	ps := strings.TrimSpace(string(psBytes))
	if err != nil {
		return nil, fmt.Errorf("Failed to find container %s: %v", pid.ContainerID, err)
	} else if ps != pid.ContainerID {
		return nil, fmt.Errorf("Container ID does not match; expected %s found %s", pid.ContainerID, ps)
	}

	// Return a driver handle
	h := &dockerHandle{
		logger:      d.logger,
		imageID:     pid.ImageID,
		containerID: pid.ContainerID,
		doneCh:      make(chan struct{}),
		waitCh:      make(chan error, 1),
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
	stop, err := exec.Command("docker", "stop", "-t", "5", h.containerID).CombinedOutput()
	if err != nil {
		log.Printf("[ERR] driver.docker: stopping container %s", stop)
		return fmt.Errorf("Failed to stop container %s: %s", h.containerID, err)
	}
	log.Printf("[INFO] driver.docker: stopped container %s", h.containerID)

	// Cleanup container
	rmContainer, err := exec.Command("docker", "rm", h.containerID).CombinedOutput()
	if err != nil {
		log.Printf("[ERR] driver.docker: removing container %s", rmContainer)
		return fmt.Errorf("Failed to remove container %s: %s", h.containerID, err)
	}
	log.Printf("[INFO] driver.docker: removed container %s", h.containerID)

	// Cleanup image. This operation may fail if the image is in use by another
	// job. That is OK. Will we log a message but continue.
	_, err = exec.Command("docker", "rmi", h.imageID).CombinedOutput()
	if err != nil {
		log.Printf("[WARN] driver.docker: failed to remove image %s; it may still be in use", h.imageID)
	} else {
		log.Printf("[INFO] driver.docker: removed image %s", h.imageID)
	}
	return nil
}

func (h *dockerHandle) run() {
	// Wait for it...
	waitBytes, err := exec.Command("docker", "wait", h.containerID).Output()
	if err != nil {
		h.logger.Printf("[ERR] driver.docker: unable to wait for %s; container already terminated", h.containerID)
	}
	wait := strings.TrimSpace(string(waitBytes))

	// If the container failed, try to get the last 10 lines of logs for our
	// error message.
	if wait != "0" {
		var logsBytes []byte
		logsBytes, err = exec.Command("docker", "logs", "--tail=10", h.containerID).Output()
		logs := string(logsBytes)
		if err == nil {
			err = fmt.Errorf("%s", logs)
		}
	}

	close(h.doneCh)
	if err != nil {
		h.waitCh <- err
	}
	close(h.waitCh)
}
