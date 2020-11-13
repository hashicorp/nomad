package state

import (
	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	TypeNodeRegistration         = "NodeRegistration"
	TypeNodeDeregistration       = "NodeDeregistration"
	TypeNodeEligibilityUpdate    = "NodeEligibility"
	TypeNodeDrain                = "NodeDrain"
	TypeNodeEvent                = "NodeStreamEvent"
	TypeDeploymentUpdate         = "DeploymentStatusUpdate"
	TypeDeploymentPromotion      = "DeploymentPromotion"
	TypeDeploymentAllocHealth    = "DeploymentAllocHealth"
	TypeAllocCreated             = "AllocCreated"
	TypeAllocUpdated             = "AllocUpdated"
	TypeAllocUpdateDesiredStatus = "AllocUpdateDesiredStatus"
	TypeEvalUpdated              = "EvalUpdated"
	TypeJobRegistered            = "JobRegistered"
	TypeJobDeregistered          = "JobDeregistered"
	TypeJobBatchDeregistered     = "JobBatchDeregistered"
	TypePlanResult               = "PlanResult"
)

var MsgTypeEvents = map[structs.MessageType]string{
	structs.NodeRegisterRequestType:                 TypeNodeRegistration,
	structs.NodeDeregisterRequestType:               TypeNodeDeregistration,
	structs.UpsertNodeEventsType:                    TypeNodeEvent,
	structs.EvalUpdateRequestType:                   TypeEvalUpdated,
	structs.AllocClientUpdateRequestType:            TypeAllocUpdated,
	structs.JobRegisterRequestType:                  TypeJobRegistered,
	structs.AllocUpdateRequestType:                  TypeAllocUpdated,
	structs.NodeUpdateStatusRequestType:             TypeNodeEvent,
	structs.JobDeregisterRequestType:                TypeJobDeregistered,
	structs.JobBatchDeregisterRequestType:           TypeJobBatchDeregistered,
	structs.AllocUpdateDesiredTransitionRequestType: TypeAllocUpdateDesiredStatus,
	structs.NodeUpdateEligibilityRequestType:        TypeNodeDrain,
	structs.NodeUpdateDrainRequestType:              TypeNodeDrain,
	structs.BatchNodeUpdateDrainRequestType:         TypeNodeDrain,
	structs.DeploymentStatusUpdateRequestType:       TypeDeploymentUpdate,
	structs.DeploymentPromoteRequestType:            TypeDeploymentPromotion,
	structs.DeploymentAllocHealthRequestType:        TypeDeploymentAllocHealth,
	structs.ApplyPlanResultsRequestType:             TypePlanResult,
}

func eventsFromChanges(tx ReadTxn, changes Changes) *structs.Events {
	eventType, ok := MsgTypeEvents[changes.MsgType]
	if !ok {
		return nil
	}

	var events []structs.Event
	for _, change := range changes.Changes {
		if event, ok := eventFromChange(change); ok {
			event.Type = eventType
			event.Index = changes.Index
			events = append(events, event)
		}
	}

	return &structs.Events{Index: changes.Index, Events: events}
}

func eventFromChange(change memdb.Change) (structs.Event, bool) {
	if change.Deleted() {
		switch before := change.Before.(type) {
		case *structs.Node:
			return structs.Event{
				Topic: structs.TopicNode,
				Key:   before.ID,
				Payload: &structs.NodeStreamEvent{
					Node: before,
				},
			}, true
		}

		return structs.Event{}, false
	}

	switch after := change.After.(type) {
	case *structs.Evaluation:
		return structs.Event{
			Topic: structs.TopicEval,
			Key:   after.ID,
			FilterKeys: []string{
				after.JobID,
				after.DeploymentID,
			},
			Namespace: after.Namespace,
			Payload: &structs.EvalEvent{
				Eval: after,
			},
		}, true

	case *structs.Allocation:
		alloc := after.Copy()

		filterKeys := []string{
			alloc.JobID,
			alloc.DeploymentID,
		}

		// remove job info to help keep size of alloc event down
		alloc.Job = nil

		return structs.Event{
			Topic:      structs.TopicAlloc,
			Key:        after.ID,
			FilterKeys: filterKeys,
			Namespace:  after.Namespace,
			Payload: &structs.AllocEvent{
				Alloc: alloc,
			},
		}, true

	case *structs.Job:
		return structs.Event{
			Topic:     structs.TopicJob,
			Key:       after.ID,
			Namespace: after.Namespace,
			Payload: &structs.JobEvent{
				Job: after,
			},
		}, true

	case *structs.Node:
		return structs.Event{
			Topic: structs.TopicNode,
			Key:   after.ID,
			Payload: &structs.NodeStreamEvent{
				Node: after,
			},
		}, true

	case *structs.Deployment:
		return structs.Event{
			Topic:      structs.TopicDeployment,
			Key:        after.ID,
			Namespace:  after.Namespace,
			FilterKeys: []string{after.JobID},
			Payload: &structs.DeploymentEvent{
				Deployment: after,
			},
		}, true
	}

	return structs.Event{}, false
}
