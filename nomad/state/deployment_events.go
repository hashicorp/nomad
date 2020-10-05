package state

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/structs"
)

func DeploymentEventFromChanges(msgType structs.MessageType, tx ReadTxn, changes Changes) (structs.Events, error) {
	var events []structs.Event

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
				return structs.Events{}, fmt.Errorf("transaction change was not a Deployment")
			}

			event := structs.Event{
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
				return structs.Events{}, fmt.Errorf("transaction change was not a Job")
			}

			event := structs.Event{
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
				return structs.Events{}, fmt.Errorf("transaction change was not an Evaluation")
			}

			event := structs.Event{
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

	return structs.Events{Index: changes.Index, Events: events}, nil
}
