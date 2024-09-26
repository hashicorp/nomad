// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package docker

import (
	"fmt"

	"github.com/docker/docker/api/types"
	containerapi "github.com/docker/docker/api/types/container"
	"github.com/hashicorp/nomad/helper/pointer"
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
	dockerClient, err := d.getDockerClient()
	if err != nil {
		return nil, false, fmt.Errorf("failed to connect to docker daemon: %s", err)
	}

	if err := d.pullInfraImage(allocID); err != nil {
		return nil, false, err
	}

	config, err := d.createSandboxContainerConfig(allocID, createSpec)
	if err != nil {
		return nil, false, err
	}

	specFromContainer := func(c *types.ContainerJSON, hostname string) *drivers.NetworkIsolationSpec {
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

	container, err = d.createContainer(dockerClient, *config, d.config.InfraImage)
	if err != nil {
		return nil, false, err
	}

	if err = d.startContainer(*container); err != nil {
		return nil, false, err
	}

	// until the container is started, InspectContainerWithOptions
	// returns a mostly-empty struct
	*container, err = dockerClient.ContainerInspect(d.ctx, container.ID)
	if err != nil {
		return nil, false, err
	}

	// keep track of this pause container for reconciliation
	d.pauseContainers.add(container.ID)

	return specFromContainer(container, createSpec.Hostname), true, nil
}

func (d *Driver) DestroyNetwork(allocID string, spec *drivers.NetworkIsolationSpec) error {

	var (
		id  string
		err error
	)

	if spec != nil {
		// if we have the spec we can just read the container id
		id = spec.Labels[dockerNetSpecLabelKey]
	} else {
		// otherwise we need to scan all the containers and find the pause container
		// associated with this allocation - this happens when the client is
		// restarted since we do not persist the network spec
		id, err = d.findPauseContainer(allocID)
	}

	if err != nil {
		return err
	}

	if id == "" {
		d.logger.Debug("failed to find pause container to cleanup", "alloc_id", allocID)
		return nil
	}

	// no longer tracking this pause container; even if we fail here we should
	// let the background reconciliation keep trying
	d.pauseContainers.remove(id)

	dockerClient, err := d.getDockerClient()
	if err != nil {
		return fmt.Errorf("failed to connect to docker daemon: %s", err)
	}

	// this is the pause container, just kill it fast
	if err := dockerClient.ContainerStop(d.ctx, id, containerapi.StopOptions{Timeout: pointer.Of(1)}); err != nil {
		d.logger.Warn("failed to stop pause container", "id", id, "error", err)
	}

	if err := dockerClient.ContainerRemove(d.ctx, id, containerapi.RemoveOptions{
		Force: true,
	}); err != nil {
		return fmt.Errorf("failed to remove pause container: %w", err)
	}

	if d.config.GC.Image {

		// The Docker image ID is needed in order to correctly update the image
		// reference count. Any error finding this, however, should not result
		// in an error shutting down the allocrunner.
		dockerImage, _, err := dockerClient.ImageInspectWithRaw(d.ctx, d.config.InfraImage)
		if err != nil {
			d.logger.Warn("InspectImage failed for infra_image container destroy",
				"image", d.config.InfraImage, "error", err)
			return nil
		}
		d.coordinator.RemoveImage(dockerImage.ID, allocID)
	}

	return nil
}

// createSandboxContainerConfig creates a docker container configuration which
// starts a container with an empty network namespace.
func (d *Driver) createSandboxContainerConfig(allocID string, createSpec *drivers.NetworkCreateRequest) (*createContainerOptions, error) {
	return &createContainerOptions{
		Name: fmt.Sprintf("nomad_init_%s", allocID),
		Config: &containerapi.Config{
			Image:    d.config.InfraImage,
			Hostname: createSpec.Hostname,
			Labels: map[string]string{
				dockerLabelAllocID: allocID,
			},
		},
		Host: &containerapi.HostConfig{
			// Set the network mode to none which creates a network namespace
			// with only a loopback interface.
			NetworkMode: "none",

			// Set the restart policy to unless-stopped. The pause container should
			// never not be running until Nomad issues a stop.
			//
			// https://docs.docker.com/engine/reference/run/#restart-policies---restart
			RestartPolicy: containerapi.RestartPolicy{Name: containerapi.RestartPolicyUnlessStopped},
		},
	}, nil
}

// pullInfraImage conditionally pulls the `infra_image` from the Docker registry
// only if its name uses the "latest" tag or the image doesn't already exist locally.
func (d *Driver) pullInfraImage(allocID string) error {
	repo, tag := parseDockerImage(d.config.InfraImage)

	dockerClient, err := d.getDockerClient()
	if err != nil {
		return err
	}

	// There's a (narrow) time-of-check-time-of-use race here. If we call
	// InspectImage and then a concurrent task shutdown happens before we call
	// IncrementImageReference, we could end up removing the image, and it
	// would no longer exist by the time we get to PullImage below.
	d.coordinator.imageLock.Lock()

	if tag != "latest" {
		dockerImage, _, err := dockerClient.ImageInspectWithRaw(d.ctx, d.config.InfraImage)
		if err != nil {
			d.logger.Debug("InspectImage failed for infra_image container pull",
				"image", d.config.InfraImage, "error", err)
		} else if dockerImage.ID != "" {
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

	_, _, err = d.coordinator.PullImage(d.config.InfraImage, authOptions, allocID, noopLogEventFn, d.config.infraImagePullTimeoutDuration, d.config.pullActivityTimeoutDuration)
	return err
}
