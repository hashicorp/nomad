package client

import (
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

type heartbeatStop struct {
	lastOk        time.Time
	allocInterval map[string]time.Duration
	allocHookCh   chan *structs.Allocation
	getRunner     func(string) (AllocRunner, error)
	logger        hclog.InterceptLogger
	state         state.StateDB
	shutdownCh    chan struct{}
	lock          *sync.RWMutex
}

func newHeartbeatStop(
	state state.StateDB,
	getRunner func(string) (AllocRunner, error),
	logger hclog.InterceptLogger,
	shutdownCh chan struct{}) *heartbeatStop {

	h := &heartbeatStop{
		allocInterval: make(map[string]time.Duration),
		allocHookCh:   make(chan *structs.Allocation),
		getRunner:     getRunner,
		logger:        logger,
		state:         state,
		shutdownCh:    shutdownCh,
		lock:          &sync.RWMutex{},
	}

	if state != nil {
		lastOk, err := state.GetLastHeartbeatOk()
		if err == nil && !lastOk.IsZero() {
			h.lastOk = lastOk
		}
	}

	return h
}

// allocHook is called after (re)storing a new AllocRunner in the client. It registers the
// allocation to be stopped if the taskgroup is configured appropriately
func (h *heartbeatStop) allocHook(alloc *structs.Allocation) {
	tg := allocTaskGroup(alloc)
	if tg.StopAfterClientDisconnect != nil {
		h.allocHookCh <- alloc
	}
}

// shouldStop is called on a restored alloc to determine if lastOk is sufficiently in the
// past that it should be prevented from restarting
func (h *heartbeatStop) shouldStop(alloc *structs.Allocation) bool {
	tg := allocTaskGroup(alloc)
	if tg.StopAfterClientDisconnect != nil {
		now := time.Now()
		return now.After(h.lastOk.Add(*tg.StopAfterClientDisconnect))
	}
	return false
}

// watch is a loop that checks for allocations that should be stopped. It also manages the
// registration of allocs to be stopped in a single thread.
func (h *heartbeatStop) watch() {
	// If we never manage to successfully contact the server, we want to stop our allocs
	// after duration + start time
	h.lastOk = time.Now()
	stop := make(chan string, 1)
	var now time.Time
	var interval time.Duration
	checkAllocs := false

	for {
		// minimize the interval
		interval = 5 * time.Second
		for _, t := range h.allocInterval {
			if t < interval {
				interval = t
			}
		}

		checkAllocs = false
		timeout := time.After(interval)

		select {
		case allocID := <-stop:
			if err := h.stopAlloc(allocID); err != nil {
				h.logger.Warn("stopping alloc %s on heartbeat timeout failed: %v", allocID, err)
				continue
			}
			delete(h.allocInterval, allocID)

		case alloc := <-h.allocHookCh:
			tg := allocTaskGroup(alloc)
			if tg.StopAfterClientDisconnect != nil {
				h.allocInterval[alloc.ID] = *tg.StopAfterClientDisconnect
			}

		case <-timeout:
			checkAllocs = true

		case <-h.shutdownCh:
			return
		}

		if !checkAllocs {
			continue
		}

		now = time.Now()
		for allocID, d := range h.allocInterval {
			if now.After(h.lastOk.Add(d)) {
				stop <- allocID
			}
		}
	}
}

// setLastOk sets the last known good heartbeat time to the current time, and persists that time to disk
func (h *heartbeatStop) setLastOk() error {
	h.lock.Lock()
	defer h.lock.Unlock()
	t := time.Now()
	h.lastOk = t
	// We may encounter an error here, but want to update the running time
	// unconditionally, since we'll actively terminate stateful tasks if it ages too
	// much. We only use the state value when restarting the client itself after a
	// crash, so it's better to update the runtime value and have the stored value stale
	return h.state.PutLastHeartbeatOk(t)
}

func (h *heartbeatStop) getLastOk() time.Time {
	h.lock.RLock()
	defer h.lock.RUnlock()
	return h.lastOk
}

// stopAlloc actually stops the allocation
func (h *heartbeatStop) stopAlloc(allocID string) error {
	runner, err := h.getRunner(allocID)
	if err != nil {
		return err
	}

	runner.Destroy()
	return nil
}

func allocTaskGroup(alloc *structs.Allocation) *structs.TaskGroup {
	for _, tg := range alloc.Job.TaskGroups {
		if tg.Name == alloc.TaskGroup {
			return tg
		}
	}
	return nil
}
