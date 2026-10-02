// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package dynamic

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/queues/queue"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

type TenantID string

type Queue struct {
	// This is using a TreeSet from Hashicorp's go-set module due to it's
	// ability for log(n) insert and delete and allows for Top(k) lookups
	queue queue.WorkloadQueue

	// tenants is used to keep track of cluster usage for this queue.
	// When workloads are placed or the  configured interval is passed,
	// cluster usage is updated for the workloads of each tenant.
	tenants map[TenantID]*Tenant

	// tMux locks the tenant map for concurrent access
	tMux sync.Mutex

	// totalFairshare is the sum of all tenant fairshare values
	totalFairshare *FairshareResources

	// conf contains user configurations for tuning the behavior of the queue
	conf *structs.DynamicQueueConfig

	// pool is the node pool where the queue is configured. It is used to get
	// allocations for building fairshare state.
	pool string

	// state is the in-memory state store used for reconciling tenant
	// workload usages
	state *state.StateStore

	logger hclog.Logger
}

// New returns the workload storage and ordering for a
// dynamic priority queue.
func New(
	logger hclog.Logger,
	ss *state.StateStore,
	conf *structs.DynamicQueueConfig,
	pool string,
) *Queue {
	return &Queue{
		queue:          queue.NewWorkloadQueue(workloadSortFn()),
		tMux:           sync.Mutex{},
		tenants:        make(map[TenantID]*Tenant),
		conf:           conf,
		totalFairshare: &FairshareResources{},
		state:          ss,
		pool:           pool,
		logger:         logger,
	}
}

func workloadSortFn() func(i, j queue.Workload) int {
	return func(i, j queue.Workload) int {
		a := i.(*dynamicPriorityWorkload)
		b := j.(*dynamicPriorityWorkload)

		if a.priority > b.priority {
			return -1
		} else if a.priority < b.priority {
			return 1
		}

		if i.Eval().CreateIndex < j.Eval().CreateIndex {
			return -1
		} else if i.Eval().CreateIndex > j.Eval().CreateIndex {
			return 1
		}
		return 0
	}
}

// Run implements base.Runnable to periodically recalculate priorities.
func (d *Queue) Run(ctx context.Context) {
	// initially calculate priorities
	d.calculatePriorities(time.Now())

	// This goroutine runs the background thread for recalculating
	// priorities on the configured interval.
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(d.conf.CalcInterval):
			d.calculatePriorities(time.Now())
		}
	}
}

// NewWorkload implements base.Queue. Jobs without a tenant ID are not
// queued.
func (d *Queue) NewWorkload(e *structs.Evaluation, j *structs.Job) (queue.Workload, bool) {
	tid := d.tenantID(j)
	if tid == "" {
		return nil, false
	}
	d.ensureTenant(tid)

	w := d.generateWorkload(e, j, tid)

	// use createTime so that workloads have consistent age
	// priority calculations after restoring from state.
	d.setWorkloadPriority(time.Unix(0, e.CreateTime), w)
	return w, true
}

func (d *Queue) Type() structs.BatchQueueType {
	return structs.BatchQueueTypeDynamic
}

// Push implements base.Queue. The workload must have been created by
// NewWorkload.
func (d *Queue) Push(w queue.Workload) {
	d.queue.Push(w)
}

func (d *Queue) Pop() (queue.Workload, bool) {
	w, ok := d.queue.Pop()
	if !ok {
		return nil, false
	}
	return w, true
}

func (d *Queue) Get(id structs.NamespacedID) (queue.Workload, bool) {
	return d.queue.Get(id)
}

func (d *Queue) Remove(id structs.NamespacedID) (queue.Workload, bool) {
	return d.queue.Remove(id)
}

// Swap implements base.Queue. The workload must have been created by
// NewWorkload.
func (d *Queue) Swap(w queue.Workload) (queue.Workload, bool) {
	return d.queue.Swap(w)
}

