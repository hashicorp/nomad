// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

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

var DefaultTimeout = 10 * time.Minute

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

		delete(c.dependencies, eval.ID)
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

	ctx, cancel := context.WithDeadline(c.mainContext, time.Now().Add(dependencyTimeout(job)))
	c.dependencies[eval.JobID] = &dependency{
		cancelFunc: cancel,
		job:        job,
		dependees:  djIDs,
	}

	go c.waitForDependency(ctx, state, eval, djIDs...)

	return false, nil
}

func (c *Coordinator) waitForDependency(ctx context.Context, state sstructs.State,
	eval *structs.Evaluation, dependeeJobIDs ...string) {

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

			ready, err := c.verifyDependencies(c.dependencies[eval.JobID].job, dj)
			if err != nil {
				c.logger.Error("failed to verify dependency", "error", err)
				continue
			}

			if ready {
				err := c.unblockDependencies(eval, dj)
				if err != nil {
					c.logger.Error("failed to unblock job", "error", err)
				}
				return
			}

		case <-ctx.Done():
			c.unblockDependencies(eval, dj)
			c.logger.Debug("dependency timeout reached", "jobID", eval.JobID)
			return
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
			c.logger.Debug("job not preset yet", "jobID", depJob.Name)
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

func (c *Coordinator) Reload(state sstructs.State, evals ...*structs.Evaluation) {
	for _, eval := range evals {
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
