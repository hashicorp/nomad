// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package dynamic

import (
	"context"
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
	queue queue.WorkloadQueue

	// qMux locks the queue during concurrent access
	qMux sync.Mutex

	// tenants is used to keep track of cluster usage for this queue.
	// When workloads are placed or the  configured interval is passed,
	// cluster usage is updated for the workloads of each tenant.
	tenants map[TenantID]*Tenant

	// qNotify allows for notifying the consumer that workloads
	// have been added to the queue
	qNotify chan struct{}

	// enqueueCh is used to buffer workloads before they
	// are processed by the manager and pushed onto the queue
	enqueueCh chan *dynamicPriorityWorkload

	// totalUsage is the sum of all tenant usages
	totalUsage *ResourceUsage

	tenantType structs.BatchQueueTenant

	metadataKey string

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

	logger hclog.Logger
}

func NewDynamicPriorityQueue(ss *state.StateStore, broker queue.Broker, qconf *structs.BatchQueue, conf *structs.DynamicQueueConfig, logger hclog.Logger) *DynamicPriorityQueue {
	return &DynamicPriorityQueue{
		queue:       queue.NewWorkloadQueue(workloadSortFn()),
		evalBroker:  broker,
		qMux:        sync.Mutex{},
		tenants:     make(map[TenantID]*Tenant),
		enqueueCh:   make(chan *dynamicPriorityWorkload, 8192),
		qNotify:     make(chan struct{}, 1),
		tenantType:  qconf.TenantType,
		metadataKey: qconf.MetadataKey,
		conf:        conf,
		totalUsage:  &ResourceUsage{},
		wg:          sync.WaitGroup{},
		state:       ss,
		logger:      logger.Named("Dynamic Priority Queue"),
	}
}

func (d *DynamicPriorityQueue) Type() structs.BatchQueueType {
	return structs.BatchQueueTypeDynamic
}

func workloadSortFn() func(i, j queue.Workload) int {
	return func(i, j queue.Workload) int {
		wait := queue.CmpWaitOnRestore(i, j)
		if wait != 0 {
			return wait
		}

		a := i.(*dynamicPriorityWorkload)
		b := j.(*dynamicPriorityWorkload)

		if a.priority > b.priority {
			return -1
		} else if a.priority < b.priority {
			return 1
		}

		if a.eval.CreateIndex < b.eval.CreateIndex {
			return -1
		} else if a.eval.CreateIndex > b.eval.CreateIndex {
			return 1
		}
		return 0
	}
}

func (d *DynamicPriorityQueue) Start(ctx context.Context) error {
	rCtx, cancel := context.WithCancel(ctx)
	d.cancel = cancel

	snap, err := d.state.Snapshot()
	if err != nil {
		d.logger.Error("failed to get state snapshot", "err", err)
		return err
	}

	if err := d.restore(snap, time.Now()); err != nil {
		return err
	}

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

// restore scans all evaluations and restores the usage state of the queue by
// detecting which evals were already placed by a previous server
func (d *DynamicPriorityQueue) restore(ss *state.StateSnapshot, now time.Time) error {
	// The DPQ needs to rebuild it's internal usage state when enabled.
	// The actual queue will be rebuilt when establishing leadership
	// via pending eval enqueuing
	d.qMux.Lock()
	defer d.qMux.Unlock()

	ws := memdb.NewWatchSet()
	iter, err := ss.Evals(ws, state.SortDefault)
	if err != nil {
		d.logger.Error("failed to get evals while enabling queue", "err", err)
		return err
	}

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		eval, ok := raw.(*structs.Evaluation)
		if !ok {
			d.logger.Error("object from eval table not an eval")
			continue
		}

		// Skip non batch jobs
		if eval.Type != structs.JobTypeBatch {
			continue
		}
		// If the eval was not a job register, skip it
		if eval.TriggeredBy != structs.EvalTriggerJobRegister {
			continue
		}
		// Pending evals will be enqueued later in leadership transfer
		if eval.Status == structs.EvalStatusPending {
			continue
		}

		w := d.generateWorkload(eval)

		// generate the tenant if it doesn't exist
		d.ensureTenant(w.tid)

		// When checking for workload placements, we never want to actually block
		// in SetEnabled, but it's also entirely possible a queue eval is blocked and
		// waiting to be placed from a previous DPQ placement. If that happens
		// we should enqueue it and push it to the front of the queue.
		placed, err := queue.IsSchedulingComplete(w, d.state)
		if err != nil {
			d.logger.Error("failed to wait for placement while enabling queue", "err", err)
		}

		if placed && evalHasPlacement(w.GetEval()) {
			d.updateUsage(w)
		}

		if !placed {
			w.waitOnRestore = true
			d.enqueueCh <- w
		}
	}

	d.decayUsage(now, ss)

	return nil
}

