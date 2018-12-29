package api

import (
	"reflect"
	"sort"
	"testing"
)

func TestAllocations_List(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	a := c.Allocations()

	// Querying when no allocs exist returns nothing
	allocs, qm, err := a.List(nil)
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

	//job := &Job{
	//ID:   helper.StringToPtr("job1"),
	//Name: helper.StringToPtr("Job #1"),
	//Type: helper.StringToPtr(JobTypeService),
	//}
	//eval, _, err := c.Jobs().Register(job, nil)
	//if err != nil {
	//t.Fatalf("err: %s", err)
	//}

	//// List the allocations again
	//allocs, qm, err = a.List(nil)
	//if err != nil {
	//t.Fatalf("err: %s", err)
	//}
	//if qm.LastIndex == 0 {
	//t.Fatalf("bad index: %d", qm.LastIndex)
	//}

	//// Check that we got the allocation back
	//if len(allocs) == 0 || allocs[0].EvalID != eval {
	//t.Fatalf("bad: %#v", allocs)
	//}
}

func TestAllocations_PrefixList(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	a := c.Allocations()

	// Querying when no allocs exist returns nothing
	allocs, qm, err := a.PrefixList("")
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

	//job := &Job{
	//ID:   helper.StringToPtr("job1"),
	//Name: helper.StringToPtr("Job #1"),
	//Type: helper.StringToPtr(JobTypeService),
	//}

	//eval, _, err := c.Jobs().Register(job, nil)
	//if err != nil {
	//t.Fatalf("err: %s", err)
	//}

	//// List the allocations by prefix
	//allocs, qm, err = a.PrefixList("foobar")
	//if err != nil {
	//t.Fatalf("err: %s", err)
	//}
	//if qm.LastIndex == 0 {
	//t.Fatalf("bad index: %d", qm.LastIndex)
	//}

	//// Check that we got the allocation back
	//if len(allocs) == 0 || allocs[0].EvalID != eval {
	//t.Fatalf("bad: %#v", allocs)
	//}
}

func TestAllocations_CreateIndexSort(t *testing.T) {
	t.Parallel()
	allocs := []*AllocationListStub{
		{CreateIndex: 2},
		{CreateIndex: 1},
		{CreateIndex: 5},
	}
	sort.Sort(AllocIndexSort(allocs))

	expect := []*AllocationListStub{
		{CreateIndex: 5},
		{CreateIndex: 2},
		{CreateIndex: 1},
	}
	if !reflect.DeepEqual(allocs, expect) {
		t.Fatalf("\n\n%#v\n\n%#v", allocs, expect)
	}
}
