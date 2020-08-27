package state

import (
	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/event"
	"github.com/hashicorp/nomad/nomad/structs"
)

// NodeEvent represents a NodeEvent change on a given Node.
type NodeEvent struct {
	Message string
	NodeID  string
}

// NNodeDrainEvent holds information related to a Node Drain
type NodeDrainEvent struct {
	NodeID        string
	Allocs        []string
	DrainStrategy structs.DrainStrategy
	Message       string
}

func (s *StateStore) NodeEventsFromChanges(tx ReadTxn, changes Changes) ([]event.Event, error) {
	var events []event.Event

	var nodeChanges map[string]*memdb.Change

	markNode := func(node string, nodeChange *memdb.Change) {
		if nodeChanges == nil {
			nodeChanges = make(map[string]*memdb.Change)
		}
		ch := nodeChanges[node]
		if ch == nil {
			nodeChanges[node] = nodeChange
		}
	}

	for _, change := range changes.Changes {
		switch change.Table {
		case "nodes":
			nRaw := change.After
			if change.After == nil {
				nRaw = change.Before
			}
			n := nRaw.(*structs.Node)
			changeCopy := change
			markNode(n.ID, &changeCopy)
		}
	}

	for node, change := range nodeChanges {
		if change != nil && change.Deleted() {
			// TODO Node delete event
			continue
		}

		ne, err := s.statusEventsForNode(tx, node, change)
		if err != nil {
			return nil, err
		}
		// Rebuild node node events
		events = append(events, ne...)
	}
	return events, nil
}

func (s *StateStore) statusEventsForNode(tx ReadTxn, node string, change *memdb.Change) ([]event.Event, error) {
	events := []event.Event{}
	if change.Created() {
		n := change.After.(*structs.Node)
		for _, e := range n.Events {
			nodeEvent := NodeEvent{Message: e.Message, NodeID: node}
			e := event.Event{
				Topic:   "NodeEvent",
				Key:     node,
				Payload: nodeEvent,
			}
			events = append(events, e)
		}
	} else if change.Updated() {
		nbefore := change.Before.(*structs.Node)
		nafter := change.After.(*structs.Node)
		newEvents := s.newNodeEvents(nbefore.Events, nafter.Events)
		for _, e := range newEvents {
			if s.isNodeDrainEvent(nbefore, nafter, newEvents) {
				allocs, err := s.AllocsByNodeTx(tx, node)
				if err != nil {
					return []event.Event{}, err
				}
				var allocIDs []string
				for _, a := range allocs {
					allocIDs = append(allocIDs, a.ID)
				}

				nde := NodeDrainEvent{
					NodeID:        node,
					DrainStrategy: *nafter.DrainStrategy,
					Allocs:        allocIDs,
					Message:       e.Message,
				}
				e := event.Event{
					Topic:   "NodeEvent",
					Key:     node,
					Payload: nde,
				}
				events = append(events, e)
			} else {

				ne := NodeEvent{
					Message: e.Message,
					NodeID:  node,
				}
				e := event.Event{
					Topic:   "NodeEvent",
					Key:     node,
					Payload: ne,
				}
				events = append(events, e)
			}
		}
	}

	return events, nil
}

func (s *StateStore) newNodeEvents(before, after []*structs.NodeEvent) []*structs.NodeEvent {
	events := []*structs.NodeEvent{}
	if len(before) == len(after) {
		return nil
	}

	for _, e := range after {
		found := false
		for _, be := range before {
			if e.String() == be.String() {
				found = true
				break
			}
		}
		if !found {
			events = append(events, e)
		}
	}
	return events
}

func (s *StateStore) isNodeDrainEvent(before, after *structs.Node, newEvents []*structs.NodeEvent) bool {
	if before.Drain != after.Drain {
		return true
	}

	for _, e := range newEvents {
		if e.Subsystem == structs.NodeEventSubsystemDrain {
			return true
		}
	}
	return false
}

func (s *StateStore) AllocsByNodeTx(tx ReadTxn, node string) ([]*structs.Allocation, error) {
	iter, err := tx.Get("allocs", "node_prefix", node)
	if err != nil {
		return nil, err
	}

	var out []*structs.Allocation
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		out = append(out, raw.(*structs.Allocation))
	}
	return out, nil
}
