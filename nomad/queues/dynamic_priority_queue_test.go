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
		testQueue := NewDynamicPriorityQueue(ss, nil, &structs.BatchQueue{}, &structs.DynamicQueueConfig{}, hclog.New(hclog.DefaultOptions))

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
		testQueue := NewDynamicPriorityQueue(ss, nil, &structs.BatchQueue{}, &structs.DynamicQueueConfig{}, hclog.New(hclog.DefaultOptions))

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
		testQueue := NewDynamicPriorityQueue(ss, nil, &structs.BatchQueue{}, &structs.DynamicQueueConfig{}, hclog.New(hclog.DefaultOptions))

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
				queue := NewDynamicPriorityQueue(ss, nil, &structs.BatchQueue{}, &structs.DynamicQueueConfig{
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
			queue := NewDynamicPriorityQueue(ss, nil, &structs.BatchQueue{}, tc.conf, hclog.New(hclog.DefaultOptions))

			lowUsageWorkload := &Workload{tid: tc.lowUsageTenant.tid, eval: &structs.Evaluation{Priority: 5}}
			highUsageWorkload := &Workload{tid: tc.highUsageTenant.tid, eval: &structs.Evaluation{Priority: 5}}

			queue.tenants[tc.lowUsageTenant.tid] = tc.lowUsageTenant
			queue.tenants[tc.highUsageTenant.tid] = tc.highUsageTenant
			queue.queue = NewWorkloadQueue()
			queue.queue.Push(lowUsageWorkload)
			queue.queue.Push(highUsageWorkload)

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

func TestDynamicPriorityQueue_Jobs(t *testing.T) {
	testCases := []struct {
		name      string
		allowedNs map[string]bool
		workloads []*Workload
		exp       structs.QueueJobsResponse
	}{
		{
			name: "status response parses workloads correctly",
			workloads: []*Workload{
				{
					id:  "eval1",
					tid: "tenantA",
					eval: &structs.Evaluation{
						ID:       "eval1",
						JobID:    "job1",
						Priority: 50,
					},
					priority:        59,
					sizeAdjustment:  2,
					ageAdjustment:   3,
					usageAdjustment: 4,
				},
				{
					id:  "eval2",
					tid: "tenantA",
					eval: &structs.Evaluation{
						ID:       "eval2",
						JobID:    "job2",
						Priority: 50,
					},
					priority:        66,
					sizeAdjustment:  12,
					ageAdjustment:   3,
					usageAdjustment: 4,
				},
				{
					id:  "eval3",
					tid: "tenantA",
					eval: &structs.Evaluation{
						ID:       "eval3",
						JobID:    "job3",
						Priority: 50,
					},
					priority:        51,
					sizeAdjustment:  0,
					ageAdjustment:   0,
					usageAdjustment: 1,
				},
			},
			exp: structs.QueueJobsResponse{
				Type: structs.BatchQueueTypeDynamic,
				Workloads: []structs.DynamicPriorityWorkload{
					{
						JobID:            "job2",
						Tenant:           "tenantA",
						Position:         1,
						AdjustedPriority: 66,
						BasePriority:     50,
						SizeAdjustment:   12,
						AgeAdjustment:    3,
						UsageAdjustment:  4,
					},
					{
						JobID:            "job1",
						Tenant:           "tenantA",
						Position:         2,
						AdjustedPriority: 59,
						BasePriority:     50,
						SizeAdjustment:   2,
						AgeAdjustment:    3,
						UsageAdjustment:  4,
					},
					{
						JobID:            "job3",
						Tenant:           "tenantA",
						Position:         3,
						AdjustedPriority: 51,
						BasePriority:     50,
						SizeAdjustment:   0,
						AgeAdjustment:    0,
						UsageAdjustment:  1,
					},
				},
			},
		},
		{
			name:      "status response screens workloads",
			allowedNs: map[string]bool{"ns1": true},
			workloads: []*Workload{
				{
					id:  "eval1",
					tid: "tenantA",
					eval: &structs.Evaluation{
						ID:        "eval1",
						JobID:     "job1",
						Namespace: "ns1",
						Priority:  50,
					},
					priority:        59,
					sizeAdjustment:  2,
					ageAdjustment:   3,
					usageAdjustment: 4,
				},
				{
					id:  "eval2",
					tid: "tenantB",
					eval: &structs.Evaluation{
						ID:        "eval2",
						JobID:     "job2",
						Namespace: "ns2",
						Priority:  50,
					},
					priority:        63,
					sizeAdjustment:  7,
					ageAdjustment:   1,
					usageAdjustment: 5,
				},
				{
					id:  "eval2",
					tid: "tenantA",
					eval: &structs.Evaluation{
						ID:        "eval2",
						JobID:     "job2",
						Namespace: "ns1",
						Priority:  50,
					},
					priority:        66,
					sizeAdjustment:  7,
					ageAdjustment:   1,
					usageAdjustment: 5,
				},
			},
			exp: structs.QueueJobsResponse{
				Type: structs.BatchQueueTypeDynamic,
				Workloads: []structs.DynamicPriorityWorkload{
					{
						JobID:            "job2",
						Tenant:           "tenantA",
						Position:         1,
						AdjustedPriority: 66,
						BasePriority:     50,
						SizeAdjustment:   7,
						AgeAdjustment:    1,
						UsageAdjustment:  5,
					},
					{
						JobID:            "job1",
						Tenant:           "tenantA",
						Position:         3,
						AdjustedPriority: 59,
						BasePriority:     50,
						SizeAdjustment:   2,
						AgeAdjustment:    3,
						UsageAdjustment:  4,
					},
				},
			},
		},
		{
			name: "order falls back to createIndex if priority is equal",
			workloads: []*Workload{
				{
					id:  "eval1",
					tid: "tenantA",
					eval: &structs.Evaluation{
						ID:          "eval1",
						JobID:       "job1",
						Priority:    50,
						CreateTime:  time.Unix(20, 0).UnixNano(),
						CreateIndex: 2,
					},
					priority:        59,
					sizeAdjustment:  2,
					ageAdjustment:   3,
					usageAdjustment: 4,
				},
				{
					id:  "eval2",
					tid: "tenantA",
					eval: &structs.Evaluation{
						ID:          "eval2",
						JobID:       "job2",
						Priority:    50,
						CreateTime:  time.Unix(10, 0).UnixNano(),
						CreateIndex: 1,
					},
					priority:        59,
					sizeAdjustment:  2,
					ageAdjustment:   3,
					usageAdjustment: 4,
				},
			},
			exp: structs.QueueJobsResponse{
				Type: structs.BatchQueueTypeDynamic,
				Workloads: []structs.DynamicPriorityWorkload{
					{
						JobID:            "job2",
						Tenant:           "tenantA",
						Position:         1,
						AdjustedPriority: 59,
						BasePriority:     50,
						SizeAdjustment:   2,
						AgeAdjustment:    3,
						UsageAdjustment:  4,
						CreatedAt:        time.Unix(10, 0).UnixNano(),
					},
					{
						JobID:            "job1",
						Tenant:           "tenantA",
						Position:         2,
						AdjustedPriority: 59,
						BasePriority:     50,
						SizeAdjustment:   2,
						AgeAdjustment:    3,
						UsageAdjustment:  4,
						CreatedAt:        time.Unix(20, 0).UnixNano(),
					},
				},
			},
		},
	}
	for _, tc := range testCases {
		testQueue := &DynamicPriorityQueue{
			queue: NewWorkloadQueue(),
		}
		for _, w := range tc.workloads {
			testQueue.queue.Push(w)
		}
		must.Eq(t, tc.exp, testQueue.Jobs(tc.allowedNs))
	}

}

func TestDynamicPriorityQueue_Tenants(t *testing.T) {
	testCases := []struct {
		name       string
		tenants    map[TenantID]*Tenant
		totalUsage *ResourceUsage
		exp        structs.QueueTenantsResponse
	}{
		{
			name: "status response parses tenants correctly",
			tenants: map[TenantID]*Tenant{
				"tenantA": {
					tid: "tenantA",
					totalUsage: &ResourceUsage{
						CPU:    100,
						Memory: 200,
					},
				},
			},
			totalUsage: &ResourceUsage{
				CPU:    400,
				Memory: 300,
			},
			exp: structs.QueueTenantsResponse{
				Type: structs.BatchQueueTypeDynamic,
				Tenants: []structs.DynamicPriorityTenant{
					{
						TenantID:       "tenantA",
						PercentageUsed: 42,
						TenantUsage:    map[string]float64{"cpu": 100, "memory": 200},
						TotalUsage:     map[string]float64{"cpu": 400, "memory": 300},
					},
				},
			},
		},
	}
	for _, tc := range testCases {
		testQueue := &DynamicPriorityQueue{
			tenants:    tc.tenants,
			totalUsage: tc.totalUsage,
		}
		must.Eq(t, tc.exp, testQueue.Tenants())
	}
}

func TestDynamicPriorityQueue_isSchedulingComplete(t *testing.T) {
	t.Run("pending eval results in false", func(t *testing.T) {
		ss := state.TestStateStore(t)
		testQueue := NewDynamicPriorityQueue(ss, nil, &structs.BatchQueue{}, &structs.DynamicQueueConfig{}, hclog.New(hclog.DefaultOptions))

		testEval := mock.Eval()
		testEval.Status = structs.EvalStatusPending
		ss.UpsertEvals(structs.MsgTypeTestSetup, 0, []*structs.Evaluation{testEval})

		workload := &Workload{
			id:   testEval.ID,
			tid:  TenantID("tenant"),
			eval: testEval.Copy(),
		}

		complete, err := testQueue.isSchedulingComplete(workload)
		must.NoError(t, err)
		must.False(t, complete)
	})

	t.Run("eval with pending blockedEval results in false", func(t *testing.T) {
		ss := state.TestStateStore(t)
		testQueue := NewDynamicPriorityQueue(ss, nil, &structs.BatchQueue{}, &structs.DynamicQueueConfig{}, hclog.New(hclog.DefaultOptions))

		testEval := mock.Eval()
		blocked := mock.Eval()

		testEval.Status = structs.EvalStatusComplete
		testEval.BlockedEval = blocked.ID
		blocked.Status = structs.EvalStatusPending

		ss.UpsertEvals(structs.MsgTypeTestSetup, 0, []*structs.Evaluation{testEval, blocked})

		workload := &Workload{
			id:   testEval.ID,
			tid:  TenantID("tenant"),
			eval: testEval.Copy(),
		}

		complete, err := testQueue.isSchedulingComplete(workload)
		must.NoError(t, err)
		must.False(t, complete)
	})

	t.Run("eval with complete blockedEval results in true", func(t *testing.T) {
		ss := state.TestStateStore(t)
		testQueue := NewDynamicPriorityQueue(ss, nil, &structs.BatchQueue{}, &structs.DynamicQueueConfig{}, hclog.New(hclog.DefaultOptions))

		testEval := mock.Eval()
		blocked := mock.Eval()

		testEval.Status = structs.EvalStatusComplete
		testEval.BlockedEval = blocked.ID
		blocked.Status = structs.EvalStatusComplete

		ss.UpsertEvals(structs.MsgTypeTestSetup, 0, []*structs.Evaluation{testEval, blocked})

		workload := &Workload{
			id:   testEval.ID,
			tid:  TenantID("tenant"),
			eval: testEval.Copy(),
		}

		complete, err := testQueue.isSchedulingComplete(workload)
		must.NoError(t, err)
		must.True(t, complete)
	})

	t.Run("complete eval with placement updates usage", func(t *testing.T) {
		ss := state.TestStateStore(t)
		testQueue := NewDynamicPriorityQueue(ss, nil, &structs.BatchQueue{}, &structs.DynamicQueueConfig{}, hclog.New(hclog.DefaultOptions))

		testEval := mock.Eval()
		testEval.Status = structs.EvalStatusComplete
		testEval.PlanAnnotations = &structs.PlanAnnotations{
			DesiredTGUpdates: map[string]*structs.DesiredUpdates{
				"group": {Place: 1},
			},
		}

		ss.UpsertEvals(structs.MsgTypeTestSetup, 0, []*structs.Evaluation{testEval})

		workload := &Workload{
			id:   testEval.ID,
			tid:  TenantID("tenant"),
			eval: testEval.Copy(),
			requestedResources: &UsageList{
				resources: &ResourceUsage{CPU: 100, Memory: 200},
			},
		}

		testQueue.ensureTenant(workload.tid)

		complete, err := testQueue.isSchedulingComplete(workload)
		must.NoError(t, err)
		must.True(t, complete)

		// Verify usage was updated
		tenant := testQueue.tenants[workload.tid]
		must.NotNil(t, tenant.placedWorkloadById[workload.id])
		must.Eq(t, 100.0, tenant.totalUsage.CPU)
		must.Eq(t, 200.0, tenant.totalUsage.Memory)
	})

	t.Run("complete eval without placement does not update usage", func(t *testing.T) {
		ss := state.TestStateStore(t)
		testQueue := NewDynamicPriorityQueue(ss, nil, &structs.BatchQueue{}, &structs.DynamicQueueConfig{}, hclog.New(hclog.DefaultOptions))

		testEval := mock.Eval()
		testEval.Status = structs.EvalStatusComplete
		testEval.PlanAnnotations = &structs.PlanAnnotations{
			DesiredTGUpdates: map[string]*structs.DesiredUpdates{
				"group": {Place: 0},
			},
		}

		ss.UpsertEvals(structs.MsgTypeTestSetup, 0, []*structs.Evaluation{testEval})

		workload := &Workload{
			id:   testEval.ID,
			tid:  TenantID("tenant"),
			eval: testEval.Copy(),
			requestedResources: &UsageList{
				resources: &ResourceUsage{CPU: 100, Memory: 200},
			},
		}

		testQueue.ensureTenant(workload.tid)

		complete, err := testQueue.isSchedulingComplete(workload)
		must.NoError(t, err)
		must.True(t, complete)

		// Verify usage was NOT updated
		tenant := testQueue.tenants[workload.tid]
		must.Nil(t, tenant.placedWorkloadById[workload.id])
		must.Eq(t, 0.0, tenant.totalUsage.CPU)
		must.Eq(t, 0.0, tenant.totalUsage.Memory)
	})
}

func TestDynamicPriorityQueue_restore(t *testing.T) {
	t.Run("unplaced workload is enqueued", func(t *testing.T) {
		ss := state.TestStateStore(t)
		testQueue := NewDynamicPriorityQueue(ss, nil, &structs.BatchQueue{
			TenantType: "namespace",
		}, &structs.DynamicQueueConfig{}, hclog.New(hclog.DefaultOptions))

		// Set the state store before calling restore
		testQueue.state = ss

		// Create a job and eval that hasn't been placed yet
		job := mock.Job()
		job.Type = structs.JobTypeBatch
		ss.UpsertJob(structs.MsgTypeTestSetup, 0, nil, job)

		now := time.Now()

		testEval := mock.Eval()
		testEval.JobID = job.ID
		testEval.Namespace = job.Namespace
		testEval.Type = structs.JobTypeBatch
		testEval.TriggeredBy = structs.EvalTriggerJobRegister
		testEval.Status = structs.EvalStatusBlocked
		testEval.CreateTime = now.UnixNano()
		ss.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{testEval})

		snap, err := ss.Snapshot()
		must.NoError(t, err)

		err = testQueue.restore(snap, now)
		must.NoError(t, err)

		// Verify the workload was enqueued
		select {
		case w := <-testQueue.enqueueCh:
			must.Eq(t, testEval.ID, w.id)
			must.Eq(t, TenantID(job.Namespace), w.tid)
			must.True(t, w.waitOnRestore)
		default:
			t.Fatal("expected workload in enqueueCh channel")
		}
	})

	t.Run("skips pending/non-batch/non-register evals", func(t *testing.T) {
		ss := state.TestStateStore(t)
		testQueue := NewDynamicPriorityQueue(ss, nil, &structs.BatchQueue{
			TenantType: "namespace",
		}, &structs.DynamicQueueConfig{}, hclog.New(hclog.DefaultOptions))

		// Set the state store before calling restore
		testQueue.state = ss

		// Create jobs for different eval types
		batchJob := mock.Job()
		batchJob.Type = structs.JobTypeBatch
		ss.UpsertJob(structs.MsgTypeTestSetup, 0, nil, batchJob)

		serviceJob := mock.Job()
		serviceJob.Type = structs.JobTypeService
		ss.UpsertJob(structs.MsgTypeTestSetup, 1, nil, serviceJob)

		// Create various evals that should be skipped
		pendingEval := mock.Eval()
		pendingEval.JobID = batchJob.ID
		pendingEval.Namespace = batchJob.Namespace
		pendingEval.Type = structs.JobTypeBatch
		pendingEval.TriggeredBy = structs.EvalTriggerJobRegister
		pendingEval.Status = structs.EvalStatusPending

		serviceEval := mock.Eval()
		serviceEval.JobID = serviceJob.ID
		serviceEval.Type = structs.JobTypeService

		nonRegisterEval := mock.Eval()
		nonRegisterEval.JobID = batchJob.ID
		nonRegisterEval.Namespace = batchJob.Namespace
		nonRegisterEval.Type = structs.JobTypeBatch
		nonRegisterEval.TriggeredBy = structs.EvalTriggerNodeUpdate
		nonRegisterEval.Status = structs.EvalStatusComplete

		ss.UpsertEvals(structs.MsgTypeTestSetup, 2, []*structs.Evaluation{
			pendingEval,
			serviceEval,
			nonRegisterEval,
		})

		snap, err := ss.Snapshot()
		must.NoError(t, err)

		err = testQueue.restore(snap, time.Now())
		must.NoError(t, err)

		// Verify no tenants were created (all evals should be skipped)
		must.Eq(t, 0, len(testQueue.tenants))
	})

	t.Run("restores usage correctly", func(t *testing.T) {
		ss := state.TestStateStore(t)
		testQueue := NewDynamicPriorityQueue(ss, nil, &structs.BatchQueue{
			TenantType: "namespace",
		}, &structs.DynamicQueueConfig{
			HalfLife: 10 * time.Second,
		}, hclog.New(hclog.DefaultOptions))

		// Set the state store before calling restore
		testQueue.state = ss

		// Create a job with task resources
		job := mock.Job()
		job.Type = structs.JobTypeBatch
		job.TaskGroups[0].Count = 2
		job.TaskGroups[0].Tasks[0].Resources.CPU = 100
		job.TaskGroups[0].Tasks[0].Resources.MemoryMB = 256
		ss.UpsertJob(structs.MsgTypeTestSetup, 0, nil, job)

		now := time.Now()

		// Create a completed eval with placement
		testEval := mock.Eval()
		testEval.JobID = job.ID
		testEval.Namespace = job.Namespace
		testEval.Type = structs.JobTypeBatch
		testEval.TriggeredBy = structs.EvalTriggerJobRegister
		testEval.Status = structs.EvalStatusComplete
		testEval.PlanAnnotations = &structs.PlanAnnotations{
			DesiredTGUpdates: map[string]*structs.DesiredUpdates{
				job.TaskGroups[0].Name: {Place: 2},
			},
		}
		testEval.ModifyTime = now.UnixNano()
		ss.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{testEval})

		snap, err := ss.Snapshot()
		must.NoError(t, err)

		err = testQueue.restore(snap, now)
		must.NoError(t, err)

		// Verify tenant was created and usage was tracked
		tenant, ok := testQueue.tenants[TenantID(job.Namespace)]
		must.True(t, ok)
		must.NotNil(t, tenant)

		// Verify the workload is tracked
		workload, ok := tenant.placedWorkloadById[testEval.ID]
		must.True(t, ok)
		must.NotNil(t, workload)

		// Expected resources: 2 tasks * (100 CPU + 256 MB)
		expectedCPU := 200.0
		expectedMemory := 512.0

		must.Eq(t, expectedCPU, tenant.totalUsage.CPU)
		must.Eq(t, expectedMemory, tenant.totalUsage.Memory)
		must.Eq(t, expectedCPU, testQueue.totalUsage.CPU)
		must.Eq(t, expectedMemory, testQueue.totalUsage.Memory)
	})

	t.Run("decays usage properly", func(t *testing.T) {
		ss := state.TestStateStore(t)
		halfLife := 10 * time.Second
		testQueue := NewDynamicPriorityQueue(ss, nil, &structs.BatchQueue{
			TenantType: "namespace",
		}, &structs.DynamicQueueConfig{
			HalfLife: halfLife,
		}, hclog.New(hclog.DefaultOptions))

		// Set the state store before calling restore
		testQueue.state = ss

		// Create a job with task resources
		job := mock.Job()
		job.Type = structs.JobTypeBatch
		job.TaskGroups[0].Count = 1
		job.TaskGroups[0].Tasks[0].Resources.CPU = 100
		job.TaskGroups[0].Tasks[0].Resources.MemoryMB = 256
		ss.UpsertJob(structs.MsgTypeTestSetup, 0, nil, job)

		// Set restore time to be exactly one half-life after eval creation
		now := time.Now()
		evalCreateTime := now.Add(-halfLife)

		// Create a completed eval with placement
		testEval := mock.Eval()
		testEval.JobID = job.ID
		testEval.Namespace = job.Namespace
		testEval.Type = structs.JobTypeBatch
		testEval.TriggeredBy = structs.EvalTriggerJobRegister
		testEval.Status = structs.EvalStatusComplete
		testEval.PlanAnnotations = &structs.PlanAnnotations{
			DesiredTGUpdates: map[string]*structs.DesiredUpdates{
				job.TaskGroups[0].Name: {Place: 2},
			},
		}
		testEval.ModifyTime = evalCreateTime.UnixNano()
		ss.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{testEval})

		snap, err := ss.Snapshot()
		must.NoError(t, err)

		err = testQueue.restore(snap, now)
		must.NoError(t, err)

		// Verify tenant was created and usage was tracked
		tenant, ok := testQueue.tenants[TenantID(job.Namespace)]
		must.True(t, ok)
		must.NotNil(t, tenant)

		// Verify the workload is tracked
		workload, ok := tenant.placedWorkloadById[testEval.ID]
		must.True(t, ok)
		must.NotNil(t, workload)

		// Expected resources after decay: 1 task (100 CPU + 256 MB) / 2 (half-life decay)
		expectedCPU := 50.0
		expectedMemory := 128.0

		must.Eq(t, expectedCPU, tenant.totalUsage.CPU)
		must.Eq(t, expectedMemory, tenant.totalUsage.Memory)
		must.Eq(t, expectedCPU, testQueue.totalUsage.CPU)
		must.Eq(t, expectedMemory, testQueue.totalUsage.Memory)
	})
}
