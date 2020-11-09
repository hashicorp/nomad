package stream

import pbstream "github.com/hashicorp/nomad/nomad/stream/proto"

var _ pbstream.EventStreamServer = (*sinkServer)(nil)

type sinkServer struct {
	broker *EventBroker
}

func (s *sinkServer) Subscribe(subReq *pbstream.SubscribeRequest, serverStream pbstream.EventStream_SubscribeServer) error {

	if err := serverStream.Send(nil); err != nil {
		return err
	}
	return nil
}

// func (s *sinkServer)

// input SubscribeRequest

// output stream Event
