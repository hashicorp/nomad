package stream

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/mock"
	pbstream "github.com/hashicorp/nomad/nomad/stream/proto"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestSinkServer_Subscribe(t *testing.T) {
	r := require.New(t)

	ctx := context.Background()
	broker := NewEventBroker(ctx, EventBrokerCfg{})
	events := &structs.Events{
		Index: 1,
		Events: []structs.Event{
			{
				Topic: structs.TopicDeployment,
				Key:   "some-job",
				Payload: structs.DeploymentEvent{
					Deployment: mock.Deployment(),
				},
			},
		},
	}
	broker.Publish(events)
	events = &structs.Events{
		Index: 2,
		Events: []structs.Event{
			{
				Topic: structs.TopicDeployment,
				Key:   "some-key",
				Payload: structs.DeploymentEvent{
					Deployment: mock.Deployment(),
				},
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
	r.Equal(int(eventBatch.Index), 1)
	r.Equal("some-job", eventBatch.Event[0].Key)

	eventBatch, err = sub.Recv()
	r.NoError(err)

	r.NotNil(eventBatch)
	r.Len(eventBatch.Event, 1)
	r.Equal(int(eventBatch.Index), 2)
	r.Equal("some-key", eventBatch.Event[0].Key)

	// shutdown assertions
	for i := 0; i < 3; i++ {
		events := &structs.Events{
			Index: uint64(10 + i),
			Events: []structs.Event{
				{
					Topic: structs.TopicDeployment,
					Key:   "some-job",
					Payload: structs.DeploymentEvent{
						Deployment: mock.Deployment(),
					},
				},
			},
		}
		broker.Publish(events)
	}

	// Stop our subscription
	cancel()

	// Ensure the code is what we expect
	_, err = sub.Recv()
	r.Error(err)
	r.Equal(status.Code(err), codes.Canceled)

	// Wait for subscriptions to be unsubscribed
	testutil.WaitForResult(func() (bool, error) {
		ok := assert.Len(t, broker.subscriptions.byToken[""], 0)
		if ok {
			return true, nil
		}
		return false, fmt.Errorf("expected broker subscriptions len to be 0")
	}, func(err error) {
		r.Fail(err.Error())
	})
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
