package api

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mitchellh/mapstructure"
)

const (
	TopicDeployment Topic = "Deployment"
	TopicEval       Topic = "Eval"
	TopicAlloc      Topic = "Alloc"
	TopicJob        Topic = "Job"
	TopicNode       Topic = "Node"
	TopicAll        Topic = "*"
)

// Events is a set of events for a corresponding index. Events returned for the
// index depend on which topics are subscribed to when a request is made.
type Events struct {
	Index  uint64
	Events []Event
	Err    error
}

// Topic is an event Topic
type Topic string

// Event holds information related to an event that occurred in Nomad.
// The Payload is a hydrated object related to the Topic
type Event struct {
	Topic      Topic
	Type       string
	Key        string
	FilterKeys []string
	Index      uint64
	Payload    map[string]interface{}
}

// Deployment returns a Deployment struct from a given event payload. If the
// Event Topic is Deployment this will return a valid Deployment
func (e *Event) Deployment() (*Deployment, error) {
	v := struct {
		Deployment *Deployment `mapstructure:"Deployment"`
	}{}
	cfg := &mapstructure.DecoderConfig{
		Result:     &v,
		DecodeHook: mapstructure.StringToTimeHookFunc(time.RFC3339),
	}

	dec, err := mapstructure.NewDecoder(cfg)
	if err != nil {
		return nil, err
	}

	if err := dec.Decode(e.Payload); err != nil {
		return nil, err
	}

	return v.Deployment, nil
}

// Evaluation returns a Evaluation struct from a given event payload. If the
// Event Topic is Evaluation this will return a valid Evaluation
func (e *Event) Evaluation() (*Evaluation, error) {
	v := struct {
		Evaluation *Evaluation `mapstructure:"Eval"`
	}{}
	cfg := &mapstructure.DecoderConfig{
		Result:     &v,
		DecodeHook: mapstructure.StringToTimeHookFunc(time.RFC3339),
	}

	dec, err := mapstructure.NewDecoder(cfg)
	if err != nil {
		return nil, err
	}

	if err := dec.Decode(e.Payload); err != nil {
		return nil, err
	}

	return v.Evaluation, nil
}

// Allocation returns a Allocation struct from a given event payload. If the
// Event Topic is Allocation this will return a valid Allocation.
func (e *Event) Allocation() (*Allocation, error) {
	v := struct {
		Allocation *Allocation `mapstructure:"Alloc"`
	}{}
	cfg := &mapstructure.DecoderConfig{
		Result:     &v,
		DecodeHook: mapstructure.StringToTimeHookFunc(time.RFC3339),
	}

	dec, err := mapstructure.NewDecoder(cfg)
	if err != nil {
		return nil, err
	}

	if err := dec.Decode(e.Payload); err != nil {
		return nil, err
	}

	return v.Allocation, nil
}

// Job returns a Job struct from a given event payload. If the
// Event Topic is Job this will return a valid Job.
func (e *Event) Job() (*Job, error) {
	v := struct {
		Job *Job `mapstructure:"Job"`
	}{}
	cfg := &mapstructure.DecoderConfig{
		Result:     &v,
		DecodeHook: mapstructure.StringToTimeHookFunc(time.RFC3339),
	}

	dec, err := mapstructure.NewDecoder(cfg)
	if err != nil {
		return nil, err
	}

	if err := dec.Decode(e.Payload); err != nil {
		return nil, err
	}

	return v.Job, nil
}

// Node returns a Node struct from a given event payload. If the
// Event Topic is Node this will return a valid Node.
func (e *Event) Node() (*Node, error) {
	v := struct {
		Node *Node `mapstructure:"Node"`
	}{}
	cfg := &mapstructure.DecoderConfig{
		Result:     &v,
		DecodeHook: mapstructure.StringToTimeHookFunc(time.RFC3339),
	}

	dec, err := mapstructure.NewDecoder(cfg)
	if err != nil {
		return nil, err
	}

	if err := dec.Decode(e.Payload); err != nil {
		return nil, err
	}

	return v.Node, nil
}

// IsHeartbeat specifies if the event is an empty heartbeat used to
// keep a connection alive.
func (e *Events) IsHeartbeat() bool {
	return e.Index == 0 && len(e.Events) == 0
}

// EventStream is used to stream events from Nomad
type EventStream struct {
	client *Client
}

// EventStream returns a handle to the Events endpoint
func (c *Client) EventStream() *EventStream {
	return &EventStream{client: c}
}

// Stream establishes a new subscription to Nomad's event stream and streams
// results back to the returned channel.
func (e *EventStream) Stream(ctx context.Context, topics map[Topic][]string, index uint64, q *QueryOptions) (<-chan *Events, error) {
	r, err := e.client.newRequest("GET", "/v1/event/stream")
	if err != nil {
		return nil, err
	}
	q = q.WithContext(ctx)
	r.setQueryOptions(q)

	// Build topic query params
	for topic, keys := range topics {
		for _, k := range keys {
			r.params.Add("topic", fmt.Sprintf("%s:%s", topic, k))
		}
	}

	_, resp, err := requireOK(e.client.doRequest(r))

	if err != nil {
		return nil, err
	}

	eventsCh := make(chan *Events, 10)
	go func() {
		defer resp.Body.Close()
		defer close(eventsCh)

		dec := json.NewDecoder(resp.Body)

		for ctx.Err() == nil {
			// Decode next newline delimited json of events
			var events Events
			if err := dec.Decode(&events); err != nil {
				// set error and fallthrough to
				// select eventsCh
				events = Events{Err: err}
			}
			if events.Err == nil && events.IsHeartbeat() {
				continue
			}

			select {
			case <-ctx.Done():
				return
			case eventsCh <- &events:
			}
		}
	}()

	return eventsCh, nil
}
