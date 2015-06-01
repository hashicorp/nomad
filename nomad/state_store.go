package nomad

import (
	"io"
	"log"
)

// The StateStore is responsible for maintaining all the Consul
// state. It is manipulated by the FSM which maintains consistency
// through the use of Raft. The goals of the StateStore are to provide
// high concurrency for read operations without blocking writes, and
// to provide write availability in the face of reads.
type StateStore struct {
	logger *log.Logger
}

// StateSnapshot is used to provide a point-in-time snapshot
type StateSnapshot struct {
	store *StateStore
}

// Close is used to abort the transaction and allow for cleanup
func (s *StateSnapshot) Close() error {
	return nil
}

// NewStateStore is used to create a new state store
func NewStateStore(logOutput io.Writer) (*StateStore, error) {
	s := &StateStore{
		logger: log.New(logOutput, "", log.LstdFlags),
	}
	return s, nil
}

// Close is used to safely shutdown the state store
func (s *StateStore) Close() error {
	return nil
}

// Snapshot is used to create a point in time snapshot
func (s *StateStore) Snapshot() (*StateSnapshot, error) {
	return nil, nil
}
