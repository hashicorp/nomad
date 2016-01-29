package nomad

import (
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

func testBlockedEvals(t *testing.T) (*BlockedEvals, *EvalBroker) {
	broker := testBroker(t, 0)
	broker.SetEnabled(true)
	blocked := NewBlockedEvals(broker)
	blocked.SetEnabled(true)
	return blocked, broker
}

func TestBlockedEvals_Block_Disabled(t *testing.T) {
	blocked, _ := testBlockedEvals(t)
	blocked.SetEnabled(false)

	// Create an escaped eval and add it to the blocked tracker.
	e := mock.Eval()
	e.Status = structs.EvalStatusBlocked
	e.EscapedComputedClass = true
	blocked.Block(e)

	// Verify block did nothing
	blockedStats := blocked.Stats()
	if blockedStats.TotalEscaped != 0 || blockedStats.TotalCaptured != 0 {
		t.Fatalf("bad: %#v", blockedStats)
	}

}

func TestBlockedEvals_UnblockEscaped(t *testing.T) {
	blocked, broker := testBlockedEvals(t)

	// Create an escaped eval and add it to the blocked tracker.
	e := mock.Eval()
	e.Status = structs.EvalStatusBlocked
	e.EscapedComputedClass = true
	blocked.Block(e)

	// Verify block caused the eval to be tracked
	blockedStats := blocked.Stats()
	if blockedStats.TotalEscaped != 1 {
		t.Fatalf("bad: %#v", blockedStats)
	}

	blocked.Unblock(123)

	// Verify Unblock caused an enqueue
	brokerStats := broker.Stats()
	if brokerStats.TotalReady != 1 {
		t.Fatalf("bad: %#v", brokerStats)
	}

	// Verify Unblock updates the stats
	blockedStats = blocked.Stats()
	if blockedStats.TotalEscaped != 0 {
		t.Fatalf("bad: %#v", blockedStats)
	}
}

func TestBlockedEvals_UnblockEligible(t *testing.T) {
	blocked, broker := testBlockedEvals(t)

	// Create a blocked eval that is eligible on a specific node class and add
	// it to the blocked tracker.
	e := mock.Eval()
	e.Status = structs.EvalStatusBlocked
	e.EligibleClasses = map[uint64]struct{}{123: struct{}{}}
	blocked.Block(e)

	// Verify block caused the eval to be tracked
	blockedStats := blocked.Stats()
	if blockedStats.TotalCaptured != 1 {
		t.Fatalf("bad: %#v", blockedStats)
	}

	blocked.Unblock(123)

	// Verify Unblock caused an enqueue
	brokerStats := broker.Stats()
	if brokerStats.TotalReady != 1 {
		t.Fatalf("bad: %#v", brokerStats)
	}

	// Verify Unblock updates the stats
	blockedStats = blocked.Stats()
	if blockedStats.TotalCaptured != 0 {
		t.Fatalf("bad: %#v", blockedStats)
	}
}

func TestBlockedEvals_UnblockIneligible(t *testing.T) {
	blocked, broker := testBlockedEvals(t)

	// Create a blocked eval that is ineligible on a specific node class and add
	// it to the blocked tracker.
	e := mock.Eval()
	e.Status = structs.EvalStatusBlocked
	e.IneligibleClasses = map[uint64]struct{}{123: struct{}{}}
	blocked.Block(e)

	// Verify block caused the eval to be tracked
	blockedStats := blocked.Stats()
	if blockedStats.TotalCaptured != 1 && blockedStats.TotalEscaped != 0 {
		t.Fatalf("bad: %#v", blockedStats)
	}

	// Should do nothing
	blocked.Unblock(123)

	// Verify Unblock didn't cause an enqueue
	brokerStats := broker.Stats()
	if brokerStats.TotalReady != 0 {
		t.Fatalf("bad: %#v", brokerStats)
	}

	// Verify Unblock updates the stats
	blockedStats = blocked.Stats()
	if blockedStats.TotalCaptured != 1 {
		t.Fatalf("bad: %#v", blockedStats)
	}
}

func TestBlockedEvals_UnblockUnknown(t *testing.T) {
	blocked, broker := testBlockedEvals(t)

	// Create a blocked eval that is ineligible on a specific node class and add
	// it to the blocked tracker.
	e := mock.Eval()
	e.Status = structs.EvalStatusBlocked
	e.EligibleClasses = map[uint64]struct{}{123: struct{}{}}
	e.IneligibleClasses = map[uint64]struct{}{456: struct{}{}}
	blocked.Block(e)

	// Verify block caused the eval to be tracked
	blockedStats := blocked.Stats()
	if blockedStats.TotalCaptured != 1 && blockedStats.TotalEscaped != 0 {
		t.Fatalf("bad: %#v", blockedStats)
	}

	// Should unblock because the eval hasn't seen this node class.
	blocked.Unblock(789)

	// Verify Unblock didn't cause an enqueue
	brokerStats := broker.Stats()
	if brokerStats.TotalReady != 1 {
		t.Fatalf("bad: %#v", brokerStats)
	}

	// Verify Unblock updates the stats
	blockedStats = blocked.Stats()
	if blockedStats.TotalCaptured != 0 {
		t.Fatalf("bad: %#v", blockedStats)
	}
}
