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
	"github.com/hashicorp/nomad/helper/uuid"
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
		workload := &Workload{eval: testEval.Copy()}
		go func() {
			err := testQueue.waitForPlacement(t.Context(), workload, ws)
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
		workload := &Workload{eval: testEval.Copy()}
		go func() {
			err := testQueue.waitForPlacement(t.Context(), workload, ws)
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
		workload := &Workload{eval: testEval.Copy()}
		go func() {
			err := testQueue.waitForPlacement(t.Context(), workload, ws)
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
		ss := state.TestStateStore(t)
		now := time.Unix(100, 0)
		eval1 := mock.Eval()
		eval2 := mock.Eval()
		missingEvalID := uuid.Generate()
		ss.UpsertEvals(structs.MsgTypeTestSetup, 0, []*structs.Evaluation{eval1, eval2})

		testCases := []struct {
			name                string
			halfLife            time.Duration
			tenants             []*Tenant
			expectedTenantUsage map[TenantID]map[string]float64
			expectedTotalUsage  map[string]float64
		}{
			{
				name:     "single tenant with cpu and memory usage",
				halfLife: 10 * time.Second,
				tenants: []*Tenant{
					{
						tid: TenantID("tenant"),
						workloadUsageByID: map[string]WorkloadUsage{
							eval1.ID: {ts: now.Add(-10 * time.Second), resources: map[string]float64{
								"cpu":    100,
								"memory": 20,
							}},
						},
					},
				},
				expectedTenantUsage: map[TenantID]map[string]float64{
					TenantID("tenant"): {
						"cpu":    50,
						"memory": 10,
					},
				},
				expectedTotalUsage: map[string]float64{
					"cpu":    50,
					"memory": 10,
				},
			},
			{
				name:     "single tenants multiple workloads",
				halfLife: 10 * time.Second,
				tenants: []*Tenant{
					{
						tid: TenantID("tenant"),
						workloadUsageByID: map[string]WorkloadUsage{
							eval1.ID: {ts: now.Add(-10 * time.Second), resources: map[string]float64{
								"cpu": 80,
							}},
							eval2.ID: {ts: now.Add(-5 * time.Second), resources: map[string]float64{
								"cpu":    100,
								"memory": 50,
							}},
						},
					},
				},
				expectedTenantUsage: map[TenantID]map[string]float64{
					TenantID("tenant"): {
						"cpu":    110.71067811865476,
						"memory": 35.35533905932738,
					},
				},
				expectedTotalUsage: map[string]float64{
					"cpu":    110.71067811865476,
					"memory": 35.35533905932738,
				},
			},
			{
				name:     "multiple tenants",
				halfLife: 10 * time.Second,
				tenants: []*Tenant{
					{
						tid: TenantID("tenantA"),
						workloadUsageByID: map[string]WorkloadUsage{
							eval1.ID: {ts: now.Add(-10 * time.Second), resources: map[string]float64{
								"memory": 40,
								"cpu":    100,
							}},
						},
					},
					{
						tid: TenantID("tenantB"),
						workloadUsageByID: map[string]WorkloadUsage{
							eval2.ID: {ts: now.Add(-10 * time.Second), resources: map[string]float64{
								"memory": 80,
								"cpu":    75,
							}},
						},
					},
				},
				expectedTenantUsage: map[TenantID]map[string]float64{
					TenantID("tenantA"): {
						"memory": 20,
						"cpu":    50,
					},
					TenantID("tenantB"): {
						"memory": 40,
						"cpu":    37.5,
					},
				},
				expectedTotalUsage: map[string]float64{
					"memory": 60,
					"cpu":    87.5,
				},
			},
			{
				name:     "multiple tenants multiple workloads",
				halfLife: 10 * time.Second,
				tenants: []*Tenant{
					{
						tid: TenantID("tenantA"),
						workloadUsageByID: map[string]WorkloadUsage{
							eval1.ID: {ts: now.Add(-10 * time.Second), resources: map[string]float64{
								"memory": 40,
								"cpu":    100,
							}},
							eval2.ID: {ts: now.Add(-10 * time.Second), resources: map[string]float64{
								"memory": 80,
								"cpu":    60,
							}},
						},
					},
					{
						tid: TenantID("tenantB"),
						workloadUsageByID: map[string]WorkloadUsage{
							eval1.ID: {ts: now.Add(-10 * time.Second), resources: map[string]float64{
								"memory": 80,
								"cpu":    75,
							}},
							eval2.ID: {ts: now.Add(-10 * time.Second), resources: map[string]float64{
								"memory": 100,
								"cpu":    50,
							}},
						},
					},
				},
				expectedTenantUsage: map[TenantID]map[string]float64{
					TenantID("tenantA"): {
						"memory": 60,
						"cpu":    80,
					},
					TenantID("tenantB"): {
						"memory": 90,
						"cpu":    62.5,
					},
				},
				expectedTotalUsage: map[string]float64{
					"memory": 150,
					"cpu":    142.5,
				},
			},
			{
				name:     "attempt to decay a GC'd workload",
				halfLife: 10 * time.Second,
				tenants: []*Tenant{
					{
						tid: TenantID("tenant"),
						workloadUsageByID: map[string]WorkloadUsage{
							missingEvalID: {ts: now.Add(-10 * time.Second), resources: map[string]float64{
								"cpu":    100,
								"memory": 20,
							}},
						},
					},
					{
						tid: TenantID("tenantB"),
						workloadUsageByID: map[string]WorkloadUsage{
							eval1.ID: {ts: now.Add(-10 * time.Second), resources: map[string]float64{
								"cpu":    100,
								"memory": 20,
							}},
						},
					},
				},
				expectedTenantUsage: map[TenantID]map[string]float64{
					TenantID("tenant"): {},
					TenantID("tenantB"): {
						"cpu":    50,
						"memory": 10,
					},
				},
				expectedTotalUsage: map[string]float64{
					"cpu":    50,
					"memory": 10,
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				queue := NewDynamicPriorityQueue(nil, &structs.BatchQueue{}, &structs.DynamicQueueConfig{
					HalfLife: tc.halfLife,
				}, hclog.New(hclog.DefaultOptions))

				for _, tenant := range tc.tenants {
					queue.tenants[tenant.tid] = tenant
				}

				snapshot, err := ss.Snapshot()
				must.NoError(t, err)

				queue.decayUsage(now, snapshot)

				for _, tenant := range tc.tenants {
					must.Eq(t, tenant.totalUsage, tc.expectedTenantUsage[tenant.tid], must.Cmp(cmpopts.EquateApprox(0, 1e-9)))
				}
				must.Eq(t, queue.totalUsage, tc.expectedTotalUsage, must.Cmp(cmpopts.EquateApprox(0, 1e-9)))
			})
		}
	})
}

