package stream

import (
	"context"
	"net"
	"testing"
	"time"

	pbstream "github.com/hashicorp/nomad/nomad/stream/proto"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

func TestSinkServer_Subscribe(t *testing.T) {
	r := require.New(t)

	ctx := context.Background()
	broker := NewEventBroker(ctx, EventBrokerCfg{})
	events := &structs.Events{
		Index: 1,
		Events: []structs.Event{
			{
				Topic: structs.TopicAll,
				Key:   "some-job",
			},
		},
	}
	broker.Publish(events)

	addr := runTestServer(t, NewSinkServer(broker))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	conn, err := grpc.DialContext(ctx, addr.String(), grpc.WithInsecure())
	require.NoError(t, err)

	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Logf(err.Error())
		}
	})

	client := pbstream.NewEventStreamClient(conn)

	sub, err := client.Subscribe(ctx, &pbstream.SubscribeRequest{
		Index: 0,
		Topics: []*pbstream.TopicFilter{
			{
				Topic: pbstream.Topic_All,
			},
		},
	})

	r.NoError(err)

	eventBatch, err := sub.Recv()
	r.NoError(err)

	r.NotNil(eventBatch)
	r.Len(eventBatch.Event, 1)
	r.Equal("some-job", eventBatch.Event[0].Key)
}

func runTestServer(t *testing.T, server *SinkServer) net.Addr {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	pbstream.RegisterEventStreamServer(grpcServer, server)

	g := new(errgroup.Group)
	g.Go(func() error {
		return grpcServer.Serve(lis)
	})

	t.Cleanup(func() {
		grpcServer.Stop()
		if err := g.Wait(); err != nil {
			t.Logf("grpc server error: %v", err)
		}
	})

	return lis.Addr()
}
