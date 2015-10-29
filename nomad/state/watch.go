package state

import (
	"sync"
)

// watchItem describes the scope of a watch. It is used to provide a uniform
// input for subscribe/unsubscribe and notification firing.
type watchItem struct {
	alloc     string
	allocNode string
	eval      string
	job       string
	node      string
	table     string
}

// watchItems is a helper used to construct a set of watchItems. It deduplicates
// the items as they are added using map keys.
type watchItems map[watchItem]struct{}

func newWatchItems(items ...watchItem) watchItems {
	wi := make(watchItems)
	for _, item := range items {
		wi.add(item)
	}
	return wi
}

// add adds an item to the watch set.
func (w watchItems) add(wi watchItem) {
	w[wi] = struct{}{}
}

// stateWatch holds shared state for watching updates. This is
// outside of StateStore so it can be shared with snapshots.
type stateWatch struct {
	items map[watchItem]*NotifyGroup
	l     sync.Mutex
}

// newStateWatch creates a new stateWatch for change notification.
func newStateWatch() *stateWatch {
	return &stateWatch{
		items: make(map[watchItem]*NotifyGroup),
	}
}

// watch subscribes a channel to the given watch item.
func (w *stateWatch) watch(wi watchItem, ch chan struct{}) {
	w.l.Lock()
	defer w.l.Unlock()

	grp, ok := w.items[wi]
	if !ok {
		grp = new(NotifyGroup)
		w.items[wi] = grp
	}
	grp.Wait(ch)
}

// stopWatch unsubscribes a channel from the given watch item.
func (w *stateWatch) stopWatch(wi watchItem, ch chan struct{}) {
	w.l.Lock()
	defer w.l.Unlock()

	if grp, ok := w.items[wi]; ok {
		grp.Clear(ch)
		if grp.Empty() {
			delete(w.items, wi)
		}
	}
}

// notify is used to fire notifications on the given watch items.
func (w *stateWatch) notify(items watchItems) {
	w.l.Lock()
	defer w.l.Unlock()

	for wi, _ := range items {
		if grp, ok := w.items[wi]; ok {
			grp.Notify()
		}
	}
}
