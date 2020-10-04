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
	TypeNodeRegistration   = "NodeRegistration"
	TypeNodeDeregistration = "NodeDeregistration"
	TypeNodeDrain          = "NodeDrain"
	TypeNodeEvent          = "NodeEvent"

	TypeDeploymentUpdate      = "DeploymentStatusUpdate"
	TypeDeploymentPromotion   = "DeploymentPromotion"
	TypeDeploymentAllocHealth = "DeploymentAllocHealth"

	TypeAllocCreated = "AllocCreated"
	TypeAllocUpdated = "AllocUpdated"

	TypeEvalUpdated = "EvalUpdated"

	TypeJobRegistered = "JobRegistered"
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

func GenericEventsFromChanges(tx ReadTxn, changes Changes) (structs.Events, error) {
	var eventType string
	switch changes.MsgType {
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
	}

	var events []structs.Event
	for _, change := range changes.Changes {
		switch change.Table {
		case "evals":
			after, ok := change.After.(*structs.Evaluation)
			if !ok {
				return structs.Events{}, fmt.Errorf("transaction change was not an Evaluation")
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
			after, ok := change.After.(*structs.Allocation)
			if !ok {
				return structs.Events{}, fmt.Errorf("transaction change was not an Allocation")
			}

			event := structs.Event{
				Topic: TopicAlloc,
				Type:  eventType,
				Index: changes.Index,
				Key:   after.ID,
				Payload: &AllocEvent{
					Alloc: after,
				},
			}

			events = append(events, event)
		case "jobs":
			after, ok := change.After.(*structs.Job)
			if !ok {
				return structs.Events{}, fmt.Errorf("transaction change was not an Allocation")
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
			after, ok := change.After.(*structs.Node)
			if !ok {
				return structs.Events{}, fmt.Errorf("transaction change was not a Node")
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

	return structs.Events{Index: changes.Index, Events: events}, nil
}
