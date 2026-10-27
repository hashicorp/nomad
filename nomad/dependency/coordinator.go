// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1
// TODOS:
// - What happened if the dependee job is cancelled or stopped?

package dependency

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/nomad/dependency/loop_detection"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

var errDependencyTimeout = errors.New("dependency timeout reached")

// This is the jobID from the dependencies, not the job that has a dependency.
// Because unblocking a dependency is triggered by updates on allocations the
// most efficient way to look up jobs that might be unblocked by the update
// is storing them by the JobID of the updated alloc.
type blockingJobID = string
type blockedJobID = string

type loopDetector interface {
	AddNodes(dependantJob string, dependeeJob ...string) error
	RemoveNode(dependantJob string) error
	CreatesCircularDependency(dependantJob string, dependeeJob ...string) bool
}

type Coordinator struct {
	logger        hclog.Logger
	lDependencies sync.RWMutex

	// The value of the dependencies per blockingJob could be an array, but it
	// is a map to avoid storing duplicated jobs.
	dependencies map[blockingJobID]map[blockedJobID]*structs.Job

	loopDetector loopDetector
}

// NewCoordinator creates a new dependency coordinator. The coordinator is
// responsible for tracking dependencies between jobs and evaluations.
//
// The broker is designed to be entirely
// in-memory and is managed by the leader node.
func NewCoordinator(logger hclog.Logger, loopDetector loopDetector) *Coordinator {
	return &Coordinator{
		logger:       logger.Named("dependency-coordinator"),
		dependencies: make(map[blockingJobID]map[blockedJobID]*structs.Job),
		loopDetector: loopDetector,
	}
}

func (c *Coordinator) removeBlockingJob(jID blockingJobID) error {
	c.lDependencies.Lock()
	defer c.lDependencies.Unlock()
	delete(c.dependencies, jID)

	return nil
}

func (c *Coordinator) removeBlockedJob(blkingJobID blockingJobID, bledJobID blockedJobID) error {
	c.lDependencies.Lock()
	defer c.lDependencies.Unlock()

	// If the blocked job itself is blocking other jobs it can't be removed
	// from the loop detector, but it can be removed from the list of
	// blocked jobs for the job that is blocking it.
	if err := c.loopDetector.RemoveNode(bledJobID); err != nil {
		if errors.Is(err, loop_detection.ErrNodeIsDependency) {
			c.logger.Info("dependencies are blocked by job ", "job", bledJobID)
			return nil
		}

		c.logger.Error("failed to remove dependency", "error", err)
		return err
	}

	delete(c.dependencies[blkingJobID], bledJobID)

	return nil
}

func (c *Coordinator) AddDependency(state *state.StateStore, blockedJob *structs.Job) error {
	if blockedJob == nil || state == nil || blockedJob.Dependencies == nil ||
		len(blockedJob.Dependencies.Jobs) == 0 {
		return nil
	}

	numDeps := len(blockedJob.Dependencies.Jobs)
	blockingJobsIDs := make([]string, 0, numDeps)

	for _, blockingJob := range blockedJob.Dependencies.Jobs {
		if blockingJob == nil || blockingJob.Name == "" {
			continue
		}

		blockingJobsIDs = append(blockingJobsIDs, blockingJob.Name)
	}

	err := c.loopDetector.AddNodes(blockedJob.ID, blockingJobsIDs...)
	if err != nil {
		return err
	}

	c.lDependencies.Lock()
	for _, blingJobID := range blockingJobsIDs {
		if _, ok := c.dependencies[blingJobID][blockedJob.ID]; ok {
			c.logger.Warn("dependency for job overwritten", "jobID", blockedJob.ID)
		}

		if c.dependencies[blingJobID] == nil {
			c.dependencies[blingJobID] = make(map[blockedJobID]*structs.Job)
		}

		c.dependencies[blingJobID][blockedJob.ID] = blockedJob
	}
	c.lDependencies.Unlock()

	return nil
}

func (c *Coordinator) UnblockedDependencies(state *state.StateStore, bkingJobID string) ([]*structs.Evaluation, error) {
	evalsToUnblock := []*structs.Evaluation{}

	
	/* 	c.lDependencies.RLock()
	   	blockedJobs := c.dependencies[bkingJobID]
	   	c.lDependencies.RUnlock()

	   	for _, blockedJob := range blockedJobs {
	   		blockers, err := VerifyDependencies(state, blockedJob)
	   		if err != nil {
	   			return evalsToUnblock, err
	   		}

	   		if len(blockers) > 0 {
	   			return evalsToUnblock, nil
	   		}

	   		c.logger.Debug("dependency ready, unblocking job", "job", blockedJob.ID)

	   		if err := c.removeBlockedJob(bkingJobID, blockedJob.ID); err != nil {
	   			c.logger.Error("unable to unblock dependencies for job ", "jobID", blockedJob)
	   			continue
	   		}

	   		blockedJobEvals, err := state.EvalsByJob(nil, blockedJob.Namespace, blockedJob.ID)
	   		if err != nil {
	   			c.logger.Error("unable to look up evaluations for blocked job", "jobID", blockedJob)
	   			continue
	   		}

	   		for _, e := range blockedJobEvals {
	   			if e.Status == structs.EvalStatusBlocked && e.TriggeredBy == structs.EvalTriggerByDependencies {
	   				evalsToUnblock = append(evalsToUnblock, e)
	   			}
	   		}

	   	}

	   	if len(blockedJobs) == 0 {
	   		if err := c.removeBlockingJob(bkingJobID); err != nil {
	   			c.logger.Error("unable to stop monitoring job for blocked dependencies", "jobID", bkingJobID)
	   		}
	   	} */

	return evalsToUnblock, nil
}

type Storage interface {
	AllocsByJob(ws memdb.WatchSet, namespace, jobID string, anyCreateIndex bool) ([]*structs.Allocation, error)
}

func VerifyDependencies(state Storage, blockedJobs *structs.Job) ([]string, error) {

	if blockedJobs.Dependencies == nil {
		return []string{}, nil
	}

	numDeps := len(blockedJobs.Dependencies.Jobs)

	djNames := make([]string, 0, numDeps)
	djs := make(map[string][]*structs.Allocation, numDeps)

	for _, depJob := range blockedJobs.Dependencies.Jobs {
		if depJob == nil || depJob.Name == "" {
			continue
		}

		if _, ok := djs[depJob.Name]; ok {
			continue
		}

		dependantAllocs, err := state.AllocsByJob(nil, blockedJobs.Namespace, depJob.Name, true)
		if err != nil {
			return []string{}, fmt.Errorf("unable to get alloc for dependency check: %w", err)
		}

		// dependentJob may be nil in cases where the dependent job has not been
		// scheduled yet. That's intentional: the presence of the key means
		// this dependency has already been processed.
		djs[depJob.Name] = dependantAllocs
		djNames = append(djNames, depJob.Name)
	}

	return verifyDependencies(blockedJobs, djs)
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

func (c *Coordinator) Reload(state *state.StateStore, evals memdb.ResultIterator) {
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

		err = c.AddDependency(state, job)
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
