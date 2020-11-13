package api

import "sort"

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

type EventSinks struct {
	client *Client
}

func (c *Client) EventSinks() *EventSinks {
	return &EventSinks{client: c}
}

func (e *EventSinks) List(q *QueryOptions) ([]*EventSink, *QueryMeta, error) {
	var resp []*EventSink
	qm, err := e.client.query("/v1/event/sinks", &resp, q)
	if err != nil {
		return nil, nil, err
	}

	sort.Slice(resp, func(i, j int) bool { return resp[i].ID < resp[j].ID })
	return resp, qm, nil
}

func (e *EventSinks) Register(eventSink *EventSink, w *WriteOptions) (*WriteMeta, error) {
	wm, err := e.client.write("/v1/event/sink/"+eventSink.ID, eventSink, nil, w)
	if err != nil {
		return nil, err
	}
	return wm, nil
}

func (e *EventSinks) Deregister(id string, w *WriteOptions) (*WriteMeta, error) {
	return e.client.delete("/v1/event/sink/"+id, nil, w)
}
