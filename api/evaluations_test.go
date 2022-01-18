package api

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/api/internal/testutil"
)

func TestEvaluations_List(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	e := c.Evaluations()

	// Listing when nothing exists returns empty
	result, qm, err := e.List(nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if qm.LastIndex != 0 {
		t.Fatalf("bad index: %d", qm.LastIndex)
	}
	if n := len(result); n != 0 {
		t.Fatalf("expected 0 evaluations, got: %d", n)
	}

	// Register a job. This will create an evaluation.
	jobs := c.Jobs()
	job := testJob()
	resp, wm, err := jobs.Register(job, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertWriteMeta(t, wm)

	// Check the evaluations again
	result, qm, err = e.List(nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertQueryMeta(t, qm)

	// if the eval fails fast there can be more than 1
	// but they are in order of most recent first, so look at the last one
	if len(result) == 0 {
		t.Fatalf("expected eval (%s), got none", resp.EvalID)
	}
	idx := len(result) - 1
	if result[idx].ID != resp.EvalID {
		t.Fatalf("expected eval (%s), got: %#v", resp.EvalID, result[idx])
	}

	// wait until the 2nd eval shows up before we try paging
	results := []*Evaluation{}
	testutil.WaitForResult(func() (bool, error) {
		results, _, err = e.List(nil)
		if len(results) < 2 || err != nil {
			return false, err
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})

	// Check the evaluations again with paging; note that while this
	// package sorts by timestamp, the actual HTTP API sorts by ID
	// so we need to use that for the NextToken
	ids := []string{results[0].ID, results[1].ID}
	sort.Strings(ids)
	result, qm, err = e.List(&QueryOptions{PerPage: int32(1), NextToken: ids[1]})
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected no evals after last one but got %v", result[0])
	}
}

func TestEvaluations_PrefixList(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	e := c.Evaluations()

	// Listing when nothing exists returns empty
	result, qm, err := e.PrefixList("abcdef")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if qm.LastIndex != 0 {
		t.Fatalf("bad index: %d", qm.LastIndex)
	}
	if n := len(result); n != 0 {
		t.Fatalf("expected 0 evaluations, got: %d", n)
	}

	// Register a job. This will create an evaluation.
	jobs := c.Jobs()
	job := testJob()
	resp, wm, err := jobs.Register(job, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertWriteMeta(t, wm)

	// Check the evaluations again
	result, qm, err = e.PrefixList(resp.EvalID[:4])
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertQueryMeta(t, qm)

	// Check if we have the right list
	if len(result) != 1 || result[0].ID != resp.EvalID {
		t.Fatalf("bad: %#v", result)
	}
}

func TestEvaluations_Info(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	e := c.Evaluations()

	// Querying a nonexistent evaluation returns error
	_, _, err := e.Info("8E231CF4-CA48-43FF-B694-5801E69E22FA", nil)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got: %s", err)
	}

	// Register a job. Creates a new evaluation.
	jobs := c.Jobs()
	job := testJob()
	resp, wm, err := jobs.Register(job, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertWriteMeta(t, wm)

	// Try looking up by the new eval ID
	result, qm, err := e.Info(resp.EvalID, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertQueryMeta(t, qm)

	// Check that we got the right result
	if result == nil || result.ID != resp.EvalID {
		t.Fatalf("expected eval %q, got: %#v", resp.EvalID, result)
	}
}

func TestEvaluations_Allocations(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	e := c.Evaluations()

	// Returns empty if no allocations
	allocs, qm, err := e.Allocations("8E231CF4-CA48-43FF-B694-5801E69E22FA", nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if qm.LastIndex != 0 {
		t.Fatalf("bad index: %d", qm.LastIndex)
	}
	if n := len(allocs); n != 0 {
		t.Fatalf("expected 0 allocs, got: %d", n)
	}
}

func TestEvaluations_Sort(t *testing.T) {
	t.Parallel()
	evals := []*Evaluation{
		{CreateIndex: 2},
		{CreateIndex: 1},
		{CreateIndex: 5},
	}
	sort.Sort(EvalIndexSort(evals))

	expect := []*Evaluation{
		{CreateIndex: 5},
		{CreateIndex: 2},
		{CreateIndex: 1},
	}
	if !reflect.DeepEqual(evals, expect) {
		t.Fatalf("\n\n%#v\n\n%#v", evals, expect)
	}
}
