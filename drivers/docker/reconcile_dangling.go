// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package docker

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-set"
)

// containerReconciler detects and kills unexpectedly running containers.
//
// Due to Docker architecture and network based communication, it is
// possible for Docker to start a container successfully, but have the
// creation API call fail with a network error.  containerReconciler
// scans for these untracked containers and kill them.
type containerReconciler struct {
	ctx       context.Context
	config    *ContainerGCConfig
	logger    hclog.Logger
	getClient func() (*docker.Client, error)

	isDriverHealthy   func() bool
	trackedContainers func() *set.Set[string]
	isNomadContainer  func(c docker.APIContainers) bool

	once sync.Once
}

func newReconciler(d *Driver) *containerReconciler {
	return &containerReconciler{
		ctx:       d.ctx,
		config:    &d.config.GC.DanglingContainers,
		getClient: d.getDockerClient,
		logger:    d.logger,

		isDriverHealthy:   func() bool { return d.previouslyDetected() && d.fingerprintSuccessful() },
		trackedContainers: d.trackedContainers,
		isNomadContainer:  isNomadContainer,
	}
}

func (r *containerReconciler) Start() {
	if !r.config.Enabled {
		r.logger.Debug("skipping dangling containers handling; is disabled")
		return
	}

	r.once.Do(func() {
		go r.removeDanglingContainersGoroutine()
	})
}

func (r *containerReconciler) removeDanglingContainersGoroutine() {
	period := r.config.period

	lastIterSucceeded := true

	// ensure that we wait for at least a period or creation timeout
	// for first container GC iteration
	// The initial period is a grace period for restore allocation
	// before a driver may kill containers launched by an earlier nomad
	// process.
	initialDelay := period
	if r.config.CreationGrace > initialDelay {
		initialDelay = r.config.CreationGrace
	}

	timer := time.NewTimer(initialDelay)
	for {
		select {
		case <-timer.C:
			if r.isDriverHealthy() {
				err := r.removeDanglingContainersIteration()
				if err != nil && lastIterSucceeded {
					r.logger.Warn("failed to remove dangling containers", "error", err)
				}
				lastIterSucceeded = (err == nil)
			}

			timer.Reset(period)
		case <-r.ctx.Done():
			return
		}
	}
}

func (r *containerReconciler) removeDanglingContainersIteration() error {
	cutoff := time.Now().Add(-r.config.CreationGrace)
	tracked := r.trackedContainers()
	untracked, err := r.untrackedContainers(tracked, cutoff)
	if err != nil {
		return fmt.Errorf("failed to find untracked containers: %v", err)
	}

	if untracked.Empty() {
		return nil
	}

	if r.config.DryRun {
		r.logger.Info("detected untracked containers", "container_ids", untracked)
		return nil
	}

	dockerClient, err := r.getClient()
	if err != nil {
		return err
	}

	for _, id := range untracked.Slice() {
		ctx, cancel := r.dockerAPIQueryContext()
		err := dockerClient.RemoveContainer(docker.RemoveContainerOptions{
			Context: ctx,
			ID:      id,
			Force:   true,
		})
		cancel()
		if err != nil {
			r.logger.Warn("failed to remove untracked container", "container_id", id, "error", err)
		} else {
			r.logger.Info("removed untracked container", "container_id", id)
		}
	}

	return nil
}

// untrackedContainers returns the ids of containers that suspected
// to have been started by Nomad but aren't tracked by this driver
func (r *containerReconciler) untrackedContainers(tracked *set.Set[string], cutoffTime time.Time) (*set.Set[string], error) {
	result := set.New[string](10)

	ctx, cancel := r.dockerAPIQueryContext()
	defer cancel()

	dockerClient, err := r.getClient()
	if err != nil {
		return nil, err
	}

	cc, err := dockerClient.ListContainers(docker.ListContainersOptions{
		Context: ctx,
		All:     false, // only reconcile running containers
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %v", err)
	}

	cutoff := cutoffTime.Unix()

	for _, c := range cc {
		if tracked.Contains(c.ID) {
			continue
		}

		if c.Created > cutoff {
			continue
		}

		if !r.isNomadContainer(c) {
			continue
		}

		result.Insert(c.ID)
	}
	return result, nil
}

// dockerAPIQueryTimeout returns a context for docker API response with an appropriate timeout
// to protect against wedged locked-up API call.
//
// We'll try hitting Docker API on subsequent iteration.
func (r *containerReconciler) dockerAPIQueryContext() (context.Context, context.CancelFunc) {
	// use a reasonable floor to avoid very small limit
	timeout := 30 * time.Second

	if timeout < r.config.period {
		timeout = r.config.period
	}

	return context.WithTimeout(context.Background(), timeout)
}

func isNomadContainer(c docker.APIContainers) bool {
	if _, ok := c.Labels[dockerLabelAllocID]; ok {
		return true
	}

	// pre-0.10 containers aren't tagged or labeled in any way,
	// so use cheap heuristic based on mount paths
	// before inspecting container details
	if !hasMount(c, "/alloc") ||
		!hasMount(c, "/local") ||
		!hasMount(c, "/secrets") ||
		!hasNomadName(c) {
		return false
	}

	return true
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

// trackedContainers returns the set of container IDs of containers that were
// started by Driver and are expected to be running. This includes both normal
// Task containers, as well as infra pause containers.
func (d *Driver) trackedContainers() *set.Set[string] {
	// collect the task containers
	ids := d.tasks.IDs()
	// now also accumulate pause containers
	return d.pauseContainers.union(ids)
}
