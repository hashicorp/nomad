package nomad

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

var (
	defaultSched = []string{
		structs.JobTypeService,
		structs.JobTypeBatch,
	}
)

func testBroker(t *testing.T, timeout time.Duration) *EvalBroker {
	if timeout == 0 {
		timeout = DefaultNackTimeout
	}
	b, err := NewEvalBroker(timeout)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	return b
}

func TestEvalBroker_Enqueue_Dequeue_Nack_Ack(t *testing.T) {
	b := testBroker(t, 0)

	// Enqueue, but broker is disabled!
	eval := mockEval()
	err := b.Enqueue(eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify nothing was done
	stats := b.Stats()
	if stats.TotalReady != 0 {
		t.Fatalf("bad: %#v", stats)
	}

	// Enable the broker, and enqueue
	b.SetEnabled(true)
	err = b.Enqueue(eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify enqueue is done
	stats = b.Stats()
	if stats.TotalReady != 1 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.ByScheduler[eval.Type].Ready != 1 {
		t.Fatalf("bad: %#v", stats)
	}

	// Dequeue should work
	out, err := b.Dequeue(defaultSched, time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != eval {
		t.Fatalf("bad : %#v", out)
	}

	// Check the stats
	stats = b.Stats()
	if stats.TotalReady != 0 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.TotalUnacked != 1 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.ByScheduler[eval.Type].Ready != 0 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.ByScheduler[eval.Type].Unacked != 1 {
		t.Fatalf("bad: %#v", stats)
	}

	// Nack back into the queue
	err = b.Nack(eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check the stats
	stats = b.Stats()
	if stats.TotalReady != 1 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.TotalUnacked != 0 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.ByScheduler[eval.Type].Ready != 1 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.ByScheduler[eval.Type].Unacked != 0 {
		t.Fatalf("bad: %#v", stats)
	}

	// Dequeue should work again
	out2, err := b.Dequeue(defaultSched, time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out2 != eval {
		t.Fatalf("bad : %#v", out2)
	}

	// Ack finally
	err = b.Ack(eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check the stats
	stats = b.Stats()
	if stats.TotalReady != 0 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.TotalUnacked != 0 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.ByScheduler[eval.Type].Ready != 0 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.ByScheduler[eval.Type].Unacked != 0 {
		t.Fatalf("bad: %#v", stats)
	}
}

func TestEvalBroker_Enqueue_Disable(t *testing.T) {
	b := testBroker(t, 0)

	// Enqueue
	eval := mockEval()
	b.SetEnabled(true)
	err := b.Enqueue(eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Flush via SetEnabled
	b.SetEnabled(false)

	// Check the stats
	stats := b.Stats()
	if stats.TotalReady != 0 {
		t.Fatalf("bad: %#v", stats)
	}
	if stats.TotalUnacked != 0 {
		t.Fatalf("bad: %#v", stats)
	}
	if _, ok := stats.ByScheduler[eval.Type]; ok {
		t.Fatalf("bad: %#v", stats)
	}
}

func TestEvalBroker_Dequeue_Timeout(t *testing.T) {
	b := testBroker(t, 0)
	b.SetEnabled(true)

	start := time.Now()
	out, err := b.Dequeue(defaultSched, 5*time.Millisecond)
	end := time.Now()

	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("unexpected: %#v", out)
	}

	if diff := end.Sub(start); diff < 5*time.Millisecond {
		t.Fatalf("bad: %#v", diff)
	}
}

// Ensure higher priority dequeued first
func TestEvalBroker_Dequeue_Priority(t *testing.T) {
	b := testBroker(t, 0)
	b.SetEnabled(true)

	eval1 := mockEval()
	eval1.Priority = 10
	b.Enqueue(eval1)

	eval2 := mockEval()
	eval2.Priority = 30
	b.Enqueue(eval2)

	eval3 := mockEval()
	eval3.Priority = 20
	b.Enqueue(eval3)

	out1, _ := b.Dequeue(defaultSched, time.Second)
	if out1 != eval2 {
		t.Fatalf("bad: %#v", out1)
	}

	out2, _ := b.Dequeue(defaultSched, time.Second)
	if out2 != eval3 {
		t.Fatalf("bad: %#v", out2)
	}

	out3, _ := b.Dequeue(defaultSched, time.Second)
	if out3 != eval1 {
		t.Fatalf("bad: %#v", out3)
	}
}

// Ensure fairness between schedulers
func TestEvalBroker_Dequeue_Fairness(t *testing.T) {
	b := testBroker(t, 0)
	b.SetEnabled(true)
	NUM := 100

	for i := 0; i < NUM; i++ {
		eval1 := mockEval()
		if i < (NUM / 2) {
			eval1.Type = structs.JobTypeService
		} else {
			eval1.Type = structs.JobTypeBatch
		}
		b.Enqueue(eval1)
	}

	counter := 0
	for i := 0; i < NUM; i++ {
		out1, _ := b.Dequeue(defaultSched, time.Second)

		switch out1.Type {
		case structs.JobTypeService:
			if counter < 0 {
				counter = 0
			}
			counter += 1
		case structs.JobTypeBatch:
			if counter > 0 {
				counter = 0
			}
			counter -= 1
		}

		// The odds are less than 1/1024 that
		// we see the same sequence 10 times in a row
		if counter >= 10 || counter <= -10 {
			t.Fatalf("unlikely sequence: %d", counter)
		}
	}
}

// Ensure we get unblocked
func TestEvalBroker_Dequeue_Blocked(t *testing.T) {
	b := testBroker(t, 0)
	b.SetEnabled(true)

	// Start with a blocked dequeue
	outCh := make(chan *structs.Evaluation, 1)
	go func() {
		start := time.Now()
		out, err := b.Dequeue(defaultSched, time.Second)
		end := time.Now()
		outCh <- out
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if d := end.Sub(start); d < 5*time.Millisecond {
			t.Fatalf("bad: %v", d)
		}
	}()

	// Wait for a bit
	time.Sleep(5 * time.Millisecond)

	// Enqueue
	eval := mockEval()
	err := b.Enqueue(eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure dequeue
	select {
	case out := <-outCh:
		if out != eval {
			t.Fatalf("bad: %v", out)
		}
	case <-time.After(time.Second):
		t.Fatalf("timeout")
	}
}

// Ensure we nack in a timely manner
func TestEvalBroker_Nack_Timeout(t *testing.T) {
	b := testBroker(t, 5*time.Millisecond)
	b.SetEnabled(true)

	// Enqueue
	eval := mockEval()
	err := b.Enqueue(eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Dequeue
	out, err := b.Dequeue(defaultSched, time.Second)
	start := time.Now()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != eval {
		t.Fatalf("bad: %v", out)
	}

	// Dequeue, should block on Nack timer
	out, err = b.Dequeue(defaultSched, time.Second)
	end := time.Now()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != eval {
		t.Fatalf("bad: %v", out)
	}

	// Check the nack timer
	if diff := end.Sub(start); diff < 5*time.Millisecond {
		t.Fatalf("bad: %#v", diff)
	}
}
