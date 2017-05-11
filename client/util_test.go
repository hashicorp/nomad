package client

import (
	"reflect"
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestDiffAllocs(t *testing.T) {
	t.Parallel()
	alloc1 := mock.Alloc() // Ignore
	alloc2 := mock.Alloc() // Update
	alloc2u := new(structs.Allocation)
	*alloc2u = *alloc2
	alloc2u.AllocModifyIndex += 1
	alloc3 := mock.Alloc() // Remove
	alloc4 := mock.Alloc() // Add

	exist := []*structs.Allocation{
		alloc1,
		alloc2,
		alloc3,
	}
	update := &allocUpdates{
		pulled: map[string]*structs.Allocation{
			alloc2u.ID: alloc2u,
			alloc4.ID:  alloc4,
		},
		filtered: map[string]struct{}{
			alloc1.ID: struct{}{},
		},
	}

	result := diffAllocs(exist, update)

	if len(result.ignore) != 1 || result.ignore[0] != alloc1 {
		t.Fatalf("Bad: %#v", result.ignore)
	}
	if len(result.added) != 1 || result.added[0] != alloc4 {
		t.Fatalf("Bad: %#v", result.added)
	}
	if len(result.removed) != 1 || result.removed[0] != alloc3 {
		t.Fatalf("Bad: %#v", result.removed)
	}
	if len(result.updated) != 1 {
		t.Fatalf("Bad: %#v", result.updated)
	}
	if result.updated[0].exist != alloc2 || result.updated[0].updated != alloc2u {
		t.Fatalf("Bad: %#v", result.updated)
	}
}

func TestShuffleStrings(t *testing.T) {
	t.Parallel()
	// Generate input
	inp := make([]string, 10)
	for idx := range inp {
		inp[idx] = structs.GenerateUUID()
	}

	// Copy the input
	orig := make([]string, len(inp))
	copy(orig, inp)

	// Shuffle
	shuffleStrings(inp)

	// Ensure order is not the same
	if reflect.DeepEqual(inp, orig) {
		t.Fatalf("shuffle failed")
	}
}
