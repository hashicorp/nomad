package nomad

import (
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
)

func TestWorker_dequeueEvaluation(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.EnabledSchedulers = []string{structs.JobTypeService}
	})
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	eval1 := mockEval()
	s1.evalBroker.Enqueue(eval1)

	// Create a worker
	w := &Worker{srv: s1, logger: s1.logger}

	// Attempt dequeue
	eval, shutdown := w.dequeueEvaluation(10 * time.Millisecond)
	if shutdown {
		t.Fatalf("should not shutdown")
	}

	// Ensure we get a sane eval
	if !reflect.DeepEqual(eval, eval1) {
		t.Fatalf("bad: %#v %#v", eval, eval1)
	}
}

func TestWorker_dequeueEvaluation_shutdown(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.EnabledSchedulers = []string{structs.JobTypeService}
	})
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	// Create a worker
	w := &Worker{srv: s1, logger: s1.logger}

	go func() {
		time.Sleep(10 * time.Millisecond)
		s1.Shutdown()
	}()

	// Attempt dequeue
	eval, shutdown := w.dequeueEvaluation(10 * time.Millisecond)
	if !shutdown {
		t.Fatalf("should not shutdown")
	}

	// Ensure we get a sane eval
	if eval != nil {
		t.Fatalf("bad: %#v", eval)
	}
}
