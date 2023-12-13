// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package apitests

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/stretchr/testify/require"
)

// TestExecStreamingInputIsInSync asserts that a rountrip of exec streaming input doesn't lose any data
func TestExecStreamingInputIsInSync(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name  string
		input api.ExecStreamingInput
	}{
		{
			"stdin_data",
			api.ExecStreamingInput{Stdin: &api.ExecStreamingIOOperation{Data: []byte("hello there")}},
		},
		{
			"stdin_close",
			api.ExecStreamingInput{Stdin: &api.ExecStreamingIOOperation{Close: true}},
		},
		{
			"tty_size",
			api.ExecStreamingInput{TTYSize: &api.TerminalSize{Height: 10, Width: 20}},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b, err := json.Marshal(c.input)
			require.NoError(t, err)

			var proto drivers.ExecTaskStreamingRequestMsg
			err = json.Unmarshal(b, &proto)
			require.NoError(t, err)

			protoB, err := json.Marshal(proto)
			require.NoError(t, err)

			var roundtrip api.ExecStreamingInput
			err = json.Unmarshal(protoB, &roundtrip)
			require.NoError(t, err)

			require.EqualValues(t, c.input, roundtrip)
		})
	}
}

// TestExecStreamingOutputIsInSync asserts that a rountrip of exec streaming input doesn't lose any data
func TestExecStreamingOutputIsInSync(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name  string
		input api.ExecStreamingOutput
	}{
		{
			"stdout_data",
			api.ExecStreamingOutput{Stdout: &api.ExecStreamingIOOperation{Data: []byte("hello there")}},
		},
		{
			"stdout_close",
			api.ExecStreamingOutput{Stdout: &api.ExecStreamingIOOperation{Close: true}},
		},
		{
			"stderr_data",
			api.ExecStreamingOutput{Stderr: &api.ExecStreamingIOOperation{Data: []byte("hello there")}},
		},
		{
			"stderr_close",
			api.ExecStreamingOutput{Stderr: &api.ExecStreamingIOOperation{Close: true}},
		},
		{
			"exited",
			api.ExecStreamingOutput{Exited: true, Result: &api.ExecStreamingExitResult{ExitCode: 21}},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b, err := json.Marshal(c.input)
			require.NoError(t, err)

			var proto drivers.ExecTaskStreamingResponseMsg
			err = json.Unmarshal(b, &proto)
			require.NoError(t, err)

			protoB, err := json.Marshal(proto)
			require.NoError(t, err)

			var roundtrip api.ExecStreamingOutput
			err = json.Unmarshal(protoB, &roundtrip)
			require.NoError(t, err)

			require.EqualValues(t, c.input, roundtrip)
		})
	}
}
