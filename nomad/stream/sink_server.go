package stream

import (
	"context"

	"github.com/davecgh/go-spew/spew"
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
	spew.Dump("HANDLER!!!!!!!!!!!!!!!")
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

	spew.Dump("SUB NEXT!!!!!!!!!!!!!!!")
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

	spew.Dump("SENTT!!!!!!!!!!!!!!!")
	err = serverStream.Send(eventBatch)
	if err != nil {
		return err
	}

	spew.Dump("RET!!!!!!!!!!!!!!!")
	return nil
}

// func (s *SinkServer)

// input SubscribeRequest

// output stream Event