func (d *Queue) calculateFairshare() {
	d.tMux.Lock()
	defer d.tMux.Unlock()

	d.totalFairshare = &FairshareResources{}
	for _, t := range d.tenants {
		t.fairshare = &FairshareResources{}
	}

	iter, err := d.state.JobsByPool(nil, d.pool)
	if err != nil {
		d.logger.Error("failed to get jobs for node pool", "node pool", d.pool)
		return
	}

	for {
		raw := iter.Next()
		if raw == nil {
			break
		}

		job, ok := raw.(*structs.Job)
		if !ok {
			continue
		}
		if job.Type != structs.JobTypeBatch {
			continue
		}

		tid := d.tenantID(job)
		if tid == "" {
			continue
		}

		allocs, err := d.state.AllocsByJob(nil, job.Namespace, job.ID, true)
		if err != nil {
			d.logger.Error("failed to get allocs for job", "jobID", job.ID)
			continue
		}

		tenant := d.tenants[tid]
		if tenant == nil {
			continue
		}

		for _, alloc := range allocs {
			if slices.Contains(d.conf.TenantFairshare.ExcludeAllocStatuses, alloc.ClientStatus) {
				continue
			}

			for _, task := range alloc.AllocatedResources.Tasks {
				tenant.fairshare.CPU += float64(task.Cpu.CpuShares)
				tenant.fairshare.Memory += float64(task.Memory.MemoryMB)

				d.totalFairshare.CPU += float64(task.Cpu.CpuShares)
				d.totalFairshare.Memory += float64(task.Memory.MemoryMB)
			}
		}
	}
}

func (d *Queue) tenantID(job *structs.Job) TenantID {
	var tid TenantID
	switch d.conf.TenantFairshare.TenantType {
	case "namespace":
		tid = TenantID(job.Namespace)
	case "metadata":
		tenantID, ok := job.Meta[d.conf.TenantFairshare.MetadataKey]
		if !ok {
			return ""
		}
		tid = TenantID(tenantID)
	default:
		d.logger.Error("unknown tenant type, this is a bug.")
		return ""
	}

	return tid
}

// generateWorkload is used to create an initial workload from a given evaluation
func (d *Queue) generateWorkload(e *structs.Evaluation, job *structs.Job, tid TenantID) *dynamicPriorityWorkload {
	requestedResources := &FairshareResources{}
	for _, tg := range job.TaskGroups {
		for _, task := range tg.Tasks {
			requestedResources.CPU += float64(task.Resources.CPU * tg.Count)
			requestedResources.Memory += float64(task.Resources.MemoryMB * tg.Count)
		}
	}

	return &dynamicPriorityWorkload{
		BaseWorkload:       queue.NewBaseWorkload(e, job, queue.WorkloadStatusQueued),
		tid:                tid,
		priority:           0,
		requestedResources: requestedResources,
	}
}

// ensureTenant creates a new tenant in the queue if it doesn't already exist.
func (d *Queue) ensureTenant(tid TenantID) {
	d.tMux.Lock()
	defer d.tMux.Unlock()

	if _, ok := d.tenants[tid]; ok {
		return
	}

	d.tenants[tid] = &Tenant{
		tid:                tid,
		placedWorkloadById: make(map[structs.NamespacedID]*dynamicPriorityWorkload),
		fairshare:          &FairshareResources{},
	}
}

// calculatePriorities iterates over all workloads in the queue and updates
// their priorities based on tenant usage, which is decayed according to the
// configured half-life, and usage weight.
func (d *Queue) calculatePriorities(now time.Time) {
	d.calculateFairshare()

	// Now that we have accurate tenant usage, calculate
	// each workloads new priority and update the queue
	d.queue.UpdateAll(func(w queue.Workload) {
		if dynamic, ok := w.(*dynamicPriorityWorkload); ok {
			d.setWorkloadPriority(now, dynamic)
		}
	})
}

// setWorkloadPriority calculates an individual workload's priority based on
func (d *Queue) setWorkloadPriority(now time.Time, w *dynamicPriorityWorkload) {
	w.priority = w.Eval().Priority +
		d.fairshareAdjustment(w) +
		d.ageAdjustment(now, w) +
		d.cpuAdjustment(w) +
		d.memAdjustment(w)
}

