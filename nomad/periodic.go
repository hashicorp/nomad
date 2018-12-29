package nomad

import (
	"container/heap"
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
)

// PeriodicDispatch is used to track and launch periodic jobs. It maintains the
// set of periodic jobs and creates derived jobs and evaluations per
// instantiation which is determined by the periodic spec.
type PeriodicDispatch struct {
	dispatcher JobEvalDispatcher
	enabled    bool

	tracked map[structs.NamespacedID]*structs.Job
	heap    *periodicHeap

	updateCh chan struct{}
	stopFn   context.CancelFunc
	logger   *log.Logger
	l        sync.RWMutex
}

// JobEvalDispatcher is an interface to submit jobs and have evaluations created
// for them.
type JobEvalDispatcher interface {
	// DispatchJob takes a job a new, untracked job and creates an evaluation
	// for it and returns the eval.
	DispatchJob(job *structs.Job) (*structs.Evaluation, error)

	// RunningChildren returns whether the passed job has any running children.
	RunningChildren(job *structs.Job) (bool, error)
}

// DispatchJob creates an evaluation for the passed job and commits both the
// evaluation and the job to the raft log. It returns the eval.
func (s *Server) DispatchJob(job *structs.Job) (*structs.Evaluation, error) {
	// Commit this update via Raft
	job.SetSubmitTime()
	req := structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Namespace: job.Namespace,
		},
	}
	fsmErr, index, err := s.raftApply(structs.JobRegisterRequestType, req)
	if err, ok := fsmErr.(error); ok && err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}

	// Create a new evaluation
	eval := &structs.Evaluation{
		ID:             uuid.Generate(),
		Namespace:      job.Namespace,
		Priority:       job.Priority,
		Type:           job.Type,
		TriggeredBy:    structs.EvalTriggerPeriodicJob,
		JobID:          job.ID,
		JobModifyIndex: index,
		Status:         structs.EvalStatusPending,
	}
	update := &structs.EvalUpdateRequest{
		Evals: []*structs.Evaluation{eval},
	}

	// Commit this evaluation via Raft
	// XXX: There is a risk of partial failure where the JobRegister succeeds
	// but that the EvalUpdate does not.
	_, evalIndex, err := s.raftApply(structs.EvalUpdateRequestType, update)
	if err != nil {
		return nil, err
	}

	// Update its indexes.
	eval.CreateIndex = evalIndex
	eval.ModifyIndex = evalIndex
	return eval, nil
}

// RunningChildren checks whether the passed job has any running children.
func (s *Server) RunningChildren(job *structs.Job) (bool, error) {
	state, err := s.fsm.State().Snapshot()
	if err != nil {
		return false, err
	}

	ws := memdb.NewWatchSet()
	prefix := fmt.Sprintf("%s%s", job.ID, structs.PeriodicLaunchSuffix)
	iter, err := state.JobsByIDPrefix(ws, job.Namespace, prefix)
	if err != nil {
		return false, err
	}

	var child *structs.Job
	for i := iter.Next(); i != nil; i = iter.Next() {
		child = i.(*structs.Job)

		// Ensure the job is actually a child.
		if child.ParentID != job.ID {
			continue
		}

		// Get the childs evaluations.
		evals, err := state.EvalsByJob(ws, child.Namespace, child.ID)
		if err != nil {
			return false, err
		}

		// Check if any of the evals are active or have running allocations.
		for _, eval := range evals {
			if !eval.TerminalStatus() {
				return true, nil
			}

			allocs, err := state.AllocsByEval(ws, eval.ID)
			if err != nil {
				return false, err
			}

			for _, alloc := range allocs {
				if !alloc.TerminalStatus() {
					return true, nil
				}
			}
		}
	}

	// There are no evals or allocations that aren't terminal.
	return false, nil
}

// NewPeriodicDispatch returns a periodic dispatcher that is used to track and
// launch periodic jobs.
func NewPeriodicDispatch(logger *log.Logger, dispatcher JobEvalDispatcher) *PeriodicDispatch {
	return &PeriodicDispatch{
		dispatcher: dispatcher,
		tracked:    make(map[structs.NamespacedID]*structs.Job),
		heap:       NewPeriodicHeap(),
		updateCh:   make(chan struct{}, 1),
		logger:     logger,
	}
}

// SetEnabled is used to control if the periodic dispatcher is enabled. It
// should only be enabled on the active leader. Disabling an active dispatcher
// will stop any launched go routine and flush the dispatcher.
func (p *PeriodicDispatch) SetEnabled(enabled bool) {
	p.l.Lock()
	defer p.l.Unlock()
	wasRunning := p.enabled
	p.enabled = enabled

	// If we are transistioning from enabled to disabled, stop the daemon and
	// flush.
	if !enabled && wasRunning {
		p.stopFn()
		p.flush()
	} else if enabled && !wasRunning {
		// If we are transitioning from disabled to enabled, run the daemon.
		ctx, cancel := context.WithCancel(context.Background())
		p.stopFn = cancel
		go p.run(ctx)
	}
}

