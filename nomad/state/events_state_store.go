package state

import (
	"fmt"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
)

// addNodeEvent is a function which wraps upsertNodeEvent
func (s *StateStore) AddNodeEvent(index uint64, nodeID string, event *structs.NodeEvent) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	return s.upsertNodeEvent(index, nodeID, event, txn)
}

// upsertNodeEvent upserts a node event for a respective node. It also maintains
// that only 10 node events are ever stored simultaneously, deleting older
// events once this bound has been reached.
func (s *StateStore) upsertNodeEvent(index uint64, nodeID string, event *structs.NodeEvent, txn *memdb.Txn) error {

	ws := memdb.NewWatchSet()
	node, err := s.NodeByID(ws, nodeID)

	if err != nil {
		return fmt.Errorf("unable to look up nodes by id %+v", err)
	}

	if node == nil {
		return fmt.Errorf("unable to look up nodes by id %s", nodeID)
	}

	event.CreateIndex = index

	nodeEvents := node.NodeEvents

	if len(nodeEvents) >= 10 {
		delta := len(nodeEvents) - 10
		nodeEvents = nodeEvents[delta+1:]
	}
	nodeEvents = append(nodeEvents, event)
	node.NodeEvents = nodeEvents

	txn.Commit()
	return nil
}
