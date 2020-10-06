package state

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	TopicDeployment structs.Topic = "Deployment"
	TopicEval       structs.Topic = "Eval"
	TopicAlloc      structs.Topic = "Alloc"
	TopicJob        structs.Topic = "Job"
	// TopicNodeRegistration   stream.Topic = "NodeRegistration"
	// TopicNodeDeregistration stream.Topic = "NodeDeregistration"
	// TopicNodeDrain          stream.Topic = "NodeDrain"
	TopicNode structs.Topic = "Node"

	// TODO(drew) Node Events use TopicNode + Type
	TypeNodeRegistration      = "NodeRegistration"
	TypeNodeDeregistration    = "NodeDeregistration"
	TypeNodeEligibilityUpdate = "NodeEligibility"
	TypeNodeDrain             = "NodeDrain"
	TypeNodeEvent             = "NodeEvent"

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
)

type JobEvent struct {
	Job *structs.Job
}

type EvalEvent struct {
	Eval *structs.Evaluation
}

type AllocEvent struct {
	Alloc *structs.Allocation
}

type DeploymentEvent struct {
	Deployment *structs.Deployment
}

type NodeEvent struct {
	Node *structs.Node
}

// NNodeDrainEvent is the Payload for a NodeDrain event. It contains
// information related to the Node being drained as well as high level
// information about the current allocations on the Node
type NodeDrainEvent struct {
	Node      *structs.Node
	JobAllocs map[string]*JobDrainDetails
}

type NodeDrainAllocDetails struct {
	ID      string
	Migrate *structs.MigrateStrategy
}

type JobDrainDetails struct {
	Type         string
	AllocDetails map[string]NodeDrainAllocDetails
}

func GenericEventsFromChanges(tx ReadTxn, changes Changes) (*structs.Events, error) {
	var eventType string
	switch changes.MsgType {
	case structs.NodeRegisterRequestType:
		eventType = TypeNodeRegistration
	case structs.UpsertNodeEventsType:
		eventType = TypeNodeEvent
	case structs.EvalUpdateRequestType:
		eventType = TypeEvalUpdated
	case structs.AllocClientUpdateRequestType:
		eventType = TypeAllocUpdated
	case structs.JobRegisterRequestType:
		eventType = TypeJobRegistered
	case structs.AllocUpdateRequestType:
		eventType = TypeAllocUpdated
	case structs.NodeUpdateStatusRequestType:
		eventType = TypeNodeEvent
	case structs.JobDeregisterRequestType:
		eventType = TypeJobDeregistered
	case structs.JobBatchDeregisterRequestType:
		eventType = TypeJobBatchDeregistered
	case structs.AllocUpdateDesiredTransitionRequestType:
		eventType = TypeAllocUpdateDesiredStatus
	case structs.NodeUpdateEligibilityRequestType:
		eventType = TypeNodeDrain
	case structs.BatchNodeUpdateDrainRequestType:
		eventType = TypeNodeDrain
	default:
		// unknown request type
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
				Topic: TopicEval,
				Type:  eventType,
				Index: changes.Index,
				Key:   after.ID,
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
			// remove job info to help keep size of alloc event down
			alloc.Job = nil

			event := structs.Event{
				Topic: TopicAlloc,
				Type:  eventType,
				Index: changes.Index,
				Key:   after.ID,
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
				Topic: TopicAlloc,
				Type:  eventType,
				Index: changes.Index,
				Key:   after.ID,
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
				Topic: TopicNode,
				Type:  eventType,
				Index: changes.Index,
				Key:   after.ID,
				Payload: &NodeEvent{
					Node: after,
				},
			}
			events = append(events, event)
		}
	}

	return &structs.Events{Index: changes.Index, Events: events}, nil
}
