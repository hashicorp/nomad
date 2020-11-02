package command

import (
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestEventCommand_EventSink_List(t *testing.T) {
	t.Parallel()

	srv, client, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &EventSinkListCommand{Meta: Meta{Ui: ui}}

	code := cmd.Run([]string{"-address=" + url})
	require.Equal(t, 0, code)
	require.Contains(t, ui.OutputWriter.String(), "No event sinks found")

	// Add a sink
	sinkClient := client.EventSinks()
	require.NotNil(t, sinkClient)

	sink := &api.EventSink{
		ID:   "test-webhooksink",
		Type: api.SinkWebhook,
		Topics: map[api.Topic][]string{
			"*":          {"*"},
			"Eval":       {"*"},
			"Deployment": {"redis"},
		},
		Address:     "http://localhost:8080",
		LatestIndex: 0,
		CreateIndex: 0,
		ModifyIndex: 0,
	}
	wm, err := sinkClient.Register(sink, nil)
	require.NoError(t, err)
	require.NotZero(t, wm.LastIndex)

	sink2 := &api.EventSink{
		ID:   "other-webhook",
		Type: api.SinkWebhook,
		Topics: map[api.Topic][]string{
			"Deployment": {"nginx", "redis"},
			"Node":       {"a46a8776-e0a3-40ee-a79a-51684145b170"},
		},
		Address:     "http://localhost:8080",
		LatestIndex: 0,
		CreateIndex: 0,
		ModifyIndex: 0,
	}

	wm2, err := sinkClient.Register(sink2, nil)
	require.NoError(t, err)
	require.Greater(t, wm2.LastIndex, wm.LastIndex)

	ui.OutputWriter.Reset()

	code = cmd.Run([]string{"-address=" + url})
	require.Equal(t, 0, code)
	require.NotContains(t, ui.OutputWriter.String(), "No event sinks found")

	got := ui.OutputWriter.String()

	// First Sink
	require.Contains(t, got, "test-webhooksink")
	require.Contains(t, got, sink.Type)
	require.Contains(t, got, sink.Address)
	require.Contains(t, got, "*[*],Deployment[redis],Eval[*]")

	// Second Sink
	require.Contains(t, got, "other-webhook")
	require.Contains(t, got, sink2.Type)
	require.Contains(t, got, sink2.Address)
	require.Contains(t, got, "Deployment[nginx redis],Node[a46a8776-e0a3-40ee-a79a-51684145b170]")
}
