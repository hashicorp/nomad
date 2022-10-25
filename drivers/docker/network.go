package docker

import (
	"fmt"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/shoenig/netlog"
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

func networkName(allocID string) string {
	return "nomad_" + allocID[0:8]
}

func (d *Driver) CreateNetwork(allocID string, createSpec *drivers.NetworkCreateRequest) (*drivers.NetworkIsolationSpec, bool, error) {
	dockerClient, _, err := d.dockerClients()
	if err != nil {
		return nil, false, fmt.Errorf("failed to connect to docker daemon: %w", err)
	}

	netlog.Yellow("DD.CreateNetwork", "alloc_id", allocID, "hostname", createSpec.Hostname)

	network, err := dockerClient.CreateNetwork(docker.CreateNetworkOptions{
		Name:       networkName(allocID),
		Driver:     "bridge",
		Scope:      "",
		IPAM:       nil,
		ConfigFrom: nil,
		Options:    nil,
		Labels: map[string]string{
			dockerNetSpecLabelKey: allocID,
		},
		CheckDuplicate: false,
		Internal:       false,
		EnableIPv6:     false,
		Attachable:     true,
		ConfigOnly:     false,
		Ingress:        false,
		Context:        nil,
	})
	if err != nil {
		return nil, false, fmt.Errorf("failed to create network via docker api: %w", err)
	}

	netlog.Yellow("DD.CreateNetwork (after)", "name", networkName(allocID), "network", fmt.Sprintf("%#v", network))

	return &drivers.NetworkIsolationSpec{
		Mode: drivers.NetIsolationModeGroup,
		Path: "",
		Labels: map[string]string{
			dockerNetSpecLabelKey: allocID,
			"docker_network_id":   network.ID,
		},
		HostsConfig: nil,
	}, false, nil

	//// Initialize docker API clients
	//client, _, err := d.dockerClients()
	//if err != nil {
	//	return nil, false, fmt.Errorf("failed to connect to docker daemon: %s", err)
	//}
	//
	//if err := d.pullInfraImage(allocID); err != nil {
	//	return nil, false, err
	//}
	//
	//config, err := d.createSandboxContainerConfig(allocID, createSpec)
	//if err != nil {
	//	return nil, false, err
	//}
	//
	//specFromContainer := func(c *docker.Container, hostname string) *drivers.NetworkIsolationSpec {
	//	spec := &drivers.NetworkIsolationSpec{
	//		Mode: drivers.NetIsolationModeGroup,
	//		Path: c.NetworkSettings.SandboxKey,
	//		Labels: map[string]string{
	//			dockerNetSpecLabelKey: c.ID,
	//		},
	//	}
	//
	//	// If the user supplied a hostname, set the label.
	//	if hostname != "" {
	//		spec.Labels[dockerNetSpecHostnameKey] = hostname
	//	}
	//
	//	return spec
	//}
	//
	//// We want to return a flag that tells us if the container already
	//// existed so that callers can decide whether or not to recreate
	//// the task's network namespace associations.
	//container, err := d.containerByName(config.Name)
	//if err != nil {
	//	return nil, false, err
	//}
	//if container != nil && container.State.Running {
	//	return specFromContainer(container, createSpec.Hostname), false, nil
	//}
	//
	//container, err = d.createContainer(client, *config, d.config.InfraImage)
	//if err != nil {
	//	return nil, false, err
	//}
	//
	//if err = d.startContainer(container); err != nil {
	//	return nil, false, err
	//}
	//
	//// until the container is started, InspectContainerWithOptions
	//// returns a mostly-empty struct
	//container, err = client.InspectContainerWithOptions(docker.InspectContainerOptions{
	//	ID: container.ID,
	//})
	//if err != nil {
	//	return nil, false, err
	//}
	//
	//return specFromContainer(container, createSpec.Hostname), true, nil
}

func (d *Driver) DestroyNetwork(allocID string, spec *drivers.NetworkIsolationSpec) error {

	netlog.Yellow("DD.DestroyNetwork", "name", networkName(allocID), "alloc_id", allocID)

	c, _, err := d.dockerClients()
	if err != nil {
		return fmt.Errorf("failed to connect to docker daemon: %s", err)
	}

	return c.RemoveNetwork(spec.Labels["docker_network_id"])

	//
	//if err := client.RemoveContainer(docker.RemoveContainerOptions{
	//	Force: true,
	//	ID:    spec.Labels[dockerNetSpecLabelKey],
	//}); err != nil {
	//	return err
	//}
	//
	//if d.config.GC.Image {
	//
	//	// The Docker image ID is needed in order to correctly update the image
	//	// reference count. Any error finding this, however, should not result
	//	// in an error shutting down the allocrunner.
	//	dockerImage, err := client.InspectImage(d.config.InfraImage)
	//	if err != nil {
	//		d.logger.Warn("InspectImage failed for infra_image container destroy",
	//			"image", d.config.InfraImage, "error", err)
	//		return nil
	//	}
	//	d.coordinator.RemoveImage(dockerImage.ID, allocID)
	//}
	//
	//return nil
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

// pullInfraImage conditionally pulls the `infra_image` from the Docker registry
// only if its name uses the "latest" tag or the image doesn't already exist locally.
func (d *Driver) pullInfraImage(allocID string) error {
	repo, tag := parseDockerImage(d.config.InfraImage)

	// There's a (narrow) time-of-check-time-of-use race here. If we call
	// InspectImage and then a concurrent task shutdown happens before we call
	// IncrementImageReference, we could end up removing the image, and it
	// would no longer exist by the time we get to PullImage below.
	d.coordinator.imageLock.Lock()

	if tag != "latest" {
		dockerImage, err := client.InspectImage(d.config.InfraImage)
		if err != nil {
			d.logger.Debug("InspectImage failed for infra_image container pull",
				"image", d.config.InfraImage, "error", err)
		} else if dockerImage != nil {
			// Image exists, so no pull is attempted; just increment its reference
			// count and unlock the image lock.
			d.coordinator.incrementImageReferenceImpl(dockerImage.ID, d.config.InfraImage, allocID)
			d.coordinator.imageLock.Unlock()
			return nil
		}
	}

	// At this point we have performed all the image work needed, so unlock. It
	// is possible in environments with slow networks that the image pull may
	// take a while, so while defer unlock would be best, this allows us to
	// remove the lock sooner.
	d.coordinator.imageLock.Unlock()

	authOptions, err := firstValidAuth(repo, []authBackend{
		authFromDockerConfig(d.config.Auth.Config),
		authFromHelper(d.config.Auth.Helper),
	})
	if err != nil {
		d.logger.Debug("auth failed for infra_image container pull", "image", d.config.InfraImage, "error", err)
	}

	_, err = d.coordinator.PullImage(d.config.InfraImage, authOptions, allocID, noopLogEventFn, d.config.infraImagePullTimeoutDuration, d.config.pullActivityTimeoutDuration)
	return err
}