// Enqueue is the method used to put evaluations on the queue.
// It generates a workload with an empty priority, appends it
// to an internal channel to be processed and added to the actual
// heap container.
func (d *DynamicPriorityQueue) Enqueue(e *structs.Evaluation) {
	w := d.generateWorkload(e)

	// in the event of an empty workload, just pass eval to eval broker
	if w == nil {
		d.evalBroker.Enqueue(e)
		return
	}

	d.enqueueCh <- w
}

// runProducer pushes workloads onto the queue and notifies the consumer
// goroutine. It also updates priorities on the configured interval.
func (d *DynamicPriorityQueue) runProducer(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case w := <-d.enqueueCh:
			d.qMux.Lock()
			// use createTime so that workloads have consistent age
			// priority calculations after restoring from state.
			d.setWorkloadPriority(time.Unix(0, w.eval.CreateTime), w)
			d.queue.Push(w)
			d.qMux.Unlock()

			// Notify Workload consumer of new workload
			select {
			case d.qNotify <- struct{}{}:
			default:
			}
		case <-time.After(d.conf.CalcInterval):
			d.qMux.Lock()
			d.calculatePriorities(time.Now())
			d.qMux.Unlock()
		}
	}
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
			d.qMux.Lock()
			w := d.queue.Pop()
			d.qMux.Unlock()

			// We don't need to pass the waitOnRestore workload
			// to the eval broker, that already happened.
			if !w.WaitOnRestore() {
				d.evalBroker.Enqueue(w.GetEval())
			}

			// Wait for the eval to be placed
			err := queue.WaitForPlacement(ctx, w, d.state, memdb.NewWatchSet())
			if err != nil {
				d.logger.Error("failure waiting for workload placement", "evalID", w.GetEval().ID)
			}

			d.qMux.Lock()
			if evalHasPlacement(w.GetEval()) {
				d.updateUsage(w)
			}
			l := d.queue.Len()
			d.qMux.Unlock()

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
func (d *DynamicPriorityQueue) generateWorkload(e *structs.Evaluation) *dynamicPriorityWorkload {
	job, err := d.state.JobByID(nil, e.Namespace, e.JobID)
	if err != nil {
		return nil
	}

	var tid TenantID
	switch d.tenantType {
	case "namespace":
		tid = TenantID(job.Namespace)
	case "metadata":
		tenantID, ok := job.Meta[d.metadataKey]
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
		id:                 e.ID,
		tid:                tid,
		priority:           0,
		eval:               e,
		requestedResources: requestedResources,
		waitOnRestore:      false,
	}
}

// ensureTenant creates a new tenant in the queue if it doesn't already exist.
func (d *DynamicPriorityQueue) ensureTenant(tid TenantID) {
	if _, ok := d.tenants[tid]; ok {
		return
	}

	d.tenants[tid] = &Tenant{
		tid:                tid,
		placedWorkloadById: make(map[string]*dynamicPriorityWorkload),
		totalUsage:         &ResourceUsage{},
	}
}

// calculatePriorities iterates over all workloads in the queue and updates
// their priorities based on tenant usage, which is decayed according to the
// configured half-life, and usage weight.
func (d *DynamicPriorityQueue) calculatePriorities(now time.Time) {
	state, err := d.state.Snapshot()
	if err != nil {
		d.logger.Error("failed to take state snapshot", "error", err)
		return
	}
	// Decay tenant workload usages first, because a workload's
	// priority relies on its tenant's usage.
	d.decayUsage(now, state)

	// Now that we have accurate tenant usage, calculate
	// each workloads new priority and update the queue
	d.queue.UpdateAll(func(w queue.Workload) {
		workload := w.(*dynamicPriorityWorkload)
		d.setWorkloadPriority(now, workload)
	})
}

// setWorkloadPriority calculates an individual workload's priority based on
func (d *DynamicPriorityQueue) setWorkloadPriority(now time.Time, w *dynamicPriorityWorkload) {
	w.priority = w.eval.Priority +
		d.usageAdjustment(w) +
		d.ageAdjustment(now, w) +
		d.sizeAdjustment(w)
}

