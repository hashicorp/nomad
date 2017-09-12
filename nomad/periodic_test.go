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
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
)

type MockJobEvalDispatcher struct {
	Jobs map[structs.NamespacedID]*structs.Job
	lock sync.Mutex
}

func NewMockJobEvalDispatcher() *MockJobEvalDispatcher {
	return &MockJobEvalDispatcher{Jobs: make(map[structs.NamespacedID]*structs.Job)}
}

func (m *MockJobEvalDispatcher) DispatchJob(job *structs.Job) (*structs.Evaluation, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	tuple := structs.NamespacedID{
		ID:        job.ID,
		Namespace: job.Namespace,
	}
	m.Jobs[tuple] = job
	return nil, nil
}

func (m *MockJobEvalDispatcher) RunningChildren(parent *structs.Job) (bool, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	for _, job := range m.Jobs {
		if job.ParentID == parent.ID && job.Namespace == parent.Namespace {
			return true, nil
		}
	}
	return false, nil
}

// LaunchTimes returns the launch times of child jobs in sorted order.
func (m *MockJobEvalDispatcher) LaunchTimes(p *PeriodicDispatch, namespace, parentID string) ([]time.Time, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	var launches []time.Time
	for _, job := range m.Jobs {
		if job.ParentID != parentID || job.Namespace != namespace {
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
	return d, m
}

// testPeriodicJob is a helper that creates a periodic job that launches at the
// passed times.
func testPeriodicJob(times ...time.Time) *structs.Job {
	job := mock.PeriodicJob()
	job.Periodic.SpecType = structs.PeriodicSpecTest

	l := make([]string, len(times))
	for i, t := range times {
		l[i] = strconv.Itoa(int(t.Round(1 * time.Second).Unix()))
	}

	job.Periodic.Spec = strings.Join(l, ",")
	return job
}

// TestPeriodicDispatch_SetEnabled test that setting enabled twice is a no-op.
// This tests the reported issue: https://github.com/hashicorp/nomad/issues/2829
func TestPeriodicDispatch_SetEnabled(t *testing.T) {
	t.Parallel()
	p, _ := testPeriodicDispatcher()

	// SetEnabled has been called once but do it again.
	p.SetEnabled(true)

	// Now disable and make sure everything is fine.
	p.SetEnabled(false)

	// Enable and track something
	p.SetEnabled(true)
	job := mock.PeriodicJob()
	if added, err := p.Add(job); err != nil || !added {
		t.Fatalf("Add failed %v %v", added, err)
	}

	tracked := p.Tracked()
	if len(tracked) != 1 {
		t.Fatalf("Add didn't track the job: %v", tracked)
	}
}

func TestPeriodicDispatch_Add_NonPeriodic(t *testing.T) {
	t.Parallel()
	p, _ := testPeriodicDispatcher()
	job := mock.Job()
	if added, err := p.Add(job); err != nil {
		t.Fatalf("Add of non-periodic job failed: %v; expect no-op", err)
	} else if added {
		t.Fatalf("Add of non-periodic job happened, expect no-op")
	}

	tracked := p.Tracked()
	if len(tracked) != 0 {
		t.Fatalf("Add of non-periodic job should be no-op: %v", tracked)
	}
}

func TestPeriodicDispatch_Add_Periodic_Parameterized(t *testing.T) {
	t.Parallel()
	p, _ := testPeriodicDispatcher()
	job := mock.PeriodicJob()
	job.ParameterizedJob = &structs.ParameterizedJobConfig{}
	if added, err := p.Add(job); err != nil || added {
		t.Fatalf("Add of periodic parameterized job failed: %v %v", added, err)
	}

	tracked := p.Tracked()
	if len(tracked) != 0 {
		t.Fatalf("Add of periodic parameterized job should be no-op: %v", tracked)
	}
}

func TestPeriodicDispatch_Add_Periodic_Stopped(t *testing.T) {
	t.Parallel()
	p, _ := testPeriodicDispatcher()
	job := mock.PeriodicJob()
	job.Stop = true
	if added, err := p.Add(job); err != nil || added {
		t.Fatalf("Add of stopped periodic job failed: %v %v", added, err)
	}

	tracked := p.Tracked()
	if len(tracked) != 0 {
		t.Fatalf("Add of periodic parameterized job should be no-op: %v", tracked)
	}
}

func TestPeriodicDispatch_Add_UpdateJob(t *testing.T) {
	t.Parallel()
	p, _ := testPeriodicDispatcher()
	job := mock.PeriodicJob()
	if added, err := p.Add(job); err != nil || !added {
		t.Fatalf("Add failed %v %v", added, err)
	}

	tracked := p.Tracked()
	if len(tracked) != 1 {
		t.Fatalf("Add didn't track the job: %v", tracked)
	}

	// Update the job and add it again.
	job.Periodic.Spec = "foo"
	if added, err := p.Add(job); err != nil || !added {
		t.Fatalf("Add failed %v", added, err)
	}

	tracked = p.Tracked()
	if len(tracked) != 1 {
		t.Fatalf("Add didn't update: %v", tracked)
	}

	if !reflect.DeepEqual(job, tracked[0]) {
		t.Fatalf("Add didn't properly update: got %v; want %v", tracked[0], job)
	}
}

func TestPeriodicDispatch_Add_Remove_Namespaced(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	p, _ := testPeriodicDispatcher()
	job := mock.PeriodicJob()
	job2 := mock.PeriodicJob()
	job2.Namespace = "test"

	added, err := p.Add(job)
	assert.Nil(err)
	assert.True(added)

	added, err = p.Add(job2)
	assert.Nil(err)
	assert.True(added)

	assert.Len(p.Tracked(), 2)

	assert.Nil(p.Remove(job2.Namespace, job2.ID))
	assert.Len(p.Tracked(), 1)
	assert.Equal(p.Tracked()[0], job)
}

func TestPeriodicDispatch_Add_RemoveJob(t *testing.T) {
	t.Parallel()
	p, _ := testPeriodicDispatcher()
	job := mock.PeriodicJob()
	if added, err := p.Add(job); err != nil || !added {
		t.Fatalf("Add failed %v", added, err)
	}

	tracked := p.Tracked()
	if len(tracked) != 1 {
		t.Fatalf("Add didn't track the job: %v", tracked)
	}

	// Update the job to be non-periodic and add it again.
	job.Periodic = nil
	if added, err := p.Add(job); err != nil || added {
		t.Fatalf("Add failed %v", added, err)
	}

	tracked = p.Tracked()
	if len(tracked) != 0 {
		t.Fatalf("Add didn't remove: %v", tracked)
	}
}

func TestPeriodicDispatch_Add_TriggersUpdate(t *testing.T) {
	t.Parallel()
	p, m := testPeriodicDispatcher()

	// Create a job that won't be evalauted for a while.
	job := testPeriodicJob(time.Now().Add(10 * time.Second))

	// Add it.
	if added, err := p.Add(job); err != nil || !added {
		t.Fatalf("Add failed %v", added, err)
	}

	// Update it to be sooner and re-add.
	expected := time.Now().Round(1 * time.Second).Add(1 * time.Second)
	job.Periodic.Spec = fmt.Sprintf("%d", expected.Unix())
	if added, err := p.Add(job); err != nil || !added {
		t.Fatalf("Add failed %v", added, err)
	}

	// Check that nothing is created.
	tuple := structs.NamespacedID{
		ID:        job.ID,
		Namespace: job.Namespace,
	}
	if _, ok := m.Jobs[tuple]; ok {
		t.Fatalf("periodic dispatcher created eval at the wrong time")
	}

	time.Sleep(2 * time.Second)

	// Check that job was launched correctly.
	times, err := m.LaunchTimes(p, job.Namespace, job.ID)
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
	if err := p.Remove("ns", "foo"); err != nil {
		t.Fatalf("Remove failed %v; expected a no-op", err)
	}
}

func TestPeriodicDispatch_Remove_Tracked(t *testing.T) {
	t.Parallel()
	p, _ := testPeriodicDispatcher()

	job := mock.PeriodicJob()
	if added, err := p.Add(job); err != nil || !added {
		t.Fatalf("Add failed %v", added, err)
	}

	tracked := p.Tracked()
	if len(tracked) != 1 {
		t.Fatalf("Add didn't track the job: %v", tracked)
	}

	if err := p.Remove(job.Namespace, job.ID); err != nil {
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
	if added, err := p.Add(job); err != nil || !added {
		t.Fatalf("Add failed %v", added, err)
	}

	// Remove the job.
	if err := p.Remove(job.Namespace, job.ID); err != nil {
		t.Fatalf("Add failed %v", err)
	}

	time.Sleep(2 * time.Second)

	// Check that an eval wasn't created.
	d := p.dispatcher.(*MockJobEvalDispatcher)
	tuple := structs.NamespacedID{
		ID:        job.ID,
		Namespace: job.Namespace,
	}
	if _, ok := d.Jobs[tuple]; ok {
		t.Fatalf("Remove didn't cancel creation of an eval")
	}
}

func TestPeriodicDispatch_ForceRun_Untracked(t *testing.T) {
	t.Parallel()
	p, _ := testPeriodicDispatcher()

	if _, err := p.ForceRun("ns", "foo"); err == nil {
		t.Fatal("ForceRun of untracked job should fail")
	}
}

func TestPeriodicDispatch_ForceRun_Tracked(t *testing.T) {
	t.Parallel()
	p, m := testPeriodicDispatcher()

	// Create a job that won't be evalauted for a while.
	job := testPeriodicJob(time.Now().Add(10 * time.Second))

	// Add it.
	if added, err := p.Add(job); err != nil || !added {
		t.Fatalf("Add failed %v", added, err)
	}

	// ForceRun the job
	if _, err := p.ForceRun(job.Namespace, job.ID); err != nil {
		t.Fatalf("ForceRun failed %v", err)
	}

	// Check that job was launched correctly.
	launches, err := m.LaunchTimes(p, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("failed to get launch times for job %q: %v", job.ID, err)
	}
	l := len(launches)
	if l != 1 {
		t.Fatalf("restorePeriodicDispatcher() created an unexpected"+
			" number of evals; got %d; want 1", l)
	}
}

func TestPeriodicDispatch_Run_DisallowOverlaps(t *testing.T) {
	t.Parallel()
	p, m := testPeriodicDispatcher()

	// Create a job that will trigger two launches but disallows overlapping.
	launch1 := time.Now().Round(1 * time.Second).Add(1 * time.Second)
	launch2 := time.Now().Round(1 * time.Second).Add(2 * time.Second)
	job := testPeriodicJob(launch1, launch2)
	job.Periodic.ProhibitOverlap = true

	// Add it.
	if added, err := p.Add(job); err != nil || !added {
		t.Fatalf("Add failed %v", added, err)
	}

	time.Sleep(3 * time.Second)

	// Check that only one job was launched.
	times, err := m.LaunchTimes(p, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("failed to get launch times for job %q", job.ID)
	}
	if len(times) != 1 {
		t.Fatalf("incorrect number of launch times for job %q; got %v", job.ID, times)
	}
	if times[0] != launch1 {
		t.Fatalf("periodic dispatcher created eval for time %v; want %v", times[0], launch1)
	}
}

func TestPeriodicDispatch_Run_Multiple(t *testing.T) {
	t.Parallel()
	p, m := testPeriodicDispatcher()

	// Create a job that will be launched twice.
	launch1 := time.Now().Round(1 * time.Second).Add(1 * time.Second)
	launch2 := time.Now().Round(1 * time.Second).Add(2 * time.Second)
	job := testPeriodicJob(launch1, launch2)

	// Add it.
	if added, err := p.Add(job); err != nil || !added {
		t.Fatalf("Add failed %v", added, err)
	}

	time.Sleep(3 * time.Second)

	// Check that job was launched correctly.
	times, err := m.LaunchTimes(p, job.Namespace, job.ID)
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
	launch := time.Now().Round(1 * time.Second).Add(1 * time.Second)
	job := testPeriodicJob(launch)
	job2 := testPeriodicJob(launch)

	// Add them.
	if added, err := p.Add(job); err != nil || !added {
		t.Fatalf("Add failed %v", added, err)
	}
	if added, err := p.Add(job2); err != nil || !added {
		t.Fatalf("Add failed %v", added, err)
	}

	if l := len(p.Tracked()); l != 2 {
		t.Fatalf("got %d tracked; want 2", l)
	}

	time.Sleep(2 * time.Second)

	// Check that the jobs were launched correctly.
	for _, job := range []*structs.Job{job, job2} {
		times, err := m.LaunchTimes(p, job.Namespace, job.ID)
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

func TestPeriodicDispatch_Run_SameID_Different_Namespace(t *testing.T) {
	t.Parallel()
	p, m := testPeriodicDispatcher()

	// Create two job that will be launched at the same time.
	launch := time.Now().Round(1 * time.Second).Add(1 * time.Second)
	job := testPeriodicJob(launch)
	job2 := testPeriodicJob(launch)
	job2.ID = job.ID
	job2.Namespace = "test"

	// Add them.
	if added, err := p.Add(job); err != nil || !added {
		t.Fatalf("Add failed %v", added, err)
	}
	if added, err := p.Add(job2); err != nil || !added {
		t.Fatalf("Add failed %v", added, err)
	}

	if l := len(p.Tracked()); l != 2 {
		t.Fatalf("got %d tracked; want 2", l)
	}

	time.Sleep(2 * time.Second)

	// Check that the jobs were launched correctly.
	for _, job := range []*structs.Job{job, job2} {
		times, err := m.LaunchTimes(p, job.Namespace, job.ID)
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
	now := time.Now().Round(1 * time.Second)
	same := now.Add(1 * time.Second)
	launch1 := same.Add(1 * time.Second)
	launch2 := same.Add(2 * time.Second)
	launch3 := same.Add(3 * time.Second)
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
		if added, err := p.Add(job); err != nil || !added {
			t.Fatalf("Add failed %v", added, err)
		}
	}

	for _, job := range toDelete {
		if err := p.Remove(job.Namespace, job.ID); err != nil {
			t.Fatalf("Remove failed %v", err)
		}
	}

	time.Sleep(5 * time.Second)
	actual := make(map[string][]time.Time, len(expected))
	for _, job := range jobs {
		launches, err := m.LaunchTimes(p, job.Namespace, job.ID)
		if err != nil {
			t.Fatalf("LaunchTimes(%v, %v) failed %v", job.Namespace, job.ID, err)
		}

		actual[job.ID] = launches
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("Unexpected launches; got %#v; want %#v", actual, expected)
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
		pJob := h.Pop()
		act = append(act, lookup[pJob.job])
	}

	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("Wrong ordering; got %v; want %v", act, exp)
	}
}

// deriveChildJob takes a parent periodic job and returns a job with fields set
// such that it appears spawned from the parent.
func deriveChildJob(parent *structs.Job) *structs.Job {
	childjob := mock.Job()
	childjob.ParentID = parent.ID
	childjob.ID = fmt.Sprintf("%s%s%v", parent.ID, structs.PeriodicLaunchSuffix, time.Now().Unix())
	return childjob
}

func TestPeriodicDispatch_RunningChildren_NoEvals(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	// Insert job.
	state := s1.fsm.State()
	job := mock.PeriodicJob()
	if err := state.UpsertJob(1000, job); err != nil {
		t.Fatalf("UpsertJob failed: %v", err)
	}

	running, err := s1.RunningChildren(job)
	if err != nil {
		t.Fatalf("RunningChildren failed: %v", err)
	}

	if running {
		t.Fatalf("RunningChildren should return false")
	}
}

func TestPeriodicDispatch_RunningChildren_ActiveEvals(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	// Insert periodic job and child.
	state := s1.fsm.State()
	job := mock.PeriodicJob()
	if err := state.UpsertJob(1000, job); err != nil {
		t.Fatalf("UpsertJob failed: %v", err)
	}

	childjob := deriveChildJob(job)
	if err := state.UpsertJob(1001, childjob); err != nil {
		t.Fatalf("UpsertJob failed: %v", err)
	}

	// Insert non-terminal eval
	eval := mock.Eval()
	eval.JobID = childjob.ID
	eval.Status = structs.EvalStatusPending
	if err := state.UpsertEvals(1002, []*structs.Evaluation{eval}); err != nil {
		t.Fatalf("UpsertEvals failed: %v", err)
	}

	running, err := s1.RunningChildren(job)
	if err != nil {
		t.Fatalf("RunningChildren failed: %v", err)
	}

	if !running {
		t.Fatalf("RunningChildren should return true")
	}
}

func TestPeriodicDispatch_RunningChildren_ActiveAllocs(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	// Insert periodic job and child.
	state := s1.fsm.State()
	job := mock.PeriodicJob()
	if err := state.UpsertJob(1000, job); err != nil {
		t.Fatalf("UpsertJob failed: %v", err)
	}

	childjob := deriveChildJob(job)
	if err := state.UpsertJob(1001, childjob); err != nil {
		t.Fatalf("UpsertJob failed: %v", err)
	}

	// Insert terminal eval
	eval := mock.Eval()
	eval.JobID = childjob.ID
	eval.Status = structs.EvalStatusPending
	if err := state.UpsertEvals(1002, []*structs.Evaluation{eval}); err != nil {
		t.Fatalf("UpsertEvals failed: %v", err)
	}

	// Insert active alloc
	alloc := mock.Alloc()
	alloc.JobID = childjob.ID
	alloc.EvalID = eval.ID
	alloc.DesiredStatus = structs.AllocDesiredStatusRun
	if err := state.UpsertAllocs(1003, []*structs.Allocation{alloc}); err != nil {
		t.Fatalf("UpsertAllocs failed: %v", err)
	}

	running, err := s1.RunningChildren(job)
	if err != nil {
		t.Fatalf("RunningChildren failed: %v", err)
	}

	if !running {
		t.Fatalf("RunningChildren should return true")
	}
}
