package state

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/stream"
	"github.com/hashicorp/nomad/nomad/structs"
)

func ApplyPlanResultEventsFromChanges(tx ReadTxn, changes Changes) (stream.Events, error) {
	var events []stream.Event
	for _, change := range changes.Changes {
		switch change.Table {
		case "deployment":
			after, ok := change.After.(*structs.Deployment)
			if !ok {
				return stream.Events{}, fmt.Errorf("transaction change was not a Deployment")
			}

			event := stream.Event{
				Topic: TopicDeployment,
				Type:  TypeDeploymentUpdate,
				Index: changes.Index,
				Key:   after.ID,
				Payload: &DeploymentEvent{
					Deployment: after,
				},
			}
			events = append(events, event)
		case "evals":
			after, ok := change.After.(*structs.Evaluation)
			if !ok {
				return stream.Events{}, fmt.Errorf("transaction change was not an Evaluation")
			}

			event := stream.Event{
				Topic: TopicEval,
				Index: changes.Index,
				Key:   after.ID,
				Payload: &EvalEvent{
					Eval: after,
				},
			}

			events = append(events, event)
		case "allocs":
			after, ok := change.After.(*structs.Allocation)
			if !ok {
				return stream.Events{}, fmt.Errorf("transaction change was not an Allocation")
			}
			before := change.Before
			var msg string
			if before == nil {
				msg = TypeAllocCreated
			} else {
				msg = TypeAllocUpdated
			}

			event := stream.Event{
				Topic: TopicAlloc,
				Type:  msg,
				Index: changes.Index,
				Key:   after.ID,
				Payload: &AllocEvent{
					Alloc: after,
				},
			}

			events = append(events, event)
		}
	}

	return stream.Events{Index: changes.Index, Events: events}, nil
}
