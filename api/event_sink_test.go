package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEventSinks_List(t *testing.T) {
	t.Parallel()

	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	eventsinks := c.EventSinks()

	// create an event sink
	sink := &EventSink{
		ID:   "testwebhook",
		Type: SinkWebhook,
		Topics: map[Topic][]string{
			"Eval": {"*"},
		},
		Address: "http://localhost:8080",
	}

	wm, err := eventsinks.Register(sink, &WriteOptions{})
	require.NoError(t, err)
	require.NotZero(t, wm.LastIndex)

	list, qm, err := eventsinks.List(nil)
	require.NoError(t, err)

	require.NotZero(t, qm.LastIndex)

	require.Len(t, list, 1)
	require.Equal(t, "testwebhook", list[0].ID)
	require.Equal(t, SinkWebhook, list[0].Type)
	require.Equal(t, sink.Topics, list[0].Topics)
	require.Equal(t, sink.Address, list[0].Address)
}

func TestEventSinks_Deregister(t *testing.T) {
	t.Parallel()

	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	eventsinks := c.EventSinks()

	// create an event sink
	sink := &EventSink{
		ID:   "testwebhook",
		Type: SinkWebhook,
		Topics: map[Topic][]string{
			"Eval": {"*"},
		},
		Address: "http://localhost:8080",
	}

	wm, err := eventsinks.Register(sink, nil)
	require.NoError(t, err)
	require.NotZero(t, wm.LastIndex)

	wm, err = eventsinks.Deregister("testwebhook", nil)
	require.NoError(t, err)
	require.NotZero(t, wm.LastIndex)

	list, qm, err := eventsinks.List(nil)
	require.NoError(t, err)
	require.NotZero(t, qm.LastIndex)
	require.Len(t, list, 0)
}
