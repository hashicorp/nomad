package scheduler

import (
	"fmt"
	"os"
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
			ID:     mock.GenerateUUID(),
			NodeID: "zip",
			Name:   "my-job.web[0]",
			Job:    oldJob,
		},

		// Ignore the 2rd
		&structs.Allocation{
			ID:     mock.GenerateUUID(),
			NodeID: "zip",
			Name:   "my-job.web[1]",
			Job:    job,
		},

		// Evict 11th
		&structs.Allocation{
			ID:     mock.GenerateUUID(),
			NodeID: "zip",
			Name:   "my-job.web[10]",
		},

		// Migrate the 3rd
		&structs.Allocation{
			ID:     mock.GenerateUUID(),
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
