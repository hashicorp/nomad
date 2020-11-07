package command

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestEventCommand_EventSink_Register(t *testing.T) {
	t.Parallel()

	srv, client, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &EventSinkRegisterCommand{Meta: Meta{Ui: ui}}

	file, err := ioutil.TempFile("", t.Name())
	require.NoError(t, err)
	defer os.Remove(file.Name())

	sink := &api.EventSink{
		ID:   "test-webhooksink",
		Type: api.SinkWebhook,
		Topics: map[api.Topic][]string{
			"*": {"*"},
		},
		Address: "http://localhost:8080",
	}

	jsonBytes, err := json.Marshal(sink)
	require.NoError(t, err)

	err = ioutil.WriteFile(file.Name(), jsonBytes, 0700)
	require.NoError(t, err)
	require.NoError(t, file.Close())

	code := cmd.Run([]string{"-address=" + url, file.Name()})
	require.Equal(t, "", ui.ErrorWriter.String())
	require.Equal(t, 0, code)
	require.Contains(t, ui.OutputWriter.String(), "Successfully registered \"test-webhooksink\" event sink!")

	sinks := client.EventSinks()
	require.NotNil(t, sinks)

	es, qm, err := sinks.List(nil)
	require.NoError(t, err)
	require.NotZero(t, qm.LastIndex)
	require.Len(t, es, 1)
}

func TestEventCommand_EventSink_Register_FromStdin(t *testing.T) {
	t.Parallel()

	srv, client, url := testServer(t, false, nil)
	defer srv.Shutdown()

	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	ui := cli.NewMockUi()
	cmd := &EventSinkRegisterCommand{
		testStdin: stdinR,
		Meta:      Meta{Ui: ui},
	}

	sink := &api.EventSink{
		ID:   "test-webhooksink",
		Type: api.SinkWebhook,
		Topics: map[api.Topic][]string{
			"*": {"*"},
		},
		Address: "http://localhost:8080",
	}

	jsonBytes, err := json.Marshal(sink)
	require.NoError(t, err)

	go func() {
		stdinW.Write(jsonBytes)
		stdinW.Close()
	}()

	code := cmd.Run([]string{"-address=" + url, "-"})
	require.Equal(t, "", ui.ErrorWriter.String())
	require.Equal(t, 0, code)
	require.Contains(t, ui.OutputWriter.String(), "Successfully registered \"test-webhooksink\" event sink!")

	sinks := client.EventSinks()
	require.NotNil(t, sinks)

	es, qm, err := sinks.List(nil)
	require.NoError(t, err)
	require.NotZero(t, qm.LastIndex)
	require.Len(t, es, 1)
}
