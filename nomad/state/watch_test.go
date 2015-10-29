package state

import (
	"testing"
)

func TestWatchItems(t *testing.T) {
	wi := make(watchItems)

	// Adding items works
	wi.add(watchItem{table: "foo"})
	wi.add(watchItem{node: "bar"})
	if len(wi) != 2 {
		t.Fatalf("expected 2 items, got: %#v", wi)
	}

	// Adding duplicates auto-dedupes
	wi.add(watchItem{table: "foo"})
	if len(wi) != 2 {
		t.Fatalf("expected 2 items, got: %#v", wi)
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

	items := make(watchItems)
	items.add(watchItem{table: "foo"})
	items.add(watchItem{table: "bar"})

	watch.notify(items)
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

	items := make(watchItems)
	items.add(watchItem{table: "foo"})
	watch.notify(items)
	if len(notify) != 0 {
		t.Fatalf("should not notify")
	}
}
