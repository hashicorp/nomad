// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package dependency

import (
	"context"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
	sstructs "github.com/hashicorp/nomad/scheduler/structs"
)

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
	logger hclog.Logger
	l      sync.RWMutex

	dependencies map[evalID]*dependency
	loopDetector loopDetector
	blockedEvals evalUnblocker
}

// TODO: Think how to rebuild out of evals!
func NewCoordinator(logger hclog.Logger, loopDetector loopDetector,
	blockedEvals evalUnblocker) *Coordinator {
	return &Coordinator{
		logger:       logger,
		dependencies: make(map[evalID]*dependency),
		loopDetector: loopDetector,
		blockedEvals: blockedEvals,
	}
}

func (c *Coordinator) unblockDependencies(eval *structs.Evaluation, dependeeJobs ...*structs.Job) error {
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

func (c *Coordinator) AddDependency(ctx context.Context, state sstructs.State, eval *structs.Evaluation) error {

	job, err := state.JobByID(nil, eval.Namespace, eval.ID)
	if err != nil {
		c.logger.Error("failed to get job by ID", "error", err)
		return err
	}

	djIDs := []string{}
	for _, d := range job.Dependencies {
		djIDs = append(djIDs, d.Job)
	}

	c.loopDetector.AddNodes(eval.JobID, djIDs...)

	ctx, cancel := context.WithDeadline(ctx, time.Now().Add(10*time.Minute))
	c.dependencies[eval.JobID] = &dependency{
		cancelFunc: cancel,
		job:        job,
		dependees:  djIDs,
	}

	go c.waitForDependency(ctx, state, eval, djIDs...)

	return nil
}

func (c *Coordinator) waitForDependency(ctx context.Context, state sstructs.State,
	eval *structs.Evaluation, dependeeJobIDs ...string) {

	for {
		ws := memdb.NewWatchSet()
		dj := []*structs.Job{}

		for _, jID := range dependeeJobIDs {
			j, err := state.JobByID(ws, eval.Namespace, jID)
			if err != nil {
				c.logger.Error("failed to get job by ID", "error", err)
			}

			dj = append(dj, j)
		}

		select {
		case <-ws.WatchCh(ctx):
			ready, err := c.verifyDependencies(c.dependencies[eval.JobID].job, dj...)
			if err != nil {
				c.logger.Error("failed to verify dependency", "error", err)
				continue
			}

			if ready {
				err := c.unblockDependencies(eval, dj...)
				if err != nil {
					c.logger.Error("failed to unblock job", "error", err)
				}
				return
			}

		case <-ctx.Done():
			return
		}
	}
}

func (c *Coordinator) verifyDependencies(dependantJob *structs.Job, dependeeJob ...*structs.Job) (bool, error) {
	return true, nil
}
