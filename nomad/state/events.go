package state

import (
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
