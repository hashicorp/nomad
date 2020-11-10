package stream

import (
	"context"

	pbstream "github.com/hashicorp/nomad/nomad/stream/proto"
	"github.com/hashicorp/nomad/nomad/structs"
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

// func (s *SinkServer)

// input SubscribeRequest

// output stream Event
