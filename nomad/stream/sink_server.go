package stream

import (
	"bytes"
	"encoding/json"
	"errors"

	pbstruct "github.com/golang/protobuf/ptypes/struct"
	pbstream "github.com/hashicorp/nomad/nomad/stream/proto"
	"github.com/hashicorp/nomad/nomad/structs"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/golang/protobuf/jsonpb"
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
}

func newEventFromStreamEvent(events structs.Events) (*pbstream.EventBatch, error) {
	batch := &pbstream.EventBatch{Index: events.Index}

	var pbEvents []*pbstream.Event
	for _, e := range events.Events {
		payload, err := eventToProtoStruct(e.Payload)
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
	batch.Event = pbEvents

	return batch, nil
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
