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
	TopicEvaluation Topic = "Evaluation"
	TopicAllocation Topic = "Allocation"
	TopicJob        Topic = "Job"
	TopicNode       Topic = "Node"
	TopicACLPolicy  Topic = "ACLPolicy"
	TopicACLToken   Topic = "ACLToken"
	TopicAll        Topic = "*"

	TypeNodeRegistration              = "NodeRegistration"
	TypeNodeDeregistration            = "NodeDeregistration"
	TypeNodeEligibilityUpdate         = "NodeEligibility"
	TypeNodeDrain                     = "NodeDrain"
	TypeNodeEvent                     = "NodeStreamEvent"
	TypeDeploymentUpdate              = "DeploymentStatusUpdate"
	TypeDeploymentPromotion           = "DeploymentPromotion"
	TypeDeploymentAllocHealth         = "DeploymentAllocHealth"
	TypeAllocationCreated             = "AllocationCreated"
	TypeAllocationUpdated             = "AllocationUpdated"
	TypeAllocationUpdateDesiredStatus = "AllocationUpdateDesiredStatus"
	TypeEvalUpdated                   = "EvaluationUpdated"
	TypeJobRegistered                 = "JobRegistered"
	TypeJobDeregistered               = "JobDeregistered"
	TypeJobBatchDeregistered          = "JobBatchDeregistered"
	TypePlanResult                    = "PlanResult"
	TypeACLTokenDeleted               = "ACLTokenDeleted"
	TypeACLTokenUpserted              = "ACLTokenUpserted"
	TypeACLPolicyDeleted              = "ACLPolicyDeleted"
	TypeACLPolicyUpserted             = "ACLPolicyUpserted"
)

// Event represents a change in Nomads state.
type Event struct {
	// Topic represents the primary object for the event
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

// EvaluationEvent holds a newly updated Eval.
type EvaluationEvent struct {
	Evaluation *Evaluation
}

// AllocationEvent holds a newly updated Allocation. The
// Allocs embedded Job has been removed to reduce size.
type AllocationEvent struct {
	Allocation *Allocation
}

// DeploymentEvent holds a newly updated Deployment.
type DeploymentEvent struct {
	Deployment *Deployment
}

// NodeStreamEvent holds a newly updated Node
type NodeStreamEvent struct {
	Node *Node
}

type ACLTokenEvent struct {
	ACLToken *ACLToken
	secretID string
}

// NewACLTokenEvent takes a token and creates a new ACLTokenEvent.  It creates
// a copy of the passed in ACLToken and empties out the copied tokens SecretID
func NewACLTokenEvent(token *ACLToken) *ACLTokenEvent {
	c := token.Copy()
	c.SecretID = ""

	return &ACLTokenEvent{
		ACLToken: c,
		secretID: token.SecretID,
	}
}

func (a *ACLTokenEvent) SecretID() string {
	return a.secretID
}

type ACLPolicyEvent struct {
	ACLPolicy *ACLPolicy
}
