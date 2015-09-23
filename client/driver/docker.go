package driver

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strings"

	docker "github.com/fsouza/go-dockerclient"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

var (
	reDockerVersion = regexp.MustCompile("Docker version ([\\d\\.]+),.+")
	reDockerSha     = regexp.MustCompile("^[a-f0-9]{64}$")
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

// containerOptionsForTask initializes a struct needed to call
// docker.client.CreateContainer()
func containerOptionsForTask(task *structs.Task, logger *log.Logger) docker.CreateContainerOptions {
	if task.Resources == nil {
		panic("task.Resources is nil and we can't constrain resource usage. We shouldn't have been able to schedule this in the first place.")
	}

	containerConfig := &docker.HostConfig{
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

	logger.Printf("[DEBUG] driver.docker: using %d bytes memory for %s", containerConfig.Memory, task.Config["image"])
	logger.Printf("[DEBUG] driver.docker: using %d cpu shares for %s", containerConfig.CPUShares, task.Config["image"])

	return docker.CreateContainerOptions{
		Config: &docker.Config{
			Image: task.Config["image"],
		},
		HostConfig: containerConfig,
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

	// Download the image
	pull, err := exec.Command("docker", "pull", image).CombinedOutput()
	if err != nil {
		d.logger.Printf("[ERR] driver.docker: pulling container %s", pull)
		return nil, fmt.Errorf("Failed to pull `%s`: %s", image, err)
	}
	d.logger.Printf("[DEBUG] driver.docker: docker pull %s:\n%s", image, pull)

	// Get the image ID (sha256). We need to keep track of this in case another
	// process pulls down a newer version of the image.
	imageIDBytes, err := exec.Command("docker", "images", "-q", "--no-trunc", image).CombinedOutput()
	imageID := strings.TrimSpace(string(imageIDBytes))
	if err != nil || imageID == "" {
		d.logger.Printf("[ERR] driver.docker: getting image id %s", imageID)
		return nil, fmt.Errorf("Failed to determine image id for `%s`: %s", image, err)
	}
	if !reDockerSha.MatchString(imageID) {
		return nil, fmt.Errorf("Image id not in expected format (sha256); found %s", imageID)
	}
	d.logger.Printf("[DEBUG] driver.docker: using image %s", imageID)
	d.logger.Printf("[INFO] driver.docker: downloaded image %s as %s", image, imageID)

	// Create a container
	container, err := client.CreateContainer(containerOptionsForTask(task, d.logger))
	if err != nil {
		d.logger.Printf("[ERR] driver.docker: %s", err)
		return nil, fmt.Errorf("Failed to create container from image %s", image)
	}
	if !reDockerSha.MatchString(container.ID) {
		return nil, fmt.Errorf("Container id not in expected format (sha256); found %s", container.ID)
	}
	d.logger.Printf("[INFO] driver.docker: created container %s", container.ID)

	// Start the container
	startBytes, err := exec.Command("docker", "start", container.ID).CombinedOutput()
	if err != nil {
		d.logger.Printf("[ERR] driver.docker: starting container %s", strings.TrimSpace(string(startBytes)))
		return nil, fmt.Errorf("Failed to start container %s", container.ID)
	}
	d.logger.Printf("[INFO] driver.docker: started container %s", container.ID)

	// Return a driver handle
	h := &dockerHandle{
		logger:      d.logger,
		imageID:     imageID,
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
