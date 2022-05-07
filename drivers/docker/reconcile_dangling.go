package docker

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	hclog "github.com/hashicorp/go-hclog"
)

// containerReconciler detects and kills unexpectedly running containers.
//
// Due to Docker architecture and network based communication, it is
// possible for Docker to start a container successfully, but have the
// creation API call fail with a network error.  containerReconciler
// scans for these untracked containers and kill them.
type containerReconciler struct {
	ctx    context.Context
	config *ContainerGCConfig
	client *docker.Client
	logger hclog.Logger

	isDriverHealthy   func() bool
	trackedContainers func() map[string]bool
	isNomadContainer  func(c docker.APIContainers) bool

	once sync.Once
}

func newReconciler(d *Driver) *containerReconciler {
	return &containerReconciler{
		ctx:    d.ctx,
		config: &d.config.GC.DanglingContainers,
		client: client,
		logger: d.logger,

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

	if len(untracked) == 0 {
		return nil
	}

	if r.config.DryRun {
		r.logger.Info("detected untracked containers", "container_ids", untracked)
		return nil
	}

	for _, id := range untracked {
		ctx, cancel := r.dockerAPIQueryContext()
		err := client.RemoveContainer(docker.RemoveContainerOptions{
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
func (r *containerReconciler) untrackedContainers(tracked map[string]bool, cutoffTime time.Time) ([]string, error) {
	result := []string{}

	ctx, cancel := r.dockerAPIQueryContext()
	defer cancel()

	cc, err := client.ListContainers(docker.ListContainersOptions{
		Context: ctx,
		All:     false, // only reconcile running containers
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %v", err)
	}

	cutoff := cutoffTime.Unix()

	for _, c := range cc {
		if tracked[c.ID] {
			continue
		}

		if c.Created > cutoff {
			continue
		}

		if !r.isNomadContainer(c) {
			continue
		}

		result = append(result, c.ID)
	}

	return result, nil
}

// dockerAPIQueryTimeout returns a context for docker API response with an appropriate timeout
// to protect against wedged locked-up API call.
//
// We'll try hitting Docker API on subsequent iteration.
func (r *containerReconciler) dockerAPIQueryContext() (context.Context, context.CancelFunc) {
	// use a reasoanble floor to avoid very small limit
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

func (d *Driver) trackedContainers() map[string]bool {
	d.tasks.lock.RLock()
	defer d.tasks.lock.RUnlock()

	r := make(map[string]bool, len(d.tasks.store))
	for _, h := range d.tasks.store {
		r[h.containerID] = true
	}

	return r
}
