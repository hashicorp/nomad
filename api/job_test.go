package api

import (
	"reflect"
	"testing"
)

func TestJobs_Register(t *testing.T) {
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Listing jobs before registering returns nothing
	resp, err := jobs.List()
	if err != nil {
		t.Fatalf("err: %s", err)
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
	eval, _, err := jobs.Register(job, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if eval == "" {
		t.Fatalf("missing eval id")
	}

	// Query the jobs back out again
	resp, err = jobs.List()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Check that we got the expected response
	expect := []*Job{job}
	if !reflect.DeepEqual(resp, expect) {
		t.Fatalf("bad: %#v", resp[0])
	}
}
