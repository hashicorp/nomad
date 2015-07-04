package nomad

import (
	"fmt"
	"io"
	"log"

	"github.com/hashicorp/go-memdb"
)

// The StateStore is responsible for maintaining all the Nomad
// state. It is manipulated by the FSM which maintains consistency
// through the use of Raft. The goals of the StateStore are to provide
// high concurrency for read operations without blocking writes, and
// to provide write availability in the face of reads.
type StateStore struct {
	logger *log.Logger
	db     *memdb.MemDB
}

// StateSnapshot is used to provide a point-in-time snapshot
type StateSnapshot struct {
	StateStore
}

// NewStateStore is used to create a new state store
func NewStateStore(logOutput io.Writer) (*StateStore, error) {
	// Create the MemDB
	db, err := memdb.NewMemDB(stateStoreSchema())
	if err != nil {
		return nil, fmt.Errorf("state store setup failed: %v", err)
	}

	// Create the state store
	s := &StateStore{
		logger: log.New(logOutput, "", log.LstdFlags),
		db:     db,
	}
	return s, nil
}

// Snapshot is used to create a point in time snapshot. Because
// we use MemDB, we just need to snapshot the state of the underlying
// database.
func (s *StateStore) Snapshot() (*StateSnapshot, error) {
	snap := &StateSnapshot{
		StateStore: StateStore{
			logger: s.logger,
			db:     s.db.Snapshot(),
		},
	}
	return snap, nil
}
