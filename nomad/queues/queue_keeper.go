// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queues

import (
	"iter"
	"maps"
	"sync"

	"github.com/hashicorp/nomad/nomad/queues/queue"
)

// newQueueKeeper makes a new queue keeper for you.
func newQueueKeeper() *queueKeeper {
	return &queueKeeper{
		m: make(map[string]qEntry),
	}
}

// qEntry tracks whether the queue was made with the global config,
// according to the caller.
type qEntry struct {
	q queue.Queue

	// TODO: we may want this later if we allow all-pools config, where it
	// becomes opt-out per pool, rather than only opt-in, but today it is
	// not used in practice.
	isGlobal bool
}

// queueKeeper keeps track of active queues.
type queueKeeper struct {
	m   map[string]qEntry
	mut sync.RWMutex
}

// Set stores the queue by pool name. Callers are expected to set isGlobalConf
// if the queue was configured using the cluster-wide default.
func (qk *queueKeeper) Set(pool string, q queue.Queue, isGlobalConf bool) {
	qk.mut.Lock()
	defer qk.mut.Unlock()
	qk.m[pool] = qEntry{q, isGlobalConf}
}

// Get gets the pool's queue by name.
func (qk *queueKeeper) Get(pool string) (queue.Queue, bool) {
	qk.mut.RLock()
	defer qk.mut.RUnlock()
	e, ok := qk.m[pool]
	return e.q, ok
}

// Stop runs Stop() on the pool's queue, if it exists.
func (qk *queueKeeper) Stop(pool string) {
	qk.mut.Lock()
	defer qk.mut.Unlock()
	if e, ok := qk.m[pool]; ok {
		e.q.Stop()
	}
}

// Delete drops a queue from the keeper. Probably should run Stop() first.
func (qk *queueKeeper) Delete(pool string) {
	qk.mut.Lock()
	defer qk.mut.Unlock()
	delete(qk.m, pool)
}

// Wipe runs Stop on all queues and then discards them.
func (qk *queueKeeper) Wipe() {
	qk.mut.Lock()
	defer qk.mut.Unlock()
	for _, e := range qk.m {
		e.q.Stop()
	}
	qk.m = make(map[string]qEntry)
}

// Iter iterates over all queues.
func (qk *queueKeeper) Iter() iter.Seq2[string, queue.Queue] {
	return qk.iter(false)
}

// IterGlobal iterates over only the queues using the global config.
func (qk *queueKeeper) IterGlobal() iter.Seq2[string, queue.Queue] {
	return qk.iter(true)
}

func (qk *queueKeeper) iter(globalOnly bool) iter.Seq2[string, queue.Queue] {
	return func(yield func(string, queue.Queue) bool) {
		// copy the map so callers can modify their &Queues while iterating.
		qk.mut.RLock()
		cp := maps.Clone(qk.m)
		qk.mut.RUnlock()

		for p, e := range cp {
			if globalOnly && !e.isGlobal {
				continue
			}
			if !yield(p, e.q) {
				return
			}
		}
	}
}
