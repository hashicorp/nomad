package nomad

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

type MockJobEvalDispatcher struct {
	Jobs map[string]*structs.Job
	lock sync.Mutex
}

func NewMockJobEvalDispatcher() *MockJobEvalDispatcher {
	return &MockJobEvalDispatcher{Jobs: make(map[string]*structs.Job)}
}

func (m *MockJobEvalDispatcher) DispatchJob(job *structs.Job) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.Jobs[job.ID] = job
	return nil
}

// LaunchTimes returns the launch times of child jobs in sorted order.
func (m *MockJobEvalDispatcher) LaunchTimes(p *PeriodicDispatch, parentID string) ([]time.Time, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	var launches []time.Time
	for _, job := range m.Jobs {
		if job.ParentID != parentID {
			continue
		}

		t, err := p.LaunchTime(job.ID)
		if err != nil {
			return nil, err
		}
		launches = append(launches, t)
	}
	sort.Sort(times(launches))
	return launches, nil
}

type times []time.Time

func (t times) Len() int           { return len(t) }
func (t times) Swap(i, j int)      { t[i], t[j] = t[j], t[i] }
func (t times) Less(i, j int) bool { return t[i].Before(t[j]) }

// testPeriodicDispatcher returns an enabled PeriodicDispatcher which uses the
// MockJobEvalDispatcher.
func testPeriodicDispatcher() (*PeriodicDispatch, *MockJobEvalDispatcher) {
	logger := log.New(os.Stderr, "", log.LstdFlags)
	m := NewMockJobEvalDispatcher()
	d := NewPeriodicDispatch(logger, m)
	d.SetEnabled(true)
	d.Start()
	return d, m
}

// testPeriodicJob is a helper that creates a periodic job that launches at the
// passed times.
func testPeriodicJob(times ...time.Time) *structs.Job {
	job := mock.PeriodicJob()
	job.Periodic.SpecType = structs.PeriodicSpecTest

	l := make([]string, len(times))
	for i, t := range times {
		l[i] = strconv.Itoa(int(t.UnixNano()))
	}

	job.Periodic.Spec = strings.Join(l, ",")
	return job
}

func TestPeriodicDispatch_Add_NonPeriodic(t *testing.T) {
	t.Parallel()
	p, _ := testPeriodicDispatcher()
	job := mock.Job()
	if err := p.Add(job); err != nil {
		t.Fatalf("Add of non-periodic job failed: %v; expect no-op", err)
	}

	tracked := p.Tracked()
	if len(tracked) != 0 {
		t.Fatalf("Add of non-periodic job should be no-op: %v", tracked)
	}
}

func TestPeriodicDispatch_Add_UpdateJob(t *testing.T) {
	t.Parallel()
	p, _ := testPeriodicDispatcher()
	job := mock.PeriodicJob()
	if err := p.Add(job); err != nil {
		t.Fatalf("Add failed %v", err)
	}

	tracked := p.Tracked()
	if len(tracked) != 1 {
		t.Fatalf("Add didn't track the job: %v", tracked)
	}

	// Update the job and add it again.
	job.Periodic.Spec = "foo"
	if err := p.Add(job); err != nil {
		t.Fatalf("Add failed %v", err)
	}

	tracked = p.Tracked()
	if len(tracked) != 1 {
		t.Fatalf("Add didn't update: %v", tracked)
	}

	if !reflect.DeepEqual(job, tracked[0]) {
		t.Fatalf("Add didn't properly update: got %v; want %v", tracked[0], job)
	}
}

func TestPeriodicDispatch_Add_TriggersUpdate(t *testing.T) {
	t.Parallel()
	p, m := testPeriodicDispatcher()

	// Create a job that won't be evalauted for a while.
	job := testPeriodicJob(time.Now().Add(10 * time.Second))

	// Add it.
	if err := p.Add(job); err != nil {
		t.Fatalf("Add failed %v", err)
	}

	// Update it to be sooner and re-add.
	expected := time.Now().Add(1 * time.Second)
	job.Periodic.Spec = fmt.Sprintf("%d", expected.UnixNano())
	if err := p.Add(job); err != nil {
		t.Fatalf("Add failed %v", err)
	}

	// Check that nothing is created.
	if _, ok := m.Jobs[job.ID]; ok {
		t.Fatalf("periodic dispatcher created eval at the wrong time")
	}

	time.Sleep(2 * time.Second)

	// Check that job was launched correctly.
	times, err := m.LaunchTimes(p, job.ID)
	if err != nil {
		t.Fatalf("failed to get launch times for job %q", job.ID)
	}
	if len(times) != 1 {
		t.Fatalf("incorrect number of launch times for job %q", job.ID)
	}
	if times[0] != expected {
		t.Fatalf("periodic dispatcher created eval for time %v; want %v", times[0], expected)
	}
}