// usageAdjustment calculates the adjustment to a workload's priority based on
// it's tenant's usage relative to the total usage, and configured weight.
func (d *DynamicPriorityQueue) usageAdjustment(w *dynamicPriorityWorkload) int {
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
func (d *DynamicPriorityQueue) decayUsage(now time.Time, state *state.StateSnapshot) {
	totalUsage := &ResourceUsage{}

	for _, tenant := range d.tenants {
		newWorkloadUsageByID := make(map[string]*dynamicPriorityWorkload)
		tenantTotalUsage := &ResourceUsage{}

		for evalId, workload := range tenant.placedWorkloadById {
			eval, err := state.EvalByID(nil, evalId)
			if err != nil || eval == nil {
				continue
			}
			decayedResources := d.decayWorkloadUsage(now, workload.requestedResources)

			tenantTotalUsage = tenantTotalUsage.Add(decayedResources.resources)
			totalUsage = totalUsage.Add(decayedResources.resources)

			workload.requestedResources = decayedResources
			newWorkloadUsageByID[evalId] = workload
		}

		tenant.totalUsage = tenantTotalUsage
		tenant.placedWorkloadById = newWorkloadUsageByID
	}
	d.totalUsage = totalUsage
}

func decayMultiplier(now, createdAt time.Time, halfLife time.Duration) float64 {
	// elapsed := time.Unix(0, ts).Sub(time.Unix(0, createdAt)).Seconds()
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

	elapsed := now.UnixNano() - w.eval.CreateTime

	age := float64(elapsed) / float64(d.conf.MaxAge)
	ageClamped := min(1.0, max(0.0, age))

	w.ageAdjustment = int(ageClamped * float64(d.conf.AgeWeight))
	return w.ageAdjustment
}

func (d *DynamicPriorityQueue) sizeAdjustment(w *dynamicPriorityWorkload) int {
	if d.conf.SizeWeight == 0 {
		return 0
	}

	size := w.requestedResources.resources.Total() / float64(d.conf.MaxSize)
	sizeClamped := min(1.0, max(0.0, size))

	w.sizeAdjustment = int((1 - sizeClamped) * float64(d.conf.SizeWeight))
	return w.sizeAdjustment
}

func (d *DynamicPriorityQueue) Jobs(sortOrder structs.SortOrder) *queue.WorkloadIter {
	d.qMux.Lock()
	sortedWorkloads := d.queue.Slice()
	d.qMux.Unlock()

	pos := 0
	workloads := []structs.QueueWorkload{}
	for _, workload := range sortedWorkloads {
		w := workload.(*dynamicPriorityWorkload)
		// waitOnRestore does not count towards position in queue
		if w.waitOnRestore {
			continue
		}
		pos++

		workloads = append(workloads, &structs.DynamicPriorityWorkload{
			JobID:            w.eval.JobID,
			Tenant:           string(w.tid),
			Namespace:        w.eval.Namespace,
			Position:         pos,
			AdjustedPriority: w.priority,
			BasePriority:     w.eval.Priority,
			UsageAdjustment:  w.usageAdjustment,
			AgeAdjustment:    w.ageAdjustment,
			SizeAdjustment:   w.sizeAdjustment,
			CreatedAt:        w.eval.CreateTime,
			CreateIndex:      w.eval.CreateIndex,
		})
	}
	iter := queue.NewWorkloadIter(workloads)

	if sortOrder != structs.SortByPriority {
		iter.SortByJobId()
	}

	return iter
}

func (d *DynamicPriorityQueue) Tenants() structs.QueueTenantsResponse {
	d.qMux.Lock()
	defer d.qMux.Unlock()

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
func (d *DynamicPriorityQueue) updateUsage(w queue.Workload) {

	workload := w.(*dynamicPriorityWorkload)
	tenant := d.tenants[workload.tid]

	_, ok := tenant.placedWorkloadById[workload.id]
	// If the workload has already been placed, don't count the usage again.
	if ok {
		return
	}

	workloadResources := workload.requestedResources
	// this method should only be called when a workload was successfully placed,
	// so we can use the ModifyTime as the for when decay will start.
	workloadResources.start = time.Unix(0, workload.eval.ModifyTime)
	tenant.totalUsage = tenant.totalUsage.Add(workloadResources.resources)
	d.totalUsage = d.totalUsage.Add(workloadResources.resources)

	tenant.placedWorkloadById[workload.id] = workload
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
