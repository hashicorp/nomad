// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package dynamic

import (
	"context"
	"errors"
	"math"
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

	// totalUsage is the sum of all tenant usages
	totalUsage *ResourceUsage

	// conf contains user configurations for tuning the behavior of the queue
	conf *structs.DynamicQueueConfig

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
}

func NewDynamicPriorityQueue(
	logger hclog.Logger,
	ss *state.StateStore,
	broker queue.Broker,
	conf *structs.DynamicQueueConfig,
	cancelFn queue.EvalCancelFn,
) *DynamicPriorityQueue {
	return &DynamicPriorityQueue{
		queue:        queue.NewWorkloadQueue(workloadSortFn()),
		evalBroker:   broker,
		tMux:         sync.Mutex{},
		tenants:      make(map[TenantID]*Tenant),
		enqueueCh:    make(chan *dynamicPriorityWorkload, 8192),
		qNotify:      make(chan struct{}, 1),
		conf:         conf,
		totalUsage:   &ResourceUsage{},
		wg:           sync.WaitGroup{},
		state:        ss,
		evalCancelFn: cancelFn,
		logger:       logger.Named("dynamic_priority_queue"),
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

	// the queue manager may have restored older evals into this queue,
	// so decay usage as appropriate.
	d.decayUsage(time.Now())

	d.wg.Go(func() {
		d.runProducer(rCtx)
	})
	d.wg.Go(func() {
		d.runConsumer(rCtx)
	})

	return nil
}

func (d *DynamicPriorityQueue) Stop() {
	d.cancel()
	d.wg.Wait()
}

func (d *DynamicPriorityQueue) Restore(eval *structs.Evaluation, j *structs.Job) error {
	w := d.generateWorkload(eval, j)

	// TODO: count allocs for the *right* version of the job?

	// generate the tenant if it doesn't exist
	d.ensureTenant(w.tid)

	placed, err := queue.IsSchedulingComplete(w, d.state)
	if err != nil {
		return err
	}
	if placed && evalHasPlacement(w.Eval()) {
		d.updateUsage(w)
	}

	if !placed {
		w.SetWaitOnRestore(true)
		d.enqueueCh <- w
	}
	return nil
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
		case <-time.After(d.conf.CalcInterval):
			d.calculatePriorities(time.Now())
		}
	}
}

func (d *DynamicPriorityQueue) cancelRedundant(w *dynamicPriorityWorkload) bool {
	// check if a workload with the same ID exists on the queue. If so,
	// either swap out the existing if the new one has a higher job version,
	// and cancel the existing's eval, or cancel the new workload's eval
	// if it has a job version that not greater than the existing one.
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
		d.logger.Info("job is nil", "jobID", w.Eval().JobID, "namespace", w.Eval().Namespace, "version", w.JobVersion())
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

			// Wait for the eval to be placed
			err := queue.WaitForPlacement(ctx, w, d.state, memdb.NewWatchSet())
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				d.logger.Error("failure waiting for workload placement", "evalID", w.Eval().ID)
			}

			if evalHasPlacement(w.Eval()) {
				d.updateUsage(w)
			}
			l := d.queue.Len()

			// If the queue still has work, notify self
			// to continue.
			if l > 0 {
				select {
				case d.qNotify <- struct{}{}:
				default:
				}
			}
		}
	}
}

// generateWorkload is used to create an initial workload from a given evaluation
func (d *DynamicPriorityQueue) generateWorkload(e *structs.Evaluation, job *structs.Job) *dynamicPriorityWorkload {
	var tid TenantID
	switch d.conf.TenantType {
	case "namespace":
		tid = TenantID(job.Namespace)
	case "metadata":
		tenantID, ok := job.Meta[d.conf.MetadataKey]
		if !ok {
			return nil
		}
		tid = TenantID(tenantID)
	default:
		d.logger.Error("unknown tenant type, this is a bug.")
		return nil
	}

	requestedResources := &UsageList{
		resources: &ResourceUsage{},
	}
	for _, tg := range job.TaskGroups {
		for _, task := range tg.Tasks {
			requestedResources.resources.AddCpu(float64(task.Resources.CPU) * float64(tg.Count))
			requestedResources.resources.AddMemory(float64(task.Resources.MemoryMB) * float64(tg.Count))
		}
	}

	return &dynamicPriorityWorkload{
		BaseWorkload:       queue.NewBaseWorkload(e, job),
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
		totalUsage:         &ResourceUsage{},
	}
}

