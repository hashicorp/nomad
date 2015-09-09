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
	job := &Job{
		ID:       "job1",
		Name:     "Job #1",
		Type:     "service",
		Priority: 1,
	}
	eval, wm, err := jobs.Register(job, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if eval == "" {
		t.Fatalf("missing eval id")
	}
	if wm.LastIndex == 0 {
		t.Fatalf("bad index: %d", wm.LastIndex)
	}

	// Query the jobs back out again
	resp, qm, err = jobs.List()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if qm.LastIndex == 0 {
		t.Fatalf("bad index: %d", qm.LastIndex)
	}

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
	job := &Job{
		ID:       "job1",
		Name:     "Job #1",
		Type:     "service",
		Priority: 1,
	}
	if _, _, err := jobs.Register(job, nil); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Query the job again and ensure it exists
	result, qm, err := jobs.Info("job1")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if qm.LastIndex == 0 {
		t.Fatalf("bad index: %d", qm.LastIndex)
	}
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
		t.Fatalf("expected 0, got: %d", qm.LastIndex)
	}
	if n := len(allocs); n != 0 {
		t.Fatalf("expected 0 allocs, got: %d", n)
	}

	// TODO: do something here to create some allocations for
	// an existing job, lookup again.
}
