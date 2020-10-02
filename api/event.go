package api

import (
	"context"
	"encoding/json"
	"fmt"
)

type Events struct {
	Index  uint64
	Events []Event
}

type Topic string

type Event struct {
	Topic      Topic
	Type       string
	Key        string
	FilterKeys []string
	Index      uint64
	Payload    interface{}
}

func (e *Events) IsHeartBeat() bool {
	return e.Index == 0 && len(e.Events) == 0
}

type EventStream struct {
	client *Client
}

func (c *Client) EventStream() *EventStream {
	return &EventStream{client: c}
}

func (e *EventStream) Stream(ctx context.Context, topics map[Topic][]string, index uint64, q *QueryOptions) (<-chan *Events, <-chan error) {

	errCh := make(chan error, 1)

	r, err := e.client.newRequest("GET", "/v1/event/stream")
	if err != nil {
		errCh <- err
		return nil, errCh
	}
	r.setQueryOptions(q)

	// Build topic query params
	for topic, keys := range topics {
		for _, k := range keys {
			r.params.Add("topic", fmt.Sprintf("%s:%s", topic, k))
		}
	}

	_, resp, err := requireOK(e.client.doRequest(r))

	if err != nil {
		errCh <- err
		return nil, errCh
	}

	eventsCh := make(chan *Events, 10)
	go func() {
		defer resp.Body.Close()

		dec := json.NewDecoder(resp.Body)

		for {
			select {
			case <-ctx.Done():
				close(eventsCh)
				return
			default:
			}

			// Decode next newline delimited json of events
			var events Events
			if err := dec.Decode(&events); err != nil {
				close(eventsCh)
				errCh <- err
				return
			}
			if events.IsHeartBeat() {
				continue
			}

			eventsCh <- &events

		}
	}()

	return eventsCh, errCh
}
