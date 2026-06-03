// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queues

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

type testBroker struct{}

func (*testBroker) Enqueue(*structs.Evaluation) {}

func TestWaitForPlacement(t *testing.T) {

	t.Run("returns if eval complete", func(t *testing.T) {
		ss := state.TestStateStore(t)
		testQueue := NewDynamicPriorityQueue(nil, &structs.BatchQueue{}, &structs.DynamicQueueConfig{}, hclog.New(hclog.DefaultOptions))
		testQueue.SetEnabled(true, ss)

		testEval := mock.Eval()
		ss.UpsertEvals(structs.MsgTypeTestSetup, 0, []*structs.Evaluation{testEval})

		ws := memdb.NewWatchSet()
		doneCh := make(chan error)
		go func() {
			err := testQueue.waitForPlacement(t.Context(), &Workload{eval: testEval}, ws)
			doneCh <- err
		}()

		testEval.Status = structs.EvalStatusComplete
		ss.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{testEval})

		done := <-doneCh

		must.NoError(t, done)
	})

	t.Run("continues watching blocked evals", func(t *testing.T) {
		ss := state.TestStateStore(t)
		testQueue := NewDynamicPriorityQueue(nil, &structs.BatchQueue{}, &structs.DynamicQueueConfig{}, hclog.New(hclog.DefaultOptions))
		testQueue.SetEnabled(true, ss)

		testEval := mock.Eval()
		blocked := mock.Eval()

		testEval.Status = structs.EvalStatusComplete
		testEval.BlockedEval = blocked.ID

		ss.UpsertEvals(structs.MsgTypeTestSetup, 0, []*structs.Evaluation{testEval, blocked})

		ws := memdb.NewWatchSet()
		doneCh := make(chan error)
		go func() {
			err := testQueue.waitForPlacement(t.Context(), &Workload{eval: testEval}, ws)
			doneCh <- err
		}()

		// We want to make sure the testQueue has begun a watch on the blocked eval
		// before continuing, which is indicated by the length of the watchset being >0.
		must.Wait(t, wait.InitialSuccess(
			wait.ErrorFunc(func() error {
				if len(ws) == 0 {
					return fmt.Errorf("blocking query not started yet")
				}
				return nil
			}),
			wait.Timeout(5*time.Second),
			wait.Gap(100*time.Millisecond),
		))

		select {
		case <-doneCh:
			t.Fatal("should not have exited")
		default:
		}

		blocked.Status = structs.EvalStatusComplete
		ss.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{blocked})

		done := <-doneCh
		must.NoError(t, done)
	})

	t.Run("continues watching next evals after eval failure", func(t *testing.T) {
		ss := state.TestStateStore(t)
		testQueue := NewDynamicPriorityQueue(nil, &structs.BatchQueue{}, &structs.DynamicQueueConfig{}, hclog.New(hclog.DefaultOptions))
		testQueue.SetEnabled(true, ss)

		testEval := mock.Eval()
		next := mock.Eval()

		testEval.Status = structs.EvalStatusFailed
		testEval.NextEval = next.ID

		ss.UpsertEvals(structs.MsgTypeTestSetup, 0, []*structs.Evaluation{testEval, next})

		ws := memdb.NewWatchSet()
		doneCh := make(chan error)
		go func() {
			err := testQueue.waitForPlacement(t.Context(), &Workload{eval: testEval}, ws)
			doneCh <- err
		}()

		// We want to make sure the testQueue has begun a watch on the blocked eval
		// before continuing, which is indicated by the length of the watchset being >0.
		must.Wait(t, wait.InitialSuccess(
			wait.ErrorFunc(func() error {
				if len(ws) == 0 {
					return fmt.Errorf("blocking query not started yet")
				}
				return nil
			}),
			wait.Timeout(5*time.Second),
			wait.Gap(100*time.Millisecond),
		))

		select {
		case <-doneCh:
			t.Fatal("should not have exited")
		default:
		}

		next.Status = structs.EvalStatusComplete
		ss.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{next})

		done := <-doneCh
		must.NoError(t, done)
	})
}

