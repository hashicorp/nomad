// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package dynamic

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/queues/queue"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

type TenantID string

type DynamicPriorityQueue struct {
	// This is using a TreeSet from Hashicorp's go-set module due to it's
	// ability for log(n) insert and delete and allows for Top(k) lookups
	queue queue.WorkloadQueue[*dynamicPriorityWorkload]

	// tenants is used to keep track of cluster usage for this queue.
	// When workloads are placed or the  configured interval is passed,
	// cluster usage is updated for the workloads of each tenant.
	tenants map[TenantID]*Tenant

	// tMux locks the tenant map for concurrent access
	tMux sync.Mutex

	// qNotify allows for notifying the consumer that workloads
	// have been added to the queue
	qNotify chan struct{}

	// enqueueCh is used to buffer workloads before they
	// are processed by the manager and pushed onto the queue
	enqueueCh chan *dynamicPriorityWorkload

	// totalFairshare is the sum of all tenant fairshare values
	totalFairshare *FairshareResources

	// conf contains user configurations for tuning the behavior of the queue
	conf *structs.DynamicQueueConfig

	// pool is the node pool where the queue is configured. It is used to get
	// allocations for building fairshare state.
	pool string

	// evalBroker is the injected broker for passing an evaluation
	// on to be scheduled by Nomad
	evalBroker queue.Broker

	// state is the in-memory state store used for both reconciling tenant
	// workload usages, and polling submitted evaluations for placement
	state *state.StateStore

	cancel context.CancelFunc
	wg     sync.WaitGroup

	evalCancelFn queue.EvalCancelFn

	logger hclog.Logger

	watcher *queue.WorkloadWatcher
}

func NewDynamicPriorityQueue(
	logger hclog.Logger,
	ss *state.StateStore,
	broker queue.Broker,
	conf *structs.DynamicQueueConfig,
	pool string,
	cancelFn queue.EvalCancelFn,
) *DynamicPriorityQueue {
	return &DynamicPriorityQueue{
		queue:          queue.NewWorkloadQueue(workloadSortFn()),
		evalBroker:     broker,
		tMux:           sync.Mutex{},
		tenants:        make(map[TenantID]*Tenant),
		enqueueCh:      make(chan *dynamicPriorityWorkload, 8192),
		qNotify:        make(chan struct{}, 1),
		conf:           conf,
		totalFairshare: &FairshareResources{},
		wg:             sync.WaitGroup{},
		state:          ss,
		pool:           pool,
		evalCancelFn:   cancelFn,
		logger:         logger.Named("dynamic_priority_queue"),
		watcher:        queue.NewWorkloadWatcher(ss, logger),
	}
}

func (d *DynamicPriorityQueue) Type() structs.BatchQueueType {
	return structs.BatchQueueTypeDynamic
}

func workloadSortFn() func(i, j *dynamicPriorityWorkload) int {
	return func(i, j *dynamicPriorityWorkload) int {
		wait := queue.CmpWaitOnRestore(i, j)
		if wait != 0 {
			return wait
		}

		if i.priority > j.priority {
			return -1
		} else if i.priority < j.priority {
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

// Start assumes that the queue is ready to get going (i.e. restore has completed)
func (d *DynamicPriorityQueue) Start(ctx context.Context) error {
	rCtx, cancel := context.WithCancel(ctx)
	d.cancel = cancel

	// before starting, calculate priorities of evals restored to the queue
	d.calculatePriorities(time.Now())

	d.wg.Go(func() {
		d.runProducer(rCtx)
	})
	d.wg.Go(func() {
		d.runConsumer(rCtx)
	})
	d.wg.Go(func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(d.conf.CalcInterval):
				d.calculatePriorities(time.Now())
			}
		}
	})

	return nil
}

func (d *DynamicPriorityQueue) Stop() {
	d.cancel()
	d.wg.Wait()
}

func (d *DynamicPriorityQueue) Restore(eval *structs.Evaluation, j *structs.Job) error {
	w := d.generateWorkload(eval, j)

	// generate the tenant if it doesn't exist
	d.ensureTenant(w.tid)

	placed, err := d.watcher.IsSchedulingComplete(w)
	if err != nil {
		return err
	}

	if !placed {
		w.SetWaitOnRestore(true)
		d.enqueueCh <- w
	}
	return nil
}

