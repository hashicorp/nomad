// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1
// TODOS:
// - What happened if the dependee job is cancelled or stopped?

package dependency

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/nomad/dependency/loop_detection"
	"github.com/hashicorp/nomad/nomad/structs"
	sstructs "github.com/hashicorp/nomad/scheduler/structs"
)

var errDependencyTimeout = errors.New("dependency timeout reached")

type evalID = string

type evalUnblocker interface {
	Unblock(computedClass string, index uint64) chan struct{}
}

type loopDetector interface {
	AddNodes(dependantJob string, dependeeJob ...string) error
	RemoveNode(dependantJob string) error
	CreatesCircularDependency(dependantJob string, dependeeJob ...string) bool
}

// This function is a raft apply, to use it here is possible only because the coordinator
// is supposed to run exclusively on the leader, allowing for the log to be applied to the FSM.
type evalUpdaterFunc func(t structs.MessageType, msg any) (any, uint64, error)

type dependency struct {
	//cancelFunc context.CancelFunc
	job       *structs.Job
	dependees []string
}

type Coordinator struct {
	mainContext   context.Context
	cancelFunc    context.CancelFunc
	logger        hclog.Logger
	lDependencies sync.RWMutex
	dependencies  map[evalID]*dependency

	loopDetector loopDetector
}

// NewCoordinator creates a new dependency coordinator. The coordinator is
// responsible for tracking dependencies between jobs and evaluations.
//
//	It will block evaluations until their dependencies are met or a timeout is
//
// reached. The coordinator will also update jobs that have unmet dependencies
// after the timeout is reached.
func NewCoordinator(logger hclog.Logger, loopDetector loopDetector,
	blockedEvals evalUnblocker, jobUpdater evalUpdaterFunc) *Coordinator {

	ctx, cancel := context.WithCancel(context.Background())
	return &Coordinator{
		mainContext:  ctx,
		cancelFunc:   cancel,
		logger:       logger.Named("dependency-coordinator"),
		dependencies: make(map[evalID]*dependency),
		loopDetector: loopDetector,
	}
}

func (c *Coordinator) removeDeps(dependeeJobs map[string][]*structs.Allocation) error {

	c.lDependencies.Lock()
	defer c.lDependencies.Unlock()

	for jobID := range dependeeJobs {

		if err := c.loopDetector.RemoveNode(jobID); err != nil {
			c.logger.Error("failed to remove dependency", "error", err)
		}
	}

	return nil
}

func (c *Coordinator) AddDependency(state sstructs.State, job *structs.Job,
	eval *structs.Evaluation) ([]string, error) {

	if eval == nil || job == nil || state == nil {
		return []string{}, nil
	}

	if job.Dependencies == nil {
		return []string{}, nil
	}

	numDeps := len(job.Dependencies.Jobs)

	djNames := make([]string, 0, numDeps)
	djs := make(map[string][]*structs.Allocation, numDeps)

	for _, depJob := range job.Dependencies.Jobs {
		if depJob == nil || depJob.Name == "" {
			continue
		}

		if _, ok := djs[depJob.Name]; ok {
			continue
		}

		dependantAllocs, err := state.AllocsByJob(nil, job.Namespace, depJob.Name, true)
		if err != nil {
			c.logger.Error("failed to get dependency job", "job_id", depJob.Name,
				"error", err)
			continue
		}

		// dependentJob may be nil in cases where the dependent job has not been
		// scheduled yet. That's intentional: the presence of the key means
		// this dependency has already been processed.
		djs[depJob.Name] = dependantAllocs
		djNames = append(djNames, depJob.Name)
	}

	blockers, err := verifyDependencies(job, djs)
	if err != nil {
		c.logger.Error("failed to verify dependencies", "error", err)
	}

	if len(blockers) == 0 {
		return []string{}, nil
	}

	err = c.loopDetector.AddNodes(eval.JobID, djNames...)
	if err != nil {
		return []string{}, err
	}

	c.lDependencies.Lock()
	if _, exists := c.dependencies[eval.ID]; exists {
		c.lDependencies.Unlock()
		return blockers, nil
	}

	c.dependencies[eval.ID] = &dependency{
		job:       job,
		dependees: djNames,
	}

	c.lDependencies.Unlock()

	return blockers, nil
}

func (c *Coordinator) VerifyDependencies(ctx context.Context, state sstructs.State,
	eval *structs.Evaluation) ([]string, error) {

	c.lDependencies.RLock()
	dep := c.dependencies[eval.ID]
	c.lDependencies.RUnlock()

	blockers, err := VerifyDependencies(state, dep.job)
	if err != nil {
		return []string{}, err
	}

	if len(blockers) == 0 {
		c.logger.Debug("dependency ready, unblocking job", "job", eval.JobID,
			"eval", eval.ID, "ready", len(blockers) > 0)
		c.lDependencies.Lock()
		delete(c.dependencies, eval.ID)
		c.lDependencies.Unlock()

		return []string{}, nil

	}

	return blockers, nil
}

