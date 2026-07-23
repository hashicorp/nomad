// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package scheduler

import (
	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs"
	sstructs "github.com/hashicorp/nomad/scheduler/structs"
)

type DependencyChecker interface {
	CheckDependency(state sstructs.State, job *structs.Job, eval *structs.Evaluation) (bool, error)
}

type BatchScheduler struct {
	dependencyChecker DependencyChecker
	GenericScheduler
}

// NewBatchScheduler is a factory function to instantiate a new batch scheduler
func NewBatchScheduler(logger log.Logger, eventsCh chan<- interface{}, state sstructs.State,
	planner sstructs.Planner, opts ...sstructs.SchedulerOption) sstructs.Scheduler {
	bs := &BatchScheduler{
		GenericScheduler: GenericScheduler{
			logger:   logger.Named("batch_sched"),
			eventsCh: eventsCh,
			state:    state,
			planner:  planner,
			batch:    true,
		},
	}

	for _, opt := range opts {
		opt(bs)
	}

	bs.nodesSetter = bs.dependencyWrapper(bs.setNodes)

	return bs
}

func (bs *BatchScheduler) setNodes(job *structs.Job) ([]*structs.Node, map[string]int, error) {

	ready, err := bs.dependencyChecker.CheckDependency(bs.state, job, bs.eval)
	if err != nil {
		return []*structs.Node{}, nil, err
	}

	if !ready {
		_, _, byDC, err := readyNodesInDCsAndPool(bs.state, job.Datacenters, job.NodePool)
		return []*structs.Node{}, byDC, err
	}

	return bs.GenericScheduler.setNodes(job)
}

// This wrapper is used to limit the initial pool of nodes for the feasibility check
// depending on if the dependencies for the job being processed are met or not.
func (bs *BatchScheduler) dependencyWrapper(next filterNodesFunc) filterNodesFunc {
	return func(job *structs.Job) ([]*structs.Node, map[string]int, error) {
		ready, err := bs.dependencyChecker.CheckDependency(bs.state, job, bs.eval)
		if err != nil || !ready {
			return []*structs.Node{}, nil, err
		}

		return next(job)
	}
}
