// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1
// TODOS:
// - What happened if the dependee job is cancelled or stopped?

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

var DefaultTimeout = 5 * time.Second
var errDependencyTimeout = errors.New("dependency timeout reached")

type evalID = string

type evalUnblocker interface {
	Unblock(computedClass string, index uint64) chan struct{}
}

type loopDetector interface {
	AddNodes(dependantJob string, dependeeJob ...string) error
	RemoveNode(dependantJob string) error
}

// This function is a raft apply, to use it here is possible only because the coordinator
// is supposed to run exclusively on the leader, allowing for the log to be applied to the FSM.
type evalUpdaterFunc func(t structs.MessageType, msg any) (any, uint64, error)

type dependency struct {
	cancelFunc context.CancelFunc
	job        *structs.Job
	dependees  []string
}

type Coordinator struct {
	mainContext context.Context
	logger      hclog.Logger
	l           sync.RWMutex

	dependencies   map[evalID]*dependency
	loopDetector   loopDetector
	blockedEvals   evalUnblocker
	jobUpdaterFunc evalUpdaterFunc
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
	return &Coordinator{
		mainContext:    context.Background(),
		logger:         logger.Named("dependency-coordinator"),
		dependencies:   make(map[evalID]*dependency),
		loopDetector:   loopDetector,
		blockedEvals:   blockedEvals,
		jobUpdaterFunc: jobUpdater,
	}
}

func (c *Coordinator) removeDeps(eval *structs.Evaluation, dependeeJobs map[string]*structs.Job) error {
	for _, job := range dependeeJobs {
		// The dependee job was never present but the dependant will be unblocked
		// after it timed out.
		if job == nil {
			continue
		}

		c.l.Lock()
		defer c.l.Unlock()

		if err := c.loopDetector.RemoveNode(eval.JobID); err != nil {
			c.logger.Error("failed to remove dependency", "error", err)
		}
	}

	return nil
}

// CheckDependency checks if the dependencies for a job are met. If they are not,
// it will trigger a block routine until the any of the blockers is updated or a timeout is reached.
// It will also return a list of the jobs blocking the evaluation.
//
// If the dependencies are met, it will return an empty slice and no error.
func (c *Coordinator) CheckDependency(state sstructs.State, job *structs.Job,
	eval *structs.Evaluation) ([]string, error) {

	if job.Dependencies == nil {
		return []string{}, nil
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

	blockers, err := c.verifyDependencies(job, djs)
	if err != nil {
		c.logger.Error("failed to verify dependencies", "error", err)
	}

	if len(blockers) == 0 {
		return []string{}, nil
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

	return blockers, nil
}

func (c *Coordinator) waitForDependency(ctx context.Context, state sstructs.State,
	eval *structs.Evaluation, dependeeJobIDs ...string) {
	dep := c.dependencies[eval.ID]
	dj := map[string]*structs.Job{}
	ws := memdb.NewWatchSet()

	for _, jID := range dependeeJobIDs {
		dependeeJob, err := state.JobByID(ws, eval.Namespace, jID)
		if err != nil {
			c.logger.Error("failed to get job by ID", "error", err)
		}

		dj[jID] = dependeeJob
	}

	defer func() {
		delete(c.dependencies, eval.ID)
		err := c.removeDeps(eval, dj)
		if err != nil {
			c.logger.Info("failed to remove dependencies", "error", err)
		}

		dep.cancelFunc()
	}()

	for {
		select {
		case <-ws.WatchCh(ctx):
			blockers, err := c.verifyDependencies(dep.job, dj)
			if err != nil {
				c.logger.Error("failed to verify dependency", "error", err)
			}

			if len(blockers) > 0 {
				c.blockedEvals.Unblock(eval.ID, dep.job.JobModifyIndex)
				c.logger.Debug("dependency ready, unblocking job", "job", eval.JobID,
					"eval", eval.ID, "ready", len(blockers) > 0)

				if err != nil {
					c.logger.Error("failed to unblock job", "error", err)
				}
			}

			return

		case <-ctx.Done():
			c.logger.Error("dependency timeout reached", "job", eval.JobID,
				"eval", eval.ID)

			err := c.deleteEval(*eval, *dep.job)
			if err != nil {
				c.logger.Error("dependency timeout reached, failed to update job", "jobID", dep.job.ID, "error", err)
			}

			return
		}
	}
}

func (c *Coordinator) deleteEval(eval structs.Evaluation, job structs.Job) error {
	/* 	job.Status = structs.JobStatusDead
	   	_, _, err := c.jobUpdaterFunc(structs.JobDeregisterRequestType, &structs.JobDeregisterRequest{
	   		JobID: job.ID,
	   		WriteRequest: structs.WriteRequest{
	   			Namespace: eval.Namespace,
	   		},
	   	})
	   	if err != nil {
	   		c.logger.Error("coordinator: failed to update eval", "eval", eval.ID, "error", err)
	   		return err
	   	} */

	eval.Status = structs.EvalStatusCancelled
	eval.StatusDescription = structs.EvalTriggeredDeps

	_, _, err := c.jobUpdaterFunc(structs.EvalDeleteRequestType, &structs.EvalDeleteRequest{
		EvalIDs: []string{eval.ID},
		WriteRequest: structs.WriteRequest{
			Namespace: eval.Namespace,
		},
	})

	if err != nil {
		c.logger.Error("coordinator: failed to update eval", "eval", eval.ID, "error", err)
		return err
	}

	return nil
}

func (c *Coordinator) verifyDependencies(dependantJob *structs.Job, jobs map[string]*structs.Job) ([]string, error) {
	var mErr multierror.Error
	blockers := []string{}

	for _, depJob := range dependantJob.Dependencies.Jobs {
		if depJob == nil {
			continue
		}

		job, ok := jobs[depJob.Name]
		if !ok {
			mErr.Errors = append(mErr.Errors, errors.New("unable to check dependency for job: "+depJob.Name))
			return []string{}, &mErr
		}

		if job == nil || !statusMatches(job.Status, depJob.Status) {
			blockers = append(blockers, depJob.Name)
		}
	}

	return blockers, mErr.ErrorOrNil()
}

// This function needs work to allow more descriptive statues, like what is done
// on the front end.
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

func NewNoOpCoordinator() *NoOpCoordinator {
	return &NoOpCoordinator{}
}

type NoOpCoordinator struct{}

func (c *NoOpCoordinator) HasDependencies(j *structs.Job) (bool, error) {
	return false, nil
}

func (c *NoOpCoordinator) CheckDependency(state sstructs.State, job *structs.Job,
	eval *structs.Evaluation) ([]string, error) {
	return []string{}, nil
}
