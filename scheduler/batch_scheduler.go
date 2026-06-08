// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package scheduler

import (
	"context"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs"
	sstructs "github.com/hashicorp/nomad/scheduler/structs"
)

type DependencyChecker interface {
	AddDependency(ctx context.Context, state sstructs.State, eval *structs.Evaluation) error
}

type BatchScheduler struct {
	dependencyChecker DependencyChecker
	GenericScheduler
}

// NewBatchScheduler is a factory function to instantiate a new batch scheduler
func NewBatchScheduler(logger log.Logger, eventsCh chan<- interface{}, state sstructs.State,
	planner sstructs.Planner, opts ...sstructs.SchedulerOption) sstructs.Scheduler {
	s := &BatchScheduler{
		GenericScheduler: GenericScheduler{
			logger:   logger.Named("batch_sched"),
			eventsCh: eventsCh,
			state:    state,
			planner:  planner,
			batch:    true,
		},
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *BatchScheduler) setNodes(job *structs.Job) ([]*structs.Node, map[string]int, error) {
	return s.GenericScheduler.setNodes(job)
}