// calculatePriorities iterates over all workloads in the queue and updates
// their priorities based on tenant usage, which is decayed according to the
// configured half-life, and usage weight.
func (d *DynamicPriorityQueue) calculatePriorities(now time.Time) {
	// Decay tenant workload usages first, because a workload's
	// priority relies on its tenant's usage.
	d.decayUsage(now)

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
		d.usageAdjustment(w) +
		d.ageAdjustment(now, w) +
		d.cpuAdjustment(w) +
		d.memAdjustment(w)
}

// usageAdjustment calculates the adjustment to a workload's priority based on
// it's tenant's usage relative to the total usage, and configured weight.
func (d *DynamicPriorityQueue) usageAdjustment(w *dynamicPriorityWorkload) int {
	d.tMux.Lock()
	defer d.tMux.Unlock()

	if d.conf.UsageWeight == 0 {
		return 0
	}

	d.ensureTenant(w.tid)
	total := d.totalUsage.Total()
	tenantUsage := d.tenants[w.tid].totalUsage.Total()

	usageRatio := 0.0
	if total > 0 {
		usageRatio = tenantUsage / total
	}
	usageAdjustment := (1 - usageRatio) * float64(d.conf.UsageWeight)
	w.usageAdjustment = int(usageAdjustment)
	return w.usageAdjustment
}

// decayUsage iterates over all tenants and decays the workload usage based on
// the time elapsed since (roughly) when the eval was placed, and the configured
// half-life. If the eval no longer exists in the state store, its workload's
// usage is removed from the calculation.
func (d *DynamicPriorityQueue) decayUsage(now time.Time) {
	d.tMux.Lock()
	defer d.tMux.Unlock()

	totalUsage := &ResourceUsage{}

	snap, err := d.state.Snapshot()
	if err != nil {
		d.logger.Error("failed to take state snapshot", "error", err)
		return
	}

	for _, tenant := range d.tenants {
		newWorkloadUsageByID := make(map[structs.NamespacedID]*dynamicPriorityWorkload)
		tenantTotalUsage := &ResourceUsage{}

		for id, workload := range tenant.placedWorkloadById {
			eval, err := snap.EvalByID(nil, workload.Eval().ID)
			if err != nil || eval == nil {
				continue
			}
			decayedResources := d.decayWorkloadUsage(now, workload.requestedResources)

			tenantTotalUsage = tenantTotalUsage.Add(decayedResources.resources)
			totalUsage = totalUsage.Add(decayedResources.resources)

			workload.requestedResources = decayedResources
			newWorkloadUsageByID[id] = workload
		}

		tenant.totalUsage = tenantTotalUsage
		tenant.placedWorkloadById = newWorkloadUsageByID
	}
	d.totalUsage = totalUsage
}

func decayMultiplier(now, createdAt time.Time, halfLife time.Duration) float64 {
	elapsed := now.Sub(createdAt)
	return math.Pow(0.5, elapsed.Seconds()/halfLife.Seconds())
}

// decayWorkloadUsage applies decay to an individual workload's usage based on
// the time elapsed since (roughly) when the eval was placed, and the configured
// half-life. It returns the decayed usage, and also updates the workload usage
func (d *DynamicPriorityQueue) decayWorkloadUsage(now time.Time, usage *UsageList) *UsageList {
	multiplier := decayMultiplier(now, usage.start, d.conf.HalfLife)

	decayed := &ResourceUsage{}
	decayed.AddCpu(usage.resources.CPU * multiplier)
	decayed.AddMemory(usage.resources.Memory * multiplier)

	return &UsageList{
		resources: decayed,
		start:     now,
	}
}

