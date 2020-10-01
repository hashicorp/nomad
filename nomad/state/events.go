package state

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/stream"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	TopicDeployment stream.Topic = "Deployment"
	TopicEval       stream.Topic = "Eval"
	TopicAlloc      stream.Topic = "Alloc"
	TopicJob        stream.Topic = "Job"
	// TopicNodeRegistration   stream.Topic = "NodeRegistration"
	// TopicNodeDeregistration stream.Topic = "NodeDeregistration"
	// TopicNodeDrain          stream.Topic = "NodeDrain"
	TopicNode stream.Topic = "Node"

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

func GenericEventsFromChanges(tx ReadTxn, changes Changes) ([]stream.Event, error) {
	var eventType string
	switch changes.MsgType {
	case structs.EvalUpdateRequestType:
		eventType = TypeEvalUpdated
	case structs.AllocClientUpdateRequestType:
		eventType = TypeAllocUpdated
	}

	var events []stream.Event
	for _, change := range changes.Changes {
		switch change.Table {
		case "evals":
			after, ok := change.After.(*structs.Evaluation)
			if !ok {
				return nil, fmt.Errorf("transaction change was not an Evaluation")
			}

			event := stream.Event{
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
				return nil, fmt.Errorf("transaction change was not an Allocation")
			}

			event := stream.Event{
				Topic: TopicAlloc,
				Type:  eventType,
				Index: changes.Index,
				Key:   after.ID,
				Payload: &AllocEvent{
					Alloc: after,
				},
			}

			events = append(events, event)
		}
	}

	return events, nil
}
