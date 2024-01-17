// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package task

import "sync"

// ID is a task ID
type ID = string

type Store interface {
	Set(ID, *Handle)
	Get(ID) (*Handle, bool)
	Del(ID)
}

func NewStore() Store {
	return &store{
		store: make(map[ID]*Handle),
	}
}

type store struct {
	lock  sync.RWMutex
	store map[ID]*Handle
}

func (s *store) Set(id ID, handle *Handle) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.store[id] = handle
}

func (s *store) Get(id ID) (*Handle, bool) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	h, exists := s.store[id]
	return h, exists
}

func (s *store) Del(id ID) {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.store, id)
}