func TestCalculatePriorities(t *testing.T) {
	eval1 := mock.Eval()
	mkTenant := func(id TenantID, ts time.Time, cpu, memory float64) *Tenant {
		return &Tenant{
			tid: id,
			workloadUsageByID: map[string]WorkloadUsage{
				eval1.ID: {ts: ts, resources: map[string]float64{"cpu": cpu, "memory": memory}},
			},
			totalUsage: map[string]float64{"cpu": cpu, "memory": memory},
		}
	}
	ss := state.TestStateStore(t)
	ss.UpsertEvals(structs.MsgTypeTestSetup, 0, []*structs.Evaluation{eval1})

	testCases := []struct {
		name                         string
		conf                         *structs.DynamicQueueConfig
		lowUsageTenant               *Tenant
		highUsageTenant              *Tenant
		expectedHigherPriorityTenant TenantID
		expectedTotalUsage           map[string]float64
	}{
		{
			name:                         "higher usage results in lower priority",
			conf:                         &structs.DynamicQueueConfig{HalfLife: 10 * time.Second, UsageWeight: 10},
			lowUsageTenant:               mkTenant(TenantID("tenant-low"), time.Unix(20, 0), 0, 55),
			highUsageTenant:              mkTenant(TenantID("tenant-high"), time.Unix(20, 0), 100, 50),
			expectedHigherPriorityTenant: TenantID("tenant-low"),
			expectedTotalUsage:           map[string]float64{"cpu": 100, "memory": 105},
		},
		{
			name:                         "decays workloads before calculating priority",
			conf:                         &structs.DynamicQueueConfig{HalfLife: 10 * time.Second, UsageWeight: 10},
			lowUsageTenant:               mkTenant(TenantID("tenant-decayed"), time.Unix(10, 0), 100, 0),
			highUsageTenant:              mkTenant(TenantID("tenant-recent"), time.Unix(20, 0), 60, 0),
			expectedHigherPriorityTenant: TenantID("tenant-decayed"),
			expectedTotalUsage:           map[string]float64{"cpu": 110, "memory": 0},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			queue := NewDynamicPriorityQueue(nil, &structs.BatchQueue{}, tc.conf, hclog.New(hclog.DefaultOptions))

			queue.SetEnabled(true, ss)

			lowUsageWorkload := &Workload{tid: tc.lowUsageTenant.tid, eval: &structs.Evaluation{Priority: 5}}
			highUsageWorkload := &Workload{tid: tc.highUsageTenant.tid, eval: &structs.Evaluation{Priority: 5}}

			queue.tenants[tc.lowUsageTenant.tid] = tc.lowUsageTenant
			queue.tenants[tc.highUsageTenant.tid] = tc.highUsageTenant
			queue.queue = WorkloadQueue{lowUsageWorkload, highUsageWorkload}

			queue.calculatePriorities(time.Unix(20, 0))

			switch tc.expectedHigherPriorityTenant {
			case tc.lowUsageTenant.tid:
				must.Greater(t, highUsageWorkload.priority, lowUsageWorkload.priority)
			case tc.highUsageTenant.tid:
				must.Greater(t, lowUsageWorkload.priority, highUsageWorkload.priority)
			default:
				t.Fatalf("test case has unknown expectedHigherPriorityTenant: %q", tc.expectedHigherPriorityTenant)
			}

			must.Eq(t, queue.totalUsage, tc.expectedTotalUsage, must.Cmp(cmpopts.EquateApprox(0, 1e-9)))
		})
	}
}
