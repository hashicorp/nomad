package scheduler

import (
	"fmt"
	"log"
	"os"
	"reflect"
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// noErr is used to assert there are no errors
func noErr(t *testing.T, err error) {
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestMaterializeTaskGroups(t *testing.T) {
	job := mock.Job()
	index := materializeTaskGroups(job)
	if len(index) != 10 {
		t.Fatalf("Bad: %#v", index)
	}

	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("my-job.web[%d]", i)
		tg, ok := index[name]
		if !ok {
			t.Fatalf("bad")
		}
		if tg != job.TaskGroups[0] {
			t.Fatalf("bad")
		}
	}
}

func TestDiffAllocs(t *testing.T) {
	job := mock.Job()
	required := materializeTaskGroups(job)

	// The "old" job has a previous modify index
	oldJob := new(structs.Job)
	*oldJob = *job
	oldJob.JobModifyIndex -= 1

	drainNode := mock.Node()
	drainNode.Drain = true

	deadNode := mock.Node()
	deadNode.Status = structs.NodeStatusDown

	tainted := map[string]*structs.Node{
		"dead":      deadNode,
		"drainNode": drainNode,
	}

	allocs := []*structs.Allocation{
		// Update the 1st
		&structs.Allocation{
			ID:     structs.GenerateUUID(),
			NodeID: "zip",
			Name:   "my-job.web[0]",
			Job:    oldJob,
		},

		// Ignore the 2rd
		&structs.Allocation{
			ID:     structs.GenerateUUID(),
			NodeID: "zip",
			Name:   "my-job.web[1]",
			Job:    job,
		},

		// Evict 11th
		&structs.Allocation{
			ID:     structs.GenerateUUID(),
			NodeID: "zip",
			Name:   "my-job.web[10]",
			Job:    oldJob,
		},

		// Migrate the 3rd
		&structs.Allocation{
			ID:     structs.GenerateUUID(),
			NodeID: "drainNode",
			Name:   "my-job.web[2]",
			Job:    oldJob,
		},
		// Mark the 4th lost
		&structs.Allocation{
			ID:     structs.GenerateUUID(),
			NodeID: "dead",
			Name:   "my-job.web[3]",
			Job:    oldJob,
		},
	}

	// Have three terminal allocs
	terminalAllocs := map[string]*structs.Allocation{
		"my-job.web[4]": &structs.Allocation{
			ID:     structs.GenerateUUID(),
			NodeID: "zip",
			Name:   "my-job.web[4]",
			Job:    job,
		},
		"my-job.web[5]": &structs.Allocation{
			ID:     structs.GenerateUUID(),
			NodeID: "zip",
			Name:   "my-job.web[5]",
			Job:    job,
		},
		"my-job.web[6]": &structs.Allocation{
			ID:     structs.GenerateUUID(),
			NodeID: "zip",
			Name:   "my-job.web[6]",
			Job:    job,
		},
	}

	diff := diffAllocs(job, tainted, required, allocs, terminalAllocs)
	place := diff.place
	update := diff.update
	migrate := diff.migrate
	stop := diff.stop
	ignore := diff.ignore
	lost := diff.lost

	// We should update the first alloc
	if len(update) != 1 || update[0].Alloc != allocs[0] {
		t.Fatalf("bad: %#v", update)
	}

	// We should ignore the second alloc
	if len(ignore) != 1 || ignore[0].Alloc != allocs[1] {
		t.Fatalf("bad: %#v", ignore)
	}

	// We should stop the 3rd alloc
	if len(stop) != 1 || stop[0].Alloc != allocs[2] {
		t.Fatalf("bad: %#v", stop)
	}

	// We should migrate the 4rd alloc
	if len(migrate) != 1 || migrate[0].Alloc != allocs[3] {
		t.Fatalf("bad: %#v", migrate)
	}

	// We should mark the 5th alloc as lost
	if len(lost) != 1 || lost[0].Alloc != allocs[4] {
		t.Fatalf("bad: %#v", migrate)
	}

	// We should place 6
	if len(place) != 6 {
		t.Fatalf("bad: %#v", place)
	}

	// Ensure that the allocations which are replacements of terminal allocs are
	// annotated
	for name, alloc := range terminalAllocs {
		for _, allocTuple := range diff.place {
			if name == allocTuple.Name {
				if !reflect.DeepEqual(alloc, allocTuple.Alloc) {
					t.Fatalf("expected: %#v, actual: %#v", alloc, allocTuple.Alloc)
				}
			}
		}
	}
}

func TestDiffSystemAllocs(t *testing.T) {
	job := mock.SystemJob()

	drainNode := mock.Node()
	drainNode.Drain = true

	deadNode := mock.Node()
	deadNode.Status = structs.NodeStatusDown

	tainted := map[string]*structs.Node{
		deadNode.ID:  deadNode,
		drainNode.ID: drainNode,
	}

	// Create three alive nodes.
	nodes := []*structs.Node{{ID: "foo"}, {ID: "bar"}, {ID: "baz"},
		{ID: "pipe"}, {ID: drainNode.ID}, {ID: deadNode.ID}}

	// The "old" job has a previous modify index
	oldJob := new(structs.Job)
	*oldJob = *job
	oldJob.JobModifyIndex -= 1

	allocs := []*structs.Allocation{
		// Update allocation on baz
		&structs.Allocation{
			ID:     structs.GenerateUUID(),
			NodeID: "baz",
			Name:   "my-job.web[0]",
			Job:    oldJob,
		},

		// Ignore allocation on bar
		&structs.Allocation{
			ID:     structs.GenerateUUID(),
			NodeID: "bar",
			Name:   "my-job.web[0]",
			Job:    job,
		},

		// Stop allocation on draining node.
		&structs.Allocation{
			ID:     structs.GenerateUUID(),
			NodeID: drainNode.ID,
			Name:   "my-job.web[0]",
			Job:    oldJob,
		},
		// Mark as lost on a dead node
		&structs.Allocation{
			ID:     structs.GenerateUUID(),
			NodeID: deadNode.ID,
			Name:   "my-job.web[0]",
			Job:    oldJob,
		},
	}

	// Have three terminal allocs
	terminalAllocs := map[string]*structs.Allocation{
		"my-job.web[0]": &structs.Allocation{
			ID:     structs.GenerateUUID(),
			NodeID: "pipe",
			Name:   "my-job.web[0]",
			Job:    job,
		},
	}

	diff := diffSystemAllocs(job, nodes, tainted, allocs, terminalAllocs)
	place := diff.place
	update := diff.update
	migrate := diff.migrate
	stop := diff.stop
	ignore := diff.ignore
	lost := diff.lost

	// We should update the first alloc
	if len(update) != 1 || update[0].Alloc != allocs[0] {
		t.Fatalf("bad: %#v", update)
	}

	// We should ignore the second alloc
	if len(ignore) != 1 || ignore[0].Alloc != allocs[1] {
		t.Fatalf("bad: %#v", ignore)
	}

	// We should stop the third alloc
	if len(stop) != 1 || stop[0].Alloc != allocs[2] {
		t.Fatalf("bad: %#v", stop)
	}

	// There should be no migrates.
	if len(migrate) != 0 {
		t.Fatalf("bad: %#v", migrate)
	}

	// We should mark the 5th alloc as lost
	if len(lost) != 1 || lost[0].Alloc != allocs[3] {
		t.Fatalf("bad: %#v", migrate)
	}

	// We should place 1
	if l := len(place); l != 2 {
		t.Fatalf("bad: %#v", l)
	}

	// Ensure that the allocations which are replacements of terminal allocs are
	// annotated
	for _, alloc := range terminalAllocs {
		for _, allocTuple := range diff.place {
			if alloc.NodeID == allocTuple.Alloc.NodeID {
				if !reflect.DeepEqual(alloc, allocTuple.Alloc) {
					t.Fatalf("expected: %#v, actual: %#v", alloc, allocTuple.Alloc)
				}
			}
		}
	}
}

func TestReadyNodesInDCs(t *testing.T) {
	state, err := state.NewStateStore(os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	node1 := mock.Node()
	node2 := mock.Node()
	node2.Datacenter = "dc2"
	node3 := mock.Node()
	node3.Datacenter = "dc2"
	node3.Status = structs.NodeStatusDown
	node4 := mock.Node()
	node4.Drain = true

	noErr(t, state.UpsertNode(1000, node1))
	noErr(t, state.UpsertNode(1001, node2))
	noErr(t, state.UpsertNode(1002, node3))
	noErr(t, state.UpsertNode(1003, node4))

	nodes, dc, err := readyNodesInDCs(state, []string{"dc1", "dc2"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(nodes) != 2 {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[0].ID == node3.ID || nodes[1].ID == node3.ID {
		t.Fatalf("Bad: %#v", nodes)
	}
	if count, ok := dc["dc1"]; !ok || count != 1 {
		t.Fatalf("Bad: dc1 count %v", count)
	}
	if count, ok := dc["dc2"]; !ok || count != 1 {
		t.Fatalf("Bad: dc2 count %v", count)
	}
}

func TestRetryMax(t *testing.T) {
	calls := 0
	bad := func() (bool, error) {
		calls += 1
		return false, nil
	}
	err := retryMax(3, bad, nil)
	if err == nil {
		t.Fatalf("should fail")
	}
	if calls != 3 {
		t.Fatalf("mis match")
	}

	calls = 0
	first := true
	reset := func() bool {
		if calls == 3 && first {
			first = false
			return true
		}
		return false
	}
	err = retryMax(3, bad, reset)
	if err == nil {
		t.Fatalf("should fail")
	}
	if calls != 6 {
		t.Fatalf("mis match")
	}

	calls = 0
	good := func() (bool, error) {
		calls += 1
		return true, nil
	}
	err = retryMax(3, good, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if calls != 1 {
		t.Fatalf("mis match")
	}
}

func TestTaintedNodes(t *testing.T) {
	state, err := state.NewStateStore(os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	node1 := mock.Node()
	node2 := mock.Node()
	node2.Datacenter = "dc2"
	node3 := mock.Node()
	node3.Datacenter = "dc2"
	node3.Status = structs.NodeStatusDown
	node4 := mock.Node()
	node4.Drain = true
	noErr(t, state.UpsertNode(1000, node1))
	noErr(t, state.UpsertNode(1001, node2))
	noErr(t, state.UpsertNode(1002, node3))
	noErr(t, state.UpsertNode(1003, node4))

	allocs := []*structs.Allocation{
		&structs.Allocation{NodeID: node1.ID},
		&structs.Allocation{NodeID: node2.ID},
		&structs.Allocation{NodeID: node3.ID},
		&structs.Allocation{NodeID: node4.ID},
		&structs.Allocation{NodeID: "12345678-abcd-efab-cdef-123456789abc"},
	}
	tainted, err := taintedNodes(state, allocs)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(tainted) != 3 {
		t.Fatalf("bad: %v", tainted)
	}

	if _, ok := tainted[node1.ID]; ok {
		t.Fatalf("Bad: %v", tainted)
	}
	if _, ok := tainted[node2.ID]; ok {
		t.Fatalf("Bad: %v", tainted)
	}

	if node, ok := tainted[node3.ID]; !ok || node == nil {
		t.Fatalf("Bad: %v", tainted)
	}

	if node, ok := tainted[node4.ID]; !ok || node == nil {
		t.Fatalf("Bad: %v", tainted)
	}

	if node, ok := tainted["12345678-abcd-efab-cdef-123456789abc"]; !ok || node != nil {
		t.Fatalf("Bad: %v", tainted)
	}
}

func TestShuffleNodes(t *testing.T) {
	// Use a large number of nodes to make the probability of shuffling to the
	// original order very low.
	nodes := []*structs.Node{
		mock.Node(),
		mock.Node(),
		mock.Node(),
		mock.Node(),
		mock.Node(),
		mock.Node(),
		mock.Node(),
		mock.Node(),
		mock.Node(),
		mock.Node(),
	}
	orig := make([]*structs.Node, len(nodes))
	copy(orig, nodes)
	shuffleNodes(nodes)
	if reflect.DeepEqual(nodes, orig) {
		t.Fatalf("should not match")
	}
}

func TestTasksUpdated(t *testing.T) {
	j1 := mock.Job()
	j2 := mock.Job()
	name := j1.TaskGroups[0].Name

	if tasksUpdated(j1, j2, name) {
		t.Fatalf("bad")
	}

	j2.TaskGroups[0].Tasks[0].Config["command"] = "/bin/other"
	if !tasksUpdated(j1, j2, name) {
		t.Fatalf("bad")
	}

	j3 := mock.Job()
	j3.TaskGroups[0].Tasks[0].Name = "foo"
	if !tasksUpdated(j1, j3, name) {
		t.Fatalf("bad")
	}

	j4 := mock.Job()
	j4.TaskGroups[0].Tasks[0].Driver = "foo"
	if !tasksUpdated(j1, j4, name) {
		t.Fatalf("bad")
	}

	j5 := mock.Job()
	j5.TaskGroups[0].Tasks = append(j5.TaskGroups[0].Tasks,
		j5.TaskGroups[0].Tasks[0])
	if !tasksUpdated(j1, j5, name) {
		t.Fatalf("bad")
	}

	j6 := mock.Job()
	j6.TaskGroups[0].Tasks[0].Resources.Networks[0].DynamicPorts = []structs.Port{
		{Label: "http", Value: 0},
		{Label: "https", Value: 0},
		{Label: "admin", Value: 0},
	}
	if !tasksUpdated(j1, j6, name) {
		t.Fatalf("bad")
	}

	j7 := mock.Job()
	j7.TaskGroups[0].Tasks[0].Env["NEW_ENV"] = "NEW_VALUE"
	if !tasksUpdated(j1, j7, name) {
		t.Fatalf("bad")
	}

	j8 := mock.Job()
	j8.TaskGroups[0].Tasks[0].User = "foo"
	if !tasksUpdated(j1, j8, name) {
		t.Fatalf("bad")
	}

	j9 := mock.Job()
	j9.TaskGroups[0].Tasks[0].Artifacts = []*structs.TaskArtifact{
		{
			GetterSource: "http://foo.com/bar",
		},
	}
	if !tasksUpdated(j1, j9, name) {
		t.Fatalf("bad")
	}

	j10 := mock.Job()
	j10.TaskGroups[0].Tasks[0].Meta["baz"] = "boom"
	if !tasksUpdated(j1, j10, name) {
		t.Fatalf("bad")
	}

	j11 := mock.Job()
	j11.TaskGroups[0].Tasks[0].Resources.CPU = 1337
	if !tasksUpdated(j1, j11, name) {
		t.Fatalf("bad")
	}

	j12 := mock.Job()
	j12.TaskGroups[0].Tasks[0].Resources.Networks[0].MBits = 100
	if !tasksUpdated(j1, j12, name) {
		t.Fatalf("bad")
	}

	j13 := mock.Job()
	j13.TaskGroups[0].Tasks[0].Resources.Networks[0].DynamicPorts[0].Label = "foobar"
	if !tasksUpdated(j1, j13, name) {
		t.Fatalf("bad")
	}

	j14 := mock.Job()
	j14.TaskGroups[0].Tasks[0].Resources.Networks[0].ReservedPorts = []structs.Port{{Label: "foo", Value: 1312}}
	if !tasksUpdated(j1, j14, name) {
		t.Fatalf("bad")
	}

	j15 := mock.Job()
	j15.TaskGroups[0].Tasks[0].Vault = &structs.Vault{Policies: []string{"foo"}}
	if !tasksUpdated(j1, j15, name) {
		t.Fatalf("bad")
	}

	j16 := mock.Job()
	j16.TaskGroups[0].EphemeralDisk.Sticky = true
	if !tasksUpdated(j1, j16, name) {
		t.Fatal("bad")
	}

	// Change group meta
	j17 := mock.Job()
	j17.TaskGroups[0].Meta["j17_test"] = "roll_baby_roll"
	if !tasksUpdated(j1, j17, name) {
		t.Fatal("bad")
	}

	// Change job meta
	j18 := mock.Job()
	j18.Meta["j18_test"] = "roll_baby_roll"
	if !tasksUpdated(j1, j18, name) {
		t.Fatal("bad")
	}
}

func TestEvictAndPlace_LimitLessThanAllocs(t *testing.T) {
	_, ctx := testContext(t)
	allocs := []allocTuple{
		allocTuple{Alloc: &structs.Allocation{ID: structs.GenerateUUID()}},
		allocTuple{Alloc: &structs.Allocation{ID: structs.GenerateUUID()}},
		allocTuple{Alloc: &structs.Allocation{ID: structs.GenerateUUID()}},
		allocTuple{Alloc: &structs.Allocation{ID: structs.GenerateUUID()}},
	}
	diff := &diffResult{}

	limit := 2
	if !evictAndPlace(ctx, diff, allocs, "", &limit) {
		t.Fatal("evictAndReplace() should have returned true")
	}

	if limit != 0 {
		t.Fatalf("evictAndReplace() should decremented limit; got %v; want 0", limit)
	}

	if len(diff.place) != 2 {
		t.Fatalf("evictAndReplace() didn't insert into diffResult properly: %v", diff.place)
	}
}

func TestEvictAndPlace_LimitEqualToAllocs(t *testing.T) {
	_, ctx := testContext(t)
	allocs := []allocTuple{
		allocTuple{Alloc: &structs.Allocation{ID: structs.GenerateUUID()}},
		allocTuple{Alloc: &structs.Allocation{ID: structs.GenerateUUID()}},
		allocTuple{Alloc: &structs.Allocation{ID: structs.GenerateUUID()}},
		allocTuple{Alloc: &structs.Allocation{ID: structs.GenerateUUID()}},
	}
	diff := &diffResult{}

	limit := 4
	if evictAndPlace(ctx, diff, allocs, "", &limit) {
		t.Fatal("evictAndReplace() should have returned false")
	}

	if limit != 0 {
		t.Fatalf("evictAndReplace() should decremented limit; got %v; want 0", limit)
	}

	if len(diff.place) != 4 {
		t.Fatalf("evictAndReplace() didn't insert into diffResult properly: %v", diff.place)
	}
}

func TestSetStatus(t *testing.T) {
	h := NewHarness(t)
	logger := log.New(os.Stderr, "", log.LstdFlags)
	eval := mock.Eval()
	status := "a"
	desc := "b"
	if err := setStatus(logger, h, eval, nil, nil, nil, status, desc, nil, ""); err != nil {
		t.Fatalf("setStatus() failed: %v", err)
	}

	if len(h.Evals) != 1 {
		t.Fatalf("setStatus() didn't update plan: %v", h.Evals)
	}

	newEval := h.Evals[0]
	if newEval.ID != eval.ID || newEval.Status != status || newEval.StatusDescription != desc {
		t.Fatalf("setStatus() submited invalid eval: %v", newEval)
	}

	// Test next evals
	h = NewHarness(t)
	next := mock.Eval()
	if err := setStatus(logger, h, eval, next, nil, nil, status, desc, nil, ""); err != nil {
		t.Fatalf("setStatus() failed: %v", err)
	}

	if len(h.Evals) != 1 {
		t.Fatalf("setStatus() didn't update plan: %v", h.Evals)
	}

	newEval = h.Evals[0]
	if newEval.NextEval != next.ID {
		t.Fatalf("setStatus() didn't set nextEval correctly: %v", newEval)
	}

	// Test blocked evals
	h = NewHarness(t)
	blocked := mock.Eval()
	if err := setStatus(logger, h, eval, nil, blocked, nil, status, desc, nil, ""); err != nil {
		t.Fatalf("setStatus() failed: %v", err)
	}

	if len(h.Evals) != 1 {
		t.Fatalf("setStatus() didn't update plan: %v", h.Evals)
	}

	newEval = h.Evals[0]
	if newEval.BlockedEval != blocked.ID {
		t.Fatalf("setStatus() didn't set BlockedEval correctly: %v", newEval)
	}

	// Test metrics
	h = NewHarness(t)
	metrics := map[string]*structs.AllocMetric{"foo": nil}
	if err := setStatus(logger, h, eval, nil, nil, metrics, status, desc, nil, ""); err != nil {
		t.Fatalf("setStatus() failed: %v", err)
	}

	if len(h.Evals) != 1 {
		t.Fatalf("setStatus() didn't update plan: %v", h.Evals)
	}

	newEval = h.Evals[0]
	if !reflect.DeepEqual(newEval.FailedTGAllocs, metrics) {
		t.Fatalf("setStatus() didn't set failed task group metrics correctly: %v", newEval)
	}

	// Test queued allocations
	h = NewHarness(t)
	queuedAllocs := map[string]int{"web": 1}

	if err := setStatus(logger, h, eval, nil, nil, metrics, status, desc, queuedAllocs, ""); err != nil {
		t.Fatalf("setStatus() failed: %v", err)
	}

	if len(h.Evals) != 1 {
		t.Fatalf("setStatus() didn't update plan: %v", h.Evals)
	}

	newEval = h.Evals[0]
	if !reflect.DeepEqual(newEval.QueuedAllocations, queuedAllocs) {
		t.Fatalf("setStatus() didn't set failed task group metrics correctly: %v", newEval)
	}

	h = NewHarness(t)
	dID := structs.GenerateUUID()
	if err := setStatus(logger, h, eval, nil, nil, metrics, status, desc, queuedAllocs, dID); err != nil {
		t.Fatalf("setStatus() failed: %v", err)
	}

	if len(h.Evals) != 1 {
		t.Fatalf("setStatus() didn't update plan: %v", h.Evals)
	}

	newEval = h.Evals[0]
	if newEval.DeploymentID != dID {
		t.Fatalf("setStatus() didn't set deployment id correctly: %v", newEval)
	}
}

func TestInplaceUpdate_ChangedTaskGroup(t *testing.T) {
	state, ctx := testContext(t)
	eval := mock.Eval()
	job := mock.Job()

	node := mock.Node()
	noErr(t, state.UpsertNode(900, node))

	// Register an alloc
	alloc := &structs.Allocation{
		ID:     structs.GenerateUUID(),
		EvalID: eval.ID,
		NodeID: node.ID,
		JobID:  job.ID,
		Job:    job,
		Resources: &structs.Resources{
			CPU:      2048,
			MemoryMB: 2048,
		},
		DesiredStatus: structs.AllocDesiredStatusRun,
		TaskGroup:     "web",
	}
	alloc.TaskResources = map[string]*structs.Resources{"web": alloc.Resources}
	noErr(t, state.UpsertJobSummary(1000, mock.JobSummary(alloc.JobID)))
	noErr(t, state.UpsertAllocs(1001, []*structs.Allocation{alloc}))

	// Create a new task group that prevents in-place updates.
	tg := &structs.TaskGroup{}
	*tg = *job.TaskGroups[0]
	task := &structs.Task{Name: "FOO"}
	tg.Tasks = nil
	tg.Tasks = append(tg.Tasks, task)

	updates := []allocTuple{{Alloc: alloc, TaskGroup: tg}}
	stack := NewGenericStack(false, ctx)

	// Do the inplace update.
	unplaced, inplace := inplaceUpdate(ctx, eval, job, stack, updates)

	if len(unplaced) != 1 || len(inplace) != 0 {
		t.Fatal("inplaceUpdate incorrectly did an inplace update")
	}

	if len(ctx.plan.NodeAllocation) != 0 {
		t.Fatal("inplaceUpdate incorrectly did an inplace update")
	}
}

func TestInplaceUpdate_NoMatch(t *testing.T) {
	state, ctx := testContext(t)
	eval := mock.Eval()
	job := mock.Job()

	node := mock.Node()
	noErr(t, state.UpsertNode(900, node))

	// Register an alloc
	alloc := &structs.Allocation{
		ID:     structs.GenerateUUID(),
		EvalID: eval.ID,
		NodeID: node.ID,
		JobID:  job.ID,
		Job:    job,
		Resources: &structs.Resources{
			CPU:      2048,
			MemoryMB: 2048,
		},
		DesiredStatus: structs.AllocDesiredStatusRun,
		TaskGroup:     "web",
	}
	alloc.TaskResources = map[string]*structs.Resources{"web": alloc.Resources}
	noErr(t, state.UpsertJobSummary(1000, mock.JobSummary(alloc.JobID)))
	noErr(t, state.UpsertAllocs(1001, []*structs.Allocation{alloc}))

	// Create a new task group that requires too much resources.
	tg := &structs.TaskGroup{}
	*tg = *job.TaskGroups[0]
	resource := &structs.Resources{CPU: 9999}
	tg.Tasks[0].Resources = resource

	updates := []allocTuple{{Alloc: alloc, TaskGroup: tg}}
	stack := NewGenericStack(false, ctx)

	// Do the inplace update.
	unplaced, inplace := inplaceUpdate(ctx, eval, job, stack, updates)

	if len(unplaced) != 1 || len(inplace) != 0 {
		t.Fatal("inplaceUpdate incorrectly did an inplace update")
	}

	if len(ctx.plan.NodeAllocation) != 0 {
		t.Fatal("inplaceUpdate incorrectly did an inplace update")
	}
}

func TestInplaceUpdate_Success(t *testing.T) {
	state, ctx := testContext(t)
	eval := mock.Eval()
	job := mock.Job()

	node := mock.Node()
	noErr(t, state.UpsertNode(900, node))

	// Register an alloc
	alloc := &structs.Allocation{
		ID:        structs.GenerateUUID(),
		EvalID:    eval.ID,
		NodeID:    node.ID,
		JobID:     job.ID,
		Job:       job,
		TaskGroup: job.TaskGroups[0].Name,
		Resources: &structs.Resources{
			CPU:      2048,
			MemoryMB: 2048,
		},
		DesiredStatus: structs.AllocDesiredStatusRun,
	}
	alloc.TaskResources = map[string]*structs.Resources{"web": alloc.Resources}
	noErr(t, state.UpsertJobSummary(999, mock.JobSummary(alloc.JobID)))
	noErr(t, state.UpsertAllocs(1001, []*structs.Allocation{alloc}))

	// Create a new task group that updates the resources.
	tg := &structs.TaskGroup{}
	*tg = *job.TaskGroups[0]
	resource := &structs.Resources{CPU: 737}
	tg.Tasks[0].Resources = resource
	newServices := []*structs.Service{
		{
			Name:      "dummy-service",
			PortLabel: "http",
		},
		{
			Name:      "dummy-service2",
			PortLabel: "http",
		},
	}

	// Delete service 2
	tg.Tasks[0].Services = tg.Tasks[0].Services[:1]

	// Add the new services
	tg.Tasks[0].Services = append(tg.Tasks[0].Services, newServices...)

	updates := []allocTuple{{Alloc: alloc, TaskGroup: tg}}
	stack := NewGenericStack(false, ctx)
	stack.SetJob(job)

	// Do the inplace update.
	unplaced, inplace := inplaceUpdate(ctx, eval, job, stack, updates)

	if len(unplaced) != 0 || len(inplace) != 1 {
		t.Fatal("inplaceUpdate did not do an inplace update")
	}

	if len(ctx.plan.NodeAllocation) != 1 {
		t.Fatal("inplaceUpdate did not do an inplace update")
	}

	if inplace[0].Alloc.ID != alloc.ID {
		t.Fatalf("inplaceUpdate returned the wrong, inplace updated alloc: %#v", inplace)
	}

	// Get the alloc we inserted.
	a := inplace[0].Alloc // TODO(sean@): Verify this is correct vs: ctx.plan.NodeAllocation[alloc.NodeID][0]
	if a.Job == nil {
		t.Fatalf("bad")
	}

	if len(a.Job.TaskGroups) != 1 {
		t.Fatalf("bad")
	}

	if len(a.Job.TaskGroups[0].Tasks) != 1 {
		t.Fatalf("bad")
	}

	if len(a.Job.TaskGroups[0].Tasks[0].Services) != 3 {
		t.Fatalf("Expected number of services: %v, Actual: %v", 3, len(a.Job.TaskGroups[0].Tasks[0].Services))
	}

	serviceNames := make(map[string]struct{}, 3)
	for _, consulService := range a.Job.TaskGroups[0].Tasks[0].Services {
		serviceNames[consulService.Name] = struct{}{}
	}
	if len(serviceNames) != 3 {
		t.Fatalf("bad")
	}

	for _, name := range []string{"dummy-service", "dummy-service2", "web-frontend"} {
		if _, found := serviceNames[name]; !found {
			t.Errorf("Expected consul service name missing: %v", name)
		}
	}
}

func TestEvictAndPlace_LimitGreaterThanAllocs(t *testing.T) {
	_, ctx := testContext(t)
	allocs := []allocTuple{
		allocTuple{Alloc: &structs.Allocation{ID: structs.GenerateUUID()}},
		allocTuple{Alloc: &structs.Allocation{ID: structs.GenerateUUID()}},
		allocTuple{Alloc: &structs.Allocation{ID: structs.GenerateUUID()}},
		allocTuple{Alloc: &structs.Allocation{ID: structs.GenerateUUID()}},
	}
	diff := &diffResult{}

	limit := 6
	if evictAndPlace(ctx, diff, allocs, "", &limit) {
		t.Fatal("evictAndReplace() should have returned false")
	}

	if limit != 2 {
		t.Fatalf("evictAndReplace() should decremented limit; got %v; want 2", limit)
	}

	if len(diff.place) != 4 {
		t.Fatalf("evictAndReplace() didn't insert into diffResult properly: %v", diff.place)
	}
}

func TestTaskGroupConstraints(t *testing.T) {
	constr := &structs.Constraint{RTarget: "bar"}
	constr2 := &structs.Constraint{LTarget: "foo"}
	constr3 := &structs.Constraint{Operand: "<"}

	tg := &structs.TaskGroup{
		Name:          "web",
		Count:         10,
		Constraints:   []*structs.Constraint{constr},
		EphemeralDisk: &structs.EphemeralDisk{},
		Tasks: []*structs.Task{
			&structs.Task{
				Driver: "exec",
				Resources: &structs.Resources{
					CPU:      500,
					MemoryMB: 256,
				},
				Constraints: []*structs.Constraint{constr2},
			},
			&structs.Task{
				Driver: "docker",
				Resources: &structs.Resources{
					CPU:      500,
					MemoryMB: 256,
				},
				Constraints: []*structs.Constraint{constr3},
			},
		},
	}

	// Build the expected values.
	expConstr := []*structs.Constraint{constr, constr2, constr3}
	expDrivers := map[string]struct{}{"exec": struct{}{}, "docker": struct{}{}}
	expSize := &structs.Resources{
		CPU:      1000,
		MemoryMB: 512,
	}

	actConstrains := taskGroupConstraints(tg)
	if !reflect.DeepEqual(actConstrains.constraints, expConstr) {
		t.Fatalf("taskGroupConstraints(%v) returned %v; want %v", tg, actConstrains.constraints, expConstr)
	}
	if !reflect.DeepEqual(actConstrains.drivers, expDrivers) {
		t.Fatalf("taskGroupConstraints(%v) returned %v; want %v", tg, actConstrains.drivers, expDrivers)
	}
	if !reflect.DeepEqual(actConstrains.size, expSize) {
		t.Fatalf("taskGroupConstraints(%v) returned %v; want %v", tg, actConstrains.size, expSize)
	}

}

func TestProgressMade(t *testing.T) {
	noopPlan := &structs.PlanResult{}
	if progressMade(nil) || progressMade(noopPlan) {
		t.Fatal("no progress plan marked as making progress")
	}

	m := map[string][]*structs.Allocation{
		"foo": []*structs.Allocation{mock.Alloc()},
	}
	both := &structs.PlanResult{
		NodeAllocation: m,
		NodeUpdate:     m,
	}
	update := &structs.PlanResult{NodeUpdate: m}
	alloc := &structs.PlanResult{NodeAllocation: m}
	deployment := &structs.PlanResult{Deployment: mock.Deployment()}
	deploymentUpdates := &structs.PlanResult{
		DeploymentUpdates: []*structs.DeploymentStatusUpdate{
			{DeploymentID: structs.GenerateUUID()},
		},
	}
	if !(progressMade(both) && progressMade(update) && progressMade(alloc) &&
		progressMade(deployment) && progressMade(deploymentUpdates)) {
		t.Fatal("bad")
	}
}

func TestDesiredUpdates(t *testing.T) {
	tg1 := &structs.TaskGroup{Name: "foo"}
	tg2 := &structs.TaskGroup{Name: "bar"}
	a2 := &structs.Allocation{TaskGroup: "bar"}

	place := []allocTuple{
		allocTuple{TaskGroup: tg1},
		allocTuple{TaskGroup: tg1},
		allocTuple{TaskGroup: tg1},
		allocTuple{TaskGroup: tg2},
	}
	stop := []allocTuple{
		allocTuple{TaskGroup: tg2, Alloc: a2},
		allocTuple{TaskGroup: tg2, Alloc: a2},
	}
	ignore := []allocTuple{
		allocTuple{TaskGroup: tg1},
	}
	migrate := []allocTuple{
		allocTuple{TaskGroup: tg2},
	}
	inplace := []allocTuple{
		allocTuple{TaskGroup: tg1},
		allocTuple{TaskGroup: tg1},
	}
	destructive := []allocTuple{
		allocTuple{TaskGroup: tg1},
		allocTuple{TaskGroup: tg2},
		allocTuple{TaskGroup: tg2},
	}
	diff := &diffResult{
		place:   place,
		stop:    stop,
		ignore:  ignore,
		migrate: migrate,
	}

	expected := map[string]*structs.DesiredUpdates{
		"foo": {
			Place:             3,
			Ignore:            1,
			InPlaceUpdate:     2,
			DestructiveUpdate: 1,
		},
		"bar": {
			Place:             1,
			Stop:              2,
			Migrate:           1,
			DestructiveUpdate: 2,
		},
	}

	desired := desiredUpdates(diff, inplace, destructive)
	if !reflect.DeepEqual(desired, expected) {
		t.Fatalf("desiredUpdates() returned %#v; want %#v", desired, expected)
	}
}

func TestUtil_AdjustQueuedAllocations(t *testing.T) {
	logger := log.New(os.Stderr, "", log.LstdFlags)
	alloc1 := mock.Alloc()
	alloc2 := mock.Alloc()
	alloc2.CreateIndex = 4
	alloc2.ModifyIndex = 4
	alloc3 := mock.Alloc()
	alloc3.CreateIndex = 3
	alloc3.ModifyIndex = 5
	alloc4 := mock.Alloc()
	alloc4.CreateIndex = 6
	alloc4.ModifyIndex = 8

	planResult := structs.PlanResult{
		NodeUpdate: map[string][]*structs.Allocation{
			"node-1": []*structs.Allocation{alloc1},
		},
		NodeAllocation: map[string][]*structs.Allocation{
			"node-1": []*structs.Allocation{
				alloc2,
			},
			"node-2": []*structs.Allocation{
				alloc3, alloc4,
			},
		},
		RefreshIndex: 3,
		AllocIndex:   16, // Should not be considered
	}

	queuedAllocs := map[string]int{"web": 2}
	adjustQueuedAllocations(logger, &planResult, queuedAllocs)

	if queuedAllocs["web"] != 1 {
		t.Fatalf("expected: %v, actual: %v", 1, queuedAllocs["web"])
	}
}

func TestUtil_UpdateNonTerminalAllocsToLost(t *testing.T) {
	node := mock.Node()
	alloc1 := mock.Alloc()
	alloc1.NodeID = node.ID
	alloc1.DesiredStatus = structs.AllocDesiredStatusStop

	alloc2 := mock.Alloc()
	alloc2.NodeID = node.ID
	alloc2.DesiredStatus = structs.AllocDesiredStatusStop
	alloc2.ClientStatus = structs.AllocClientStatusRunning

	alloc3 := mock.Alloc()
	alloc3.NodeID = node.ID
	alloc3.DesiredStatus = structs.AllocDesiredStatusStop
	alloc3.ClientStatus = structs.AllocClientStatusComplete

	alloc4 := mock.Alloc()
	alloc4.NodeID = node.ID
	alloc4.DesiredStatus = structs.AllocDesiredStatusStop
	alloc4.ClientStatus = structs.AllocClientStatusFailed

	allocs := []*structs.Allocation{alloc1, alloc2, alloc3, alloc4}
	plan := structs.Plan{
		NodeUpdate: make(map[string][]*structs.Allocation),
	}
	tainted := map[string]*structs.Node{node.ID: node}

	updateNonTerminalAllocsToLost(&plan, tainted, allocs)

	allocsLost := make([]string, 0, 2)
	for _, alloc := range plan.NodeUpdate[node.ID] {
		allocsLost = append(allocsLost, alloc.ID)
	}
	expected := []string{alloc1.ID, alloc2.ID}
	if !reflect.DeepEqual(allocsLost, expected) {
		t.Fatalf("actual: %v, expected: %v", allocsLost, expected)
	}
}
