package state

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/structs"
)

// NodeDeregisterEventFromChanges generates a NodeDeregistrationEvent from a set
// of transaction changes.
func NodeDeregisterEventFromChanges(tx ReadTxn, changes Changes) (*structs.Events, error) {
	var events []structs.Event
	for _, change := range changes.Changes {
		switch change.Table {
		case "nodes":
			before, ok := change.Before.(*structs.Node)
			if !ok {
				return nil, fmt.Errorf("transaction change was not a Node")
			}

			event := structs.Event{
				Topic: structs.TopicNode,
				Type:  TypeNodeDeregistration,
				Index: changes.Index,
				Key:   before.ID,
				Payload: &NodeEvent{
					Node: before,
				},
			}
			events = append(events, event)
		}
	}
	return &structs.Events{Index: changes.Index, Events: events}, nil
}
