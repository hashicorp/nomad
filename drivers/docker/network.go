package docker

import (
	"fmt"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/nomad/plugins/drivers"
)

// infraContainerImage is the image used for the parent namespace container
const infraContainerImage = "gcr.io/google_containers/pause-amd64:3.0"

// dockerNetSpecLabelKey is used when creating a parent container for
// shared networking. It is a label whos value identifies the container ID of
// the parent container so tasks can configure their network mode accordingly
const dockerNetSpecLabelKey = "docker_sandbox_container_id"

func (d *Driver) CreateNetwork(allocID string) (*drivers.NetworkIsolationSpec, error) {
	// Initialize docker API clients
	client, _, err := d.dockerClients()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to docker daemon: %s", err)
	}

	repo, _ := parseDockerImage(infraContainerImage)
	authOptions, err := firstValidAuth(repo, []authBackend{
		authFromDockerConfig(d.config.Auth.Config),
		authFromHelper(d.config.Auth.Helper),
	})
	if err != nil {
		d.logger.Debug("auth failed for infra container image pull", "image", infraContainerImage, "error", err)
	}
	_, err = d.coordinator.PullImage(infraContainerImage, authOptions, allocID, noopLogEventFn)
	if err != nil {
		return nil, err
	}

	config, err := d.createSandboxContainerConfig(allocID)
	if err != nil {
		return nil, err
	}

	container, err := d.createContainer(client, *config, infraContainerImage)
	if err != nil {
		return nil, err
	}

	if err := d.startContainer(container); err != nil {
		return nil, err
	}

	c, err := client.InspectContainer(container.ID)
	if err != nil {
		return nil, err
	}

	return &drivers.NetworkIsolationSpec{
		Mode: drivers.NetIsolationModeGroup,
		Path: c.NetworkSettings.SandboxKey,
		Labels: map[string]string{
			dockerNetSpecLabelKey: c.ID,
		},
	}, nil
}

func (d *Driver) DestroyNetwork(allocID string, spec *drivers.NetworkIsolationSpec) error {
	client, _, err := d.dockerClients()
	if err != nil {
		return fmt.Errorf("failed to connect to docker daemon: %s", err)
	}

	return client.RemoveContainer(docker.RemoveContainerOptions{
		Force: true,
		ID:    spec.Labels[dockerNetSpecLabelKey],
	})
}

// createSandboxContainerConfig creates a docker container configuration which
// starts a container with an empty network namespace
func (d *Driver) createSandboxContainerConfig(allocID string) (*docker.CreateContainerOptions, error) {

	return &docker.CreateContainerOptions{
		Name: fmt.Sprintf("nomad_init_%s", allocID),
		Config: &docker.Config{
			Image: infraContainerImage,
		},
		HostConfig: &docker.HostConfig{
			// set the network mode to none which creates a network namespace with
			// only a loopback interface
			NetworkMode: "none",
		},
	}, nil
}
