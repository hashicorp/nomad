package scheduler

import (
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"
)

// Test that we properly create the bitmap even when the alloc set includes an
// allocation with a higher count than the current min count and it is byte
// alligned.
// Ensure no regerssion from: https://github.com/hashicorp/nomad/issues/3008
func TestBitmapFrom(t *testing.T) {
	input := map[string]*structs.Allocation{
		"8": {
			JobID:     "foo",
			TaskGroup: "bar",
			Name:      "foo.bar[8]",
		},
	}
	b := bitmapFrom(input, 1)
	exp := uint(16)
	if act := b.Size(); act != exp {
		t.Fatalf("got %d; want %d", act, exp)
	}

	b = bitmapFrom(input, 8)
	if act := b.Size(); act != exp {
		t.Fatalf("got %d; want %d", act, exp)
	}
}
