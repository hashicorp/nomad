// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package checkstore

import (
	"maps"
	"slices"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/serviceregistration/checks"
	"github.com/hashicorp/nomad/client/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// A Shim is used to track the latest check status information, one layer above
// the client persistent store so we can do efficient indexing, etc.
type Shim interface {
	// Set the latest result for a specific check.
	Set(allocID string, result *structs.CheckQueryResult) error

	// List the latest results for a specific allocation.
	List(allocID string) map[structs.CheckID]*structs.CheckQueryResult

	// Difference returns the set of IDs being stored that are not in ids.
	Difference(allocID string, ids []structs.CheckID) []structs.CheckID

	// Remove will remove ids from the cache and persistent store.
	Remove(allocID string, ids []structs.CheckID) error

	// Purge results for a specific allocation.
	Purge(allocID string) error

	// Snapshot returns a copy of the current status of every check indexed by
	// checkID, for use by CheckWatcher.
	Snapshot() map[string]string
}

type shim struct {
	log hclog.Logger

	db state.StateDB

	lock    sync.RWMutex
	current checks.ClientResults
}

// NewStore creates a new store.
func NewStore(log hclog.Logger, db state.StateDB) Shim {
	s := &shim{
		log:     log.Named("check_store"),
		db:      db,
		current: make(checks.ClientResults),
	}
	s.restore()
	return s
}

func (s *shim) restore() {
	s.lock.Lock()
	defer s.lock.Unlock()

	results, err := s.db.GetCheckResults()
	if err != nil {
		s.log.Error("failed to restore health check results", "error", err)
		// may as well continue and let the check observers repopulate - maybe
		// the persistent storage error was transitory
		return
	}

	for id, m := range results {
		s.current[id] = maps.Clone(m)
	}
}

func (s *shim) Set(allocID string, qr *structs.CheckQueryResult) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if _, exists := s.current[allocID]; !exists {
		s.current[allocID] = make(map[structs.CheckID]*structs.CheckQueryResult)
	}

	// lookup existing result
	previous, exists := s.current[allocID][qr.ID]

	// only insert a stub if no result exists yet
	if qr.Status == structs.CheckPending && exists {
		return nil
	}

	s.log.Trace("setting check status", "alloc_id", allocID, "check_id", qr.ID, "status", qr.Status)

	// always keep in-memory shim up to date with latest result
	s.current[allocID][qr.ID] = qr

	// only update persistent store if status changes (optimization)
	// on Client restart restored check results may be outdated but the status
	// is the same as the most recent result
	if !exists || previous.Status != qr.Status {
		if err := s.db.PutCheckResult(allocID, qr); err != nil {
			s.log.Error("failed to set check status", "alloc_id", allocID, "check_id", qr.ID, "error", err)
			return err
		}
	}

	return nil
}

func (s *shim) List(allocID string) map[structs.CheckID]*structs.CheckQueryResult {
	s.lock.RLock()
	defer s.lock.RUnlock()

	m, exists := s.current[allocID]
	if !exists {
		return nil
	}

	return maps.Clone(m)
}

func (s *shim) Purge(allocID string) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	// remove from our map
	delete(s.current, allocID)

	// remove from persistent store
	return s.db.PurgeCheckResults(allocID)
}

func (s *shim) Remove(allocID string, ids []structs.CheckID) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	// remove from cache
	for _, id := range ids {
		delete(s.current[allocID], id)
	}

	// remove from persistent store
	return s.db.DeleteCheckResults(allocID, ids)
}

func (s *shim) Difference(allocID string, ids []structs.CheckID) []structs.CheckID {
	s.lock.Lock()
	defer s.lock.Unlock()

	var remove []structs.CheckID
	for id := range s.current[allocID] {
		if !slices.Contains(ids, id) {
			remove = append(remove, id)
		}
	}

	return remove
}

func (s *shim) Snapshot() map[string]string {
	s.lock.RLock()
	defer s.lock.RUnlock()

	result := make(map[string]string)
	for _, m := range s.current {
		for checkID, status := range m {
			result[string(checkID)] = string(status.Status)
		}
	}
	return result
}