func (d *DynamicPriorityQueue) ageAdjustment(now time.Time, w *dynamicPriorityWorkload) int {
	if d.conf.AgeWeight == 0 {
		return 0
	}

	elapsed := now.UnixNano() - w.Eval().CreateTime

	age := float64(elapsed) / float64(d.conf.MaxAge)
	ageClamped := min(1.0, max(0.0, age))

	w.ageAdjustment = int(ageClamped * float64(d.conf.AgeWeight))
	return w.ageAdjustment
}

func (d *DynamicPriorityQueue) cpuAdjustment(w *dynamicPriorityWorkload) int {
	if d.conf.CpuWeight == 0 {
		return 0
	}

	size := w.requestedResources.resources.CPU / float64(d.conf.MaxCpu)
	sizeClamped := min(1.0, max(0.0, size))

	w.cpuAdjustment = int((1 - sizeClamped) * float64(d.conf.CpuWeight))
	return w.cpuAdjustment
}

func (d *DynamicPriorityQueue) memAdjustment(w *dynamicPriorityWorkload) int {
	if d.conf.MemWeight == 0 {
		return 0
	}

	size := w.requestedResources.resources.Memory / float64(d.conf.MaxMemory)
	sizeClamped := min(1.0, max(0.0, size))

	w.memAdjustment = int((1 - sizeClamped) * float64(d.conf.MemWeight))
	return w.memAdjustment
}

func (d *DynamicPriorityQueue) Jobs(sortOrder structs.SortOrder) *queue.WorkloadIter {
	pos := 0
	workloads := []structs.QueueWorkload{}

	d.queue.Iterate(func(workload queue.Workload) {
		w := workload.(*dynamicPriorityWorkload)
		// waitOnRestore does not count towards position in queue
		if w.WaitOnRestore() {
			return
		}
		pos++

		e := w.Eval()
		workloads = append(workloads, &structs.DynamicPriorityWorkload{
			JobID:            e.JobID,
			Tenant:           string(w.tid),
			Namespace:        e.Namespace,
			Position:         pos,
			AdjustedPriority: w.priority,
			BasePriority:     e.Priority,
			UsageAdjustment:  w.usageAdjustment,
			AgeAdjustment:    w.ageAdjustment,
			CpuAdjustment:    w.cpuAdjustment,
			MemoryAdjustment: w.memAdjustment,
			CreatedAt:        e.CreateTime,
			CreateIndex:      e.CreateIndex,
		})
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
			TenantID:       string(t.tid),
			PercentageUsed: t.totalPercentageUsed(d.totalUsage),
			TenantUsage:    t.totalUsage.UsageByResource(),
			TotalUsage:     d.totalUsage.UsageByResource(),
		})
	}
	return structs.QueueTenantsResponse{
		Type:    structs.BatchQueueTypeDynamic,
		Tenants: tenants,
	}
}

// updateUsage updates the tenant and total usage for a given workload.
func (d *DynamicPriorityQueue) updateUsage(workload *dynamicPriorityWorkload) {
	d.tMux.Lock()
	defer d.tMux.Unlock()

	tenant := d.tenants[workload.tid]

	_, ok := tenant.placedWorkloadById[workload.ID()]
	// If the workload has already been placed, don't count the usage again.
	if ok {
		return
	}

	workloadResources := workload.requestedResources
	// this method should only be called when a workload was successfully placed,
	// so we can use the ModifyTime as the for when decay will start.
	workloadResources.start = time.Unix(0, workload.Eval().ModifyTime)
	tenant.totalUsage = tenant.totalUsage.Add(workloadResources.resources)
	d.totalUsage = d.totalUsage.Add(workloadResources.resources)

	tenant.placedWorkloadById[workload.ID()] = workload
}

func evalHasPlacement(e *structs.Evaluation) bool {
	if e.PlanAnnotations != nil && e.PlanAnnotations.DesiredTGUpdates != nil {
		for _, update := range e.PlanAnnotations.DesiredTGUpdates {
			if update.Place > 0 {
				return true
			}
		}
	}
	return false
}
