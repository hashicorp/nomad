package stream

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	pbstruct "github.com/golang/protobuf/ptypes/struct"
	pbstream "github.com/hashicorp/nomad/nomad/stream/proto"
	"github.com/hashicorp/nomad/nomad/structs"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ pbstream.EventStreamServer = (*SinkServer)(nil)

type SinkServer struct {
	broker *EventBroker
}

func NewSinkServer(broker *EventBroker) *SinkServer {
	return &SinkServer{
		broker: broker,
	}
}

func (s *SinkServer) Subscribe(pbReq *pbstream.SubscribeRequest, serverStream pbstream.EventStream_SubscribeServer) error {
	req := &SubscribeRequest{
		Index:  pbReq.Index,
		Topics: map[structs.Topic][]string{},
	}

	for _, topicFilter := range pbReq.Topics {
		topicKey := topicFilter.String()
		if topicFilter.GetTopic() == pbstream.Topic_All {
			topicKey = "*"
		}

		req.Topics[structs.Topic(topicKey)] = []string{"*"}
	}

	sub, err := s.broker.Subscribe(req)
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()
	ctx := serverStream.Context()

	for {
		event, err := sub.Next(ctx)
		switch {
		case errors.Is(err, ErrSubscriptionClosed):
			return status.Error(codes.Aborted, err.Error())
		case err != nil:
			return err
		}

		e, err := newEventFromStreamEvent(event)
		if err != nil {
			return err
		}

		if err := serverStream.Send(e); err != nil {
			return err
		}
	}

	// resultCh := make(chan result, 10)
	// go func() {
	// 	defer cancel()
	// 	for {
	// 		events, err := sub.Next(ctx)
	// 		if err != nil {
	// 			select {
	// 			case resultCh <- result{err: err}:
	// 			case <-ctx.Done():
	// 			}
	// 			return
	// 		}

	// 		e := newEventFrom
	// 		eventBatch := &pbstream.EventBatch{
	// 			Index: events.Index,
	// 		}

	// 		for _, e := range events.Events {
	// 			ebEvent := &pbstream.Event{
	// 				Topic:   pbstream.Topic(pbstream.Topic_value[string(e.Topic)]),
	// 				Key:     e.Key,
	// 				Payload: e.Payload,
	// 			}
	// 			eventBatch.Event = append(eventBatch.Event, ebEvent)
	// 		}
	// 	}

	// }()

	event, err := sub.Next(context.Background())
	if err != nil {
		return err
	}

	eventBatch := &pbstream.EventBatch{
		Index: event.Index,
	}

	for _, e := range event.Events {
		ebEvent := &pbstream.Event{
			Topic: pbstream.Topic(pbstream.Topic_value[string(e.Topic)]),
			Key:   e.Key,
		}
		eventBatch.Event = append(eventBatch.Event, ebEvent)
	}

	err = serverStream.Send(eventBatch)
	if err != nil {
		return err
	}

	return nil
}

func newEventFromStreamEvent(events structs.Events) (*pbstream.EventBatch, error) {
	e := &pbstream.EventBatch{Index: events.Index}

	var pbEvents []*pbstream.Event
	for _, evnts := range events.Events {
		payload, err := eventToProtoStruct(evnts.Payload)
		if err != nil {
			return nil, err
		}
		pbe := &pbstream.Event{
			Topic:   pbstream.Topic(pbstream.Topic_value[string(e.Topic)]),
			Key:     e.Key,
			Payload: payload,
		}
		pbEvents = append(pbEvents, pbe)
	}
	e.Event = pbEvents

	return e, nil
}

func eventToProtoStruct(payload interface{}) (*pbstruct.Struct, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	reader := bytes.NewReader(b)

	pbs := &pbstruct.Struct{}

	if err = jsonpb.Unmarshal(reader, pbs); err != nil {
		return nil, err
	}
	return pbs, nil
}
