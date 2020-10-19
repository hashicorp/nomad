package state

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	TypeNodeRegistration         = "NodeRegistration"
	TypeNodeDeregistration       = "NodeDeregistration"
	TypeNodeEligibilityUpdate    = "NodeEligibility"
	TypeNodeDrain                = "NodeDrain"
	TypeNodeEvent                = "NodeEvent"
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

// JobEvent holds a newly updated Job.
type JobEvent struct {
	Job *structs.Job
}

// EvalEvent holds a newly updated Eval.
type EvalEvent struct {
	Eval *structs.Evaluation
}

// AllocEvent holds a newly updated Allocation. The
// Allocs embedded Job has been removed to reduce size.
type AllocEvent struct {
	Alloc *structs.Allocation
}

// DeploymentEvent holds a newly updated Deployment.
type DeploymentEvent struct {
	Deployment *structs.Deployment
}

// NodeEvent holds a newly updated Node
type NodeEvent struct {
	Node *structs.Node
}

type NodeDrainAllocDetails struct {
	ID      string
	Migrate *structs.MigrateStrategy
}

type JobDrainDetails struct {
	Type         string
	AllocDetails map[string]NodeDrainAllocDetails
}

var MsgTypeEvents = map[structs.MessageType]string{
	structs.NodeRegisterRequestType:                 TypeNodeRegistration,
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

// GenericEventsFromChanges returns a set of events for a given set of
// transaction changes. It currently ignores Delete operations.
func GenericEventsFromChanges(tx ReadTxn, changes Changes) (*structs.Events, error) {
	eventType, ok := MsgTypeEvents[changes.MsgType]
	if !ok {
		return nil, nil
	}

	var events []structs.Event
	for _, change := range changes.Changes {
		switch change.Table {
		case "evals":
			if change.Deleted() {
				return nil, nil
			}
			after, ok := change.After.(*structs.Evaluation)
			if !ok {
				return nil, fmt.Errorf("transaction change was not an Evaluation")
			}

			event := structs.Event{
				Topic: structs.TopicEval,
				Type:  eventType,
				Index: changes.Index,
				Key:   after.ID,
				FilterKeys: []string{
					after.JobID,
					after.DeploymentID,
				},
				Namespace: after.Namespace,
				Payload: &EvalEvent{
					Eval: after,
				},
			}

			events = append(events, event)

		case "allocs":
			if change.Deleted() {
				return nil, nil
			}
			after, ok := change.After.(*structs.Allocation)
			if !ok {
				return nil, fmt.Errorf("transaction change was not an Allocation")
			}

			alloc := after.Copy()

			filterKeys := []string{
				alloc.JobID,
				alloc.DeploymentID,
			}

			// remove job info to help keep size of alloc event down
			alloc.Job = nil

			event := structs.Event{
				Topic:      structs.TopicAlloc,
				Type:       eventType,
				Index:      changes.Index,
				Key:        after.ID,
				FilterKeys: filterKeys,
				Namespace:  after.Namespace,
				Payload: &AllocEvent{
					Alloc: alloc,
				},
			}

			events = append(events, event)
		case "jobs":
			if change.Deleted() {
				return nil, nil
			}
			after, ok := change.After.(*structs.Job)
			if !ok {
				return nil, fmt.Errorf("transaction change was not an Allocation")
			}

			event := structs.Event{
				Topic:     structs.TopicJob,
				Type:      eventType,
				Index:     changes.Index,
				Key:       after.ID,
				Namespace: after.Namespace,
				Payload: &JobEvent{
					Job: after,
				},
			}

			events = append(events, event)
		case "nodes":
			if change.Deleted() {
				return nil, nil
			}
			after, ok := change.After.(*structs.Node)
			if !ok {
				return nil, fmt.Errorf("transaction change was not a Node")
			}

			event := structs.Event{
				Topic: structs.TopicNode,
				Type:  eventType,
				Index: changes.Index,
				Key:   after.ID,
				Payload: &NodeEvent{
					Node: after,
				},
			}
			events = append(events, event)
		case "deployment":
			if change.Deleted() {
				return nil, nil
			}
			after, ok := change.After.(*structs.Deployment)
			if !ok {
				return nil, fmt.Errorf("transaction change was not a Node")
			}

			event := structs.Event{
				Topic:      structs.TopicDeployment,
				Type:       eventType,
				Index:      changes.Index,
				Key:        after.ID,
				Namespace:  after.Namespace,
				FilterKeys: []string{after.JobID},
				Payload: &DeploymentEvent{
					Deployment: after,
				},
			}
			events = append(events, event)
		}
	}

	return &structs.Events{Index: changes.Index, Events: events}, nil
}
