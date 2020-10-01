package state

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/stream"
	"github.com/hashicorp/nomad/nomad/structs"
)

func DeploymentEventFromChanges(msgType structs.MessageType, tx ReadTxn, changes Changes) ([]stream.Event, error) {
	var events []stream.Event

	var eventType string
	switch msgType {
	case structs.DeploymentStatusUpdateRequestType:
		eventType = TypeDeploymentUpdate
	case structs.DeploymentPromoteRequestType:
		eventType = TypeDeploymentPromotion
	case structs.DeploymentAllocHealthRequestType:
		eventType = TypeDeploymentAllocHealth
	}

	for _, change := range changes.Changes {
		switch change.Table {
		case "deployment":
			after, ok := change.After.(*structs.Deployment)
			if !ok {
				return nil, fmt.Errorf("transaction change was not a Deployment")
			}

			event := stream.Event{
				Topic:      TopicDeployment,
				Type:       eventType,
				Index:      changes.Index,
				Key:        after.ID,
				FilterKeys: []string{after.JobID},
				Payload: &DeploymentEvent{
					Deployment: after,
				},
			}

			events = append(events, event)
		case "jobs":
			after, ok := change.After.(*structs.Job)
			if !ok {
				return nil, fmt.Errorf("transaction change was not a Job")
			}

			event := stream.Event{
				Topic: TopicJob,
				Type:  eventType,
				Index: changes.Index,
				Key:   after.ID,
				Payload: &JobEvent{
					Job: after,
				},
			}

			events = append(events, event)
		case "allocs":
			// TODO(drew) determine how to handle alloc updates during deployment
		case "evals":
			after, ok := change.After.(*structs.Evaluation)
			if !ok {
				return nil, fmt.Errorf("transaction change was not an Evaluation")
			}

			event := stream.Event{
				Topic:      TopicEval,
				Type:       eventType,
				Index:      changes.Index,
				Key:        after.ID,
				FilterKeys: []string{after.DeploymentID, after.JobID},
				Payload: &EvalEvent{
					Eval: after,
				},
			}

			events = append(events, event)
		}
	}

	return events, nil
}
