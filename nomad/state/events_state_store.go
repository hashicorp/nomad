package state

import (
	"fmt"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
)

// addNodeEvent is a function which wraps upsertNodeEvent
func (s *StateStore) AddNodeEvent(index uint64, node *structs.Node, events []*structs.NodeEvent) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	err := s.upsertNodeEvents(index, node, events, txn)
	txn.Commit()
	return err
}

// upsertNodeEvent upserts a node event for a respective node. It also maintains
// that only 10 node events are ever stored simultaneously, deleting older
// events once this bound has been reached.
func (s *StateStore) upsertNodeEvents(index uint64, node *structs.Node, events []*structs.NodeEvent, txn *memdb.Txn) error {

	// Copy the existing node
	copyNode := new(structs.Node)
	*copyNode = *node

	nodeEvents := node.NodeEvents

	for _, e := range events {
		e.CreateIndex = index
		e.ModifyIndex = index

		// keep node events pruned to below 10 simultaneously
		if len(nodeEvents) >= 10 {
			delta := len(nodeEvents) - 10
			nodeEvents = nodeEvents[delta+1:]
		}
		nodeEvents = append(nodeEvents, e)
		copyNode.NodeEvents = nodeEvents
	}

	// Insert the node
	if err := txn.Insert("nodes", copyNode); err != nil {
		return fmt.Errorf("node update failed: %v", err)
	}
	if err := txn.Insert("index", &IndexEntry{"nodes", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return nil
}
