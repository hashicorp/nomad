// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1
// TODOS:
// - Add a way to cancel the evaluation and job when the dependency timeout is reached
// - There is a exception happening when the dependency timeout is reached and the job is dispatched. The job is dispatched but the evaluation is not removed from the dependencies map. This causes a memory leak and the evaluation will never be unblocked.
// - Error when unmarshalling a job with dependencies. The error is "json: cannot unmarshal object into Go struct field Job.Dependencies of type string". This is because the job dependencies are defined as a string in the job struct but the API returns an object. The job struct should be updated to match the API response.
package dependency

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/scheduler/loop_detection"
	sstructs "github.com/hashicorp/nomad/scheduler/structs"
)

var DefaultTimeout = 2 * time.Minute
var errDependencyTimeout = errors.New("dependency timeout reached")

type evalID = string

type evalUnblocker interface {
	Unblock(computedClass string, index uint64) chan struct{}
}

type loopDetector interface {
	AddNodes(dependantJob string, dependeeJob ...string) error
	RemoveNode(dependantJob string) error
}

type dependency struct {
	cancelFunc context.CancelFunc
	job        *structs.Job
	dependees  []string
}

type Coordinator struct {
	mainContext context.Context
	logger      hclog.Logger
	l           sync.RWMutex

	dependencies map[evalID]*dependency
	loopDetector loopDetector
	blockedEvals evalUnblocker
}

// NewCoordinator does blah blah blah
func NewCoordinator(logger hclog.Logger, loopDetector loopDetector,
	blockedEvals evalUnblocker) *Coordinator {
	return &Coordinator{
		mainContext:  context.Background(),
		logger:       logger.Named("dependency-coordinator"),
		dependencies: make(map[evalID]*dependency),
		loopDetector: loopDetector,
		blockedEvals: blockedEvals,
	}
}

func (c *Coordinator) unblockDependencies(eval *structs.Evaluation, dependeeJobs map[string]*structs.Job) error {
	for _, job := range dependeeJobs {

		c.blockedEvals.Unblock(eval.ID, job.JobModifyIndex)

		c.l.Lock()
		defer c.l.Unlock()

		if err := c.loopDetector.RemoveNode(eval.JobID); err != nil {
			c.logger.Error("failed to remove dependency", "error", err)
		}
	}

	return nil
}

func (c *Coordinator) CheckDependency(state sstructs.State, job *structs.Job, eval *structs.Evaluation) (bool, error) {

	if job.Dependencies == nil {
		return true, nil
	}

	djSet := map[string]struct{}{}
	for _, depJob := range job.Dependencies.Jobs {
		if depJob == nil || depJob.Name == "" {
			continue
		}

		djSet[depJob.Name] = struct{}{}
	}

	djIDs := make([]string, 0, len(djSet))
	for jobID := range djSet {
		djIDs = append(djIDs, jobID)
	}

	djs := map[string]*structs.Job{}
	for _, jID := range djIDs {
		j, err := state.JobByID(nil, job.Namespace, jID)
		if err != nil {
			c.logger.Error("failed to get job by ID", "error", err)
			continue
		}
		djs[jID] = j
	}

	ready, err := c.verifyDependencies(job, djs)
	if err != nil {
		c.logger.Error("failed to verify dependencies", "error", err)
	}

	if ready {
		return true, nil
	}

	c.loopDetector.AddNodes(eval.JobID, djIDs...)

	ctx, cancel := context.WithDeadlineCause(c.mainContext,
		time.Now().Add(dependencyTimeout(job)), errDependencyTimeout)
	c.dependencies[eval.ID] = &dependency{
		cancelFunc: cancel,
		job:        job,
		dependees:  djIDs,
	}

	go c.waitForDependency(ctx, state, eval, djIDs...)

	return false, nil
}

func (c *Coordinator) waitForDependency(ctx context.Context, state sstructs.State,
	eval *structs.Evaluation, dependeeJobIDs ...string) {
	dep := c.dependencies[eval.ID]
	defer func() {
		dep.cancelFunc()
		delete(c.dependencies, eval.ID)
		c.logger.Error(" this is running!! **** ")
	}()

	for {
		ws := memdb.NewWatchSet()
		dj := map[string]*structs.Job{}

		for _, jID := range dependeeJobIDs {
			j, err := state.JobByID(ws, eval.Namespace, jID)
			if err != nil {
				c.logger.Error("failed to get job by ID", "error", err)
			}

			dj[jID] = j
		}

		select {
		case <-ws.WatchCh(ctx):
			ready, err := c.verifyDependencies(dep.job, dj)
			if err != nil {
				c.logger.Error("failed to verify dependency", "error", err)
				continue
			}

			if ready {
				c.logger.Error("dependency ready, unblocking job", "job", eval.JobID, "eval", eval.ID, "ready", ready)
				err := c.unblockDependencies(eval, dj)
				if err != nil {
					c.logger.Error("failed to unblock job", "error", err)
				}
				return
			}

		case <-ctx.Done():
			c.logger.Error("dependency timeout reached", "job", eval.JobID, "eval", eval.ID)
			if context.Cause(ctx) == errDependencyTimeout &&
				dep.job.Dependencies.ActionOnTimeout == structs.DependencyActionDispatch {
				c.logger.Error("dependency timeout reached, dispatching job",
					"job", eval.JobID, "eval", eval.ID, "action", dep.job.Dependencies.ActionOnTimeout)
				c.unblockDependencies(eval, dj)
			}

			// TODO: Find a way to delete the eval and cancel the job
			break
		}
	}

}

func (c *Coordinator) verifyDependencies(dependantJob *structs.Job, jobs map[string]*structs.Job) (bool, error) {
	var mErr multierror.Error
	ready := true

	for _, depJob := range dependantJob.Dependencies.Jobs {
		if depJob == nil {
			continue
		}

		job, ok := jobs[depJob.Name]
		if !ok {
			mErr.Errors = append(mErr.Errors, errors.New("unable to check dependency for job: "+depJob.Name))
			ready = false
			break
		}

		if job == nil || !statusMatches(job.Status, depJob.Status) {
			ready = false
			break
		}
	}

	return ready, mErr.ErrorOrNil()
}

func statusMatches(actual, expected string) bool {
	if expected == "" {
		return actual == ""
	}

	if expected == "completed" {
		return actual == structs.JobStatusDead
	}

	return actual == expected
}

func dependencyTimeout(job *structs.Job) time.Duration {
	timeout := DefaultTimeout
	if job.Dependencies != nil && job.Dependencies.Timeout > 0 {
		timeout = job.Dependencies.Timeout
	}

	if timeout <= 0 {
		return DefaultTimeout
	}

	return timeout
}

func (c *Coordinator) Stop() {
	c.mainContext.Done()
	c.dependencies = nil
}

func (c *Coordinator) HasDependencies(j *structs.Job) (bool, error) {
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
		_, err = c.CheckDependency(state, job, eval)
		if err != nil {
			c.logger.Error("failed to check dependency", "error", err)
		}
	}
}

type NoOpCoordinator struct{}

func (c *NoOpCoordinator) HasDependencies(j *structs.Job) (bool, error) {
	return false, nil
}

func (c *NoOpCoordinator) CheckDependency(state sstructs.State, job *structs.Job,
	eval *structs.Evaluation) (bool, error) {
	return true, nil
}