// fairshareAdjustment calculates the adjustment to a workload's priority based on
// its tenant's fairshare relative to the total, and configured weight.
func (d *Queue) fairshareAdjustment(w *dynamicPriorityWorkload) int {
	d.tMux.Lock()
	defer d.tMux.Unlock()

	if d.conf.TenantFairshare.CpuWeight == 0 && d.conf.TenantFairshare.MemoryWeight == 0 {
		return 0
	}

	total := d.totalFairshare.Total()
	if total == 0 {
		return 0
	}

	tenant := d.tenants[w.tid]

	cpuRatio := tenant.fairshare.CPU / d.totalFairshare.CPU
	memRatio := tenant.fairshare.Memory / d.totalFairshare.Memory

	cpuAdjustment := (1 - cpuRatio) * float64(d.conf.TenantFairshare.CpuWeight)
	memAdjustment := (1 - memRatio) * float64(d.conf.TenantFairshare.MemoryWeight)

	w.fairshareAdjustment = int(cpuAdjustment + memAdjustment)
	return w.fairshareAdjustment
}

func (d *Queue) ageAdjustment(now time.Time, w *dynamicPriorityWorkload) int {
	if d.conf.Age.Weight == 0 {
		return 0
	}

	elapsed := now.UnixNano() - w.Eval().CreateTime

	age := float64(elapsed) / float64(d.conf.Age.Max)
	ageClamped := min(1.0, max(0.0, age))

	w.ageAdjustment = int(ageClamped * float64(d.conf.Age.Weight))
	return w.ageAdjustment
}

func (d *Queue) cpuAdjustment(w *dynamicPriorityWorkload) int {
	if d.conf.JobSize.CpuWeight == 0 {
		return 0
	}

	size := w.requestedResources.CPU / float64(d.conf.JobSize.CpuMax)
	sizeClamped := min(1.0, max(0.0, size))

	w.cpuAdjustment = int((1 - sizeClamped) * float64(d.conf.JobSize.CpuWeight))
	return w.cpuAdjustment
}

func (d *Queue) memAdjustment(w *dynamicPriorityWorkload) int {
	if d.conf.JobSize.MemoryWeight == 0 {
		return 0
	}

	size := w.requestedResources.Memory / float64(d.conf.JobSize.MemoryMax)
	sizeClamped := min(1.0, max(0.0, size))

	w.memAdjustment = int((1 - sizeClamped) * float64(d.conf.JobSize.MemoryWeight))
	return w.memAdjustment
}

// Jobs implements base.Viewable.
func (d *Queue) Jobs(sortOrder structs.SortOrder, inProgress []queue.Workload) *queue.WorkloadIter {
	pos := 0
	workloads := []structs.QueueWorkload{}

	for _, workload := range inProgress {
		if w, ok := workload.(*dynamicPriorityWorkload); ok {
			workloads = append(workloads, w.ToStruct(pos))
		}
	}

	d.queue.Iterate(func(workload queue.Workload) {
		if w, ok := workload.(*dynamicPriorityWorkload); ok {
			pos++
			workloads = append(workloads, w.ToStruct(pos))
		}
	})

	iter := queue.NewWorkloadIter(workloads)

	if sortOrder != structs.SortByPriority {
		iter.SortByJobId()
	}

	return iter
}

// Tenants implements base.Viewable.
func (d *Queue) Tenants() structs.QueueTenantsResponse {
	d.tMux.Lock()
	defer d.tMux.Unlock()

	tenants := []structs.DynamicPriorityTenant{}
	for _, t := range d.tenants {
		tenants = append(tenants, structs.DynamicPriorityTenant{
			TenantID:        string(t.tid),
			PercentageUsed:  t.totalPercentageUsed(d.totalFairshare),
			TenantFairshare: t.fairshare.ByResource(),
			TotalFairshare:  d.totalFairshare.ByResource(),
		})
	}
	return structs.QueueTenantsResponse{
		Type:    structs.BatchQueueTypeDynamic,
		Tenants: tenants,
	}
}
