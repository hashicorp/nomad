package state

import (
	"testing"
)

func TestWatchItems(t *testing.T) {
	// No items returns empty slice
	wi := make(watchItems)
	if items := wi.items(); len(items) != 0 {
		t.Fatalf("expected empty, got: %#v", items)
	}

	// Adding items works
	wi.add(watchItem{table: "foo"})
	wi.add(watchItem{nodeID: "bar"})
	if items := wi.items(); len(items) != 2 {
		t.Fatalf("expected 2 items, got: %#v", items)
	}

	// Adding duplicates auto-dedupes
	wi.add(watchItem{table: "foo"})
	if items := wi.items(); len(items) != 2 {
		t.Fatalf("expected 2 items, got: %#v", items)
	}
}

func TestStateWatch_watch(t *testing.T) {
	watch := newStateWatch()
	notify1 := make(chan struct{}, 1)
	notify2 := make(chan struct{}, 1)
	notify3 := make(chan struct{}, 1)

	// Notifications trigger subscribed channels
	watch.watch(watchItem{table: "foo"}, notify1)
	watch.watch(watchItem{table: "bar"}, notify2)
	watch.watch(watchItem{table: "baz"}, notify3)

	watch.notify(watchItem{table: "foo"}, watchItem{table: "bar"})
	if len(notify1) != 1 {
		t.Fatalf("should notify")
	}
	if len(notify2) != 1 {
		t.Fatalf("should notify")
	}
	if len(notify3) != 0 {
		t.Fatalf("should not notify")
	}
}

func TestStateWatch_stopWatch(t *testing.T) {
	watch := newStateWatch()
	notify := make(chan struct{})

	// First subscribe
	watch.watch(watchItem{table: "foo"}, notify)

	// Unsubscribe stop notifications
	watch.stopWatch(watchItem{table: "foo"}, notify)
	watch.notify(watchItem{table: "foo"})
	if len(notify) != 0 {
		t.Fatalf("should not notify")
	}
}
