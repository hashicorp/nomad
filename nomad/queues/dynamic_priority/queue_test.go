// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package dynamic

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/queues/queue"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestDynamicPriorityQueue_calculatePriorities(t *testing.T) {
	eval1 := mock.Eval()
	mkTenant := func(id TenantID, cpu, memory float64) *Tenant {
		return &Tenant{
			tid: id,
			placedWorkloadById: map[structs.NamespacedID]*dynamicPriorityWorkload{
				{ID: eval1.ID}: {
					BaseWorkload:       queue.NewBaseWorkload(eval1, mock.Job(), queue.WorkloadStatusQueued),
					requestedResources: &FairshareResources{CPU: cpu, Memory: memory},
				},
			},
			fairshare: &FairshareResources{CPU: cpu, Memory: memory},
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
		expectedTotalUsage           *FairshareResources
	}{
		{
			name:                         "higher usage results in lower priority",
			conf:                         &structs.DynamicQueueConfig{TenantFairshare: structs.TenantFairshareConfig{CpuWeight: 10, MemoryWeight: 10}},
			lowUsageTenant:               mkTenant(TenantID("tenant-low"), 0, 55),
			highUsageTenant:              mkTenant(TenantID("tenant-high"), 100, 50),
			expectedHigherPriorityTenant: TenantID("tenant-low"),
			expectedTotalUsage:           &FairshareResources{CPU: 100, Memory: 105},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			q := New(hclog.New(hclog.DefaultOptions), ss, tc.conf, "")

			lowUsageWorkload := &dynamicPriorityWorkload{
				tid:          tc.lowUsageTenant.tid,
				BaseWorkload: queue.NewBaseWorkload(&structs.Evaluation{Priority: 5}, &structs.Job{}, queue.WorkloadStatusQueued),
			}
			highUsageWorkload := &dynamicPriorityWorkload{
				tid:          tc.highUsageTenant.tid,
				BaseWorkload: queue.NewBaseWorkload(&structs.Evaluation{Priority: 5}, &structs.Job{}, queue.WorkloadStatusQueued),
			}

			q.tenants[tc.lowUsageTenant.tid] = tc.lowUsageTenant
			q.tenants[tc.highUsageTenant.tid] = tc.highUsageTenant
			// Set totalFairshare to the sum so fairshareAdjustment can compute ratios without
			// calling calculateFairshare (which requires a running state store).
			q.totalFairshare = &FairshareResources{
				CPU:    tc.lowUsageTenant.fairshare.CPU + tc.highUsageTenant.fairshare.CPU,
				Memory: tc.lowUsageTenant.fairshare.Memory + tc.highUsageTenant.fairshare.Memory,
			}
			q.queue = queue.NewWorkloadQueue(workloadSortFn())
			q.queue.Push(lowUsageWorkload)
			q.queue.Push(highUsageWorkload)

			// Update priorities directly without recalculating fairshare.
			q.queue.UpdateAll(func(w queue.Workload) {
				workload := w.(*dynamicPriorityWorkload)
				q.setWorkloadPriority(time.Unix(20, 0), workload)
			})

			switch tc.expectedHigherPriorityTenant {
			case tc.lowUsageTenant.tid:
				must.Greater(t, highUsageWorkload.priority, lowUsageWorkload.priority)
			case tc.highUsageTenant.tid:
				must.Greater(t, lowUsageWorkload.priority, highUsageWorkload.priority)
			default:
				t.Fatalf("test case has unknown expectedHigherPriorityTenant: %q", tc.expectedHigherPriorityTenant)
			}

			must.Eq(t, q.totalFairshare, tc.expectedTotalUsage, must.Cmp(cmpopts.EquateApprox(0, 1e-9)))
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
			conf: &structs.DynamicQueueConfig{JobSize: structs.JobSizeConfig{CpuWeight: 10, CpuMax: 1000, MemoryWeight: 10, MemoryMax: 1000}},
			workload: &dynamicPriorityWorkload{requestedResources: &FairshareResources{
				CPU:    1000,
				Memory: 1000,
			}},
			exp: 0,
		},
		{
			name: "smaller requests results in expected adjustment",
			conf: &structs.DynamicQueueConfig{JobSize: structs.JobSizeConfig{CpuWeight: 10, CpuMax: 1000, MemoryWeight: 10, MemoryMax: 1000}},
			workload: &dynamicPriorityWorkload{requestedResources: &FairshareResources{
				CPU:    50,
				Memory: 50,
			}},
			exp: 9,
		},
		{
			name: "negative weight results in negative adjustment",
			conf: &structs.DynamicQueueConfig{JobSize: structs.JobSizeConfig{CpuWeight: -10, CpuMax: 1000, MemoryWeight: -10, MemoryMax: 1000}},
			workload: &dynamicPriorityWorkload{requestedResources: &FairshareResources{
				CPU:    50,
				Memory: 50,
			}},
			exp: -9,
		},
	}

	for _, tc := range testCases {
		testQueue := &Queue{
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
			conf: &structs.DynamicQueueConfig{Age: structs.AgeConfig{Weight: 10, Max: time.Second * 10}},
			workload: &dynamicPriorityWorkload{
				BaseWorkload: queue.NewBaseWorkload(&structs.Evaluation{}, mock.Job(), queue.WorkloadStatusQueued),
			},
			nowTime: time.Time{},
			exp:     0,
		},
		{
			name: "greater than max age results in max adjustment",
			conf: &structs.DynamicQueueConfig{Age: structs.AgeConfig{Weight: 10, Max: time.Second * 10}},
			workload: &dynamicPriorityWorkload{
				BaseWorkload: queue.NewBaseWorkload(&structs.Evaluation{
					CreateTime: time.Time{}.UnixNano(),
				}, mock.Job(), queue.WorkloadStatusQueued),
			},
			nowTime: time.Time{}.Add(30 * time.Second),
			exp:     10,
		},
		{
			name: "aging eval results in expected adjustment",
			conf: &structs.DynamicQueueConfig{Age: structs.AgeConfig{Weight: 10, Max: time.Second * 10}},
			workload: &dynamicPriorityWorkload{
				BaseWorkload: queue.NewBaseWorkload(&structs.Evaluation{
					CreateTime: time.Time{}.UnixNano(),
				}, mock.Job(), queue.WorkloadStatusQueued),
			},
			nowTime: time.Time{}.Add(2 * time.Second),
			exp:     2,
		},
	}

	for _, tc := range testCases {
		testQueue := &Queue{
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
					}, &structs.Job{ID: "job1"}, ""),
					tid:                 "tenantA",
					priority:            59,
					ageAdjustment:       3,
					fairshareAdjustment: 4,
				},
			},
			exp: &queue.WorkloadIter{
				Workloads: []structs.QueueWorkload{
					&structs.DynamicPriorityWorkload{
						JobID:               "job1",
						Tenant:              "tenantA",
						Position:            1,
						Status:              "",
						AdjustedPriority:    59,
						BasePriority:        50,
						AgeAdjustment:       3,
						FairshareAdjustment: 4,
						CreatedAt:           time.Unix(20, 0).UnixNano(),
						CreateIndex:         10,
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
					}, &structs.Job{ID: "job3"}, ""),
					tid:                 "tenantA",
					priority:            59,
					ageAdjustment:       3,
					fairshareAdjustment: 4,
				},
				{
					BaseWorkload: queue.NewBaseWorkload(&structs.Evaluation{
						ID:          "eval2",
						JobID:       "job2",
						Priority:    50,
						CreateIndex: 12,
					}, &structs.Job{ID: "job2"}, ""),
					tid:                 "tenantA",
					priority:            66,
					ageAdjustment:       3,
					fairshareAdjustment: 4,
				},
				{
					BaseWorkload: queue.NewBaseWorkload(&structs.Evaluation{
						ID:          "eval1",
						JobID:       "job1",
						Priority:    50,
						CreateIndex: 10,
					}, &structs.Job{ID: "job1"}, ""),
					tid:                 "tenantA",
					priority:            51,
					ageAdjustment:       0,
					fairshareAdjustment: 1,
				},
			},
			exp: &queue.WorkloadIter{
				Workloads: []structs.QueueWorkload{
					&structs.DynamicPriorityWorkload{
						JobID:               "job1",
						Tenant:              "tenantA",
						Position:            3,
						AdjustedPriority:    51,
						BasePriority:        50,
						AgeAdjustment:       0,
						FairshareAdjustment: 1,
						CreateIndex:         10,
					},
					&structs.DynamicPriorityWorkload{
						JobID:               "job2",
						Tenant:              "tenantA",
						Position:            1,
						AdjustedPriority:    66,
						BasePriority:        50,
						AgeAdjustment:       3,
						FairshareAdjustment: 4,
						CreateIndex:         12,
					},
					&structs.DynamicPriorityWorkload{
						JobID:               "job3",
						Tenant:              "tenantA",
						Position:            2,
						AdjustedPriority:    59,
						BasePriority:        50,
						AgeAdjustment:       3,
						FairshareAdjustment: 4,
						CreateIndex:         14,
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
					}, &structs.Job{ID: "job1"}, ""),
					tid:                 "tenantA",
					priority:            59,
					ageAdjustment:       3,
					fairshareAdjustment: 4,
				},
				{
					BaseWorkload: queue.NewBaseWorkload(&structs.Evaluation{
						ID:          "eval2",
						JobID:       "job2",
						Priority:    50,
						CreateIndex: 12,
					}, &structs.Job{ID: "job2"}, ""),
					tid:                 "tenantA",
					priority:            66,
					ageAdjustment:       3,
					fairshareAdjustment: 4,
				},
				{
					BaseWorkload: queue.NewBaseWorkload(&structs.Evaluation{
						ID:          "eval3",
						JobID:       "job3",
						Priority:    50,
						CreateIndex: 10,
					}, &structs.Job{ID: "job3"}, ""),
					tid:                 "tenantA",
					priority:            51,
					ageAdjustment:       0,
					fairshareAdjustment: 1,
				},
			},
			exp: &queue.WorkloadIter{
				Workloads: []structs.QueueWorkload{
					&structs.DynamicPriorityWorkload{
						JobID:               "job2",
						Tenant:              "tenantA",
						Position:            1,
						AdjustedPriority:    66,
						BasePriority:        50,
						AgeAdjustment:       3,
						FairshareAdjustment: 4,
						CreateIndex:         12,
					},
					&structs.DynamicPriorityWorkload{
						JobID:               "job1",
						Tenant:              "tenantA",
						Position:            2,
						AdjustedPriority:    59,
						BasePriority:        50,
						AgeAdjustment:       3,
						FairshareAdjustment: 4,
						CreateIndex:         14,
					},
					&structs.DynamicPriorityWorkload{
						JobID:               "job3",
						Tenant:              "tenantA",
						Position:            3,
						AdjustedPriority:    51,
						BasePriority:        50,
						AgeAdjustment:       0,
						FairshareAdjustment: 1,
						CreateIndex:         10,
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
					}, &structs.Job{ID: "job1"}, ""),
					tid:                 "tenantA",
					priority:            59,
					ageAdjustment:       3,
					fairshareAdjustment: 4,
				},
				{
					BaseWorkload: queue.NewBaseWorkload(&structs.Evaluation{
						ID:          "eval2",
						JobID:       "job2",
						Priority:    50,
						CreateTime:  time.Unix(10, 0).UnixNano(),
						CreateIndex: 10,
					}, &structs.Job{ID: "job2"}, ""),
					tid:                 "tenantA",
					priority:            59,
					ageAdjustment:       3,
					fairshareAdjustment: 4,
				},
			},
			exp: &queue.WorkloadIter{
				Workloads: []structs.QueueWorkload{
					&structs.DynamicPriorityWorkload{
						JobID:               "job2",
						Tenant:              "tenantA",
						Position:            1,
						AdjustedPriority:    59,
						BasePriority:        50,
						AgeAdjustment:       3,
						FairshareAdjustment: 4,
						CreatedAt:           time.Unix(10, 0).UnixNano(),
						CreateIndex:         10,
					},
					&structs.DynamicPriorityWorkload{
						JobID:               "job1",
						Tenant:              "tenantA",
						Position:            2,
						AdjustedPriority:    59,
						BasePriority:        50,
						AgeAdjustment:       3,
						FairshareAdjustment: 4,
						CreatedAt:           time.Unix(20, 0).UnixNano(),
						CreateIndex:         12,
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
					BaseWorkload: queue.NewBaseWorkload(&structs.Evaluation{
						ID:          "eval1",
						JobID:       "job1",
						Priority:    50,
						CreateTime:  time.Unix(20, 0).UnixNano(),
						CreateIndex: 12,
					}, &structs.Job{ID: "job1"}, queue.WorkloadStatusQueued),
					tid:                 "tenantA",
					priority:            59,
					ageAdjustment:       3,
					fairshareAdjustment: 4,
				},
				{
					BaseWorkload: queue.NewBaseWorkload(&structs.Evaluation{
						ID:          "eval2",
						JobID:       "job2",
						Priority:    50,
						CreateTime:  time.Unix(10, 0).UnixNano(),
						CreateIndex: 10,
					}, &structs.Job{ID: "job2"}, queue.WorkloadStatusPlacing),
					tid:                 "tenantA",
					priority:            60,
					ageAdjustment:       3,
					fairshareAdjustment: 4,
				},
			},
			exp: &queue.WorkloadIter{
				Workloads: []structs.QueueWorkload{
					&structs.DynamicPriorityWorkload{
						JobID:               "job2",
						Tenant:              "tenantA",
						Position:            0,
						Status:              "placing",
						AdjustedPriority:    60,
						BasePriority:        50,
						AgeAdjustment:       3,
						FairshareAdjustment: 4,
						CreatedAt:           time.Unix(10, 0).UnixNano(),
						CreateIndex:         10,
					},
					&structs.DynamicPriorityWorkload{
						JobID:               "job1",
						Tenant:              "tenantA",
						Position:            1,
						Status:              "queued",
						AdjustedPriority:    59,
						BasePriority:        50,
						AgeAdjustment:       3,
						FairshareAdjustment: 4,
						CreatedAt:           time.Unix(20, 0).UnixNano(),
						CreateIndex:         12,
					},
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ss := state.TestStateStore(t)
			testQueue := New(
				hclog.New(hclog.DefaultOptions),
				ss,
				&structs.DynamicQueueConfig{TenantFairshare: structs.TenantFairshareConfig{TenantType: "namespace"}},
				"",
			)

			var inProgress []queue.Workload
			for _, w := range tc.workloads {
				if w.Status() == "placing" {
					inProgress = append(inProgress, w)
				} else {
					testQueue.Push(w)
				}
			}

			got := testQueue.Jobs(tc.sortOrder, inProgress)
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
		name           string
		tenants        map[TenantID]*Tenant
		totalFairshare *FairshareResources
		exp            structs.QueueTenantsResponse
	}{
		{
			name: "status response parses tenants correctly",
			tenants: map[TenantID]*Tenant{
				"tenantA": {
					tid: "tenantA",
					fairshare: &FairshareResources{
						CPU:    100,
						Memory: 200,
					},
				},
			},
			totalFairshare: &FairshareResources{
				CPU:    400,
				Memory: 300,
			},
			exp: structs.QueueTenantsResponse{
				Type: structs.BatchQueueTypeDynamic,
				Tenants: []structs.DynamicPriorityTenant{
					{
						TenantID:        "tenantA",
						PercentageUsed:  42,
						TenantFairshare: map[string]float64{"cpu": 100, "memory": 200},
						TotalFairshare:  map[string]float64{"cpu": 400, "memory": 300},
					},
				},
			},
		},
	}
	for _, tc := range testCases {
		testQueue := &Queue{
			tenants:        tc.tenants,
			totalFairshare: tc.totalFairshare,
		}
		must.Eq(t, tc.exp, testQueue.Tenants())
	}
}

func TestDynamicPriorityQueue_NewWorkload(t *testing.T) {
	t.Run("workload has tenant and priority", func(t *testing.T) {
		ss := state.TestStateStore(t)
		q := New(hclog.New(hclog.DefaultOptions), ss, &structs.DynamicQueueConfig{
			TenantFairshare: structs.TenantFairshareConfig{TenantType: structs.TenantTypeNamespace},
			JobSize:         structs.JobSizeConfig{CpuWeight: 10, CpuMax: 1_000_000},
		}, "")

		job := mock.BatchJob()
		eval := mock.Eval()
		eval.Priority = 50
		eval.CreateTime = time.Now().UnixNano()

		wl, ok := q.NewWorkload(eval, job)
		must.True(t, ok)
		w := wl.(*dynamicPriorityWorkload)
		must.Eq(t, eval, w.Eval())
		must.Eq(t, job.NamespacedID(), w.ID())
		must.Eq(t, TenantID(job.Namespace), w.tid)
		must.Eq(t, queue.WorkloadStatusQueued, w.Status())
		must.Positive(t, w.requestedResources.CPU)

		// priority is set immediately, rather than on the next Run() tick
		must.Positive(t, w.cpuAdjustment)
		must.Eq(t, eval.Priority+w.cpuAdjustment, w.priority)

		tenant, ok := q.tenants[TenantID(job.Namespace)]
		must.True(t, ok)
		must.NotNil(t, tenant)
	})

	t.Run("empty tenant is not queued", func(t *testing.T) {
		ss := state.TestStateStore(t)
		q := New(hclog.New(hclog.DefaultOptions), ss, &structs.DynamicQueueConfig{
			TenantFairshare: structs.TenantFairshareConfig{
				TenantType:  structs.TenantTypeMetadata,
				MetadataKey: "tenant",
			},
		}, "")

		job := mock.BatchJob()
		w, ok := q.NewWorkload(mock.Eval(), job)
		must.False(t, ok)
		must.True(t, w == nil, must.Sprint("expected untyped nil workload"))
		must.MapEmpty(t, q.tenants)

		job.Meta = map[string]string{"tenant": "team-a"}
		w, ok = q.NewWorkload(mock.Eval(), job)
		must.True(t, ok)
		must.Eq(t, TenantID("team-a"), w.(*dynamicPriorityWorkload).tid)
		must.MapContainsKey(t, q.tenants, TenantID("team-a"))
	})
}

func TestDynamicPriorityQueue_calculateFairshare(t *testing.T) {
	// upsertJobAndAllocs upserts a batch job and the given allocs into the state store,
	// linking each alloc to the job.
	upsertJobAndAllocs := func(t *testing.T, ss *state.StateStore, job *structs.Job, allocs ...*structs.Allocation) {
		t.Helper()
		must.NoError(t, ss.UpsertNamespaces(1, []*structs.Namespace{{Name: job.Namespace}}))
		must.NoError(t, ss.UpsertJob(structs.MsgTypeTestSetup, 2, nil, job))
		for i, alloc := range allocs {
			alloc.JobID = job.ID
			alloc.Namespace = job.Namespace
			must.NoError(t, ss.UpsertAllocs(structs.MsgTypeTestSetup, uint64(100+i), []*structs.Allocation{alloc}))
		}
	}

	nsConf := &structs.DynamicQueueConfig{
		TenantFairshare: structs.TenantFairshareConfig{TenantType: structs.TenantTypeNamespace},
	}

	t.Run("no jobs results in zero fairshare", func(t *testing.T) {
		ss := state.TestStateStore(t)

		q := New(hclog.New(hclog.DefaultOptions), ss, nsConf, structs.NodePoolDefault)
		q.tenants["default"] = &Tenant{}

		q.calculateFairshare()

		must.Eq(t, &FairshareResources{}, q.totalFairshare)
		must.Eq(t, &FairshareResources{}, q.tenants["default"].fairshare)
	})

	t.Run("accumulates resources for tenant correctly", func(t *testing.T) {
		ss := state.TestStateStore(t)

		job := mock.BatchJob()
		a1, a2 := mock.Alloc(), mock.Alloc()

		upsertJobAndAllocs(t, ss, job, a1, a2)

		q := New(hclog.New(hclog.DefaultOptions), ss, nsConf, structs.NodePoolDefault)
		q.tenants["default"] = &Tenant{}

		q.calculateFairshare()

		must.Eq(t, &FairshareResources{CPU: 1000, Memory: 512}, q.totalFairshare)
		must.Eq(t, &FairshareResources{CPU: 1000, Memory: 512}, q.tenants["default"].fairshare)
	})

	t.Run("excluded alloc status is not counted", func(t *testing.T) {
		ss := state.TestStateStore(t)

		job := mock.BatchJob()
		alloc := mock.Alloc()
		alloc.ClientStatus = structs.AllocClientStatusFailed

		upsertJobAndAllocs(t, ss, job, alloc)

		q := New(
			hclog.New(hclog.DefaultOptions),
			ss,
			&structs.DynamicQueueConfig{TenantFairshare: structs.TenantFairshareConfig{
				TenantType:           structs.TenantTypeNamespace,
				ExcludeAllocStatuses: []string{structs.AllocClientStatusFailed},
			}},
			structs.NodePoolDefault,
		)

		q.tenants["default"] = &Tenant{}
		q.calculateFairshare()

		must.Eq(t, &FairshareResources{}, q.totalFairshare)
		must.Eq(t, &FairshareResources{}, q.tenants["default"].fairshare)
	})

	t.Run("non-batch job is skipped", func(t *testing.T) {
		ss := state.TestStateStore(t)

		alloc := mock.Alloc()

		upsertJobAndAllocs(t, ss, mock.Job(), alloc) // service job, not batch
		q := New(hclog.New(hclog.DefaultOptions), ss, nsConf, structs.NodePoolDefault)
		q.tenants["default"] = &Tenant{}

		q.calculateFairshare()

		must.Eq(t, &FairshareResources{}, q.totalFairshare)
		must.Eq(t, &FairshareResources{}, q.tenants["default"].fairshare)
	})

	t.Run("two tenants each accumulate their own resources", func(t *testing.T) {
		ss := state.TestStateStore(t)

		jobA := mock.BatchJob()
		jobA.Namespace = "ns-a"
		allocA := mock.Alloc()

		jobB := mock.BatchJob()
		jobB.Namespace = "ns-b"
		allocB := mock.Alloc()

		upsertJobAndAllocs(t, ss, jobA, allocA)
		upsertJobAndAllocs(t, ss, jobB, allocB)

		q := New(hclog.New(hclog.DefaultOptions), ss, nsConf, structs.NodePoolDefault)
		q.tenants["ns-a"] = &Tenant{}
		q.tenants["ns-b"] = &Tenant{}

		q.calculateFairshare()

		must.Eq(t, &FairshareResources{CPU: 1000, Memory: 512}, q.totalFairshare)
		must.Eq(t, &FairshareResources{CPU: 500, Memory: 256}, q.tenants["ns-a"].fairshare)
		must.Eq(t, &FairshareResources{CPU: 500, Memory: 256}, q.tenants["ns-b"].fairshare)
	})

}