func (d *DynamicPriorityQueue) calculateFairshare() {
	d.tMux.Lock()
	defer d.tMux.Unlock()

	d.totalFairshare = &FairshareResources{}

	iter, err := d.state.JobsByPool(nil, d.pool)
	if err != nil {
		d.logger.Error("failed to get jobs for node pool", "node pool", d.pool)
		return
	}

	for _, t := range d.tenants {
		t.fairshare = &FairshareResources{}
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

// Enqueue is the method used to put evaluations on the queue.
// It generates a workload with an empty priority, appends it
// to an internal channel to be processed and added to the actual
// heap container.
func (d *DynamicPriorityQueue) Enqueue(e *structs.Evaluation, j *structs.Job) {
	w := d.generateWorkload(e, j)

	// in the event of an empty workload, just pass eval to eval broker
	if w == nil {
		d.evalBroker.Enqueue(e)
		return
	}

	d.enqueueCh <- w
}

func (d *DynamicPriorityQueue) Dequeue(id structs.NamespacedID) *structs.Evaluation {
	wl := d.queue.Remove(id)

	if wl != nil {
		return wl.Eval()
	}

	return nil
}

// runProducer pushes workloads onto the queue and notifies the consumer
// goroutine. It also updates priorities on the configured interval.
func (d *DynamicPriorityQueue) runProducer(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case w := <-d.enqueueCh:

			// check if allocs for that job+version exist or we are already watching,
			// for this job+version for placement.
			// These cases indicate we have already processed this job+version and this
			// is a duplicate evaluation that can be handled directly by the eval broker.
			if d.shouldSkipQueue(w) {
				d.evalBroker.Enqueue(w.Eval())
				continue
			}

			// use createTime so that workloads have consistent age
			// priority calculations after restoring from state.
			d.setWorkloadPriority(time.Unix(0, w.Eval().CreateTime), w)

			if d.cancelRedundant(w) {
				continue
			}

			d.queue.Push(w)

			// Notify Workload consumer of new workload
			select {
			case d.qNotify <- struct{}{}:
			default:
			}
		}
	}
}

func (d *DynamicPriorityQueue) cancelRedundant(w *dynamicPriorityWorkload) bool {
	// check if a workload with the same ID exists on the queue. If so,
	// keep whichever job is newer, and cancel the eval for the other one.
	existing, ok := d.queue.Get(w.ID())
	if ok {
		// use an update here because it removes and replaces the workload
		// in the same locking transaction.
		if w.JobVersion() > existing.JobVersion() {
			d.queue.UpdateByID(w)
			d.evalCancelFn(existing.Eval())
		} else {
			d.evalCancelFn(w.Eval())
		}
		return true
	}

	return false
}

// shouldSkipQueue lets the caller know if an alloc already exists for the job, or we are currently watching the job.
// In these situations, we don't want to enqueue the workload, and the caller should directly enqueue the workload
// on the eval broker.
func (d *DynamicPriorityQueue) shouldSkipQueue(w *dynamicPriorityWorkload) bool {
	j, err := d.state.JobByIDAndVersion(nil, w.Eval().Namespace, w.Eval().JobID, w.JobVersion())
	if err != nil {
		d.logger.Error("failed to get job by version")
	}
	if j == nil {
		return false
	}
	allocs, _ := d.state.AllocsByJob(nil, w.Eval().Namespace, w.Eval().JobID, true)
	for _, a := range allocs {
		if a.Job.Version == j.Version {
			return true
		}
	}

	// TODO this does not check the workload we are currently "watching"
	return false
}

// runConsumer pops the highest priority workloads off the queue one
// at a time, enqueues them onto the Eval Broker, and waits for them
// to be placed before continuing.
func (d *DynamicPriorityQueue) runConsumer(ctx context.Context) {

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.qNotify:
			// Pop a workload off the queue if available
			w := d.queue.Pop()

			// We don't need to pass the waitOnRestore workload
			// to the eval broker, that already happened.
			if !w.WaitOnRestore() {
				d.evalBroker.Enqueue(w.Eval())
			}

			// Start watching for placement
			err := d.watcher.WaitForPlacement(ctx, w, memdb.NewWatchSet())
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				d.logger.Error("failure waiting for workload placement", "evalID", w.Eval().ID)
			}

			l := d.queue.Len()

			if l > 0 {
				select {
				case d.qNotify <- struct{}{}:
				default:
				}
			}
		}
	}
}

