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

func NodeDrainEventFromChanges(tx ReadTxn, changes Changes) (*structs.Events, error) {
	var events []structs.Event
	for _, change := range changes.Changes {
		switch change.Table {
		case "nodes":
			after, ok := change.After.(*structs.Node)
			if !ok {
				return nil, fmt.Errorf("transaction change was not a Node")
			}

			// retrieve allocations currently on node
			allocs, err := allocsByNodeTxn(tx, nil, after.ID)
			if err != nil {
				return nil, fmt.Errorf("retrieving allocations for node drain event: %w", err)
			}

			// build job/alloc details for node drain
			jobAllocs := make(map[string]*JobDrainDetails)
			for _, a := range allocs {
				if _, ok := jobAllocs[a.Job.Name]; !ok {
					jobAllocs[a.Job.Name] = &JobDrainDetails{
						AllocDetails: make(map[string]NodeDrainAllocDetails),
						Type:         a.Job.Type,
					}
				}

				jobAllocs[a.Job.Name].AllocDetails[a.ID] = NodeDrainAllocDetails{
					Migrate: a.MigrateStrategy(),
					ID:      a.ID,
				}
			}

			event := structs.Event{
				Topic: structs.TopicNode,
				Type:  TypeNodeDrain,
				Index: changes.Index,
				Key:   after.ID,
				Payload: &NodeDrainEvent{
					Node:      after,
					JobAllocs: jobAllocs,
				},
			}
			events = append(events, event)
		}
	}
	return &structs.Events{Index: changes.Index, Events: events}, nil
}