func TestDecayUsage(t *testing.T) {
	t.Run("decays usage by half after half-life", func(t *testing.T) {
		queue := NewDynamicPriorityQueue(nil, &structs.BatchQueue{}, &structs.DynamicQueueConfig{
			HalfLife: 10 * time.Second,
		}, hclog.New(hclog.DefaultOptions))

		tenantA := &Tenant{
			tid: TenantID("tenant-a"),
			workloads: map[string]*Workload{
				"w1": {},
			},
			usage: map[string]float64{
				"cpu":    100,
				"memory": 20,
			},
		}

		tenantB := &Tenant{
			tid: TenantID("tenant-b"),
			workloads: map[string]*Workload{
				"w2": {},
			},
			usage: map[string]float64{
				"cpu":    60,
				"memory": 40,
			},
		}

		now := time.Unix(100, 0)
		queue.lastUpdated = now.Add(-10 * time.Second)
		queue.tenants[tenantA.tid] = tenantA
		queue.tenants[tenantB.tid] = tenantB

		queue.decayUsage(now)

		must.Eq(t, tenantA.usage, map[string]float64{"cpu": 50, "memory": 10}, must.Cmp(cmpopts.EquateApprox(0, 1e-9)))
		must.Eq(t, tenantB.usage, map[string]float64{"cpu": 30, "memory": 20}, must.Cmp(cmpopts.EquateApprox(0, 1e-9)))
		must.Eq(t, queue.totalUsage, map[string]float64{"cpu": 80, "memory": 30}, must.Cmp(cmpopts.EquateApprox(0, 1e-9)))
	})

	t.Run("initializes lastUpdated without decay when zero", func(t *testing.T) {
		queue := NewDynamicPriorityQueue(nil, &structs.BatchQueue{}, &structs.DynamicQueueConfig{
			HalfLife: 10 * time.Second,
		}, hclog.New(hclog.DefaultOptions))

		now := time.Unix(100, 0)
		tenantA := &Tenant{
			tid: TenantID("tenant-a"),
			workloads: map[string]*Workload{
				"w1": {},
			},
			usage: map[string]float64{
				"cpu":    42,
				"memory": 84,
			},
		}
		queue.tenants[tenantA.tid] = tenantA

		queue.decayUsage(now)

		must.Eq(t, now, queue.lastUpdated)
		must.Eq(t, tenantA.usage, map[string]float64{"cpu": 42, "memory": 84}, must.Cmp(cmpopts.EquateApprox(0, 1e-9)))
		must.Eq(t, queue.totalUsage, map[string]float64{"cpu": 42, "memory": 84}, must.Cmp(cmpopts.EquateApprox(0, 1e-9)))
	})
}

func TestCalculatePriorities(t *testing.T) {
	queue := NewDynamicPriorityQueue(nil, &structs.BatchQueue{}, &structs.DynamicQueueConfig{
		HalfLife:    10 * time.Second,
		UsageWeight: 10,
	}, hclog.New(hclog.DefaultOptions))

	lowUsageTenant := &Tenant{
		tid:       TenantID("tenant-low"),
		workloads: map[string]*Workload{},
		usage: map[string]float64{
			"cpu":    0,
			"memory": 55,
		},
	}
	highUsageTenant := &Tenant{
		tid:       TenantID("tenant-high"),
		workloads: map[string]*Workload{},
		usage: map[string]float64{
			"cpu":    100,
			"memory": 50,
		},
	}

	lowUsageWorkload := &Workload{
		tid:  lowUsageTenant.tid,
		eval: &structs.Evaluation{Priority: 5},
	}
	highUsageWorkload := &Workload{
		tid:  highUsageTenant.tid,
		eval: &structs.Evaluation{Priority: 5},
	}

	lowUsageTenant.workloads["w1"] = lowUsageWorkload
	highUsageTenant.workloads["w2"] = highUsageWorkload
	queue.tenants[lowUsageTenant.tid] = lowUsageTenant
	queue.tenants[highUsageTenant.tid] = highUsageTenant
	queue.queue = WorkloadQueue{lowUsageWorkload, highUsageWorkload}

	queue.calculatePriorities(time.Unix(20, 0))

	must.Greater(t, highUsageWorkload.priority, lowUsageWorkload.priority)
	must.Eq(t, queue.totalUsage, map[string]float64{"cpu": 100, "memory": 105}, must.Cmp(cmpopts.EquateApprox(0, 1e-9)))
}
