package docker

import (
	"fmt"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/nomad/plugins/drivers"
)

// dockerNetSpecLabelKey is used when creating a parent container for
// shared networking. It is a label whos value identifies the container ID of
// the parent container so tasks can configure their network mode accordingly
const dockerNetSpecLabelKey = "docker_sandbox_container_id"

func (d *Driver) CreateNetwork(allocID string) (*drivers.NetworkIsolationSpec, bool, error) {
	// Initialize docker API clients
	client, _, err := d.dockerClients()
	if err != nil {
		return nil, false, fmt.Errorf("failed to connect to docker daemon: %s", err)
	}

	repo, _ := parseDockerImage(d.config.InfraImage)
	authOptions, err := firstValidAuth(repo, []authBackend{
		authFromDockerConfig(d.config.Auth.Config),
		authFromHelper(d.config.Auth.Helper),
	})
	if err != nil {
		d.logger.Debug("auth failed for infra container image pull", "image", d.config.InfraImage, "error", err)
	}
	_, err = d.coordinator.PullImage(d.config.InfraImage, authOptions, allocID, noopLogEventFn, d.config.infraImagePullTimeoutDuration, d.config.pullActivityTimeoutDuration)
	if err != nil {
		return nil, false, err
	}

	config, err := d.createSandboxContainerConfig(allocID)
	if err != nil {
		return nil, false, err
	}

	specFromContainer := func(c *docker.Container) *drivers.NetworkIsolationSpec {
		return &drivers.NetworkIsolationSpec{
			Mode: drivers.NetIsolationModeGroup,
			Path: c.NetworkSettings.SandboxKey,
			Labels: map[string]string{
				dockerNetSpecLabelKey: c.ID,
			},
		}
	}

	// We want to return a flag that tells us if the container already
	// existed so that callers can decide whether or not to recreate
	// the task's network namespace associations.
	container, err := d.containerByName(config.Name)
	if err != nil {
		return nil, false, err
	}
	if container != nil && container.State.Running {
		return specFromContainer(container), false, nil
	}

	container, err = d.createContainer(client, *config, d.config.InfraImage)
	if err != nil {
		return nil, false, err
	}

	if err = d.startContainer(container); err != nil {
		return nil, false, err
	}

	// until the container is started, InspectContainerWithOptions
	// returns a mostly-empty struct
	container, err = client.InspectContainerWithOptions(docker.InspectContainerOptions{
		ID: container.ID,
	})
	if err != nil {
		return nil, false, err
	}

	return specFromContainer(container), true, nil
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
			Image: d.config.InfraImage,
		},
		HostConfig: &docker.HostConfig{
			// set the network mode to none which creates a network namespace with
			// only a loopback interface
			NetworkMode: "none",
		},
	}, nil
}
