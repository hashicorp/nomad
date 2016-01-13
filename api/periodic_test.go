package api

import (
	"strings"
	"testing"

	"github.com/hashicorp/nomad/testutil"
)

func TestPeriodicForce(t *testing.T) {
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()
	periodic := c.PeriodicJobs()

	// Force-eval on a non-existent job fails
	_, _, err := periodic.Force("job1", nil)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got: %#v", err)
	}

	// Create a new job
	job := testPeriodicJob()
	_, _, err = jobs.Register(job, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.WaitForResult(func() (bool, error) {
		out, _, err := jobs.Info(job.ID, nil)
		if err != nil || out == nil || out.ID != job.ID {
			return false, err
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})

	// Try force again
	evalID, wm, err := periodic.Force(job.ID, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertWriteMeta(t, wm)

	if evalID == "" {
		t.Fatalf("empty evalID")
	}

	// Retrieve the eval
	evals := c.Evaluations()
	eval, qm, err := evals.Info(evalID, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertQueryMeta(t, qm)
	if eval.ID == evalID {
		return
	}
	t.Fatalf("evaluation %q missing", evalID)
}
