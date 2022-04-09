package docker

import (
	"fmt"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/nomad/plugins/drivers"
)

const (
	// dockerNetSpecLabelKey is the label added when we create a pause
	// container to own the network namespace, and the NetworkIsolationSpec we
	// get back from CreateNetwork has this label set as the container ID.
	// We'll use this to generate a hostname for the task in the event the user
	// did not specify a custom one. Please see dockerNetSpecHostnameKey.
	dockerNetSpecLabelKey = "docker_sandbox_container_id"

	// dockerNetSpecHostnameKey is the label added when we create a pause
	// container and the task group network include a user supplied hostname
	// parameter.
	dockerNetSpecHostnameKey = "docker_sandbox_hostname"
)

func (d *Driver) CreateNetwork(allocID string, createSpec *drivers.NetworkCreateRequest) (*drivers.NetworkIsolationSpec, bool, error) {
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

	config, err := d.createSandboxContainerConfig(allocID, createSpec)
	if err != nil {
		return nil, false, err
	}

	specFromContainer := func(c *docker.Container, hostname string) *drivers.NetworkIsolationSpec {
		spec := &drivers.NetworkIsolationSpec{
			Mode: drivers.NetIsolationModeGroup,
			Path: c.NetworkSettings.SandboxKey,
			Labels: map[string]string{
				dockerNetSpecLabelKey: c.ID,
			},
		}

		// If the user supplied a hostname, set the label.
		if hostname != "" {
			spec.Labels[dockerNetSpecHostnameKey] = hostname
		}

		return spec
	}

	// We want to return a flag that tells us if the container already
	// existed so that callers can decide whether or not to recreate
	// the task's network namespace associations.
	container, err := d.containerByName(config.Name)
	if err != nil {
		return nil, false, err
	}
	if container != nil && container.State.Running {
		return specFromContainer(container, createSpec.Hostname), false, nil
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

	return specFromContainer(container, createSpec.Hostname), true, nil
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
// starts a container with an empty network namespace.
func (d *Driver) createSandboxContainerConfig(allocID string, createSpec *drivers.NetworkCreateRequest) (*docker.CreateContainerOptions, error) {

	return &docker.CreateContainerOptions{
		Name: fmt.Sprintf("nomad_init_%s", allocID),
		Config: &docker.Config{
			Image:    d.config.InfraImage,
			Hostname: createSpec.Hostname,
		},
		HostConfig: &docker.HostConfig{
			// Set the network mode to none which creates a network namespace
			// with only a loopback interface.
			NetworkMode: "none",
		},
	}, nil
}
