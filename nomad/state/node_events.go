package state

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/stream"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	TopicNodeRegistration   = "NodeRegistration"
	TopicNodeDeregistration = "NodeDeregistration"
)

type NodeRegistrationEvent struct {
	Event      *structs.NodeEvent
	NodeStatus string
}

type NodeDeregistrationEvent struct {
	NodeID string
}

// NodeRegisterEventFromChanges generates a NodeRegistrationEvent from a set
// of transaction changes.
func NodeRegisterEventFromChanges(tx ReadTxn, changes Changes) ([]stream.Event, error) {
	var events []stream.Event
	for _, change := range changes.Changes {
		switch change.Table {
		case "nodes":
			after, ok := change.After.(*structs.Node)
			if !ok {
				return nil, fmt.Errorf("transaction change was not a Node")
			}

			event := stream.Event{
				Topic: TopicNodeRegistration,
				Index: changes.Index,
				Key:   after.ID,
				Payload: &NodeRegistrationEvent{
					Event:      after.Events[len(after.Events)-1],
					NodeStatus: after.Status,
				},
			}
			events = append(events, event)
		}
	}
	return events, nil
}

// NodeDeregisterEventFromChanges generates a NodeDeregistrationEvent from a set
// of transaction changes.
func NodeDeregisterEventFromChanges(tx ReadTxn, changes Changes) ([]stream.Event, error) {
	var events []stream.Event
	for _, change := range changes.Changes {
		switch change.Table {
		case "nodes":
			before, ok := change.Before.(*structs.Node)
			if !ok {
				return nil, fmt.Errorf("transaction change was not a Node")
			}

			event := stream.Event{
				Topic: TopicNodeDeregistration,
				Index: changes.Index,
				Key:   before.ID,
				Payload: &NodeDeregistrationEvent{
					NodeID: before.ID,
				},
			}
			events = append(events, event)
		}
	}
	return events, nil
}