// Tracked returns the set of tracked job IDs.
func (p *PeriodicDispatch) Tracked() []*structs.Job {
	p.l.RLock()
	defer p.l.RUnlock()
	tracked := make([]*structs.Job, len(p.tracked))
	i := 0
	for _, job := range p.tracked {
		tracked[i] = job
		i++
	}
	return tracked
}

// Add begins tracking of a periodic job. If it is already tracked, it acts as
// an update to the jobs periodic spec. The method returns whether the job was
// added and any error that may have occurred.
func (p *PeriodicDispatch) Add(job *structs.Job) (added bool, err error) {
	p.l.Lock()
	defer p.l.Unlock()

	// Do nothing if not enabled
	if !p.enabled {
		return false, nil
	}

	// If we were tracking a job and it has been disabled, made non-periodic,
	// stopped or is parameterized, remove it
	disabled := !job.IsPeriodic() || !job.Periodic.Enabled || job.Stopped() || job.IsParameterized()

	tuple := structs.NamespacedID{
		ID:        job.ID,
		Namespace: job.Namespace,
	}
	_, tracked := p.tracked[tuple]
	if disabled {
		if tracked {
			p.removeLocked(tuple)
		}

		// If the job is disabled and we aren't tracking it, do nothing.
		return false, nil
	}

	// Add or update the job.
	p.tracked[tuple] = job
	next := job.Periodic.Next(time.Now().In(job.Periodic.GetLocation()))
	if tracked {
		if err := p.heap.Update(job, next); err != nil {
			return false, fmt.Errorf("failed to update job %q (%s) launch time: %v", job.ID, job.Namespace, err)
		}
		p.logger.Printf("[DEBUG] nomad.periodic: updated periodic job %q (%s)", job.ID, job.Namespace)
	} else {
		if err := p.heap.Push(job, next); err != nil {
			return false, fmt.Errorf("failed to add job %v: %v", job.ID, err)
		}
		p.logger.Printf("[DEBUG] nomad.periodic: registered periodic job %q (%s)", job.ID, job.Namespace)
	}

	// Signal an update.
	select {
	case p.updateCh <- struct{}{}:
	default:
	}

	return true, nil
}

// Remove stops tracking the passed job. If the job is not tracked, it is a
// no-op.
func (p *PeriodicDispatch) Remove(namespace, jobID string) error {
	p.l.Lock()
	defer p.l.Unlock()
	return p.removeLocked(structs.NamespacedID{
		ID:        jobID,
		Namespace: namespace,
	})
}

// Remove stops tracking the passed job. If the job is not tracked, it is a
// no-op. It assumes this is called while a lock is held.
func (p *PeriodicDispatch) removeLocked(jobID structs.NamespacedID) error {
	// Do nothing if not enabled
	if !p.enabled {
		return nil
	}

	job, tracked := p.tracked[jobID]
	if !tracked {
		return nil
	}

	delete(p.tracked, jobID)
	if err := p.heap.Remove(job); err != nil {
		return fmt.Errorf("failed to remove tracked job %q (%s): %v", jobID.ID, jobID.Namespace, err)
	}

	// Signal an update.
	select {
	case p.updateCh <- struct{}{}:
	default:
	}

	p.logger.Printf("[DEBUG] nomad.periodic: deregistered periodic job %q (%s)", jobID.ID, jobID.Namespace)
	return nil
}

// ForceRun causes the periodic job to be evaluated immediately and returns the
// subsequent eval.
func (p *PeriodicDispatch) ForceRun(namespace, jobID string) (*structs.Evaluation, error) {
	p.l.Lock()

	// Do nothing if not enabled
	if !p.enabled {
		p.l.Unlock()
		return nil, fmt.Errorf("periodic dispatch disabled")
	}

	tuple := structs.NamespacedID{
		ID:        jobID,
		Namespace: namespace,
	}
	job, tracked := p.tracked[tuple]
	if !tracked {
		p.l.Unlock()
		return nil, fmt.Errorf("can't force run non-tracked job %q (%s)", jobID, namespace)
	}

	p.l.Unlock()
	return p.createEval(job, time.Now().In(job.Periodic.GetLocation()))
}

// shouldRun returns whether the long lived run function should run.
func (p *PeriodicDispatch) shouldRun() bool {
	p.l.RLock()
	defer p.l.RUnlock()
	return p.enabled
}

