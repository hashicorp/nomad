// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

// Package anonymous provdies a way of allocating UID/GID to be used by Nomad
// tasks with no associated service user in the operating system.
package anonymous

import (
	"errors"
	"math/rand"
	"strconv"
	"sync"

	"github.com/hashicorp/go-set/v2"
	"github.com/hashicorp/nomad/helper"
)

var (
	ErrPoolExhausted = errors.New("anonymous: user credentials exhausted")
	ErrReleaseUnused = errors.New("anonymous: release of unused user credentials")
)

// none indicates no anonymous user
const none = 0

// A UGID is a combination User (UID) and Group (GID). Since Nomad is allocating
// these values together from the same pool it can ensure they are always
// matching values, thus encoding them with one value.
type UGID int

func (id UGID) String() string {
	return strconv.Itoa(int(id))
}

type Pool interface {
	// Restore a UGID currently in use by a Task during a Nomad client restore.
	Restore(UGID)

	// Acquire returns a UGID that is not currently in use.
	Acquire() (UGID, error)

	// Release returns a UGID no longer being used into the pool.
	Release(UGID) error
}

// New creates a UGID pool for the UID/GID range from low to high, inclusive.
//
// Typically this should be a large range so as to decrease the likelyhood of
// rapid UID/GID reuse.
func New(low, high UGID) Pool {
	return &pool{
		low:  low,
		high: high,
		lock: new(sync.Mutex),
		used: set.New[UGID](32),
	}
}

type pool struct {
	low  UGID
	high UGID

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
	if p.used.Size() == int((p.high-p.low)+1) {
		return none, ErrPoolExhausted
	}

	// attempt to select a random ugid
	if id := p.random(); id != none {
		return id, nil
	}

	// slow case where we iterate each id looking for one that is not used
	for id := p.low; id <= p.high; id++ {
		if !p.used.Contains(id) {
			p.used.Insert(id)
			return id, nil
		}
	}

	panic("bug: pool exhausted ugids")
}

func (p *pool) random() UGID {
	const maxAttempts = 10
	size := int64(p.high - p.low)
	tries := int(min(maxAttempts, size))
	for attempt := 0; attempt < tries; attempt++ {
		id := UGID(rand.Int63n(size)) + p.low
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
