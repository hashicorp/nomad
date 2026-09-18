// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package dynamic

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/queues/queue"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

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
						placedWorkloadById: map[structs.NamespacedID]*dynamicPriorityWorkload{
							{ID: eval1.ID}: {
								BaseWorkload: queue.NewBaseWorkload(eval1, mock.Job()),
								requestedResources: &UsageList{start: now.Add(-10 * time.Second),
									resources: &ResourceUsage{
										CPU:    100,
										Memory: 20,
									},
								},
							},
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
						placedWorkloadById: map[structs.NamespacedID]*dynamicPriorityWorkload{
							{ID: eval1.ID}: {
								BaseWorkload: queue.NewBaseWorkload(eval1, mock.Job()),
								requestedResources: &UsageList{start: now.Add(-10 * time.Second),
									resources: &ResourceUsage{
										CPU: 80,
									},
								},
							},
							{ID: eval2.ID}: {
								BaseWorkload: queue.NewBaseWorkload(eval2, mock.Job()),
								requestedResources: &UsageList{start: now.Add(-5 * time.Second),
									resources: &ResourceUsage{
										CPU:    100,
										Memory: 50,
									},
								},
							},
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
						placedWorkloadById: map[structs.NamespacedID]*dynamicPriorityWorkload{
							{ID: eval1.ID}: {
								BaseWorkload: queue.NewBaseWorkload(eval1, mock.Job()),
								requestedResources: &UsageList{start: now.Add(-10 * time.Second),
									resources: &ResourceUsage{
										Memory: 40,
										CPU:    100,
									},
								},
							},
						},
					},
					{
						tid: TenantID("tenantB"),
						placedWorkloadById: map[structs.NamespacedID]*dynamicPriorityWorkload{
							{ID: eval2.ID}: {
								BaseWorkload: queue.NewBaseWorkload(eval2, mock.Job()),
								requestedResources: &UsageList{start: now.Add(-10 * time.Second),
									resources: &ResourceUsage{
										Memory: 80,
										CPU:    75,
									},
								},
							},
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
						placedWorkloadById: map[structs.NamespacedID]*dynamicPriorityWorkload{
							{ID: eval1.ID}: {
								BaseWorkload: queue.NewBaseWorkload(eval1, mock.Job()),
								requestedResources: &UsageList{start: now.Add(-10 * time.Second),
									resources: &ResourceUsage{
										Memory: 40,
										CPU:    100,
									},
								},
							},
							{ID: eval2.ID}: {
								BaseWorkload: queue.NewBaseWorkload(eval2, mock.Job()),
								requestedResources: &UsageList{start: now.Add(-10 * time.Second),
									resources: &ResourceUsage{
										Memory: 80,
										CPU:    60,
									},
								},
							},
						},
					},
					{
						tid: TenantID("tenantB"),
						placedWorkloadById: map[structs.NamespacedID]*dynamicPriorityWorkload{
							{ID: eval1.ID}: {
								BaseWorkload: queue.NewBaseWorkload(eval1, mock.Job()),
								requestedResources: &UsageList{start: now.Add(-10 * time.Second),
									resources: &ResourceUsage{
										Memory: 80,
										CPU:    75,
									},
								},
							},
							{ID: eval2.ID}: {
								BaseWorkload: queue.NewBaseWorkload(eval2, mock.Job()),
								requestedResources: &UsageList{start: now.Add(-10 * time.Second),
									resources: &ResourceUsage{
										Memory: 100,
										CPU:    50,
									},
								},
							},
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
						placedWorkloadById: map[structs.NamespacedID]*dynamicPriorityWorkload{
							{ID: missingEvalID}: {
								BaseWorkload: queue.NewBaseWorkload(&structs.Evaluation{ID: missingEvalID}, mock.Job()),
								requestedResources: &UsageList{start: now.Add(-10 * time.Second),
									resources: &ResourceUsage{
										CPU:    100,
										Memory: 20,
									},
								},
							},
						},
					},
					{
						tid: TenantID("tenantB"),
						placedWorkloadById: map[structs.NamespacedID]*dynamicPriorityWorkload{
							{ID: eval1.ID}: {
								BaseWorkload: queue.NewBaseWorkload(eval1, mock.Job()),
								requestedResources: &UsageList{start: now.Add(-10 * time.Second),
									resources: &ResourceUsage{
										CPU:    100,
										Memory: 20,
									},
								},
							},
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
				queue := NewDynamicPriorityQueue(hclog.New(hclog.DefaultOptions), ss, nil, &structs.DynamicQueueConfig{
					HalfLife: tc.halfLife,
				}, nil)

				for _, tenant := range tc.tenants {
					queue.tenants[tenant.tid] = tenant
				}

				queue.decayUsage(now)

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
			placedWorkloadById: map[structs.NamespacedID]*dynamicPriorityWorkload{
				{ID: eval1.ID}: {
					BaseWorkload: queue.NewBaseWorkload(eval1, mock.Job()),
					requestedResources: &UsageList{start: ts,
						resources: &ResourceUsage{CPU: cpu, Memory: memory},
					},
				},
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
			q := NewDynamicPriorityQueue(hclog.New(hclog.DefaultOptions), ss, nil, tc.conf, nil)

			lowUsageWorkload := &dynamicPriorityWorkload{
				tid:          tc.lowUsageTenant.tid,
				BaseWorkload: queue.NewBaseWorkload(&structs.Evaluation{Priority: 5}, &structs.Job{}),
			}
			highUsageWorkload := &dynamicPriorityWorkload{
				tid:          tc.highUsageTenant.tid,
				BaseWorkload: queue.NewBaseWorkload(&structs.Evaluation{Priority: 5}, &structs.Job{}),
			}

			q.tenants[tc.lowUsageTenant.tid] = tc.lowUsageTenant
			q.tenants[tc.highUsageTenant.tid] = tc.highUsageTenant
			q.queue = queue.NewWorkloadQueue(workloadSortFn())
			q.queue.Push(lowUsageWorkload)
			q.queue.Push(highUsageWorkload)

			q.calculatePriorities(time.Unix(20, 0))

			switch tc.expectedHigherPriorityTenant {
			case tc.lowUsageTenant.tid:
				must.Greater(t, highUsageWorkload.priority, lowUsageWorkload.priority)
			case tc.highUsageTenant.tid:
				must.Greater(t, lowUsageWorkload.priority, highUsageWorkload.priority)
			default:
				t.Fatalf("test case has unknown expectedHigherPriorityTenant: %q", tc.expectedHigherPriorityTenant)
			}

			must.Eq(t, q.totalUsage, tc.expectedTotalUsage, must.Cmp(cmpopts.EquateApprox(0, 1e-9)))
		})
	}
}

func TestDynamicPriorityQueue_resourceAdjustments(t *testing.T) {
	testCases := []struct {
		name     string
		conf     *structs.DynamicQueueConfig
		workload *dynamicPriorityWorkload
		exp      int
	}{
		{
			name: "larger requests results in 0 adjustment",
			conf: &structs.DynamicQueueConfig{
				CpuWeight: 10,
				MaxCpu:    1000,
				MemWeight: 10,
				MaxMemory: 1000,
			},
			workload: &dynamicPriorityWorkload{requestedResources: &UsageList{
				resources: &ResourceUsage{
					CPU:    1000,
					Memory: 1000,
				},
			}},
			exp: 0,
		},
		{
			name: "smaller requests results in expected adjustment",
			conf: &structs.DynamicQueueConfig{
				CpuWeight: 10,
				MaxCpu:    1000,
				MemWeight: 10,
				MaxMemory: 1000,
			},
			workload: &dynamicPriorityWorkload{requestedResources: &UsageList{
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
				CpuWeight: -10,
				MaxCpu:    1000,
				MemWeight: -10,
				MaxMemory: 1000,
			},
			workload: &dynamicPriorityWorkload{requestedResources: &UsageList{
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
		must.Eq(t, tc.exp, testQueue.cpuAdjustment(tc.workload), must.Sprint(tc.name))
		must.Eq(t, tc.exp, testQueue.memAdjustment(tc.workload), must.Sprint(tc.name))
	}
}

func TestDynamicPriorityQueue_ageAdjustment(t *testing.T) {
	testCases := []struct {
		name     string
		conf     *structs.DynamicQueueConfig
		workload *dynamicPriorityWorkload
		nowTime  time.Time
		exp      int
	}{
		{
			name: "createTime and now equal results in 0 age adjustment",
			conf: &structs.DynamicQueueConfig{
				AgeWeight: 10,
				MaxAge:    time.Second * 10,
			},
			workload: &dynamicPriorityWorkload{
				BaseWorkload: queue.NewBaseWorkload(&structs.Evaluation{}, mock.Job()),
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
			workload: &dynamicPriorityWorkload{
				BaseWorkload: queue.NewBaseWorkload(&structs.Evaluation{
					CreateTime: time.Time{}.UnixNano(),
				}, mock.Job()),
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
			workload: &dynamicPriorityWorkload{
				BaseWorkload: queue.NewBaseWorkload(&structs.Evaluation{
					CreateTime: time.Time{}.UnixNano(),
				}, mock.Job()),
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
		sortOrder structs.SortOrder
		placing   int
		completed int
		workloads []*dynamicPriorityWorkload
		exp       *queue.WorkloadIter
	}{
		{
			name:      "status response parses workloads correctly",
			sortOrder: structs.SortDefault,
			workloads: []*dynamicPriorityWorkload{
				{
					BaseWorkload: queue.NewBaseWorkload(&structs.Evaluation{
						ID:          "eval1",
						JobID:       "job1",
						Priority:    50,
						CreateTime:  time.Unix(20, 0).UnixNano(),
						CreateIndex: 10,
					}, &structs.Job{ID: "job1"}),
					tid:             "tenantA",
					priority:        59,
					ageAdjustment:   3,
					usageAdjustment: 4,
				},
			},
			exp: &queue.WorkloadIter{
				Workloads: []structs.QueueWorkload{
					&structs.DynamicPriorityWorkload{
						JobID:            "job1",
						Tenant:           "tenantA",
						Position:         1,
						AdjustedPriority: 59,
						BasePriority:     50,
						AgeAdjustment:    3,
						UsageAdjustment:  4,
						CreatedAt:        time.Unix(20, 0).UnixNano(),
						CreateIndex:      10,
					},
				},
			},
		},
		{
			name:      "default sort returns workloads in order of jobID",
			sortOrder: structs.SortDefault,
			workloads: []*dynamicPriorityWorkload{
				{
					BaseWorkload: queue.NewBaseWorkload(&structs.Evaluation{
						ID:          "eval3",
						JobID:       "job3",
						Priority:    50,
						CreateIndex: 14,
					}, &structs.Job{ID: "job3"}),
					tid:             "tenantA",
					priority:        59,
					ageAdjustment:   3,
					usageAdjustment: 4,
				},
				{
					BaseWorkload: queue.NewBaseWorkload(&structs.Evaluation{
						ID:          "eval2",
						JobID:       "job2",
						Priority:    50,
						CreateIndex: 12,
					}, &structs.Job{ID: "job2"}),
					tid:             "tenantA",
					priority:        66,
					ageAdjustment:   3,
					usageAdjustment: 4,
				},
				{
					BaseWorkload: queue.NewBaseWorkload(&structs.Evaluation{
						ID:          "eval1",
						JobID:       "job1",
						Priority:    50,
						CreateIndex: 10,
					}, &structs.Job{ID: "job1"}),
					tid:             "tenantA",
					priority:        51,
					ageAdjustment:   0,
					usageAdjustment: 1,
				},
			},
			exp: &queue.WorkloadIter{
				Workloads: []structs.QueueWorkload{
					&structs.DynamicPriorityWorkload{
						JobID:            "job1",
						Tenant:           "tenantA",
						Position:         3,
						AdjustedPriority: 51,
						BasePriority:     50,
						AgeAdjustment:    0,
						UsageAdjustment:  1,
						CreateIndex:      10,
					},
					&structs.DynamicPriorityWorkload{
						JobID:            "job2",
						Tenant:           "tenantA",
						Position:         1,
						AdjustedPriority: 66,
						BasePriority:     50,
						AgeAdjustment:    3,
						UsageAdjustment:  4,
						CreateIndex:      12,
					},
					&structs.DynamicPriorityWorkload{
						JobID:            "job3",
						Tenant:           "tenantA",
						Position:         2,
						AdjustedPriority: 59,
						BasePriority:     50,
						AgeAdjustment:    3,
						UsageAdjustment:  4,
						CreateIndex:      14,
					},
				},
			},
		},
		{
			name:      "priority order returns workloads in order of adjusted priority",
			sortOrder: structs.SortByPriority,
			workloads: []*dynamicPriorityWorkload{
				{
					BaseWorkload: queue.NewBaseWorkload(&structs.Evaluation{
						ID:          "eval1",
						JobID:       "job1",
						Priority:    50,
						CreateIndex: 14,
					}, &structs.Job{ID: "job1"}),
					tid:             "tenantA",
					priority:        59,
					ageAdjustment:   3,
					usageAdjustment: 4,
				},
				{
					BaseWorkload: queue.NewBaseWorkload(&structs.Evaluation{
						ID:          "eval2",
						JobID:       "job2",
						Priority:    50,
						CreateIndex: 12,
					}, &structs.Job{ID: "job2"}),
					tid:             "tenantA",
					priority:        66,
					ageAdjustment:   3,
					usageAdjustment: 4,
				},
				{
					BaseWorkload: queue.NewBaseWorkload(&structs.Evaluation{
						ID:          "eval3",
						JobID:       "job3",
						Priority:    50,
						CreateIndex: 10,
					}, &structs.Job{ID: "job3"}),
					tid:             "tenantA",
					priority:        51,
					ageAdjustment:   0,
					usageAdjustment: 1,
				},
			},
			exp: &queue.WorkloadIter{
				Workloads: []structs.QueueWorkload{
					&structs.DynamicPriorityWorkload{
						JobID:            "job2",
						Tenant:           "tenantA",
						Position:         1,
						AdjustedPriority: 66,
						BasePriority:     50,
						AgeAdjustment:    3,
						UsageAdjustment:  4,
						CreateIndex:      12,
					},
					&structs.DynamicPriorityWorkload{
						JobID:            "job1",
						Tenant:           "tenantA",
						Position:         2,
						AdjustedPriority: 59,
						BasePriority:     50,
						AgeAdjustment:    3,
						UsageAdjustment:  4,
						CreateIndex:      14,
					},
					&structs.DynamicPriorityWorkload{
						JobID:            "job3",
						Tenant:           "tenantA",
						Position:         3,
						AdjustedPriority: 51,
						BasePriority:     50,
						AgeAdjustment:    0,
						UsageAdjustment:  1,
						CreateIndex:      10,
					},
				},
			},
		},
		{
			name:      "priority order falls back to createIndex if priority is equal",
			sortOrder: structs.SortByPriority,
			workloads: []*dynamicPriorityWorkload{
				{
					BaseWorkload: queue.NewBaseWorkload(&structs.Evaluation{
						ID:          "eval2",
						JobID:       "job1",
						Priority:    50,
						CreateTime:  time.Unix(20, 0).UnixNano(),
						CreateIndex: 12,
					}, &structs.Job{ID: "job1"}),
					tid:             "tenantA",
					priority:        59,
					ageAdjustment:   3,
					usageAdjustment: 4,
				},
				{
					BaseWorkload: queue.NewBaseWorkload(&structs.Evaluation{
						ID:          "eval2",
						JobID:       "job2",
						Priority:    50,
						CreateTime:  time.Unix(10, 0).UnixNano(),
						CreateIndex: 10,
					}, &structs.Job{ID: "job2"}),
					tid:             "tenantA",
					priority:        59,
					ageAdjustment:   3,
					usageAdjustment: 4,
				},
			},
			exp: &queue.WorkloadIter{
				Workloads: []structs.QueueWorkload{
					&structs.DynamicPriorityWorkload{
						JobID:            "job2",
						Tenant:           "tenantA",
						Position:         1,
						AdjustedPriority: 59,
						BasePriority:     50,
						AgeAdjustment:    3,
						UsageAdjustment:  4,
						CreatedAt:        time.Unix(10, 0).UnixNano(),
						CreateIndex:      10,
					},
					&structs.DynamicPriorityWorkload{
						JobID:            "job1",
						Tenant:           "tenantA",
						Position:         2,
						AdjustedPriority: 59,
						BasePriority:     50,
						AgeAdjustment:    3,
						UsageAdjustment:  4,
						CreatedAt:        time.Unix(20, 0).UnixNano(),
						CreateIndex:      12,
					},
				},
			},
		},
		{
			name:      "tracks placing workloads and returns them in the results",
			sortOrder: structs.SortByPriority,
			placing:   1,
			workloads: []*dynamicPriorityWorkload{
				{
					id:  "eval1",
					tid: "tenantA",
					eval: &structs.Evaluation{
						ID:          "eval1",
						JobID:       "job1",
						Priority:    50,
						CreateTime:  time.Unix(20, 0).UnixNano(),
						CreateIndex: 12,
					},
					priority:        59,
					status:          "queued",
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
						CreateIndex: 10,
					},
					priority:        60,
					status:          "placing",
					ageAdjustment:   3,
					usageAdjustment: 4,
				},
			},
			exp: &queue.WorkloadIter{
				Workloads: []structs.QueueWorkload{
					&structs.DynamicPriorityWorkload{
						JobID:            "job2",
						Tenant:           "tenantA",
						Position:         0,
						Status:           "placing",
						AdjustedPriority: 60,
						BasePriority:     50,
						AgeAdjustment:    3,
						UsageAdjustment:  4,
						CreatedAt:        time.Unix(10, 0).UnixNano(),
						CreateIndex:      10,
					},
					&structs.DynamicPriorityWorkload{
						JobID:            "job1",
						Tenant:           "tenantA",
						Position:         1,
						Status:           "queued",
						AdjustedPriority: 59,
						BasePriority:     50,
						AgeAdjustment:    3,
						UsageAdjustment:  4,
						CreatedAt:        time.Unix(20, 0).UnixNano(),
						CreateIndex:      12,
					},
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ss := state.TestStateStore(t)
			testQueue := NewDynamicPriorityQueue(hclog.New(hclog.DefaultOptions), ss, nil, &structs.DynamicQueueConfig{TenantType: "namespace"})
			testQueue.queue = queue.NewWorkloadQueue(workloadSortFn())

			for _, w := range tc.workloads {
				if w.status == "placing" {
					testQueue.watcher.TrackPlacement(w)
				} else {
					testQueue.queue.Push(w)
				}
			}

			got := testQueue.Jobs(tc.sortOrder)
			must.Eq(t, len(tc.exp.Workloads), len(got.Workloads))
			for i := range tc.exp.Workloads {
				expWorkload := tc.exp.Workloads[i].(*structs.DynamicPriorityWorkload)
				gotWorkload := got.Workloads[i].(*structs.DynamicPriorityWorkload)
				must.Eq(t, *expWorkload, *gotWorkload)
			}
		})
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

func TestDynamicPriorityQueue_restore(t *testing.T) {
	t.Run("unplaced workload is enqueued", func(t *testing.T) {
		ss := state.TestStateStore(t)
		testQueue := NewDynamicPriorityQueue(hclog.New(hclog.DefaultOptions), ss, nil, &structs.DynamicQueueConfig{
			TenantType: structs.TenantTypeNamespace,
		}, nil)

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

		err := testQueue.Restore(testEval, job)
		must.NoError(t, err)

		// Verify the workload was enqueued
		select {
		case w := <-testQueue.enqueueCh:
			must.Eq(t, testEval.ID, w.Eval().ID)
			must.Eq(t, TenantID(job.Namespace), w.tid)
			must.True(t, w.WaitOnRestore())
		default:
			t.Fatal("expected workload in enqueueCh channel")
		}
	})

	t.Run("restores usage correctly", func(t *testing.T) {
		ss := state.TestStateStore(t)
		testQueue := NewDynamicPriorityQueue(hclog.New(hclog.DefaultOptions), ss, nil, &structs.DynamicQueueConfig{
			TenantType: structs.TenantTypeNamespace,
			HalfLife:   10 * time.Second,
		}, nil)

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

		err := testQueue.Restore(testEval, job)
		must.NoError(t, err)

		// Verify tenant was created and usage was tracked
		tenant, ok := testQueue.tenants[TenantID(job.Namespace)]
		must.True(t, ok)
		must.NotNil(t, tenant)

		// Verify the workload is tracked
		workload, ok := tenant.placedWorkloadById[job.NamespacedID()]
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
		testQueue := NewDynamicPriorityQueue(hclog.New(hclog.DefaultOptions), ss, nil, &structs.DynamicQueueConfig{
			TenantType: structs.TenantTypeNamespace,
			HalfLife:   halfLife,
		}, nil)

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

		err := testQueue.Restore(testEval, job)
		must.NoError(t, err)

		// Restore records usage starting at eval.ModifyTime; decay is applied
		// separately during queue startup and periodic recalculation.
		testQueue.decayUsage(now)

		// Verify tenant was created and usage was tracked
		tenant, ok := testQueue.tenants[TenantID(job.Namespace)]
		must.True(t, ok)
		must.NotNil(t, tenant)

		// Verify the workload is tracked
		workload, ok := tenant.placedWorkloadById[job.NamespacedID()]
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

func TestDynamicPriorityQueue_cancelRedundant(t *testing.T) {
	newWorkload := func(evalID string, jobVersion uint64) *dynamicPriorityWorkload {
		j := mock.Job()
		j.ID = "test-job-ID"
		j.Version = jobVersion

		e := mock.Eval()
		e.ID = evalID
		e.JobID = j.ID

		return &dynamicPriorityWorkload{
			BaseWorkload: queue.NewBaseWorkload(e, j),
		}
	}

	newQueue := func() (*DynamicPriorityQueue, []string) {
		cancelled := make([]string, 1)
		q := &DynamicPriorityQueue{
			queue: queue.NewWorkloadQueue(workloadSortFn()),
			evalCancelFn: func(e *structs.Evaluation) error {
				cancelled[0] = e.ID
				return nil
			},
		}
		return q, cancelled
	}

	t.Run("no existing workload return false", func(t *testing.T) {
		q, cancelled := newQueue()

		must.False(t, q.cancelRedundant(newWorkload("test", 1)))
		must.Eq(t, "", cancelled[0])
	})

	t.Run("incoming workload with higher version is inserted", func(t *testing.T) {
		q, cancelled := newQueue()
		// seed queue with workload
		initial := newWorkload("initial", 1)
		q.queue.Push(initial)

		newVersion := newWorkload("new", 2)
		must.True(t, q.cancelRedundant(newVersion))
		must.Eq(t, cancelled[0], initial.Eval().ID)

		// get the new workload from the queue and assert it is
		// the new version workload
		wl, ok := q.queue.Get(newVersion.ID())
		must.True(t, ok)
		must.Eq(t, wl, newVersion)
	})

	t.Run("incoming redundant workload is cancelled", func(t *testing.T) {
		q, cancelled := newQueue()
		// seed queue with workload
		initial := newWorkload("initial", 1)
		q.queue.Push(initial)

		redundant := newWorkload("redundant", 1)
		must.True(t, q.cancelRedundant(redundant))
		must.Eq(t, cancelled[0], redundant.Eval().ID)

		// Get the original workload from the queue to assert
		// the redundant workload was not inserted.
		wl, ok := q.queue.Get(initial.ID())
		must.True(t, ok)
		must.Eq(t, wl, initial)
	})
}
