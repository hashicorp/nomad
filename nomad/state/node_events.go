package state

import (
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

func NodeRegisterEventFromChanges(tx ReadTxn, changes Changes) ([]stream.Event, error) {
	var events []stream.Event
	for _, change := range changes.Changes {
		switch change.Table {
		case "nodes":
			after := change.After.(*structs.Node)

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

func NodeDeregisterEventFromChanges(tx ReadTxn, changes Changes) ([]stream.Event, error) {
	var event stream.Event
	for _, change := range changes.Changes {
		switch change.Table {
		case "nodes":
			before := change.Before.(*structs.Node)

			event = stream.Event{
				Topic: TopicNodeDeregistration,
				Index: changes.Index,
				Key:   before.ID,
				Payload: &NodeDeregistrationEvent{
					NodeID: before.ID,
				},
			}
		}
	}
	return []stream.Event{event}, nil
}
