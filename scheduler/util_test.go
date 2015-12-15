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
	oldJob.ModifyIndex -= 1

	tainted := map[string]bool{
		"dead": true,
		"zip":  false,
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
		},

		// Migrate the 3rd
		&structs.Allocation{
			ID:     structs.GenerateUUID(),
			NodeID: "dead",
			Name:   "my-job.web[2]",
		},
	}

	diff := diffAllocs(job, tainted, required, allocs)
	place := diff.place
	update := diff.update
	migrate := diff.migrate
	stop := diff.stop
	ignore := diff.ignore

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

	// We should place 7
	if len(place) != 7 {
		t.Fatalf("bad: %#v", place)
	}
}

func TestDiffSystemAllocs(t *testing.T) {
	job := mock.SystemJob()

	// Create three alive nodes.
	nodes := []*structs.Node{{ID: "foo"}, {ID: "bar"}, {ID: "baz"}}

	// The "old" job has a previous modify index
	oldJob := new(structs.Job)
	*oldJob = *job
	oldJob.ModifyIndex -= 1

	tainted := map[string]bool{
		"dead": true,
		"baz":  false,
	}

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

		// Stop allocation on dead.
		&structs.Allocation{
			ID:     structs.GenerateUUID(),
			NodeID: "dead",
			Name:   "my-job.web[0]",
		},
	}

	diff := diffSystemAllocs(job, nodes, tainted, allocs)
	place := diff.place
	update := diff.update
	migrate := diff.migrate
	stop := diff.stop
	ignore := diff.ignore

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

	// We should place 1
	if len(place) != 1 {
		t.Fatalf("bad: %#v", place)
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

	nodes, err := readyNodesInDCs(state, []string{"dc1", "dc2"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(nodes) != 2 {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[0].ID == node3.ID || nodes[1].ID == node3.ID {
		t.Fatalf("Bad: %#v", nodes)
	}
}

func TestRetryMax(t *testing.T) {
	calls := 0
	bad := func() (bool, error) {
		calls += 1
		return false, nil
	}
	err := retryMax(3, bad)
	if err == nil {
		t.Fatalf("should fail")
	}
	if calls != 3 {
		t.Fatalf("mis match")
	}

	calls = 0
	good := func() (bool, error) {
		calls += 1
		return true, nil
	}
	err = retryMax(3, good)
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
		&structs.Allocation{NodeID: "blah"},
	}
	tainted, err := taintedNodes(state, allocs)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(tainted) != 5 {
		t.Fatalf("bad: %v", tainted)
	}
	if tainted[node1.ID] || tainted[node2.ID] {
		t.Fatalf("Bad: %v", tainted)
	}
	if !tainted[node3.ID] || !tainted[node4.ID] || !tainted["blah"] {
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

	if tasksUpdated(j1.TaskGroups[0], j2.TaskGroups[0]) {
		t.Fatalf("bad")
	}

	j2.TaskGroups[0].Tasks[0].Config["command"] = "/bin/other"
	if !tasksUpdated(j1.TaskGroups[0], j2.TaskGroups[0]) {
		t.Fatalf("bad")
	}

	j3 := mock.Job()
	j3.TaskGroups[0].Tasks[0].Name = "foo"
	if !tasksUpdated(j1.TaskGroups[0], j3.TaskGroups[0]) {
		t.Fatalf("bad")
	}

	j4 := mock.Job()
	j4.TaskGroups[0].Tasks[0].Driver = "foo"
	if !tasksUpdated(j1.TaskGroups[0], j4.TaskGroups[0]) {
		t.Fatalf("bad")
	}

	j5 := mock.Job()
	j5.TaskGroups[0].Tasks = append(j5.TaskGroups[0].Tasks,
		j5.TaskGroups[0].Tasks[0])
	if !tasksUpdated(j1.TaskGroups[0], j5.TaskGroups[0]) {
		t.Fatalf("bad")
	}

	j6 := mock.Job()
	j6.TaskGroups[0].Tasks[0].Resources.Networks[0].DynamicPorts = []structs.Port{{"http", 0}, {"https", 0}, {"admin", 0}}
	if !tasksUpdated(j1.TaskGroups[0], j6.TaskGroups[0]) {
		t.Fatalf("bad")
	}

	j7 := mock.Job()
	j7.TaskGroups[0].Tasks[0].Env["NEW_ENV"] = "NEW_VALUE"
	if !tasksUpdated(j1.TaskGroups[0], j7.TaskGroups[0]) {
		t.Fatalf("bad")
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
	if err := setStatus(logger, h, eval, nil, status, desc); err != nil {
		t.Fatalf("setStatus() failed: %v", err)
	}

	if len(h.Evals) != 1 {
		t.Fatalf("setStatus() didn't update plan: %v", h.Evals)
	}

	newEval := h.Evals[0]
	if newEval.ID != eval.ID || newEval.Status != status || newEval.StatusDescription != desc {
		t.Fatalf("setStatus() submited invalid eval: %v", newEval)
	}

	h = NewHarness(t)
	next := mock.Eval()
	if err := setStatus(logger, h, eval, next, status, desc); err != nil {
		t.Fatalf("setStatus() failed: %v", err)
	}

	if len(h.Evals) != 1 {
		t.Fatalf("setStatus() didn't update plan: %v", h.Evals)
	}

	newEval = h.Evals[0]
	if newEval.NextEval != next.ID {
		t.Fatalf("setStatus() didn't set nextEval correctly: %v", newEval)
	}
}

func TestInplaceUpdate_ChangedTaskGroup(t *testing.T) {
	state, ctx := testContext(t)
	eval := mock.Eval()
	job := mock.Job()

	node := mock.Node()
	noErr(t, state.UpsertNode(1000, node))

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
	}
	alloc.TaskResources = map[string]*structs.Resources{"web": alloc.Resources}
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
	unplaced := inplaceUpdate(ctx, eval, job, stack, updates)

	if len(unplaced) != 1 {
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
	noErr(t, state.UpsertNode(1000, node))

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
	}
	alloc.TaskResources = map[string]*structs.Resources{"web": alloc.Resources}
	noErr(t, state.UpsertAllocs(1001, []*structs.Allocation{alloc}))

	// Create a new task group that requires too much resources.
	tg := &structs.TaskGroup{}
	*tg = *job.TaskGroups[0]
	resource := &structs.Resources{CPU: 9999}
	tg.Tasks[0].Resources = resource

	updates := []allocTuple{{Alloc: alloc, TaskGroup: tg}}
	stack := NewGenericStack(false, ctx)

	// Do the inplace update.
	unplaced := inplaceUpdate(ctx, eval, job, stack, updates)

	if len(unplaced) != 1 {
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
	noErr(t, state.UpsertNode(1000, node))

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
	alloc.PopulateServiceIDs()
	noErr(t, state.UpsertAllocs(1001, []*structs.Allocation{alloc}))

	if alloc.Services["web-frontend"] == "" {
		t.Fatal("Service ID needs to be generated for service")
	}

	// Create a new task group that updates the resources.
	tg := &structs.TaskGroup{}
	*tg = *job.TaskGroups[0]
	resource := &structs.Resources{CPU: 737}
	tg.Tasks[0].Resources = resource
	tg.Tasks[0].Services = []*structs.Service{}

	updates := []allocTuple{{Alloc: alloc, TaskGroup: tg}}
	stack := NewGenericStack(false, ctx)
	stack.SetJob(job)

	// Do the inplace update.
	unplaced := inplaceUpdate(ctx, eval, job, stack, updates)

	if len(unplaced) != 0 {
		t.Fatal("inplaceUpdate did not do an inplace update")
	}

	if len(ctx.plan.NodeAllocation) != 1 {
		t.Fatal("inplaceUpdate did not do an inplace update")
	}

	// Get the alloc we inserted.
	a := ctx.plan.NodeAllocation[alloc.NodeID][0]
	if len(a.Services) != 0 {
		t.Fatalf("Expected number of services: %v, Actual: %v", 0, len(a.Services))
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
		Name:        "web",
		Count:       10,
		Constraints: []*structs.Constraint{constr},
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

func TestInitTaskState(t *testing.T) {
	tg := &structs.TaskGroup{
		Tasks: []*structs.Task{
			&structs.Task{Name: "foo"},
			&structs.Task{Name: "bar"},
		},
	}
	expPending := map[string]*structs.TaskState{
		"foo": &structs.TaskState{State: structs.TaskStatePending},
		"bar": &structs.TaskState{State: structs.TaskStatePending},
	}
	expDead := map[string]*structs.TaskState{
		"foo": &structs.TaskState{State: structs.TaskStateDead},
		"bar": &structs.TaskState{State: structs.TaskStateDead},
	}
	actPending := initTaskState(tg, structs.TaskStatePending)
	actDead := initTaskState(tg, structs.TaskStateDead)

	if !(reflect.DeepEqual(expPending, actPending) && reflect.DeepEqual(expDead, actDead)) {
		t.Fatal("Expected and actual not equal")
	}
}
