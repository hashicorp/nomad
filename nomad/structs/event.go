package structs

import (
	"fmt"
	"net/url"
	"strings"

	multierror "github.com/hashicorp/go-multierror"
)

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

type EventSinkProgressRequest struct {
	Sinks []*EventSink
	WriteRequest
}

type EventSinkUpsertRequest struct {
	Sink *EventSink
	WriteRequest
}

type EventSinkSpecificRequest struct {
	ID string
	QueryOptions
}

type EventSinkResponse struct {
	Sink *EventSink
	QueryMeta
}

type EventSinkDeleteRequest struct {
	IDs []string
	WriteRequest
}

type EventSinkListRequest struct {
	QueryOptions
}

type EventSinkListResponse struct {
	Sinks []*EventSink
	QueryMeta
}

type SinkType string

const (
	SinkWebhook SinkType = "webhook"
)

type EventSink struct {
	ID   string
	Type SinkType

	Topics map[Topic][]string

	Address string

	// LatestIndex is the latest reported index that was successfully sent.
	// MangedSinks periodically check in to update the LatestIndex so that a
	// minimal amount of events are resent when reestablishing an event sink
	LatestIndex uint64

	CreateIndex uint64
	ModifyIndex uint64
}

func (e *EventSink) Validate() error {
	var mErr multierror.Error

	if e.ID == "" {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("Missing sink ID"))
	} else if strings.Contains(e.ID, " ") {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("Sink ID contains a space"))
	} else if strings.Contains(e.ID, "\000") {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("Sink ID contains a null character"))
	}

	switch e.Type {
	case SinkWebhook:
		if e.Address == "" {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("Webhook sink requires a valid Address"))
		} else if _, err := url.Parse(e.Address); err != nil {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("Webhook sink Address must be a valid url: %w", err))
		}
	default:
		mErr.Errors = append(mErr.Errors, fmt.Errorf("Sink type invalid"))
	}

	return mErr.ErrorOrNil()
}
