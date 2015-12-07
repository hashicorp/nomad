package nomad

import (
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
)

// MockPeriodic can be used by other tests that want to mock the periodic
// dispatcher.
type MockPeriodic struct {
	Enabled bool
	Jobs    map[string]*structs.Job
}

func NewMockPeriodic() *MockPeriodic {
	return &MockPeriodic{Jobs: make(map[string]*structs.Job)}
}

func (m *MockPeriodic) SetEnabled(enabled bool) {
	m.Enabled = enabled
}

func (m *MockPeriodic) Add(job *structs.Job) error {
	if job == nil {
		return fmt.Errorf("Must pass non nil job")
	}

	m.Jobs[job.ID] = job
	return nil
}

func (m *MockPeriodic) Remove(jobID string) error {
	delete(m.Jobs, jobID)
	return nil
}

func (m *MockPeriodic) ForceRun(jobID string) error {
	return nil
}

func (m *MockPeriodic) LaunchTime(jobID string) (time.Time, error) {
	return time.Time{}, nil
}

func (m *MockPeriodic) Start() {}

func (m *MockPeriodic) Flush() {
	m.Jobs = make(map[string]*structs.Job)
}

func (m *MockPeriodic) Tracked() []structs.Job {
	tracked := make([]structs.Job, len(m.Jobs))
	i := 0
	for _, job := range m.Jobs {
		tracked[i] = *job
		i++
	}
	return tracked
}

func testPeriodicJob(times ...time.Time) *structs.Job {
	job := mock.PeriodicJob()
	job.Periodic.SpecType = structs.PeriodicSpecTest

	l := make([]string, len(times))
	for i, t := range times {
		l[i] = strconv.Itoa(int(t.Unix()))
	}

	job.Periodic.Spec = strings.Join(l, ",")
	return job
}

// createdEvals returns the set of evaluations created from the passed periodic
// job in sorted order, with the earliest job launch first.
func createdEvals(p *PeriodicDispatch, periodicJobID string) (PeriodicEvals, error) {
	state := p.srv.fsm.State()
	iter, err := state.ChildJobs(periodicJobID)
	if err != nil {
		return nil, fmt.Errorf("failed to look up children of job %v: %v", periodicJobID, err)
	}

	var evals PeriodicEvals
	for i := iter.Next(); i != nil; i = iter.Next() {
		job := i.(*structs.Job)
		childEvals, err := state.EvalsByJob(job.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to look up evals for job %v: %v", job.ID, err)
		}

		for _, eval := range childEvals {
			launch, err := p.LaunchTime(eval.JobID)
			if err != nil {
				return nil, fmt.Errorf("failed to get launch time for eval %v: %v", eval, err)
			}

			pEval := &PeriodicEval{
				Eval:      eval,
				JobLaunch: launch,
			}

			evals = append(evals, pEval)
		}
	}

	// Return the sorted evals.
	sort.Sort(evals)
	return evals, nil
}

// PeriodicEval stores the evaluation and launch time for an instantiated
// periodic job.
type PeriodicEval struct {
	Eval      *structs.Evaluation
	JobLaunch time.Time
}

// For sorting.
type PeriodicEvals []*PeriodicEval

func (p PeriodicEvals) Len() int           { return len(p) }
func (p PeriodicEvals) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p PeriodicEvals) Less(i, j int) bool { return p[i].JobLaunch.Before(p[j].JobLaunch) }

func TestPeriodicDispatch_Add_NonPeriodic(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	job := mock.Job()
	if err := s1.periodicDispatcher.Add(job); err != nil {
		t.Fatalf("Add of non-periodic job failed: %v; expect no-op", err)
	}

	tracked := s1.periodicDispatcher.Tracked()
	if len(tracked) != 0 {
		t.Fatalf("Add of non-periodic job should be no-op: %v", tracked)
	}
}

func TestPeriodicDispatch_Add_UpdateJob(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	job := mock.PeriodicJob()
	if err := s1.periodicDispatcher.Add(job); err != nil {
		t.Fatalf("Add failed %v", err)
	}

	tracked := s1.periodicDispatcher.Tracked()
	if len(tracked) != 1 {
		t.Fatalf("Add didn't track the job: %v", tracked)
	}

	// Update the job and add it again.
	job.Periodic.Spec = "foo"
	if err := s1.periodicDispatcher.Add(job); err != nil {
		t.Fatalf("Add failed %v", err)
	}

	tracked = s1.periodicDispatcher.Tracked()
	if len(tracked) != 1 {
		t.Fatalf("Add didn't update: %v", tracked)
	}

	if !reflect.DeepEqual(*job, tracked[0]) {
		t.Fatalf("Add didn't properly update: got %v; want %v", tracked[0], job)
	}
}

