package api

import (
	"testing"
)

func TestAllocs_List(t *testing.T) {
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	a := c.Allocs()

	// Querying when no allocs exist returns nothing
	allocs, qm, err := a.List()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if qm.LastIndex != 0 {
		t.Fatalf("bad index: %d", qm.LastIndex)
	}
	if n := len(allocs); n != 0 {
		t.Fatalf("expected 0 allocs, got: %d", n)
	}

	// TODO: do something that causes an allocation to actually happen
	// so we can query for them.
	return

	job := &Job{
		ID:   "job1",
		Name: "Job #1",
		Type: "service",
	}
	eval, _, err := c.Jobs().Register(job, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// List the allocations again
	allocs, qm, err = a.List()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if qm.LastIndex == 0 {
		t.Fatalf("bad index: %d", qm.LastIndex)
	}

	// Check that we got the allocation back
	if len(allocs) == 0 || allocs[0].EvalID != eval {
		t.Fatalf("bad: %#v", allocs)
	}
}
