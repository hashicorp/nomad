// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scheduler

import (
	"fmt"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/scheduler/structs"
)

const (
	// SchedulerVersion is the version of the scheduler. Changes to the
	// scheduler that are incompatible with prior schedulers will increment this
	// version. It is used to disallow dequeueing when the versions do not match
	// across the leader and the dequeueing scheduler.
	SchedulerVersion uint16 = 1
)

// BuiltinSchedulers contains the built in registered schedulers
// which are available
var BuiltinSchedulers = map[string]structs.Factory{
	"service":  NewServiceScheduler,
	"batch":    NewBatchScheduler,
	"system":   NewSystemScheduler,
	"sysbatch": NewSysBatchScheduler,
}

// NewScheduler is used to instantiate and return a new scheduler
// given the scheduler name, initial state, and planner.
func NewScheduler(
	name string, logger log.Logger, eventsCh chan<- interface{}, state structs.State, planner structs.Planner,
) (structs.Scheduler, error) {
	// Lookup the factory function
	factory, ok := BuiltinSchedulers[name]
	if !ok {
		return nil, fmt.Errorf("unknown scheduler '%s'", name)
	}

	// Instantiate the scheduler
	sched := factory(logger, eventsCh, state, planner)
	return sched, nil
}
