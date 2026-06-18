// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queues

import (
	"container/heap"
	"context"
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

var ErrWatchedEvalNotFound = errors.New("watched evaluation not found")

type TenantID string

type DynamicPriorityQueue struct {
	// tenants is used to keep track of cluster usage for this queue.
	// When workloads are placed or the  configured interval is passed,
	// cluster usage is updated for the workloads of each tenant.
	tenants map[TenantID]*Tenant

	// queue is the main datastructure that contains all pending workloads
	//
	// TODO: at the moment, this is using the go stdlib container/heap package,
	// but we may want to switch to treeset from Hashicorp's go-set.
	// Why? Both have O(logn) push/pop. Heap has constant time peeking, but
	// we don't use that. We do want to iterate over workloads quickly, which
	// we can do with a red-black tree.
	queue WorkloadQueue

	// qMux locks the queue during concurrent access
	qMux sync.Mutex

	// qNotify allows for notifying the consumer that workloads
	// have been added to the queue
	qNotify chan struct{}

	// enqueueCh is used to buffer workloads before they
	// are processed by the manager and pushed onto the queue
	enqueueCh chan *Workload

	// totalUsage is the sum of all tenant usages
	totalUsage *ResourceUsage

	tenantType structs.BatchQueueTenant

	metadataKey string

	// conf contains user configurations for tuning the behavior of the queue
	conf *structs.DynamicQueueConfig

	// evalBroker is the injected broker for passing an evaluation
	// on to be scheduled by Nomad
	evalBroker Broker

	// enabled tracks whether the server running the batch job queue is the leader
	// so should process evaluations
	enabled atomic.Bool

	// state is the in-memory state store used for both reconciling tenant
	// workload usages, and polling submitted evaluations for placement
	state  *state.StateStore
	logger hclog.Logger
}

type Tenant struct {
	tid                TenantID
	placedWorkloadById map[string]*Workload
	totalUsage         *ResourceUsage
}

func (t *Tenant) totalPercentageUsed(totalUsage *ResourceUsage) int {
	if totalUsage.Total() == 0 {
		return 0
	}

	return int((t.totalUsage.Total() / totalUsage.Total()) * 100)
}

type UsageList struct {
	resources *ResourceUsage
	start     time.Time
}

type Workload struct {
	id                 string
	tid                TenantID
	priority           int
	eval               *structs.Evaluation
	requestedResources *UsageList
	index              int

	sizeAdjustment  int
	ageAdjustment   int
	usageAdjustment int
}

func NewDynamicPriorityQueue(broker Broker, qconf *structs.BatchQueue, conf *structs.DynamicQueueConfig, logger hclog.Logger) *DynamicPriorityQueue {
	return &DynamicPriorityQueue{
		tenants:     make(map[TenantID]*Tenant),
		queue:       WorkloadQueue{},
		enqueueCh:   make(chan *Workload, 8192),
		evalBroker:  broker,
		qMux:        sync.Mutex{},
		qNotify:     make(chan struct{}, 1),
		tenantType:  qconf.TenantType,
		metadataKey: qconf.MetadataKey,
		conf:        conf,
		logger:      logger.Named("Dynamic Priority Queue"),
		totalUsage:  &ResourceUsage{},
	}
}

func (d *DynamicPriorityQueue) Start(ctx context.Context) error {
	go d.runProducer(ctx)
	go d.runConsumer(ctx)

	return nil
}

func (d *DynamicPriorityQueue) SetEnabled(val bool, state *state.StateStore) {
	// rebuild internal state from statestore, unimplemented
	d.state = state
	d.enabled.Store(val)
}

// Enqueue is the method used to put evaluations on the queue.
// It generates a workload with an empty priority, appends it
// to an internal channel to be processed and added to the actual
// heap container.
func (d *DynamicPriorityQueue) Enqueue(e *structs.Evaluation) {
	if !d.enabled.Load() {
		return
	}

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
			d.setWorkloadPriority(time.Now(), w)
			heap.Push(&d.queue, w)
			d.qMux.Unlock()

			// Notify Workload consumer of new workload
			select {
			case d.qNotify <- struct{}{}:
			default:
			}
		case <-time.After(d.conf.CalcInterval):
			if !d.enabled.Load() {
				continue
			}

			d.qMux.Lock()
			d.calculatePriorities(time.Now())
			heap.Init(&d.queue)
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
			workload := heap.Pop(&d.queue).(*Workload)
			d.qMux.Unlock()

			// Give the eval to the eval broker
			d.evalBroker.Enqueue(workload.eval)

			// Wait for the eval to be placed
			err := d.waitForPlacement(ctx, workload, memdb.NewWatchSet())
			if err != nil {
				d.logger.Error("failure waiting for workload placement", "evalID", workload.id)
			}

			d.qMux.Lock()
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
func (d *DynamicPriorityQueue) generateWorkload(e *structs.Evaluation) *Workload {
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

	return &Workload{
		id:                 e.ID,
		tid:                tid,
		priority:           0,
		eval:               e,
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
		placedWorkloadById: make(map[string]*Workload),
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
	// each workloads new priority
	for _, workload := range d.queue {
		d.setWorkloadPriority(now, workload)
	}
}

// setWorkloadPriority calculates an individual workload's priority based on
func (d *DynamicPriorityQueue) setWorkloadPriority(now time.Time, w *Workload) {
	w.priority = w.eval.Priority +
		d.usageAdjustment(w) +
		d.ageAdjustment(now, w) +
		d.sizeAdjustment(w)
}

// usageAdjustment calculates the adjustment to a workload's priority based on
// it's tenant's usage relative to the total usage, and configured weight.
func (d *DynamicPriorityQueue) usageAdjustment(w *Workload) int {
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
		newWorkloadUsageByID := make(map[string]*Workload)
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

func (d *DynamicPriorityQueue) ageAdjustment(now time.Time, w *Workload) int {
	if d.conf.AgeWeight == 0 {
		return 0
	}

	elapsed := now.UnixNano() - w.eval.CreateTime

	age := float64(elapsed) / float64(d.conf.MaxAge)
	ageClamped := min(1.0, max(0.0, age))

	w.ageAdjustment = int(ageClamped * float64(d.conf.AgeWeight))
	return w.ageAdjustment
}

func (d *DynamicPriorityQueue) sizeAdjustment(w *Workload) int {
	if d.conf.SizeWeight == 0 {
		return 0
	}

	size := w.requestedResources.resources.Total() / float64(d.conf.MaxSize)
	sizeClamped := min(1.0, max(0.0, size))

	w.sizeAdjustment = int((1 - sizeClamped) * float64(d.conf.SizeWeight))
	return w.sizeAdjustment
}

// waitForPlacement follows a given evalutation in the state store until it, or it's nexted/blocked evals
// have been marked terminal, indicating the workload has been scheduled.
//
// Note: If a job with an unsatisfiable contraint is given to the Eval Broker, this function will block
// until a Nomad operator manually intervenes and stops the job. In the future, we can add an optional
// configurable timeout for this blocking query.
func (d *DynamicPriorityQueue) waitForPlacement(ctx context.Context, workload *Workload, ws memdb.WatchSet) error {
	eval := workload.eval
	for !eval.TerminalStatus() || eval.BlockedEval != "" || eval.NextEval != "" {
		id := eval.ID

		if eval.BlockedEval != "" {
			id = eval.BlockedEval
		} else if eval.NextEval != "" {
			id = eval.NextEval
		}

		snap, err := d.state.Snapshot()
		if err != nil {
			return err
		}

		// TODO: handle snapshot restores
		abandonCh := snap.AbandonCh()
		ws.Add(abandonCh)

		eval, err = snap.EvalByID(ws, id)
		if err != nil {
			return err
		}
		if eval == nil {
			return ErrWatchedEvalNotFound
		}

		workload.eval = eval

		if eval.TerminalStatus() {

			continue
		}

		// If the latest version of the eval isn't terminal, wait for an update
		if err = ws.WatchCtx(ctx); err != nil {
			return err
		}

		// The watch channel will be closed, we should delete it to
		// prevent immediately firing on the next WatchCtx
		for k := range ws {
			delete(ws, k)
		}
	}
	if eval.Status == structs.EvalStatusComplete {
		// If the eval is terminal and has plan annotations, something might
		// have been placed and we should update tenant usage accordingly.
		if eval.PlanAnnotations != nil && eval.PlanAnnotations.DesiredTGUpdates != nil {
			d.qMux.Lock()
			d.updateUsage(workload)
			d.qMux.Unlock()
		}
	}

	return nil
}

func (d *DynamicPriorityQueue) Jobs(namespaces map[string]bool) structs.QueueJobsResponse {
	d.qMux.Lock()
	defer d.qMux.Unlock()

	workloads := []structs.DynamicPriorityWorkload{}
	for _, w := range d.queue {
		if (namespaces != nil) && !namespaces[w.eval.Namespace] {
			continue
		}
		workloads = append(workloads, structs.DynamicPriorityWorkload{
			JobID:            w.eval.JobID,
			Tenant:           string(w.tid),
			Index:            w.index,
			AdjustedPriority: w.priority,
			BasePriority:     w.eval.Priority,
			UsageAdjustment:  w.usageAdjustment,
			AgeAdjustment:    w.ageAdjustment,
			SizeAdjustment:   w.sizeAdjustment,
		})
	}
	return structs.QueueJobsResponse{
		Type:      structs.BatchQueueTypeDynamic,
		Workloads: workloads,
	}
}

func (d *DynamicPriorityQueue) Tenants() structs.QueueTenantsResponse {
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

// updateUsage updates the tenant and total usage for a given workload if
// anything has been placed.
func (d *DynamicPriorityQueue) updateUsage(workload *Workload) {
	tenant := d.tenants[workload.tid]
	placed := false

	for _, desired := range workload.eval.PlanAnnotations.DesiredTGUpdates {
		if desired.Place != 0 {
			placed = true
			break
		}
	}

	if placed {
		_, ok := tenant.placedWorkloadById[workload.id]
		// If the workload has already been placed, don't count the usage again.
		if ok {
			return
		}

		workloadResources := workload.requestedResources
		workloadResources.start = time.Unix(0, workload.eval.ModifyTime)
		tenant.totalUsage = tenant.totalUsage.Add(workloadResources.resources)
		d.totalUsage = d.totalUsage.Add(workloadResources.resources)

		tenant.placedWorkloadById[workload.id] = workload
	}
}