// run is a long-lived function that waits till a job's periodic spec is met and
// then creates an evaluation to run the job.
func (p *PeriodicDispatch) run(ctx context.Context) {
	var launchCh <-chan time.Time
	for p.shouldRun() {
		job, launch := p.nextLaunch()
		if launch.IsZero() {
			launchCh = nil
		} else {
			launchDur := launch.Sub(time.Now().In(job.Periodic.GetLocation()))
			launchCh = time.After(launchDur)
			p.logger.Printf("[DEBUG] nomad.periodic: launching job %q (%s) in %s", job.ID, job.Namespace, launchDur)
		}

		select {
		case <-ctx.Done():
			return
		case <-p.updateCh:
			continue
		case <-launchCh:
			p.dispatch(job, launch)
		}
	}
}

// dispatch creates an evaluation for the job and updates its next launchtime
// based on the passed launch time.
func (p *PeriodicDispatch) dispatch(job *structs.Job, launchTime time.Time) {
	p.l.Lock()

	nextLaunch := job.Periodic.Next(launchTime)
	if err := p.heap.Update(job, nextLaunch); err != nil {
		p.logger.Printf("[ERR] nomad.periodic: failed to update next launch of periodic job %q (%s): %v", job.ID, job.Namespace, err)
	}

	// If the job prohibits overlapping and there are running children, we skip
	// the launch.
	if job.Periodic.ProhibitOverlap {
		running, err := p.dispatcher.RunningChildren(job)
		if err != nil {
			msg := fmt.Sprintf("[ERR] nomad.periodic: failed to determine if"+
				" periodic job %q (%s) has running children: %v", job.ID, job.Namespace, err)
			p.logger.Println(msg)
			p.l.Unlock()
			return
		}

		if running {
			msg := fmt.Sprintf("[DEBUG] nomad.periodic: skipping launch of"+
				" periodic job %q (%s) because job prohibits overlap", job.ID, job.Namespace)
			p.logger.Println(msg)
			p.l.Unlock()
			return
		}
	}

	p.logger.Printf("[DEBUG] nomad.periodic: launching job %q (%v) at %v", job.ID, job.Namespace, launchTime)
	p.l.Unlock()
	p.createEval(job, launchTime)
}

// nextLaunch returns the next job to launch and when it should be launched. If
// the next job can't be determined, an error is returned. If the dispatcher is
// stopped, a nil job will be returned.
func (p *PeriodicDispatch) nextLaunch() (*structs.Job, time.Time) {
	// If there is nothing wait for an update.
	p.l.RLock()
	defer p.l.RUnlock()
	if p.heap.Length() == 0 {
		return nil, time.Time{}
	}

	nextJob := p.heap.Peek()
	if nextJob == nil {
		return nil, time.Time{}
	}

	return nextJob.job, nextJob.next
}

// createEval instantiates a job based on the passed periodic job and submits an
// evaluation for it. This should not be called with the lock held.
func (p *PeriodicDispatch) createEval(periodicJob *structs.Job, time time.Time) (*structs.Evaluation, error) {
	derived, err := p.deriveJob(periodicJob, time)
	if err != nil {
		return nil, err
	}

	eval, err := p.dispatcher.DispatchJob(derived)
	if err != nil {
		p.logger.Printf("[ERR] nomad.periodic: failed to dispatch job %q (%s): %v",
			periodicJob.ID, periodicJob.Namespace, err)
		return nil, err
	}

	return eval, nil
}

// deriveJob instantiates a new job based on the passed periodic job and the
// launch time.
func (p *PeriodicDispatch) deriveJob(periodicJob *structs.Job, time time.Time) (
	derived *structs.Job, err error) {

	// Have to recover in case the job copy panics.
	defer func() {
		if r := recover(); r != nil {
			p.logger.Printf("[ERR] nomad.periodic: deriving job from"+
				" periodic job %q (%s) failed; deregistering from periodic runner: %v",
				periodicJob.ID, periodicJob.Namespace, r)

			p.Remove(periodicJob.Namespace, periodicJob.ID)
			derived = nil
			err = fmt.Errorf("Failed to create a copy of the periodic job %q (%s): %v",
				periodicJob.ID, periodicJob.Namespace, r)
		}
	}()

	// Create a copy of the periodic job, give it a derived ID/Name and make it
	// non-periodic.
	derived = periodicJob.Copy()
	derived.ParentID = periodicJob.ID
	derived.ID = p.derivedJobID(periodicJob, time)
	derived.Name = derived.ID
	derived.Periodic = nil
	return
}

// deriveJobID returns a job ID based on the parent periodic job and the launch
// time.
func (p *PeriodicDispatch) derivedJobID(periodicJob *structs.Job, time time.Time) string {
	return fmt.Sprintf("%s%s%d", periodicJob.ID, structs.PeriodicLaunchSuffix, time.Unix())
}