func (d *DynamicPriorityQueue) tenantID(job *structs.Job) TenantID {
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
func (d *DynamicPriorityQueue) generateWorkload(e *structs.Evaluation, job *structs.Job) *dynamicPriorityWorkload {
	tid := d.tenantID(job)
	if tid == "" {
		return nil
	}

	d.ensureTenant(tid)

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
func (d *DynamicPriorityQueue) ensureTenant(tid TenantID) {
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
func (d *DynamicPriorityQueue) calculatePriorities(now time.Time) {
	d.calculateFairshare()

	// Now that we have accurate tenant usage, calculate
	// each workloads new priority and update the queue
	d.queue.UpdateAll(func(w queue.Workload) {
		workload := w.(*dynamicPriorityWorkload)
		d.setWorkloadPriority(now, workload)
	})
}

// setWorkloadPriority calculates an individual workload's priority based on
func (d *DynamicPriorityQueue) setWorkloadPriority(now time.Time, w *dynamicPriorityWorkload) {
	w.priority = w.Eval().Priority +
		d.fairshareAdjustment(w) +
		d.ageAdjustment(now, w) +
		d.cpuAdjustment(w) +
		d.memAdjustment(w)
}

// fairshareAdjustment calculates the adjustment to a workload's priority based on
// its tenant's fairshare relative to the total, and configured weight.
func (d *DynamicPriorityQueue) fairshareAdjustment(w *dynamicPriorityWorkload) int {
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

func (d *DynamicPriorityQueue) ageAdjustment(now time.Time, w *dynamicPriorityWorkload) int {
	if d.conf.Age.Weight == 0 {
		return 0
	}

	elapsed := now.UnixNano() - w.Eval().CreateTime

	age := float64(elapsed) / float64(d.conf.Age.Max)
	ageClamped := min(1.0, max(0.0, age))

	w.ageAdjustment = int(ageClamped * float64(d.conf.Age.Weight))
	return w.ageAdjustment
}

func (d *DynamicPriorityQueue) cpuAdjustment(w *dynamicPriorityWorkload) int {
	if d.conf.JobSize.CpuWeight == 0 {
		return 0
	}

	size := w.requestedResources.CPU / float64(d.conf.JobSize.MaxCpu)
	sizeClamped := min(1.0, max(0.0, size))

	w.cpuAdjustment = int((1 - sizeClamped) * float64(d.conf.JobSize.CpuWeight))
	return w.cpuAdjustment
}

func (d *DynamicPriorityQueue) memAdjustment(w *dynamicPriorityWorkload) int {
	if d.conf.JobSize.MemoryWeight == 0 {
		return 0
	}

	size := w.requestedResources.Memory / float64(d.conf.JobSize.MaxMemory)
	sizeClamped := min(1.0, max(0.0, size))

	w.memAdjustment = int((1 - sizeClamped) * float64(d.conf.JobSize.MemoryWeight))
	return w.memAdjustment
}

func (d *DynamicPriorityQueue) Jobs(sortOrder structs.SortOrder) *queue.WorkloadIter {
	pos := 0
	workloads := []structs.QueueWorkload{}

	var newDynamicWorkloadStruct = func(w *dynamicPriorityWorkload) *structs.DynamicPriorityWorkload {
		e := w.Eval()
		return &structs.DynamicPriorityWorkload{
			JobID:               e.JobID,
			Tenant:              string(w.tid),
			Status:              w.Status(),
			Namespace:           e.Namespace,
			Position:            pos,
			AdjustedPriority:    w.priority,
			BasePriority:        e.Priority,
			FairshareAdjustment: w.fairshareAdjustment,
			AgeAdjustment:       w.ageAdjustment,
			CpuAdjustment:       w.cpuAdjustment,
			MemoryAdjustment:    w.memAdjustment,
			CreatedAt:           e.CreateTime,
			CreateIndex:         e.CreateIndex,
		}
	}

	for _, workload := range d.watcher.GetInProgressWorkloads() {
		w := workload.(*dynamicPriorityWorkload)
		workloads = append(workloads, newDynamicWorkloadStruct(w))
	}

	d.queue.Iterate(func(workload queue.Workload) {
		w := workload.(*dynamicPriorityWorkload)
		// waitOnRestore does not count towards position in queue
		if w.WaitOnRestore() {
			return
		}
		pos++
		workloads = append(workloads, newDynamicWorkloadStruct(w))
	})

	iter := queue.NewWorkloadIter(workloads)

	if sortOrder != structs.SortByPriority {
		iter.SortByJobId()
	}

	return iter
}

func (d *DynamicPriorityQueue) Tenants() structs.QueueTenantsResponse {
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
