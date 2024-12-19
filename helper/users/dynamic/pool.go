// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package dynamic provides a way of allocating UID/GID to be used by Nomad
// tasks with no associated service users managed by the operating system.
package dynamic

import (
	"errors"
	"math/rand"
	"strconv"
	"sync"

	"github.com/hashicorp/go-set/v3"
	"github.com/hashicorp/nomad/helper"
)

var (
	ErrPoolExhausted = errors.New("users: uid/gid pool exhausted")
	ErrReleaseUnused = errors.New("users: release of unused uid/gid")
	ErrCannotParse   = errors.New("users: unable to parse uid/gid from username")
)

// none indicates no dynamic user
const none = 0

// doNotEnable indicates functionality should be disabled
const doNotEnable = -1

// A UGID is a combination User (UID) and Group (GID). Since Nomad is
// allocating these values together from the same pool it can ensure they are
// always matching values, thus encoding them with one value.
type UGID int

// String returns the string representation of a UGID.
//
// It's just the numbers.
func (id UGID) String() string {
	return strconv.Itoa(int(id))
}

// A Pool is used to manage a reserved set of UID/GID values. A UGID can be
// acquired from the pool and released back to the pool. To support client
// restarts, specific UGIDs can be marked as in-use and can later be released
// back into the pool.
type Pool interface {
	// Restore a UGID currently in use by a Task during a Nomad client restore.
	Restore(UGID)

	// Acquire returns a UGID that is not currently in use.
	Acquire() (UGID, error)

	// Release returns a UGID no longer being used into the pool.
	Release(UGID) error
}

// PoolConfig contains options for creating a new Pool.
type PoolConfig struct {
	// MinUGID is the minimum value for a UGID allocated from the pool.
	MinUGID int

	// MaxUGID is the maximum value for a UGID allocated from the pool.
	MaxUGID int
}

// disable will return true if either min or max is set to Disable (-1),
// indicating the client should not enable the dynamic workload users
// functionality
func (p *PoolConfig) disable() bool {
	return p.MinUGID == doNotEnable || p.MaxUGID == doNotEnable
}

// New creates a Pool with the given PoolConfig options.
func New(opts *PoolConfig) Pool {
	if opts == nil {
		panic("bug: users pool cannot be nil")
	}
	if opts.disable() {
		return new(noopPool)
	}
	if opts.MinUGID < 0 {
		panic("bug: users pool min must be >= 0")
	}
	if opts.MaxUGID < opts.MinUGID {
		panic("bug: users pool max must be >= min")
	}
	// a small but reasonable number of tasks to expect
	const defaultPoolCapacity = 32
	return &pool{
		min:  UGID(opts.MinUGID),
		max:  UGID(opts.MaxUGID),
		lock: new(sync.Mutex),
		used: set.New[UGID](defaultPoolCapacity),
	}
}

// noopPool is an implementation of Pool that does not allow acquiring ugids
type noopPool struct{}

func (*noopPool) Restore(UGID) {}
func (*noopPool) Acquire() (UGID, error) {
	return 0, errors.New("dynamic workload users disabled")
}
func (*noopPool) Release(UGID) error {
	// avoid giving an error if a client is restarted with a new config
	// that disables dynamic workload users but still has a task running
	// making use of one
	return nil
}

type pool struct {
	min UGID
	max UGID

	lock *sync.Mutex
	used *set.Set[UGID]
}

func (p *pool) Restore(id UGID) {
	helper.WithLock(p.lock, func() {
		p.used.Insert(id)
	})
}

func (p *pool) Acquire() (UGID, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	// optimize the case where the pool is exhausted
	if p.used.Size() == int((p.max-p.min)+1) {
		return none, ErrPoolExhausted
	}

	// attempt to select a random ugid
	if id := p.random(); id != none {
		return id, nil
	}

	// slow case where we iterate each id looking for one that is not used
	for id := p.min; id <= p.max; id++ {
		if !p.used.Contains(id) {
			p.used.Insert(id)
			return id, nil
		}
	}

	// we checked for this case up top; if we get here there is a bug
	panic("bug: pool exhausted")
}

// random will attempt to select a random UGID from the pool
func (p *pool) random() UGID {
	// make up to 10 attempts to find a random unused UGID
	// if all 10 attempts fail, return the sentinel indicating as much
	const maxAttempts = 10
	size := int64(p.max - p.min)
	tries := int(min(maxAttempts, size))
	for attempt := 0; attempt < tries; attempt++ {
		id := UGID(rand.Int63n(size)) + p.min
		if p.used.Insert(id) {
			return id
		}
	}
	return none
}

func (p *pool) Release(id UGID) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	if !p.used.Remove(id) {
		return ErrReleaseUnused
	}

	return nil
}