// LaunchTime returns the launch time of the job. This is only valid for
// jobs created by PeriodicDispatch and will otherwise return an error.
func (p *PeriodicDispatch) LaunchTime(jobID string) (time.Time, error) {
	index := strings.LastIndex(jobID, structs.PeriodicLaunchSuffix)
	if index == -1 {
		return time.Time{}, fmt.Errorf("couldn't parse launch time from eval: %v", jobID)
	}

	launch, err := strconv.Atoi(jobID[index+len(structs.PeriodicLaunchSuffix):])
	if err != nil {
		return time.Time{}, fmt.Errorf("couldn't parse launch time from eval: %v", jobID)
	}

	return time.Unix(int64(launch), 0), nil
}

// flush clears the state of the PeriodicDispatcher
func (p *PeriodicDispatch) flush() {
	p.updateCh = make(chan struct{}, 1)
	p.tracked = make(map[structs.NamespacedID]*structs.Job)
	p.heap = NewPeriodicHeap()
	p.stopFn = nil
}

// periodicHeap wraps a heap and gives operations other than Push/Pop.
type periodicHeap struct {
	index map[structs.NamespacedID]*periodicJob
	heap  periodicHeapImp
}

type periodicJob struct {
	job   *structs.Job
	next  time.Time
	index int
}

func NewPeriodicHeap() *periodicHeap {
	return &periodicHeap{
		index: make(map[structs.NamespacedID]*periodicJob),
		heap:  make(periodicHeapImp, 0),
	}
}

func (p *periodicHeap) Push(job *structs.Job, next time.Time) error {
	tuple := structs.NamespacedID{
		ID:        job.ID,
		Namespace: job.Namespace,
	}
	if _, ok := p.index[tuple]; ok {
		return fmt.Errorf("job %q (%s) already exists", job.ID, job.Namespace)
	}

	pJob := &periodicJob{job, next, 0}
	p.index[tuple] = pJob
	heap.Push(&p.heap, pJob)
	return nil
}

func (p *periodicHeap) Pop() *periodicJob {
	if len(p.heap) == 0 {
		return nil
	}

	pJob := heap.Pop(&p.heap).(*periodicJob)
	tuple := structs.NamespacedID{
		ID:        pJob.job.ID,
		Namespace: pJob.job.Namespace,
	}
	delete(p.index, tuple)
	return pJob
}

func (p *periodicHeap) Peek() *periodicJob {
	if len(p.heap) == 0 {
		return nil
	}

	return p.heap[0]
}

func (p *periodicHeap) Contains(job *structs.Job) bool {
	tuple := structs.NamespacedID{
		ID:        job.ID,
		Namespace: job.Namespace,
	}
	_, ok := p.index[tuple]
	return ok
}

func (p *periodicHeap) Update(job *structs.Job, next time.Time) error {
	tuple := structs.NamespacedID{
		ID:        job.ID,
		Namespace: job.Namespace,
	}
	if pJob, ok := p.index[tuple]; ok {
		// Need to update the job as well because its spec can change.
		pJob.job = job
		pJob.next = next
		heap.Fix(&p.heap, pJob.index)
		return nil
	}

	return fmt.Errorf("heap doesn't contain job %q (%s)", job.ID, job.Namespace)
}

func (p *periodicHeap) Remove(job *structs.Job) error {
	tuple := structs.NamespacedID{
		ID:        job.ID,
		Namespace: job.Namespace,
	}
	if pJob, ok := p.index[tuple]; ok {
		heap.Remove(&p.heap, pJob.index)
		delete(p.index, tuple)
		return nil
	}

	return fmt.Errorf("heap doesn't contain job %q (%s)", job.ID, job.Namespace)
}

func (p *periodicHeap) Length() int {
	return len(p.heap)
}

type periodicHeapImp []*periodicJob

func (h periodicHeapImp) Len() int { return len(h) }

func (h periodicHeapImp) Less(i, j int) bool {
	// Two zero times should return false.
	// Otherwise, zero is "greater" than any other time.
	// (To sort it at the end of the list.)
	// Sort such that zero times are at the end of the list.
	iZero, jZero := h[i].next.IsZero(), h[j].next.IsZero()
	if iZero && jZero {
		return false
	} else if iZero {
		return false
	} else if jZero {
		return true
	}

	return h[i].next.Before(h[j].next)
}

func (h periodicHeapImp) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *periodicHeapImp) Push(x interface{}) {
	n := len(*h)
	job := x.(*periodicJob)
	job.index = n
	*h = append(*h, job)
}

func (h *periodicHeapImp) Pop() interface{} {
	old := *h
	n := len(old)
	job := old[n-1]
	job.index = -1 // for safety
	*h = old[0 : n-1]
	return job
}
