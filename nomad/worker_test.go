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

	// Create the evaluation
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

func TestWorker_sendAck(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.EnabledSchedulers = []string{structs.JobTypeService}
	})
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	// Create the evaluation
	eval1 := mockEval()
	s1.evalBroker.Enqueue(eval1)

	// Create a worker
	w := &Worker{srv: s1, logger: s1.logger}

	// Attempt dequeue
	eval, _ := w.dequeueEvaluation(10 * time.Millisecond)

	// Check the depth is 0, 1 unacked
	stats := s1.evalBroker.Stats()
	if stats.TotalReady != 0 && stats.TotalUnacked != 1 {
		t.Fatalf("bad: %#v", stats)
	}

	// Send the Nack
	w.sendAck(eval.ID, false)

	// Check the depth is 1, nothing unacked
	stats = s1.evalBroker.Stats()
	if stats.TotalReady != 1 && stats.TotalUnacked != 0 {
		t.Fatalf("bad: %#v", stats)
	}

	// Attempt dequeue
	eval, _ = w.dequeueEvaluation(10 * time.Millisecond)

	// Send the Ack
	w.sendAck(eval.ID, true)

	// Check the depth is 0
	stats = s1.evalBroker.Stats()
	if stats.TotalReady != 0 && stats.TotalUnacked != 0 {
		t.Fatalf("bad: %#v", stats)
	}
}
