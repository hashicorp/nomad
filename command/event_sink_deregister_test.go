package command

import (
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestEventCommand_EventSink_Deregister(t *testing.T) {
	t.Parallel()

	srv, client, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &EventSinkDeregisterCommand{Meta: Meta{Ui: ui}}

	sinks := client.EventSinks()
	require.NotNil(t, sinks)

	sink := &api.EventSink{
		ID:   "test-webhooksink",
		Type: api.SinkWebhook,
		Topics: map[api.Topic][]string{
			"*": {"*"},
		},
		Address:     "http://localhost:8080",
		LatestIndex: 0,
		CreateIndex: 0,
		ModifyIndex: 0,
	}
	wm, err := sinks.Register(sink, nil)
	require.NoError(t, err)
	require.NotZero(t, wm.LastIndex)

	code := cmd.Run([]string{"-address=" + url, "test-webhooksink"})
	require.Equal(t, "", ui.ErrorWriter.String())
	require.Equal(t, 0, code)
	require.Contains(t, ui.OutputWriter.String(), "Successfully deregistered \"test-webhooksink\" event sink!")

	es, qm, err := sinks.List(nil)
	require.NoError(t, err)
	require.NotZero(t, qm.LastIndex)
	require.Len(t, es, 0)
}