func TestPeriodicDispatch_Remove_Untracked(t *testing.T) {
	t.Parallel()
	p, _ := testPeriodicDispatcher()
	if err := p.Remove("foo"); err != nil {
		t.Fatalf("Remove failed %v; expected a no-op", err)
	}
}

func TestPeriodicDispatch_Remove_Tracked(t *testing.T) {
	t.Parallel()
	p, _ := testPeriodicDispatcher()

	job := mock.PeriodicJob()
	if err := p.Add(job); err != nil {
		t.Fatalf("Add failed %v", err)
	}

	tracked := p.Tracked()
	if len(tracked) != 1 {
		t.Fatalf("Add didn't track the job: %v", tracked)
	}

	if err := p.Remove(job.ID); err != nil {
		t.Fatalf("Remove failed %v", err)
	}

	tracked = p.Tracked()
	if len(tracked) != 0 {
		t.Fatalf("Remove didn't untrack the job: %v", tracked)
	}
}

func TestPeriodicDispatch_Remove_TriggersUpdate(t *testing.T) {
	t.Parallel()
	p, _ := testPeriodicDispatcher()

	// Create a job that will be evaluated soon.
	job := testPeriodicJob(time.Now().Add(1 * time.Second))

	// Add it.
	if err := p.Add(job); err != nil {
		t.Fatalf("Add failed %v", err)
	}

	// Remove the job.
	if err := p.Remove(job.ID); err != nil {
		t.Fatalf("Add failed %v", err)
	}

	time.Sleep(2 * time.Second)

	// Check that an eval wasn't created.
	d := p.dispatcher.(*MockJobEvalDispatcher)
	if _, ok := d.Jobs[job.ID]; ok {
		t.Fatalf("Remove didn't cancel creation of an eval")
	}
}

func TestPeriodicDispatch_ForceRun_Untracked(t *testing.T) {
	t.Parallel()
	p, _ := testPeriodicDispatcher()

	if err := p.ForceRun("foo"); err == nil {
		t.Fatal("ForceRun of untracked job should fail")
	}
}

func TestPeriodicDispatch_ForceRun_Tracked(t *testing.T) {
	t.Parallel()
	p, m := testPeriodicDispatcher()

	// Create a job that won't be evalauted for a while.
	job := testPeriodicJob(time.Now().Add(10 * time.Second))

	// Add it.
	if err := p.Add(job); err != nil {
		t.Fatalf("Add failed %v", err)
	}

	// ForceRun the job
	if err := p.ForceRun(job.ID); err != nil {
		t.Fatalf("ForceRun failed %v", err)
	}

	// Check that job was launched correctly.
	launches, err := m.LaunchTimes(p, job.ID)
	if err != nil {
		t.Fatalf("failed to get launch times for job %q: %v", job.ID, err)
	}
	l := len(launches)
	if l != 1 {
		t.Fatalf("restorePeriodicDispatcher() created an unexpected"+
			" number of evals; got %d; want 1", l)
	}
}

func TestPeriodicDispatch_Run_Multiple(t *testing.T) {
	t.Parallel()
	p, m := testPeriodicDispatcher()

	// Create a job that will be launched twice.
	launch1 := time.Now().Add(1 * time.Second)
	launch2 := time.Now().Add(2 * time.Second)
	job := testPeriodicJob(launch1, launch2)

	// Add it.
	if err := p.Add(job); err != nil {
		t.Fatalf("Add failed %v", err)
	}

	time.Sleep(3 * time.Second)

	// Check that job was launched correctly.
	times, err := m.LaunchTimes(p, job.ID)
	if err != nil {
		t.Fatalf("failed to get launch times for job %q", job.ID)
	}
	if len(times) != 2 {
		t.Fatalf("incorrect number of launch times for job %q", job.ID)
	}
	if times[0] != launch1 {
		t.Fatalf("periodic dispatcher created eval for time %v; want %v", times[0], launch1)
	}
	if times[1] != launch2 {
		t.Fatalf("periodic dispatcher created eval for time %v; want %v", times[1], launch2)
	}
}

