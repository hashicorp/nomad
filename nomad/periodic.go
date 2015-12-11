package nomad

import (
	"container/heap"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

// PeriodicRunner is the interface for tracking and launching periodic jobs at
// their periodic spec.
type PeriodicRunner interface {
	Start()
	SetEnabled(enabled bool)
	Add(job *structs.Job) error
	Remove(jobID string) error
	ForceRun(jobID string) error
	Tracked() []structs.Job
	Flush()
}

// PeriodicDispatch is used to track and launch periodic jobs. It maintains the
// set of periodic jobs and creates derived jobs and evaluations per
// instantiation which is determined by the periodic spec.
type PeriodicDispatch struct {
	srv     *Server
	enabled bool
	running bool

	tracked map[string]*structs.Job
	heap    *periodicHeap

	updateCh chan struct{}
	stopCh   chan struct{}
	logger   *log.Logger
	l        sync.RWMutex
}

// NewPeriodicDispatch returns a periodic dispatcher that is used to track and
// launch periodic jobs.
func NewPeriodicDispatch(srv *Server) *PeriodicDispatch {
	return &PeriodicDispatch{
		srv:      srv,
		tracked:  make(map[string]*structs.Job),
		heap:     NewPeriodicHeap(),
		updateCh: make(chan struct{}, 1),
		stopCh:   make(chan struct{}, 1),
		logger:   srv.logger,
	}
}

// SetEnabled is used to control if the periodic dispatcher is enabled. It
// should only be enabled on the active leader. Disabling an active dispatcher
// will stop any launched go routine and flush the dispatcher.
func (p *PeriodicDispatch) SetEnabled(enabled bool) {
	p.l.Lock()
	p.enabled = enabled
	p.l.Unlock()
	if !enabled {
		p.stopCh <- struct{}{}
		p.Flush()
	}
}

// Start begins the goroutine that creates derived jobs and evals.
func (p *PeriodicDispatch) Start() {
	p.l.Lock()
	p.running = true
	p.l.Unlock()
	go p.run()
}

// Tracked returns the set of tracked job IDs.
func (p *PeriodicDispatch) Tracked() []structs.Job {
	p.l.RLock()
	defer p.l.RUnlock()
	tracked := make([]structs.Job, len(p.tracked))
	i := 0
	for _, job := range p.tracked {
		tracked[i] = *job
		i++
	}
	return tracked
}

// Add begins tracking of a periodic job. If it is already tracked, it acts as
// an update to the jobs periodic spec.
func (p *PeriodicDispatch) Add(job *structs.Job) error {
	p.l.Lock()
	defer p.l.Unlock()

	// Do nothing if not enabled
	if !p.enabled {
		return fmt.Errorf("periodic dispatch disabled")
	}

	// If we were tracking a job and it has been disabled or made non-periodic remove it.
	disabled := !job.IsPeriodic() || !job.Periodic.Enabled
	_, tracked := p.tracked[job.ID]
	if tracked && disabled {
		return p.Remove(job.ID)
	}

	// If the job is diabled and we aren't tracking it, do nothing.
	if disabled {
		return nil
	}

	// Add or update the job.
	p.tracked[job.ID] = job
	next := job.Periodic.Next(time.Now())
	if tracked {
		if err := p.heap.Update(job, next); err != nil {
			return fmt.Errorf("failed to update job %v launch time: %v", job.ID, err)
		}
	} else {
		if err := p.heap.Push(job, next); err != nil {
			return fmt.Errorf("failed to add job %v", job.ID, err)
		}
	}

	// Signal an update.
	if p.running {
		select {
		case p.updateCh <- struct{}{}:
		default:
		}
	}

	return nil
}

// Remove stops tracking the passed job. If the job is not tracked, it is a
// no-op.
func (p *PeriodicDispatch) Remove(jobID string) error {
	p.l.Lock()
	defer p.l.Unlock()

	// Do nothing if not enabled
	if !p.enabled {
		return fmt.Errorf("periodic dispatch disabled")
	}

	if job, tracked := p.tracked[jobID]; tracked {
		delete(p.tracked, jobID)
		if err := p.heap.Remove(job); err != nil {
			return fmt.Errorf("failed to remove tracked job %v: %v", jobID, err)
		}
	}

	// Signal an update.
	if p.running {
		select {
		case p.updateCh <- struct{}{}:
		default:
		}
	}

	return nil
}

// ForceRun causes the periodic job to be evaluated immediately.
func (p *PeriodicDispatch) ForceRun(jobID string) error {
	p.l.Lock()
	defer p.l.Unlock()

	// Do nothing if not enabled
	if !p.enabled {
		return fmt.Errorf("periodic dispatch disabled")
	}

	job, tracked := p.tracked[jobID]
	if !tracked {
		return fmt.Errorf("can't force run non-tracked job %v", jobID)
	}

	return p.createEval(job, time.Now())
}

// run is a long-lived function that waits til a job's periodic spec is met and
// then creates an evaluation to run the job.
func (p *PeriodicDispatch) run() {
	// Do nothing if not enabled.
	p.l.RLock()
	if !p.enabled {
		p.l.RUnlock()
		return
	}
	p.l.RUnlock()

	now := time.Now().Local()

PICK:
	// If there is nothing wait for an update.
	p.l.RLock()
	if p.heap.Length() == 0 {
		p.l.RUnlock()
		<-p.updateCh
		p.l.RLock()
	}

	nextJob, err := p.heap.Peek()
	p.l.RUnlock()
	if err != nil {
		p.logger.Printf("[ERR] nomad.periodic_dispatch: failed to determine next periodic job: %v", err)
		return
	}

	launchTime := nextJob.next

	// If there are only invalid times, wait for an update.
	if launchTime.IsZero() {
		<-p.updateCh
		goto PICK
	}

	select {
	case <-p.stopCh:
		return
	case <-p.updateCh:
		goto PICK
	case <-time.After(nextJob.next.Sub(now)):
		// Get the current time so that we don't miss any jobs will we are
		// creating evals.
		nowUpdate := time.Now()

		// Create evals for all the jobs with the same launch time.
		p.l.Lock()
		for {
			if p.heap.Length() == 0 {
				break
			}

			j, err := p.heap.Peek()
			if err != nil {
				p.logger.Printf("[ERR] nomad.periodic_dispatch: failed to determine next periodic job: %v", err)
				break
			}

			if j.next != launchTime {
				break
			}

			if err := p.heap.Update(j.job, j.job.Periodic.Next(nowUpdate)); err != nil {
				p.logger.Printf("[ERR] nomad.periodic_dispatch: failed to update next launch of periodic job: %v", j.job.ID, err)
			}

			// TODO(alex): Want to be able to check that there isn't a previously
			// running cron job for this job.
			go p.createEval(j.job, launchTime)
		}

		p.l.Unlock()
		now = nowUpdate
	}

	goto PICK
}

// createEval instantiates a job based on the passed periodic job and submits an
// evaluation for it.
func (p *PeriodicDispatch) createEval(periodicJob *structs.Job, time time.Time) error {
	derived, err := p.deriveJob(periodicJob, time)
	if err != nil {
		return err
	}

	// Commit this update via Raft
	req := structs.JobRegisterRequest{Job: derived}
	_, index, err := p.srv.raftApply(structs.JobRegisterRequestType, req)
	if err != nil {
		p.logger.Printf("[ERR] nomad.periodic_dispatch: Register failed: %v", err)
		return err
	}

	// Create a new evaluation
	eval := &structs.Evaluation{
		ID:             structs.GenerateUUID(),
		Priority:       derived.Priority,
		Type:           derived.Type,
		TriggeredBy:    structs.EvalTriggerJobRegister,
		JobID:          derived.ID,
		JobModifyIndex: index,
		Status:         structs.EvalStatusPending,
	}
	update := &structs.EvalUpdateRequest{
		Evals: []*structs.Evaluation{eval},
	}

	// Commit this evaluation via Raft
	// XXX: There is a risk of partial failure where the JobRegister succeeds
	// but that the EvalUpdate does not.
	_, _, err = p.srv.raftApply(structs.EvalUpdateRequestType, update)
	if err != nil {
		p.logger.Printf("[ERR] nomad.periodic_dispatch: Eval create failed: %v", err)
		return err
	}

	return nil
}

// deriveJob instantiates a new job based on the passed periodic job and the
// launch time.
// TODO: these jobs need to be marked as GC'able
func (p *PeriodicDispatch) deriveJob(periodicJob *structs.Job, time time.Time) (
	derived *structs.Job, err error) {

	// Have to recover in case the job copy panics.
	defer func() {
		if r := recover(); r != nil {
			p.logger.Printf("[ERR] nomad.periodic_dispatch: deriving job from"+
				" periodic job %v failed; deregistering from periodic runner: %v",
				periodicJob.ID, r)
			p.Remove(periodicJob.ID)
			derived = nil
			err = fmt.Errorf("Failed to create a copy of the periodic job %v: %v", periodicJob.ID, r)
		}
	}()

	// Create a copy of the periodic job, give it a derived ID/Name and make it
	// non-periodic.
	derived = periodicJob.Copy()
	derived.ParentID = periodicJob.ID
	derived.ID = p.derivedJobID(periodicJob, time)
	derived.Name = periodicJob.ID
	derived.Periodic = nil
	return
}

// deriveJobID returns a job ID based on the parent periodic job and the launch
// time.
func (p *PeriodicDispatch) derivedJobID(periodicJob *structs.Job, time time.Time) string {
	return fmt.Sprintf("%s-%d", periodicJob.ID, time.Unix())
}

// CreatedEvals returns the set of evaluations created from the passed periodic
// job.
func (p *PeriodicDispatch) CreatedEvals(periodicJobID string) ([]*structs.Evaluation, error) {
	state := p.srv.fsm.State()
	iter, err := state.ChildJobs(periodicJobID)
	if err != nil {
		return nil, fmt.Errorf("failed to look up children of job %v: %v", periodicJobID, err)
	}

	var evals []*structs.Evaluation
	for i := iter.Next(); i != nil; i = iter.Next() {
		job := i.(*structs.Job)
		childEvals, err := state.EvalsByJob(job.ID)
		if err != nil {
			fmt.Errorf("failed to look up evals for job %v: %v", job.ID, err)
		}

		evals = append(evals, childEvals...)
	}

	return evals, nil
}

// Flush clears the state of the PeriodicDispatcher
func (p *PeriodicDispatch) Flush() {
	p.l.Lock()
	defer p.l.Unlock()
	p.stopCh = make(chan struct{}, 1)
	p.updateCh = make(chan struct{}, 1)
	p.tracked = make(map[string]*structs.Job)
	p.heap = NewPeriodicHeap()
}

// TODO
type periodicHeap struct {
	index map[string]*periodicJob
	heap  periodicHeapImp
}

type periodicJob struct {
	job   *structs.Job
	next  time.Time
	index int
}

func NewPeriodicHeap() *periodicHeap {
	return &periodicHeap{
		index: make(map[string]*periodicJob),
		heap:  make(periodicHeapImp, 0),
	}
}

func (p *periodicHeap) Push(job *structs.Job, next time.Time) error {
	if _, ok := p.index[job.ID]; ok {
		return fmt.Errorf("job %v already exists", job.ID)
	}

	pJob := &periodicJob{job, next, 0}
	p.index[job.ID] = pJob
	heap.Push(&p.heap, pJob)
	return nil
}

func (p *periodicHeap) Pop() (*periodicJob, error) {
	if len(p.heap) == 0 {
		return nil, errors.New("heap is empty")
	}

	pJob := heap.Pop(&p.heap).(*periodicJob)
	delete(p.index, pJob.job.ID)
	return pJob, nil
}

func (p *periodicHeap) Peek() (periodicJob, error) {
	if len(p.heap) == 0 {
		return periodicJob{}, errors.New("heap is empty")
	}

	return *(p.heap[0]), nil
}

func (p *periodicHeap) Contains(job *structs.Job) bool {
	_, ok := p.index[job.ID]
	return ok
}

func (p *periodicHeap) Update(job *structs.Job, next time.Time) error {
	if job, ok := p.index[job.ID]; ok {
		p.heap.update(job, next)
		return nil
	}

	return fmt.Errorf("heap doesn't contain job %v", job.ID)
}

func (p *periodicHeap) Remove(job *structs.Job) error {
	if pJob, ok := p.index[job.ID]; ok {
		heap.Remove(&p.heap, pJob.index)
		delete(p.index, job.ID)
		return nil
	}

	return fmt.Errorf("heap doesn't contain job %v", job.ID)
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

// update modifies the priority and next time of an periodic job in the queue.
func (h *periodicHeapImp) update(job *periodicJob, next time.Time) {
	job.next = next
	heap.Fix(h, job.index)
}
