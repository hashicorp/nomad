// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package tests

import (
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	sstructs "github.com/hashicorp/nomad/scheduler/structs"
	"github.com/shoenig/test/must"
)

// RejectPlan is used to always reject the entire plan and force a state refresh
type RejectPlan struct {
	*Harness
}

func (r *RejectPlan) ServersMeetMinimumVersion(minVersion *version.Version, checkFailedServers bool) bool {
	return r.ServersMeetMinimumVersion(minVersion, checkFailedServers)
}

func (r *RejectPlan) SubmitPlan(*structs.Plan) (*structs.PlanResult, sstructs.State, error) {
	result := new(structs.PlanResult)
	result.RefreshIndex = r.NextIndex()
	return result, r.State, nil
}

func (r *RejectPlan) UpdateEval(eval *structs.Evaluation) error {
	return nil
}

func (r *RejectPlan) CreateEval(*structs.Evaluation) error {
	return nil
}

func (r *RejectPlan) ReblockEval(*structs.Evaluation) error {
	return nil
}

// Harness is a lightweight testing harness for schedulers. It manages a state
// store copy and provides the planner interface. It can be extended for various
// testing uses or for invoking the scheduler without side effects.
type Harness struct {
	t testing.TB

	*sstructs.PlanBuilder
}

// NewHarness is used to make a new testing harness
func NewHarness(t testing.TB) *Harness {
	state := state.TestStateStore(t)
	plan := sstructs.NewPlanWithStateAndIndex(state, 1, true)
	h := &Harness{
		t:           t,
		PlanBuilder: plan,
	}
	return h
}

// NewHarnessWithState creates a new harness with the given state for testing
// purposes.
func NewHarnessWithState(t testing.TB, state *state.StateStore) *Harness {
	plan := sstructs.NewPlanWithStateAndIndex(state, 1, false)
	return &Harness{
		t:           t,
		PlanBuilder: plan,
	}
}

// Snapshot is used to snapshot the current state
func (h *Harness) Snapshot() sstructs.State {
	snap, _ := h.State.Snapshot()
	return snap
}

// Scheduler is used to return a new scheduler from
// a snapshot of current state using the harness for planning.
func (h *Harness) Scheduler(factory sstructs.Factory) sstructs.Scheduler {
	logger := testlog.HCLogger(h.t)
	eventsCh := make(chan interface{})

	// Listen for and log events from the scheduler.
	go func() {
		for e := range eventsCh {
			switch event := e.(type) {
			case *sstructs.PortCollisionEvent:
				h.t.Errorf("unexpected worker eval event: %v", event.Reason)
			}
		}
	}()

	return factory(logger, eventsCh, h.Snapshot(), h)
}

// Process is used to process an evaluation given a factory
// function to create the scheduler
func (h *Harness) Process(factory sstructs.Factory, eval *structs.Evaluation) error {
	sched := h.Scheduler(factory)
	return sched.Process(eval)
}

func (h *Harness) AssertEvalStatus(t testing.TB, state string) {
	must.Len(t, 1, h.Evals)
	update := h.Evals[0]
	must.Eq(t, state, update.Status)
}

// CreateAlloc is helper method to create allocations with given jobs and
// resources
func CreateAlloc(id string, job *structs.Job, resource *structs.Resources) *structs.Allocation {
	return CreateAllocInner(id, job, resource, nil, nil)
}

// CreateAllocWithTaskgroupNetwork is is helper method to create allocation with
// network at the task group level
func CreateAllocWithTaskgroupNetwork(id string, job *structs.Job, resource *structs.Resources, tgNet *structs.NetworkResource) *structs.Allocation {
	return CreateAllocInner(id, job, resource, nil, tgNet)
}

func CreateAllocWithDevice(id string, job *structs.Job, resource *structs.Resources, allocatedDevices *structs.AllocatedDeviceResource) *structs.Allocation {
	return CreateAllocInner(id, job, resource, allocatedDevices, nil)
}

func CreateAllocInner(id string, job *structs.Job, resource *structs.Resources, allocatedDevices *structs.AllocatedDeviceResource, tgNetwork *structs.NetworkResource) *structs.Allocation {
	alloc := &structs.Allocation{
		ID:    id,
		Job:   job,
		JobID: job.ID,
		TaskResources: map[string]*structs.Resources{
			"web": resource,
		},
		Namespace:     structs.DefaultNamespace,
		EvalID:        uuid.Generate(),
		DesiredStatus: structs.AllocDesiredStatusRun,
		ClientStatus:  structs.AllocClientStatusRunning,
		TaskGroup:     "web",
		AllocatedResources: &structs.AllocatedResources{
			Tasks: map[string]*structs.AllocatedTaskResources{
				"web": {
					Cpu: structs.AllocatedCpuResources{
						CpuShares: int64(resource.CPU),
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: int64(resource.MemoryMB),
					},
					Networks: resource.Networks,
				},
			},
		},
	}

	if allocatedDevices != nil {
		alloc.AllocatedResources.Tasks["web"].Devices = []*structs.AllocatedDeviceResource{allocatedDevices}
	}

	if tgNetwork != nil {
		alloc.AllocatedResources.Shared = structs.AllocatedSharedResources{
			Networks: []*structs.NetworkResource{tgNetwork},
		}
	}
	return alloc
}
