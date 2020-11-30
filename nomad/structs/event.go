package structs

// EventStreamRequest is used to stream events from a servers EventBroker
type EventStreamRequest struct {
	Topics map[Topic][]string
	Index  int

	QueryOptions
}

type EventStreamWrapper struct {
	Error *RpcError
	Event *EventJson
}

type Topic string

const (
	TopicDeployment Topic = "Deployment"
	TopicEval       Topic = "Eval"
	TopicAlloc      Topic = "Alloc"
	TopicJob        Topic = "Job"
	TopicNode       Topic = "Node"
	TopicAll        Topic = "*"

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

// Event represents a change in Nomads state.
type Event struct {
	// Topic represeents the primary object for the event
	Topic Topic

	// Type is a short string representing the reason for the event
	Type string

	// Key is the primary identifier of the Event, The involved objects ID
	Key string

	// Namespace is the namespace of the object, If the object is not namespace
	// aware (Node) it is left blank
	Namespace string

	// FilterKeys are a set of additional related keys that are used to include
	// events during filtering.
	FilterKeys []string

	// Index is the raft index that corresponds to the event
	Index uint64

	// Payload is the Event itself see state/events.go for a list of events
	Payload interface{}
}

// Events is a wrapper that contains a set of events for a given index.
type Events struct {
	Index  uint64
	Events []Event
}

// EventJson is a wrapper for a JSON object
type EventJson struct {
	Data []byte
}

func (j *EventJson) Copy() *EventJson {
	n := new(EventJson)
	*n = *j
	n.Data = make([]byte, len(j.Data))
	copy(n.Data, j.Data)
	return n
}

// JobEvent holds a newly updated Job.
type JobEvent struct {
	Job *Job
}

// EvalEvent holds a newly updated Eval.
type EvalEvent struct {
	Eval *Evaluation
}

// AllocEvent holds a newly updated Allocation. The
// Allocs embedded Job has been removed to reduce size.
type AllocEvent struct {
	Alloc *Allocation
}

// DeploymentEvent holds a newly updated Deployment.
type DeploymentEvent struct {
	Deployment *Deployment
}

// NodeStreamEvent holds a newly updated Node
type NodeStreamEvent struct {
	Node *Node
}