func TestPeriodicDispatch_Add_TriggersUpdate(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	// Create a job that won't be evalauted for a while.
	job := testPeriodicJob(time.Now().Add(10 * time.Second))

	// Add it.
	if err := s1.periodicDispatcher.Add(job); err != nil {
		t.Fatalf("Add failed %v", err)
	}

	// Update it to be sooner and re-add.
	expected := time.Now().Add(1 * time.Second)
	job.Periodic.Spec = fmt.Sprintf("%d", expected.Unix())
	if err := s1.periodicDispatcher.Add(job); err != nil {
		t.Fatalf("Add failed %v", err)
	}

	time.Sleep(2 * time.Second)

	// Check that an eval was created for the right time.
	evals, err := createdEvals(s1.periodicDispatcher, job.ID)
	if err != nil {
		t.Fatalf("createdEvals(%v) failed %v", job.ID, err)
	}

	if len(evals) != 1 {
		t.Fatalf("Unexpected number of evals created; got %#v; want 1", evals)
	}

	eval := evals[0].Eval
	expID := s1.periodicDispatcher.derivedJobID(job, expected)
	if eval.JobID != expID {
		t.Fatalf("periodic dispatcher created eval at the wrong time; got %v; want %v",
			eval.JobID, expID)
	}
}

func TestPeriodicDispatch_Remove_Untracked(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	if err := s1.periodicDispatcher.Remove("foo"); err != nil {
		t.Fatalf("Remove failed %v; expected a no-op", err)
	}
}

func TestPeriodicDispatch_Remove_Tracked(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	job := mock.PeriodicJob()
	if err := s1.periodicDispatcher.Add(job); err != nil {
		t.Fatalf("Add failed %v", err)
	}

	tracked := s1.periodicDispatcher.Tracked()
	if len(tracked) != 1 {
		t.Fatalf("Add didn't track the job: %v", tracked)
	}

	if err := s1.periodicDispatcher.Remove(job.ID); err != nil {
		t.Fatalf("Remove failed %v", err)
	}

	tracked = s1.periodicDispatcher.Tracked()
	if len(tracked) != 0 {
		t.Fatalf("Remove didn't untrack the job: %v", tracked)
	}
}

func TestPeriodicDispatch_Remove_TriggersUpdate(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	// Create a job that will be evaluated soon.
	job := testPeriodicJob(time.Now().Add(1 * time.Second))

	// Add it.
	if err := s1.periodicDispatcher.Add(job); err != nil {
		t.Fatalf("Add failed %v", err)
	}

	// Remove the job.
	if err := s1.periodicDispatcher.Remove(job.ID); err != nil {
		t.Fatalf("Add failed %v", err)
	}

	time.Sleep(2 * time.Second)

	// Check that an eval wasn't created.
	evals, err := createdEvals(s1.periodicDispatcher, job.ID)
	if err != nil {
		t.Fatalf("createdEvals(%v) failed %v", job.ID, err)
	}

	if len(evals) != 0 {
		t.Fatalf("Remove didn't cancel creation of an eval: %#v", evals)
	}
}

func TestPeriodicDispatch_ForceRun_Untracked(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	if err := s1.periodicDispatcher.ForceRun("foo"); err == nil {
		t.Fatal("ForceRun of untracked job should fail")
	}
}

func TestPeriodicDispatch_ForceRun_Tracked(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	// Create a job that won't be evalauted for a while.
	job := testPeriodicJob(time.Now().Add(10 * time.Second))

	// Add it.
	if err := s1.periodicDispatcher.Add(job); err != nil {
		t.Fatalf("Add failed %v", err)
	}

	// ForceRun the job
	if err := s1.periodicDispatcher.ForceRun(job.ID); err != nil {
		t.Fatalf("ForceRun failed %v", err)
	}

	// Check that an eval was created for the right time.
	evals, err := createdEvals(s1.periodicDispatcher, job.ID)
	if err != nil {
		t.Fatalf("createdEvals(%v) failed %v", job.ID, err)
	}

	if len(evals) != 1 {
		t.Fatalf("Unexpected number of evals created; got %#v; want 1", evals)
	}
}

func TestPeriodicDispatch_Run_Multiple(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	// Create a job that will be launched twice.
	launch1 := time.Now().Add(1 * time.Second)
	launch2 := time.Now().Add(2 * time.Second)
	job := testPeriodicJob(launch1, launch2)

	// Add it.
	if err := s1.periodicDispatcher.Add(job); err != nil {
		t.Fatalf("Add failed %v", err)
	}

	time.Sleep(3 * time.Second)

	// Check that the evals were created correctly
	evals, err := createdEvals(s1.periodicDispatcher, job.ID)
	if err != nil {
		t.Fatalf("createdEvals(%v) failed %v", job.ID, err)
	}

	d := s1.periodicDispatcher
	expected := []string{d.derivedJobID(job, launch1), d.derivedJobID(job, launch2)}
	for i, eval := range evals {
		if eval.Eval.JobID != expected[i] {
			t.Fatalf("eval created incorrectly; got %v; want %v", eval.Eval.JobID, expected[i])
		}
	}
}

