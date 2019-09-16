package docker

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	docker "github.com/fsouza/go-dockerclient"
)

func (d *Driver) removeDanglingContainersGoroutine() {
	if !d.config.GC.DanglingContainers.Enabled {
		d.logger.Debug("skipping dangling containers handling; is disabled")
		return
	}

	period := d.config.GC.DanglingContainers.period

	succeeded := true

	// ensure that we wait for at least a period or creation timeout
	// for first container GC iteration
	// The initial period is a grace period for restore allocation
	// before a driver may kill containers launched by an earlier nomad
	// process.
	initialDelay := period
	if d.config.GC.DanglingContainers.creationTimeout > initialDelay {
		initialDelay = d.config.GC.DanglingContainers.creationTimeout
	}

	timer := time.NewTimer(initialDelay)
	for {
		select {
		case <-timer.C:
			if d.previouslyDetected() && d.fingerprintSuccessful() {
				err := d.removeDanglingContainersIteration()
				if err != nil && succeeded {
					d.logger.Warn("failed to remove dangling containers", "error", err)
				}
				succeeded = (err == nil)
			}

			timer.Reset(period)
		case <-d.ctx.Done():
			return
		}
	}
}

func (d *Driver) removeDanglingContainersIteration() error {
	tracked := d.trackedContainers()
	untracked, err := d.untrackedContainers(tracked, d.config.GC.DanglingContainers.creationTimeout)
	if err != nil {
		return fmt.Errorf("failed to find untracked containers: %v", err)
	}

	for _, id := range untracked {
		d.logger.Info("removing untracked container", "container_id", id)
		err := client.RemoveContainer(docker.RemoveContainerOptions{
			ID:    id,
			Force: true,
		})
		if err != nil {
			d.logger.Warn("failed to remove untracked container", "container_id", id, "error", err)
		}
	}

	return nil
}

// untrackedContainers returns the ids of containers that suspected
// to have been started by Nomad but aren't tracked by this driver
func (d *Driver) untrackedContainers(tracked map[string]bool, creationTimeout time.Duration) ([]string, error) {
	result := []string{}

	cc, err := client.ListContainers(docker.ListContainersOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %v", err)
	}

	cutoff := time.Now().Add(-creationTimeout).Unix()

	for _, c := range cc {
		if tracked[c.ID] {
			continue
		}

		if c.Created > cutoff {
			continue
		}

		if !d.isNomadContainer(c) {
			continue
		}

		result = append(result, c.ID)
	}

	return result, nil
}

func (d *Driver) isNomadContainer(c docker.APIContainers) bool {
	if _, ok := c.Labels["com.hashicorp.nomad.alloc_id"]; ok {
		return true
	}

	// pre-0.10 containers aren't tagged or labeled in any way,
	// so use cheap heauristic based on mount paths
	// before inspecting container details
	if !hasMount(c, "/alloc") ||
		!hasMount(c, "/local") ||
		!hasMount(c, "/secrets") ||
		!hasNomadName(c) {
		return false
	}

	// double check before killing process
	ctx, cancel := context.WithTimeout(d.ctx, 20*time.Second)
	defer cancel()

	ci, err := client.InspectContainerWithContext(c.ID, ctx)
	if err != nil {
		return false
	}

	env := ci.Config.Env
	return hasEnvVar(env, "NOMAD_ALLOC_ID") &&
		hasEnvVar(env, "NOMAD_GROUP_NAME")
}

func hasMount(c docker.APIContainers, p string) bool {
	for _, m := range c.Mounts {
		if m.Destination == p {
			return true
		}
	}

	return false
}

var nomadContainerNamePattern = regexp.MustCompile(`\/.*-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

func hasNomadName(c docker.APIContainers) bool {
	for _, n := range c.Names {
		if nomadContainerNamePattern.MatchString(n) {
			return true
		}
	}

	return false
}

func hasEnvVar(vars []string, key string) bool {
	for _, v := range vars {
		if strings.HasPrefix(v, key+"=") {
			return true
		}
	}

	return false
}

func (d *Driver) trackedContainers() map[string]bool {
	d.tasks.lock.RLock()
	defer d.tasks.lock.RUnlock()

	r := make(map[string]bool, len(d.tasks.store))
	for _, h := range d.tasks.store {
		r[h.containerID] = true
	}

	return r
}
