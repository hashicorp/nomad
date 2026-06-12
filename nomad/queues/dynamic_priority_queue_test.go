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

func TestDynamicPriorityQueue_waitForPlacement(t *testing.T) {

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

func TestDynamicPriorityQueue_decayUsage(t *testing.T) {
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
			expectedTenantUsage map[TenantID]*ResourceUsage
			expectedTotalUsage  *ResourceUsage
		}{
			{
				name:     "single tenant with cpu and memory usage",
				halfLife: 10 * time.Second,
				tenants: []*Tenant{
					{
						tid: TenantID("tenant"),
						placedWorkloadById: map[string]*Workload{
							eval1.ID: {requestedResources: &UsageList{start: now.Add(-10 * time.Second), resources: &ResourceUsage{
								CPU:    100,
								Memory: 20,
							}}},
						},
					},
				},
				expectedTenantUsage: map[TenantID]*ResourceUsage{
					TenantID("tenant"): {
						CPU:    50,
						Memory: 10,
					},
				},
				expectedTotalUsage: &ResourceUsage{
					CPU:    50,
					Memory: 10,
				},
			},
			{
				name:     "single tenants multiple workloads",
				halfLife: 10 * time.Second,
				tenants: []*Tenant{
					{
						tid: TenantID("tenant"),
						placedWorkloadById: map[string]*Workload{
							eval1.ID: {requestedResources: &UsageList{start: now.Add(-10 * time.Second), resources: &ResourceUsage{
								CPU: 80,
							}}},
							eval2.ID: {requestedResources: &UsageList{start: now.Add(-5 * time.Second), resources: &ResourceUsage{
								CPU:    100,
								Memory: 50,
							}}},
						},
					},
				},
				expectedTenantUsage: map[TenantID]*ResourceUsage{
					TenantID("tenant"): {
						CPU:    110.71067811865476,
						Memory: 35.35533905932738,
					},
				},
				expectedTotalUsage: &ResourceUsage{
					CPU:    110.71067811865476,
					Memory: 35.35533905932738,
				},
			},
			{
				name:     "multiple tenants",
				halfLife: 10 * time.Second,
				tenants: []*Tenant{
					{
						tid: TenantID("tenantA"),
						placedWorkloadById: map[string]*Workload{
							eval1.ID: {requestedResources: &UsageList{start: now.Add(-10 * time.Second), resources: &ResourceUsage{
								Memory: 40,
								CPU:    100,
							}}},
						},
					},
					{
						tid: TenantID("tenantB"),
						placedWorkloadById: map[string]*Workload{
							eval2.ID: {requestedResources: &UsageList{start: now.Add(-10 * time.Second), resources: &ResourceUsage{
								Memory: 80,
								CPU:    75,
							}}},
						},
					},
				},
				expectedTenantUsage: map[TenantID]*ResourceUsage{
					TenantID("tenantA"): {
						Memory: 20,
						CPU:    50,
					},
					TenantID("tenantB"): {
						Memory: 40,
						CPU:    37.5,
					},
				},
				expectedTotalUsage: &ResourceUsage{
					Memory: 60,
					CPU:    87.5,
				},
			},
			{
				name:     "multiple tenants multiple workloads",
				halfLife: 10 * time.Second,
				tenants: []*Tenant{
					{
						tid: TenantID("tenantA"),
						placedWorkloadById: map[string]*Workload{
							eval1.ID: {requestedResources: &UsageList{start: now.Add(-10 * time.Second), resources: &ResourceUsage{
								Memory: 40,
								CPU:    100,
							}}},
							eval2.ID: {requestedResources: &UsageList{start: now.Add(-10 * time.Second), resources: &ResourceUsage{
								Memory: 80,
								CPU:    60,
							}}},
						},
					},
					{
						tid: TenantID("tenantB"),
						placedWorkloadById: map[string]*Workload{
							eval1.ID: {requestedResources: &UsageList{start: now.Add(-10 * time.Second), resources: &ResourceUsage{
								Memory: 80,
								CPU:    75,
							}}},
							eval2.ID: {requestedResources: &UsageList{start: now.Add(-10 * time.Second), resources: &ResourceUsage{
								Memory: 100,
								CPU:    50,
							}}},
						},
					},
				},
				expectedTenantUsage: map[TenantID]*ResourceUsage{
					TenantID("tenantA"): {
						Memory: 60,
						CPU:    80,
					},
					TenantID("tenantB"): {
						Memory: 90,
						CPU:    62.5,
					},
				},
				expectedTotalUsage: &ResourceUsage{
					Memory: 150,
					CPU:    142.5,
				},
			},
			{
				name:     "attempt to decay a GC'd workload",
				halfLife: 10 * time.Second,
				tenants: []*Tenant{
					{
						tid: TenantID("tenant"),
						placedWorkloadById: map[string]*Workload{
							missingEvalID: {requestedResources: &UsageList{start: now.Add(-10 * time.Second), resources: &ResourceUsage{
								CPU:    100,
								Memory: 20,
							}}},
						},
					},
					{
						tid: TenantID("tenantB"),
						placedWorkloadById: map[string]*Workload{
							eval1.ID: {requestedResources: &UsageList{start: now.Add(-10 * time.Second), resources: &ResourceUsage{
								CPU:    100,
								Memory: 20,
							}}},
						},
					},
				},
				expectedTenantUsage: map[TenantID]*ResourceUsage{
					TenantID("tenant"): {},
					TenantID("tenantB"): {
						CPU:    50,
						Memory: 10,
					},
				},
				expectedTotalUsage: &ResourceUsage{
					CPU:    50,
					Memory: 10,
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

func TestDynamicPriorityQueue_calculatePriorities(t *testing.T) {
	eval1 := mock.Eval()
	mkTenant := func(id TenantID, ts time.Time, cpu, memory float64) *Tenant {
		return &Tenant{
			tid: id,
			placedWorkloadById: map[string]*Workload{
				eval1.ID: {requestedResources: &UsageList{start: ts, resources: &ResourceUsage{CPU: cpu, Memory: memory}}},
			},
			totalUsage: &ResourceUsage{CPU: cpu, Memory: memory},
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
		expectedTotalUsage           *ResourceUsage
	}{
		{
			name:                         "higher usage results in lower priority",
			conf:                         &structs.DynamicQueueConfig{HalfLife: 10 * time.Second, UsageWeight: 10},
			lowUsageTenant:               mkTenant(TenantID("tenant-low"), time.Unix(20, 0), 0, 55),
			highUsageTenant:              mkTenant(TenantID("tenant-high"), time.Unix(20, 0), 100, 50),
			expectedHigherPriorityTenant: TenantID("tenant-low"),
			expectedTotalUsage:           &ResourceUsage{CPU: 100, Memory: 105},
		},
		{
			name:                         "decays workloads before calculating priority",
			conf:                         &structs.DynamicQueueConfig{HalfLife: 10 * time.Second, UsageWeight: 10},
			lowUsageTenant:               mkTenant(TenantID("tenant-decayed"), time.Unix(10, 0), 100, 0),
			highUsageTenant:              mkTenant(TenantID("tenant-recent"), time.Unix(20, 0), 60, 0),
			expectedHigherPriorityTenant: TenantID("tenant-decayed"),
			expectedTotalUsage:           &ResourceUsage{CPU: 110, Memory: 0},
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

func TestDynamicPriorityQueue_sizeAdjustment(t *testing.T) {
	testCases := []struct {
		name     string
		conf     *structs.DynamicQueueConfig
		workload *Workload
		exp      int
	}{
		{
			name: "larger size results in 0 adjustment",
			conf: &structs.DynamicQueueConfig{
				SizeWeight: 10,
				MaxSize:    1000,
			},
			workload: &Workload{requestedResources: &UsageList{
				resources: &ResourceUsage{
					CPU:    500,
					Memory: 500,
				},
			}},
			exp: 0,
		},
		{
			name: "smaller sized job results in expected adjustment",
			conf: &structs.DynamicQueueConfig{
				SizeWeight: 10,
				MaxSize:    1000,
			},
			workload: &Workload{requestedResources: &UsageList{
				resources: &ResourceUsage{
					CPU:    50,
					Memory: 50,
				},
			}},
			exp: 9,
		},
		{
			name: "negative weight results in negative adjustment",
			conf: &structs.DynamicQueueConfig{
				SizeWeight: -10,
				MaxSize:    1000,
			},
			workload: &Workload{requestedResources: &UsageList{
				resources: &ResourceUsage{
					CPU:    50,
					Memory: 50,
				},
			}},
			exp: -9,
		},
	}

	for _, tc := range testCases {
		testQueue := &DynamicPriorityQueue{
			conf: tc.conf,
		}
		must.Eq(t, tc.exp, testQueue.sizeAdjustment(tc.workload), must.Sprint(tc.name))
	}
}

func TestDynamicPriorityQueue_ageAdjustment(t *testing.T) {
	testCases := []struct {
		name     string
		conf     *structs.DynamicQueueConfig
		workload *Workload
		nowTime  time.Time
		exp      int
	}{
		{
			name: "createTime and now equal results in 0 age adjustment",
			conf: &structs.DynamicQueueConfig{
				AgeWeight: 10,
				MaxAge:    time.Second * 10,
			},
			workload: &Workload{
				eval: &structs.Evaluation{
					CreateTime: time.Time{}.UnixNano(),
				},
			},
			nowTime: time.Time{},
			exp:     0,
		},
		{
			name: "greater than max age results in max adjustment",
			conf: &structs.DynamicQueueConfig{
				AgeWeight: 10,
				MaxAge:    time.Second * 10,
			},
			workload: &Workload{
				eval: &structs.Evaluation{
					CreateTime: time.Time{}.UnixNano(),
				},
			},
			// nowTime: 30 * time.Second,
			nowTime: time.Time{}.Add(30 * time.Second),
			exp:     10,
		},
		{
			name: "aging eval results in expected adjustment",
			conf: &structs.DynamicQueueConfig{
				AgeWeight: 10,
				MaxAge:    time.Second * 10,
			},
			workload: &Workload{
				eval: &structs.Evaluation{
					CreateTime: time.Time{}.UnixNano(),
				},
			},
			nowTime: time.Time{}.Add(2 * time.Second),
			exp:     2,
		},
	}

	for _, tc := range testCases {
		testQueue := &DynamicPriorityQueue{
			conf: tc.conf,
		}
		must.Eq(t, tc.exp, testQueue.ageAdjustment(tc.nowTime, tc.workload), must.Sprint(tc.name))
	}
}