func TestPeriodicDispatch_Run_SameTime(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	// Create two job that will be launched at the same time.
	launch := time.Now().Add(1 * time.Second)
	job := testPeriodicJob(launch)
	job2 := testPeriodicJob(launch)

	// Add them.
	if err := s1.periodicDispatcher.Add(job); err != nil {
		t.Fatalf("Add failed %v", err)
	}
	if err := s1.periodicDispatcher.Add(job2); err != nil {
		t.Fatalf("Add failed %v", err)
	}

	time.Sleep(2 * time.Second)

	// Check that the evals were created correctly
	for _, job := range []*structs.Job{job, job2} {
		evals, err := createdEvals(s1.periodicDispatcher, job.ID)
		if err != nil {
			t.Fatalf("createdEvals(%v) failed %v", job.ID, err)
		}

		if len(evals) != 1 {
			t.Fatalf("expected 1 eval; got %#v", evals)
		}

		d := s1.periodicDispatcher
		expected := d.derivedJobID(job, launch)
		if evals[0].Eval.JobID != expected {
			t.Fatalf("eval created incorrectly; got %v; want %v", evals[0].Eval.JobID, expected)
		}
	}
}

// This test adds and removes a bunch of jobs, some launching at the same time,
// some after each other and some invalid times, and ensures the correct
// behavior.
func TestPeriodicDispatch_Complex(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	// Create some jobs launching at different times.
	now := time.Now()
	same := now.Add(1 * time.Second)
	launch1 := same.Add(1 * time.Second)
	launch2 := same.Add(1500 * time.Millisecond)
	launch3 := same.Add(2 * time.Second)
	invalid := now.Add(-200 * time.Second)

	// Create two jobs launching at the same time.
	job1 := testPeriodicJob(same)
	job2 := testPeriodicJob(same)

	// Create a job that will never launch.
	job3 := testPeriodicJob(invalid)

	// Create a job that launches twice.
	job4 := testPeriodicJob(launch1, launch3)

	// Create a job that launches once.
	job5 := testPeriodicJob(launch2)

	// Create 3 jobs we will delete.
	job6 := testPeriodicJob(same)
	job7 := testPeriodicJob(launch1, launch3)
	job8 := testPeriodicJob(launch2)

	// Create a map of expected eval job ids.
	d := s1.periodicDispatcher
	expected := map[string][]string{
		job1.ID: []string{d.derivedJobID(job1, same)},
		job2.ID: []string{d.derivedJobID(job2, same)},
		job3.ID: nil,
		job4.ID: []string{
			d.derivedJobID(job4, launch1),
			d.derivedJobID(job4, launch3),
		},
		job5.ID: []string{d.derivedJobID(job5, launch2)},
		job6.ID: nil,
		job7.ID: nil,
		job8.ID: nil,
	}

	// Shuffle the jobs so they can be added randomly
	jobs := []*structs.Job{job1, job2, job3, job4, job5, job6, job7, job8}
	toDelete := []*structs.Job{job6, job7, job8}
	shuffle(jobs)
	shuffle(toDelete)

	for _, job := range jobs {
		if err := s1.periodicDispatcher.Add(job); err != nil {
			t.Fatalf("Add failed %v", err)
		}
	}

	for _, job := range toDelete {
		if err := s1.periodicDispatcher.Remove(job.ID); err != nil {
			t.Fatalf("Remove failed %v", err)
		}
	}

	time.Sleep(4 * time.Second)
	actual := make(map[string][]string, len(expected))
	for _, job := range jobs {
		evals, err := createdEvals(s1.periodicDispatcher, job.ID)
		if err != nil {
			t.Fatalf("createdEvals(%v) failed %v", job.ID, err)
		}

		var jobs []string
		for _, eval := range evals {
			jobs = append(jobs, eval.Eval.JobID)
		}
		actual[job.ID] = jobs
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("Unexpected evals; got %#v; want %#v", actual, expected)
	}
}

func shuffle(jobs []*structs.Job) {
	rand.Seed(time.Now().Unix())
	for i := range jobs {
		j := rand.Intn(len(jobs))
		jobs[i], jobs[j] = jobs[j], jobs[i]
	}
}

func TestPeriodicHeap_Order(t *testing.T) {
	t.Parallel()
	h := NewPeriodicHeap()
	j1 := mock.PeriodicJob()
	j2 := mock.PeriodicJob()
	j3 := mock.PeriodicJob()

	lookup := map[*structs.Job]string{
		j1: "j1",
		j2: "j2",
		j3: "j3",
	}

	h.Push(j1, time.Time{})
	h.Push(j2, time.Unix(10, 0))
	h.Push(j3, time.Unix(11, 0))

	exp := []string{"j2", "j3", "j1"}
	var act []string
	for i := 0; i < 3; i++ {
		pJob, err := h.Pop()
		if err != nil {
			t.Fatal(err)
		}

		act = append(act, lookup[pJob.job])
	}

	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("Wrong ordering; got %v; want %v", act, exp)
	}
}
