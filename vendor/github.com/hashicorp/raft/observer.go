package raft

import (
	"sync/atomic"
)

// Observation is sent along the given channel to observers when an event occurs.
type Observation struct {
	Raft *Raft
	Data interface{}
}

// LeaderObservation is used for the data when leadership changes.
type LeaderObservation struct {
	leader string
}

// nextObserverId is used to provide a unique ID for each observer to aid in
// deregistration.
var nextObserverId uint64

// FilterFn is a function that can be registered in order to filter observations
// by returning false.
type FilterFn func(o *Observation) bool

// Observer describes what to do with a given observation.
type Observer struct {
	// channel receives observations.
	channel chan Observation

	// blocking, if true, will cause Raft to block when sending an observation
	// to this observer. This should generally be set to false.
	blocking bool

	// filter will be called to determine if an observation should be sent to
	// the channel.
	filter FilterFn

	// id is the ID of this observer in the Raft map.
	id uint64

	// numObserved and numDropped are performance counters for this observer.
	numObserved uint64
	numDropped  uint64
}

// Create a new observer with the specified channel, blocking behavior, and
// filter (filter can be nil).
func NewObserver(channel chan Observation, blocking bool, filter FilterFn) *Observer {
	return &Observer{
		channel:  channel,
		blocking: blocking,
		filter:   filter,
		id:       atomic.AddUint64(&nextObserverId, 1),
	}
}

// GetNumObserved returns the number of observations.
func (or *Observer) GetNumObserved() uint64 {
	return atomic.LoadUint64(&or.numObserved)
}

// GetNumDropped returns the number of dropped observations due to blocking.
func (or *Observer) GetNumDropped() uint64 {
	return atomic.LoadUint64(&or.numDropped)
}

// Register a new observer.
func (r *Raft) RegisterObserver(or *Observer) {
	r.observersLock.Lock()
	defer r.observersLock.Unlock()
	r.observers[or.id] = or
}

// Deregister an observer.
func (r *Raft) DeregisterObserver(or *Observer) {
	r.observersLock.Lock()
	defer r.observersLock.Unlock()
	delete(r.observers, or.id)
}

// Send an observation to every observer.
func (r *Raft) observe(o interface{}) {
	// In general observers should not block. But in any case this isn't
	// disastrous as we only hold a read lock, which merely prevents
	// registration / deregistration of observers.
	r.observersLock.RLock()
	defer r.observersLock.RUnlock()
	for _, or := range r.observers {
		// It's wasteful to do this in the loop, but for the common case
		// where there are no observers we won't create any objects.
		ob := Observation{Raft: r, Data: o}
		if or.filter != nil && !or.filter(&ob) {
			continue
		}
		if or.channel == nil {
			continue
		}
		if or.blocking {
			or.channel <- ob
			atomic.AddUint64(&or.numObserved, 1)
		} else {
			select {
			case or.channel <- ob:
				atomic.AddUint64(&or.numObserved, 1)
			default:
				atomic.AddUint64(&or.numDropped, 1)
			}
		}
	}
}
