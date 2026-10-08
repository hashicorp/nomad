// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package scheduler

import (
	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/dependency"
	"github.com/hashicorp/nomad/nomad/structs"
	sstructs "github.com/hashicorp/nomad/scheduler/structs"
)

type BatchScheduler struct {
	*GenericScheduler
}

// NewBatchScheduler is a factory function to instantiate a new batch scheduler
func NewBatchScheduler(logger log.Logger, eventsCh chan<- any, state sstructs.State,
	planner sstructs.Planner) sstructs.Scheduler {

	s := NewServiceScheduler(logger, eventsCh, state,
		planner)

	bs := &BatchScheduler{
		GenericScheduler: s.(*GenericScheduler),
	}

	bs.nodesSetter = bs.dependencyWrapper(bs.GenericScheduler.setNodes)
	bs.GenericScheduler.batch = true
	return bs
}

// This wrapper is used to limit the initial pool of nodes for the feasibility check
// depending on if the dependencies for the job being processed are met or not.
func (bs *BatchScheduler) dependencyWrapper(next filterNodesFunc) filterNodesFunc {
	return func(job *structs.Job) ([]*structs.Node, map[string]int, error) {
		blockers, err := dependency.VerifyDependencies(bs.state, job)
		if err != nil {
			return []*structs.Node{}, nil, err
		}

		if len(blockers) > 0 {
			bs.blockers = blockers
			return []*structs.Node{}, nil, err
		}

		return next(job)
	}
}
