package api

import (
	"reflect"
	"strings"
	"testing"
)

func TestJobs_Register(t *testing.T) {
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Listing jobs before registering returns nothing
	resp, qm, err := jobs.List()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if qm.LastIndex != 0 {
		t.Fatalf("bad index: %d", qm.LastIndex)
	}
	if n := len(resp); n != 0 {
		t.Fatalf("expected 0 jobs, got: %d", n)
	}

	// Create a job and attempt to register it
	job := testJob()
	eval, wm, err := jobs.Register(job, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if eval == "" {
		t.Fatalf("missing eval id")
	}
	assertWriteMeta(t, wm)

	// Query the jobs back out again
	resp, qm, err = jobs.List()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertQueryMeta(t, qm)

	// Check that we got the expected response
	expect := []*Job{job}
	if !reflect.DeepEqual(resp, expect) {
		t.Fatalf("bad: %#v", resp[0])
	}
}

func TestJobs_Info(t *testing.T) {
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Trying to retrieve a job by ID before it exists
	// returns an error
	_, _, err := jobs.Info("job1")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got: %#v", err)
	}

	// Register the job
	job := testJob()
	_, wm, err := jobs.Register(job, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertWriteMeta(t, wm)

	// Query the job again and ensure it exists
	result, qm, err := jobs.Info("job1")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertQueryMeta(t, qm)

	// Check that the result is what we expect
	if !reflect.DeepEqual(result, job) {
		t.Fatalf("expect: %#v, got: %#v", job, result)
	}
}

func TestJobs_Allocations(t *testing.T) {
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Looking up by a non-existent job returns nothing
	allocs, qm, err := jobs.Allocations("job1")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if qm.LastIndex != 0 {
		t.Fatalf("bad index: %d", qm.LastIndex)
	}
	if n := len(allocs); n != 0 {
		t.Fatalf("expected 0 allocs, got: %d", n)
	}

	// TODO: do something here to create some allocations for
	// an existing job, lookup again.
}

func TestJobs_Evaluations(t *testing.T) {
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Looking up by a non-existent job ID returns nothing
	evals, qm, err := jobs.Evaluations("job1")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if qm.LastIndex != 0 {
		t.Fatalf("bad index: %d", qm.LastIndex)
	}
	if n := len(evals); n != 0 {
		t.Fatalf("expected 0 evals, got: %d", n)
	}

	// Insert a job. This also creates an evaluation so we should
	// be able to query that out after.
	job := testJob()
	evalID, wm, err := jobs.Register(job, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertWriteMeta(t, wm)

	// Look up the evaluations again.
	evals, qm, err = jobs.Evaluations("job1")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertQueryMeta(t, qm)

	// Check that we got the evals back
	if n := len(evals); n == 0 || evals[0].ID != evalID {
		t.Fatalf("expected 1 eval (%s), got: %#v", evalID, evals)
	}
}

func TestJobs_Delete(t *testing.T) {
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Register a new job
	job := testJob()
	_, wm, err := jobs.Register(job, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertWriteMeta(t, wm)

	// Attempting delete on non-existing job does not error
	wm2, err := jobs.Delete("nope", nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertWriteMeta(t, wm2)

	// Deleting an existing job works
	wm3, err := jobs.Delete("job1", nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertWriteMeta(t, wm3)

	// Check that the job is really gone
	result, qm, err := jobs.List()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertQueryMeta(t, qm)
	if n := len(result); n != 0 {
		t.Fatalf("expected 0 jobs, got: %d", n)
	}
}
