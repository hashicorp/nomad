package nomad

import (
	"fmt"
	"io"
	"log"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
)

// The StateStore is responsible for maintaining all the Nomad
// state. It is manipulated by the FSM which maintains consistency
// through the use of Raft. The goals of the StateStore are to provide
// high concurrency for read operations without blocking writes, and
// to provide write availability in the face of reads. EVERY object
// returned as a result of a read against the state store should be
// considered a constant and NEVER modified in place.
type StateStore struct {
	logger *log.Logger
	db     *memdb.MemDB
}

// StateSnapshot is used to provide a point-in-time snapshot
type StateSnapshot struct {
	StateStore
}

// StateRestore is used to optimize the performance when
// restoring state by only using a single large transaction
// instead of thousands of sub transactions
type StateRestore struct {
	txn *memdb.Txn
}

// Abort is used to abort the restore operation
func (s *StateRestore) Abort() {
	s.txn.Abort()
}

// Commit is used to commit the restore operation
func (s *StateRestore) Commit() {
	s.txn.Commit()
}

// IndexEntry is used with the "index" table
// for managing the latest Raft index affecting a table.
type IndexEntry struct {
	Key   string
	Value uint64
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

// Restore is used to optimize the efficiency of rebuilding
// state by minimizing the number of transactions and checking
// overhead.
func (s *StateStore) Restore() (*StateRestore, error) {
	txn := s.db.Txn(true)
	return &StateRestore{txn}, nil
}

// RegisterNode is used to register a node or update a node definition
func (s *StateStore) RegisterNode(index uint64, node *structs.Node) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	// Check if the node already exists
	existing, err := txn.First("nodes", "id", node.ID)
	if err != nil {
		return fmt.Errorf("node lookup failed: %v", err)
	}

	// Setup the indexes correctly
	if existing != nil {
		node.CreateIndex = existing.(*structs.Node).CreateIndex
		node.ModifyIndex = index
	} else {
		node.CreateIndex = index
		node.ModifyIndex = index
	}

	// Insert the node
	if err := txn.Insert("nodes", node); err != nil {
		return fmt.Errorf("node insert failed: %v", err)
	}
	if err := txn.Insert("index", &IndexEntry{"nodes", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	txn.Commit()
	return nil
}

// DeregisterNode is used to deregister a node
func (s *StateStore) DeregisterNode(index uint64, nodeID string) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	// Lookup the node
	existing, err := txn.First("nodes", "id", nodeID)
	if err != nil {
		return fmt.Errorf("node lookup failed: %v", err)
	}
	if existing == nil {
		return fmt.Errorf("node not found")
	}

	// Delete the node
	if err := txn.Delete("nodes", existing); err != nil {
		return fmt.Errorf("node delete failed: %v", err)
	}
	if err := txn.Insert("index", &IndexEntry{"nodes", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	// TODO: Handle the existing allocations, probably need
	// to change their states back to pending and kick the scheduler
	// to force it to move things around

	txn.Commit()
	return nil
}

// UpdateNodeStatus is used to update the status of a node
func (s *StateStore) UpdateNodeStatus(index uint64, nodeID string, status string) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	// Lookup the node
	existing, err := txn.First("nodes", "id", nodeID)
	if err != nil {
		return fmt.Errorf("node lookup failed: %v", err)
	}
	if existing == nil {
		return fmt.Errorf("node not found")
	}

	// Copy the existing node
	existingNode := existing.(*structs.Node)
	copyNode := new(structs.Node)
	*copyNode = *existingNode

	// Update the status in the copy
	copyNode.Status = status
	copyNode.ModifyIndex = index

	// Insert the node
	if err := txn.Insert("nodes", copyNode); err != nil {
		return fmt.Errorf("node update failed: %v", err)
	}
	if err := txn.Insert("index", &IndexEntry{"nodes", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	txn.Commit()
	return nil
}

// GetNodeByID is used to lookup a node by ID
func (s *StateStore) GetNodeByID(nodeID string) (*structs.Node, error) {
	txn := s.db.Txn(false)

	existing, err := txn.First("nodes", "id", nodeID)
	if err != nil {
		return nil, fmt.Errorf("node lookup failed: %v", err)
	}

	if existing != nil {
		return existing.(*structs.Node), nil
	}
	return nil, nil
}

// Nodes returns an iterator over all the nodes
func (s *StateStore) Nodes() (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	// Walk the entire nodes table
	iter, err := txn.Get("nodes", "id")
	if err != nil {
		return nil, err
	}
	return iter, nil
}

// GetIndex finds the matching index value
func (s *StateStore) GetIndex(name string) (uint64, error) {
	txn := s.db.Txn(false)

	// Lookup the first matching index
	out, err := txn.First("index", "id", name)
	if err != nil {
		return 0, err
	}
	if out == nil {
		return 0, nil
	}
	return out.(*IndexEntry).Value, nil
}

// Indexes returns an iterator over all the indexes
func (s *StateStore) Indexes() (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	// Walk the entire nodes table
	iter, err := txn.Get("index", "id")
	if err != nil {
		return nil, err
	}
	return iter, nil
}

// NodeRestore is used to restore a node
func (r *StateRestore) NodeRestore(node *structs.Node) error {
	if err := r.txn.Insert("nodes", node); err != nil {
		return fmt.Errorf("node insert failed: %v", err)
	}
	return nil
}

func (r *StateRestore) IndexRestore(idx *IndexEntry) error {
	if err := r.txn.Insert("index", idx); err != nil {
		return fmt.Errorf("index insert failed: %v", err)
	}
	return nil
}
