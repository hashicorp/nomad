package nomad

import (
	"log"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/scheduler"
	"github.com/hashicorp/nomad/testutil"
)

type NoopScheduler struct {
	state   scheduler.State
	planner scheduler.Planner
	eval    *structs.Evaluation
	err     error
}

func (n *NoopScheduler) Process(eval *structs.Evaluation) error {
	if n.state == nil {
		panic("missing state")
	}
	if n.planner == nil {
		panic("missing planner")
	}
	n.eval = eval
	return n.err
}

func init() {
	scheduler.BuiltinSchedulers["noop"] = func(logger *log.Logger, s scheduler.State, p scheduler.Planner) scheduler.Scheduler {
		n := &NoopScheduler{
			state:   s,
			planner: p,
		}
		return n
	}
}

func TestWorker_dequeueEvaluation(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.EnabledSchedulers = []string{structs.JobTypeService}
	})
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	// Create the evaluation
	eval1 := mock.Eval()
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
	eval1 := mock.Eval()
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

func TestWorker_waitForIndex(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.EnabledSchedulers = []string{structs.JobTypeService}
	})
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	// Get the current index
	index := s1.raft.AppliedIndex()

	// Cause an increment
	go func() {
		time.Sleep(10 * time.Millisecond)
		s1.raft.Barrier(0)
	}()

	// Wait for a future index
	w := &Worker{srv: s1, logger: s1.logger}
	err := w.waitForIndex(index+1, time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Cause a timeout
	err = w.waitForIndex(index+100, 10*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("err: %v", err)
	}
}

func TestWorker_invokeScheduler(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.EnabledSchedulers = []string{structs.JobTypeService}
	})
	defer s1.Shutdown()

	w := &Worker{srv: s1, logger: s1.logger}
	eval := mock.Eval()
	eval.Type = "noop"

	err := w.invokeScheduler(eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestWorker_SubmitPlan(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.EnabledSchedulers = []string{structs.JobTypeService}
	})
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	// Register node
	node := mock.Node()
	testRegisterNode(t, s1, node)

	// Create an allocation plan
	alloc := mock.Alloc()
	plan := &structs.Plan{
		NodeAllocation: map[string][]*structs.Allocation{
			node.ID: []*structs.Allocation{alloc},
		},
	}

	// Attempt to submit a plan
	w := &Worker{srv: s1, logger: s1.logger}
	result, state, err := w.SubmitPlan(plan)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should have no update
	if state != nil {
		t.Fatalf("unexpected state update")
	}

	// Result should have allocated
	if result == nil {
		t.Fatalf("missing result")
	}

	if result.AllocIndex == 0 {
		t.Fatalf("Bad: %#v", result)
	}
	if len(result.NodeAllocation) != 1 {
		t.Fatalf("Bad: %#v", result)
	}
}

func TestWorker_SubmitPlan_MissingNodeRefresh(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.EnabledSchedulers = []string{structs.JobTypeService}
	})
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	// Register node
	node := mock.Node()
	testRegisterNode(t, s1, node)

	// Create an allocation plan, with unregistered node
	node2 := mock.Node()
	alloc := mock.Alloc()
	plan := &structs.Plan{
		NodeAllocation: map[string][]*structs.Allocation{
			node2.ID: []*structs.Allocation{alloc},
		},
	}

	// Attempt to submit a plan
	w := &Worker{srv: s1, logger: s1.logger}
	result, state, err := w.SubmitPlan(plan)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Result should have allocated
	if result == nil {
		t.Fatalf("missing result")
	}

	// Expect no allocation and forced refresh
	if result.AllocIndex != 0 {
		t.Fatalf("Bad: %#v", result)
	}
	if result.RefreshIndex == 0 {
		t.Fatalf("Bad: %#v", result)
	}
	if len(result.NodeAllocation) != 0 {
		t.Fatalf("Bad: %#v", result)
	}

	// Should have an update
	if state == nil {
		t.Fatalf("expected state update")
	}
}