func TestPeriodicDispatch_Run_SameTime(t *testing.T) {
	t.Parallel()
	p, m := testPeriodicDispatcher()

	// Create two job that will be launched at the same time.
	launch := time.Now().Add(1 * time.Second)
	job := testPeriodicJob(launch)
	job2 := testPeriodicJob(launch)

	// Add them.
	if err := p.Add(job); err != nil {
		t.Fatalf("Add failed %v", err)
	}
	if err := p.Add(job2); err != nil {
		t.Fatalf("Add failed %v", err)
	}

	time.Sleep(2 * time.Second)

	// Check that the jobs were launched correctly.
	for _, job := range []*structs.Job{job, job2} {
		times, err := m.LaunchTimes(p, job.ID)
		if err != nil {
			t.Fatalf("failed to get launch times for job %q", job.ID)
		}
		if len(times) != 1 {
			t.Fatalf("incorrect number of launch times for job %q; got %d; want 1", job.ID, len(times))
		}
		if times[0] != launch {
			t.Fatalf("periodic dispatcher created eval for time %v; want %v", times[0], launch)
		}
	}
}

// This test adds and removes a bunch of jobs, some launching at the same time,
// some after each other and some invalid times, and ensures the correct
// behavior.
func TestPeriodicDispatch_Complex(t *testing.T) {
	t.Parallel()
	p, m := testPeriodicDispatcher()

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
	expected := map[string][]time.Time{
		job1.ID: []time.Time{same},
		job2.ID: []time.Time{same},
		job3.ID: nil,
		job4.ID: []time.Time{launch1, launch3},
		job5.ID: []time.Time{launch2},
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
		if err := p.Add(job); err != nil {
			t.Fatalf("Add failed %v", err)
		}
	}

	for _, job := range toDelete {
		if err := p.Remove(job.ID); err != nil {
			t.Fatalf("Remove failed %v", err)
		}
	}

	time.Sleep(4 * time.Second)
	actual := make(map[string][]time.Time, len(expected))
	for _, job := range jobs {
		launches, err := m.LaunchTimes(p, job.ID)
		if err != nil {
			t.Fatalf("LaunchTimes(%v) failed %v", job.ID, err)
		}

		actual[job.ID] = launches
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("Unexpected launches; got %#v; want %#v", actual, expected)
	}
}

func TestPeriodicDispatch_NextLaunch(t *testing.T) {
	t.Parallel()
	p, _ := testPeriodicDispatcher()

	// Create two job that will be launched at the same time.
	invalid := time.Unix(0, 0)
	expected := time.Now().Add(1 * time.Second)
	job := testPeriodicJob(invalid)
	job2 := testPeriodicJob(expected)

	// Make sure the periodic dispatcher isn't running.
	close(p.stopCh)
	p.stopCh = make(chan struct{})

	// Run nextLaunch.
	timeout := make(chan struct{})
	var j *structs.Job
	var launch time.Time
	var err error
	go func() {
		j, launch, err = p.nextLaunch()
		close(timeout)
	}()

	// Add them.
	if err := p.Add(job); err != nil {
		t.Fatalf("Add failed %v", err)
	}

	// Delay adding a valid job.
	time.Sleep(200 * time.Millisecond)
	if err := p.Add(job2); err != nil {
		t.Fatalf("Add failed %v", err)
	}

	select {
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	case <-timeout:
		if err != nil {
			t.Fatalf("nextLaunch() failed: %v", err)
		}
		if j != job2 {
			t.Fatalf("Incorrect job returned; got %v; want %v", j, job2)
		}
		if launch != expected {
			t.Fatalf("Incorrect launch time; got %v; want %v", launch, expected)
		}
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
