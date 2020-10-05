package stream

import (
	"context"
	"github.com/hashicorp/nomad/nomad/structs"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEventPublisher_PublishChangesAndSubscribe(t *testing.T) {
	subscription := &SubscribeRequest{
		Topics: map[structs.Topic][]string{
			"Test": []string{"sub-key"},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	publisher := NewEventPublisher(ctx, EventPublisherCfg{EventBufferSize: 100, EventBufferTTL: DefaultTTL})
	sub, err := publisher.Subscribe(subscription)
	require.NoError(t, err)
	eventCh := consumeSubscription(ctx, sub)

	// Now subscriber should block waiting for updates
	assertNoResult(t, eventCh)

	events := []structs.Event{{
		Index:   1,
		Topic:   "Test",
		Key:     "sub-key",
		Payload: "sample payload",
	}}
	publisher.Publish(structs.Events{Index: 1, Events: events})

	// Subscriber should see the published event
	result := nextResult(t, eventCh)
	require.NoError(t, result.Err)
	expected := []structs.Event{{Payload: "sample payload", Key: "sub-key", Topic: "Test", Index: 1}}
	require.Equal(t, expected, result.Events)

	// Now subscriber should block waiting for updates
	assertNoResult(t, eventCh)

	// Publish a second event
	events = []structs.Event{{
		Index:   2,
		Topic:   "Test",
		Key:     "sub-key",
		Payload: "sample payload 2",
	}}
	publisher.Publish(structs.Events{Index: 2, Events: events})

	result = nextResult(t, eventCh)
	require.NoError(t, result.Err)
	expected = []structs.Event{{Payload: "sample payload 2", Key: "sub-key", Topic: "Test", Index: 2}}
	require.Equal(t, expected, result.Events)
}

func TestEventPublisher_ShutdownClosesSubscriptions(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	publisher := NewEventPublisher(ctx, EventPublisherCfg{})

	sub1, err := publisher.Subscribe(&SubscribeRequest{})
	require.NoError(t, err)
	defer sub1.Unsubscribe()

	sub2, err := publisher.Subscribe(&SubscribeRequest{})
	require.NoError(t, err)
	defer sub2.Unsubscribe()

	cancel() // Shutdown

	err = consumeSub(context.Background(), sub1)
	require.Equal(t, err, ErrSubscriptionClosed)

	_, err = sub2.Next(context.Background())
	require.Equal(t, err, ErrSubscriptionClosed)
}

func consumeSubscription(ctx context.Context, sub *Subscription) <-chan subNextResult {
	eventCh := make(chan subNextResult, 1)
	go func() {
		for {
			es, err := sub.Next(ctx)
			eventCh <- subNextResult{
				Events: es.Events,
				Err:    err,
			}
			if err != nil {
				return
			}
		}
	}()
	return eventCh
}

type subNextResult struct {
	Events []structs.Event
	Err    error
}

func nextResult(t *testing.T, eventCh <-chan subNextResult) subNextResult {
	t.Helper()
	select {
	case next := <-eventCh:
		return next
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("no event after 100ms")
	}
	return subNextResult{}
}

func assertNoResult(t *testing.T, eventCh <-chan subNextResult) {
	t.Helper()
	select {
	case next := <-eventCh:
		require.NoError(t, next.Err)
		require.Len(t, next.Events, 1)
		t.Fatalf("received unexpected event: %#v", next.Events[0].Payload)
	case <-time.After(100 * time.Millisecond):
	}
}

func consumeSub(ctx context.Context, sub *Subscription) error {
	for {
		_, err := sub.Next(ctx)
		if err != nil {
			return err
		}
	}
}
