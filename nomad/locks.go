// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

var (
	errTimerNotFound = errors.New("lock doesn't have a running timer ")
)

// restoreLockTTLTimers iterates the stored variables and creates a lock TTL
// timer for each variable lock. This is used during leadership establishment
// to populate the in-memory timer.
func (s *Server) restoreLockTTLTimers() error {

	varIterator, err := s.fsm.State().Variables(nil)
	if err != nil {
		return fmt.Errorf("failed to list variables for lock TTL restore: %v", err)
	}

	// Iterate the variables, identifying each one that is associated to a lock
	// and adding a TTL timer for each.
	for varInterface := varIterator.Next(); varInterface != nil; varInterface = varIterator.Next() {
		if realVar, ok := varInterface.(*structs.VariableEncrypted); ok && realVar.IsLock() {

			// The variable will be modified in order to show that it no longer
			// is held. We therefore need to ensure we perform modifications on
			// a copy.
			s.CreateVariableLockTTLTimer(realVar.Copy())
		}
	}

	return nil
}

// CreateVariableLockTTLTimer creates a TTL timer for the given lock.
// It is in charge of integrating the delay after the TTL expires.
func (s *Server) CreateVariableLockTTLTimer(variable structs.VariableEncrypted) {
	s.logger.Debug("locks: adding lock", "namespace", variable.Namespace, "path", variable.Path)
	// Adjust the given TTL by multiplier of 2. This is done to give a client a
	// grace period and to compensate for network and processing delays. The
	// contract is that a variable lock is not expired before the TTL expires,
	// but there is no explicit promise about the upper bound so this is
	// allowable.
	lockTTL := variable.Lock.TTL * 2

	// The lock ID is used a couple of times, so grab this now.
	lockID := variable.LockID()

	lock := s.lockTTLTimer.Get(lockID)
	if lock != nil {
		// If this was to happen, there is a sync issue somewhere else
		s.logger.Error("attempting to recreate existing lock: %s", lockID)
		return
	}

	s.lockTTLTimer.Create(lockID, lockTTL, func() {
		s.logger.Debug("locks: lock TTL expired, starting delay",
			"namespace", variable.Namespace, "path", variable.Path, "ttl", variable.Lock.TTL)
		s.lockTTLTimer.StopAndRemove(lockID)

		_ = time.AfterFunc(variable.Lock.LockDelay, func() {
			s.invalidateVariableLock(variable)
		})
	})
}

// invalidateVariableLock exponentially tries to update Nomad's state to remove
// the lock ID from the variable. This can be used when a variable lock's TTL
// has expired.
func (s *Server) invalidateVariableLock(variable structs.VariableEncrypted) {
	lockID := variable.LockID()

	s.logger.Debug("locks: lock delay expired, removing lock",
		"namespace", variable.Namespace, "path", variable.Path)

	// Remove the lock from the variable
	variable.VariableMetadata.Lock = nil

	args := structs.VarApplyStateRequest{
		Op: structs.VarOpLockRelease,
		Var: &structs.VariableEncrypted{
			VariableMetadata: variable.VariableMetadata,
		},
		WriteRequest: structs.WriteRequest{
			Region:    s.Region(),
			Namespace: variable.Namespace,
		},
	}

	// Retry with exponential backoff to remove the lock
	for attempt := 0; attempt < maxAttemptsToRaftApply; attempt++ {
		_, _, err := s.raftApply(structs.VarApplyStateRequestType, args)
		if err == nil {
			return
		}

		s.logger.Error("lock expiration failed",
			"namespace", variable.Namespace, "path", variable.Path,
			"lock_id", lockID, "error", err)
		// This exponential backoff will extend the lock Delay beyond the expected
		// time if there is any raft error, should we make it dependant on the LockDelay?
		time.Sleep((1 << attempt) * 10 * time.Second)
	}
}

func (s *Server) RenewTTLTimer(variable structs.VariableEncrypted) error {
	lockID := variable.LockID()

	s.logger.Debug("locks: renewing the lock",
		"namespace", variable.Namespace, "path", variable.Path, "ttl", variable.Lock.TTL)

	lock := s.lockTTLTimer.Get(lockID)
	if lock == nil {
		return errTimerNotFound
	}

	// Adjust the given TTL by multiplier of 2. This is done to give a client a
	// grace period and to compensate for network and processing delays. The
	// contract is that a variable lock is not expired before the TTL expires,
	// but there is no explicit promise about the upper bound so this is
	// allowable.
	lockTTL := variable.Lock.TTL * 2

	// The create function resets the timer when it exists already, there is no
	// need to provide the release function again.
	s.lockTTLTimer.Create(lockID, lockTTL, nil)
	return nil
}

// RemoveVariableLockTTLTimer creates a TTL timer for the given lock. The
// passed ID is expected to be generated via the variable NamespacedID
// function.
func (s *Server) RemoveVariableLockTTLTimer(variable structs.VariableEncrypted) {

	// The lock ID is used a couple of times, so grab this now.
	lockID := variable.LockID()

	lock := s.lockTTLTimer.Get(lockID)
	if lock == nil {
		// If this was to happen, there is a sync issue somewhere else.
		s.logger.Error("attempting to removed missing lock: %s", lockID)
		return
	}

	s.lockTTLTimer.StopAndRemove(lockID)
}