func VerifyDependencies(state sstructs.State, dependantJob *structs.Job) ([]string, error) {

	if dependantJob.Dependencies == nil {
		return []string{}, nil
	}

	numDeps := len(dependantJob.Dependencies.Jobs)

	djNames := make([]string, 0, numDeps)
	djs := make(map[string][]*structs.Allocation, numDeps)

	for _, depJob := range dependantJob.Dependencies.Jobs {
		if depJob == nil || depJob.Name == "" {
			continue
		}

		if _, ok := djs[depJob.Name]; ok {
			continue
		}

		dependantAllocs, err := state.AllocsByJob(nil, dependantJob.Namespace, depJob.Name, true)
		if err != nil {
			return []string{}, fmt.Errorf("unable to get alloc for dependency check: %w", err)
		}

		// dependentJob may be nil in cases where the dependent job has not been
		// scheduled yet. That's intentional: the presence of the key means
		// this dependency has already been processed.
		djs[depJob.Name] = dependantAllocs
		djNames = append(djNames, depJob.Name)
	}

	return verifyDependencies(dependantJob, djs)
}

func verifyDependencies(dependantJob *structs.Job, jobsAllocs map[string][]*structs.Allocation) ([]string, error) {
	var mErr multierror.Error
	blockers := []string{}

	if dependantJob == nil || dependantJob.Dependencies == nil {
		return blockers, mErr.ErrorOrNil()
	}

	for _, dependencyJob := range dependantJob.Dependencies.Jobs {

		allocs, ok := jobsAllocs[dependencyJob.Name]
		if !ok {
			mErr.Errors = append(mErr.Errors, errors.New("unable to check dependency for job: "+dependencyJob.Name))
			return []string{}, &mErr
		}

		if !conditionsMatch(allocs, dependencyJob.Status) {
			blockers = append(blockers, dependencyJob.Name)
		}
	}

	return blockers, mErr.ErrorOrNil()
}

// Possible states of the dependency
//   - - Complete: (Batch/Sysbatch only) All expected allocations are complete
//   - - Running: (Batch/Sysbatch only) All expected allocations are running
//   - - Recovering: Some allocations are pending
//   - - Failed: All allocations are failed, lost, or unplaced
//   - - Lost: All allocations are unknown
func conditionsMatch(allocs []*structs.Allocation, expectedState string) bool {
	if len(allocs) == 0 {
		return false
	}

	result := true
	switch expectedState {
	case structs.JobDependencyComplete:
		for _, a := range allocs {
			result = result && a.ClientStatus == structs.AllocClientStatusComplete
		}

	case structs.JobDependencyRunning:
		for _, a := range allocs {
			result = result && a.ClientStatus == structs.AllocClientStatusRunning
		}

	case structs.JobDependencyRecovering:
		for _, a := range allocs {
			result = result && (a.ClientStatus == structs.AllocClientStatusRunning ||
				a.ClientStatus == structs.AllocClientStatusPending)
		}

	case structs.JobDependencyFailed:
		for _, a := range allocs {
			result = result && a.ClientStatus == structs.AllocClientStatusFailed
		}

	case structs.JobDependencyLost:
		for _, a := range allocs {
			result = result && a.ClientStatus == structs.AllocClientStatusUnknown
		}

	default:
		result = false
	}

	return result
}

func dependencyTimeout(job *structs.Job) time.Duration {
	timeout := structs.JobDependencyTimeoutDefault
	if job != nil && job.Dependencies != nil && job.Dependencies.Timeout != nil && *job.Dependencies.Timeout > 0 {
		timeout = *job.Dependencies.Timeout
	}

	return timeout
}

func (c *Coordinator) Stop() {
	c.mainContext.Done()
	c.cancelFunc()

}

func (c *Coordinator) HasActiveDependents(j *structs.Job) (bool, error) {
	err := c.loopDetector.RemoveNode(j.ID)
	if err != nil {
		if errors.Is(err, loop_detection.ErrNodeIsDependency) {
			return true, nil
		}

		if !errors.Is(err, loop_detection.ErrNodeNotFound) {
			return false, err
		}
	}

	return false, nil
}

func (c *Coordinator) Reload(state sstructs.State, evals memdb.ResultIterator) {
	for {
		raw := evals.Next()
		if raw == nil {
			break
		}

		eval, ok := raw.(*structs.Evaluation)
		if !ok {
			c.logger.Error("failed to cast evaluation")
			continue
		}

		job, err := state.JobByID(nil, eval.Namespace, eval.JobID)
		if err != nil {
			c.logger.Error("failed to get job by ID", "error", err)
			continue
		}
		_, err = c.AddDependency(state, job, eval)
		if err != nil {
			c.logger.Error("failed to check dependency", "error", err)
		}
	}
}

func (c *Coordinator) CreatesCircularDependency(job *structs.Job) bool {
	if job.Dependencies == nil {
		return false
	}

	numDeps := len(job.Dependencies.Jobs)
	djNames := make([]string, 0, numDeps)
	for _, depJob := range job.Dependencies.Jobs {
		djNames = append(djNames, depJob.Name)
	}

	return c.loopDetector.CreatesCircularDependency(job.Name, djNames...)
}

func NewNoOpCoordinator() *NoOpCoordinator {
	return &NoOpCoordinator{}
}

type NoOpCoordinator struct{}

func (c *NoOpCoordinator) HasActiveDependents(j *structs.Job) (bool, error) {
	return false, nil
}

func (c *NoOpCoordinator) CheckDependency(state sstructs.State, job *structs.Job,
	eval *structs.Evaluation) ([]string, error) {
	return []string{}, nil
}

func (c *NoOpCoordinator) CreatesCircularDependency(j *structs.Job) bool {
	return false
}
