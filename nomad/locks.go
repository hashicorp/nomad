package nomad

import (
	"fmt"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
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

// CreateVariableLockTTLTimer creates a TTL timer for the given lock. The
// passed ID is expected to be generated via the variable NamespacedID
// function.
func (s *Server) CreateVariableLockTTLTimer(variable structs.VariableEncrypted) {

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
		s.logger.Error("attempting to lock a locked variable: %s", lockID)
	}

	s.lockTTLTimer.Create(lockID, lockTTL, func() {
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
	s.lockTTLTimer.StopAndRemove(lockID)

	// Remove the lock from teh variable
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
		s.logger.Error("variable lock expiration failed",
			"namespace", variable.Namespace, "path", variable.Path,
			"lock_id", lockID, "error", err)
		// This exponential backoff will extend the lock Delay beyond the expected
		// time if there is any raft error, should we make it dependant on the LockDelay?
		time.Sleep((1 << attempt) * 10 * time.Second)
	}
}

func (s *Server) RenewTTLTimer(variable structs.VariableEncrypted) {
	lockID := variable.LockID()
	lock := s.lockTTLTimer.Get(lockID)
	if lock == nil {
		// If this was to happen, there is a sync issue somewhere else.
		s.logger.Error("attempting to renew an unlocked variable: %s", lockID)
		return
	}

	// The create function resets the timer when it exists already, there is no
	// need to provide the release function again.
	s.lockTTLTimer.Create(lockID, variable.Lock.TTL, nil)
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
		s.logger.Error("attempting to renew an unlocked variable: %s", lockID)
		return
	}

	// The create function resets the timer when it exists already, there is no
	// need to provide the release function again.
	s.lockTTLTimer.Create(lockID, variable.Lock.TTL, nil)
}
