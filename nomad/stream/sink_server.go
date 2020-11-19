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
		topic := topicFilter.GetTopic()
		topicKey := topic.String()
		if topic == pbstream.Topic_All {
			topicKey = "*"
		}

		if len(topicFilter.FilterKeys) == 0 {
			req.Topics[structs.Topic(topicKey)] = []string{"*"}
		} else {
			req.Topics[structs.Topic(topicKey)] = topicFilter.FilterKeys
		}
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
		pbe := &pbstream.Event{
			Topic: pbstream.Topic(pbstream.Topic_value[string(e.Topic)]),
			Type:  e.Type,
			Key:   e.Key,
		}

		switch p := e.Payload.(type) {
		case *structs.NodeStreamEvent:
			pbe.Payload = &pbstream.Event_NodeEvent{
				NodeEvent: &pbstream.NodeEvent{
					Node: &pbstream.Node{
						ID:         p.Node.ID,
						Datacenter: p.Node.Datacenter,
						Name:       p.Node.Name,
					},
				},
			}
		case *structs.DeploymentEvent:
			pbe.Payload = &pbstream.Event_DeploymentEvent{
				DeploymentEvent: &pbstream.DeploymentEvent{
					Deployment: &pbstream.Deployment{
						ID:    p.Deployment.ID,
						JobID: p.Deployment.JobID,
					},
				},
			}
		case *structs.EvalEvent:
			pbe.Payload = &pbstream.Event_EvaluationEvent{
				EvaluationEvent: &pbstream.EvaluationEvent{
					Evaluation: &pbstream.Evaluation{
						ID:    p.Eval.ID,
						JobID: p.Eval.JobID,
					},
				},
			}
		case *structs.JobEvent:
			pbe.Payload = &pbstream.Event_JobEvent{
				JobEvent: &pbstream.JobEvent{
					Job: &pbstream.Job{
						ID:   p.Job.ID,
						Name: p.Job.Name,
					},
				},
			}
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
