package api

import (
	"strings"
	"testing"
)

func TestEvaluations_List(t *testing.T) {
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
	evalID, wm, err := jobs.Register(job, nil)
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

	// Check if we have the right list
	if len(result) != 1 || result[0].ID != evalID {
		t.Fatalf("bad: %#v", result)
	}
}

func TestEvaluations_Info(t *testing.T) {
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	e := c.Evaluations()

	// Querying a non-existent evaluation returns error
	_, _, err := e.Info("8E231CF4-CA48-43FF-B694-5801E69E22FA", nil)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got: %s", err)
	}

	// Register a job. Creates a new evaluation.
	jobs := c.Jobs()
	job := testJob()
	evalID, wm, err := jobs.Register(job, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertWriteMeta(t, wm)

	// Try looking up by the new eval ID
	result, qm, err := e.Info(evalID, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertQueryMeta(t, qm)

	// Check that we got the right result
	if result == nil || result.ID != evalID {
		t.Fatalf("expected eval %q, got: %#v", evalID, result)
	}
}

func TestEvaluations_Allocations(t *testing.T) {
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
