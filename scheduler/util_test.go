package scheduler

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
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

func TestIndexAllocs(t *testing.T) {
	allocs := []*structs.Allocation{
		&structs.Allocation{Name: "foo"},
		&structs.Allocation{Name: "foo"},
		&structs.Allocation{Name: "bar"},
	}
	index := indexAllocs(allocs)
	if len(index) != 2 {
		t.Fatalf("bad: %#v", index)
	}
	if len(index["foo"]) != 2 {
		t.Fatalf("bad: %#v", index)
	}
	if len(index["bar"]) != 1 {
		t.Fatalf("bad: %#v", index)
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
	existing := indexAllocs(allocs)

	place, update, migrate, evict, ignore := diffAllocs(job, tainted, required, existing)

	// We should update the first alloc
	if len(update) != 1 || update[0].ID != allocs[0].ID {
		t.Fatalf("bad: %#v", update)
	}

	// We should ignore the second alloc
	if len(ignore) != 1 || ignore[0].ID != allocs[1].ID {
		t.Fatalf("bad: %#v", ignore)
	}

	// We should evict the 3rd alloc
	if len(evict) != 1 || evict[0].ID != allocs[2].ID {
		t.Fatalf("bad: %#v", evict)
	}

	// We should migrate the 4rd alloc
	if len(migrate) != 1 || migrate[0].ID != allocs[3].ID {
		t.Fatalf("bad: %#v", migrate)
	}

	// We should place 7
	if len(place) != 7 {
		t.Fatalf("bad: %#v", place)
	}
}

func TestAddEvictsToPlan(t *testing.T) {
	allocs := []*structs.Allocation{
		&structs.Allocation{
			ID:     mock.GenerateUUID(),
			NodeID: "zip",
			Name:   "foo",
		},
		&structs.Allocation{
			ID:     mock.GenerateUUID(),
			NodeID: "zip",
			Name:   "foo",
		},
		&structs.Allocation{
			ID:     mock.GenerateUUID(),
			NodeID: "zip",
			Name:   "bar",
		},
	}
	plan := &structs.Plan{
		NodeEvict: make(map[string][]string),
	}
	index := indexAllocs(allocs)

	evict := []allocNameID{
		allocNameID{Name: "foo", ID: allocs[0].ID},
		allocNameID{Name: "bar", ID: allocs[2].ID},
	}
	addEvictsToPlan(plan, evict, index)

	nodeEvict := plan.NodeEvict["zip"]
	if len(nodeEvict) != 2 {
		t.Fatalf("bad: %#v %v", plan, nodeEvict)
	}
	if nodeEvict[0] != allocs[0].ID || nodeEvict[1] != allocs[2].ID {
		t.Fatalf("bad: %v", nodeEvict)
	}
}
