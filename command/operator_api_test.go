// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

// TestOperatorAPICommand_Paths asserts that the op api command normalizes
// various path formats to the proper full address.
func TestOperatorAPICommand_Paths(t *testing.T) {
	ci.Parallel(t)

	hits := make(chan *url.URL, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits <- r.URL
	}))
	defer ts.Close()

	// Always expect the same URL to be hit
	expected := "/v1/jobs"

	buf := bytes.NewBuffer(nil)
	ui := &cli.BasicUi{
		ErrorWriter: buf,
		Writer:      buf,
	}
	cmd := &OperatorAPICommand{Meta: Meta{Ui: ui}}

	// Assert that absolute paths are appended to the configured address
	exitCode := cmd.Run([]string{"-address=" + ts.URL, "/v1/jobs"})
	require.Zero(t, exitCode, buf.String())

	select {
	case hit := <-hits:
		require.Equal(t, expected, hit.String())
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for hit")
	}

	buf.Reset()

	// Assert that full URLs are used as-is even if an invalid address is
	// set.
	exitCode = cmd.Run([]string{"-address=ftp://127.0.0.2:1", ts.URL + "/v1/jobs"})
	require.Zero(t, exitCode, buf.String())

	select {
	case hit := <-hits:
		require.Equal(t, expected, hit.String())
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for hit")
	}

	buf.Reset()

	// Assert that URLs lacking a scheme are used even if an invalid
	// address is set.
	exitCode = cmd.Run([]string{"-address=ftp://127.0.0.2:1", ts.Listener.Addr().String() + "/v1/jobs"})
	require.Zero(t, exitCode, buf.String())

	select {
	case hit := <-hits:
		require.Equal(t, expected, hit.String())
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for hit")
	}
}

// TestOperatorAPICommand_Curl asserts that -dryrun outputs a valid curl
// command.
func TestOperatorAPICommand_Curl(t *testing.T) {
	ci.Parallel(t)

	buf := bytes.NewBuffer(nil)
	ui := &cli.BasicUi{
		ErrorWriter: buf,
		Writer:      buf,
	}
	cmd := &OperatorAPICommand{Meta: Meta{Ui: ui}}

	exitCode := cmd.Run([]string{
		"-dryrun",
		"-address=http://127.0.0.1:1",
		"-region=not even a valid region",
		`-filter=this == "that" or this != "foo"`,
		"-X", "POST",
		"-token=acl-token",
		"-H", "Some-Other-Header: ok",
		"/url",
	})
	require.Zero(t, exitCode, buf.String())

	expected := `curl \
  -X POST \
  -H 'Some-Other-Header: ok' \
  -H 'X-Nomad-Token: acl-token' \
  http://127.0.0.1:1/url?filter=this+%3D%3D+%22that%22+or+this+%21%3D+%22foo%22&region=not+even+a+valid+region
`
	require.Equal(t, expected, buf.String())
}

func Test_pathToURL(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name              string
		inputConfig       *api.Config
		inputPath         string
		expectedOutputURL string
	}{
		{
			name: "https address via config",
			inputConfig: &api.Config{
				Address:   "https://nomad.systems:4646",
				TLSConfig: &api.TLSConfig{},
			},
			inputPath:         "/v1/jobs",
			expectedOutputURL: "https://nomad.systems:4646/v1/jobs",
		},
		{
			name: "http address via config",
			inputConfig: &api.Config{
				Address:   "http://nomad.systems:4646",
				TLSConfig: &api.TLSConfig{},
			},
			inputPath:         "/v1/jobs",
			expectedOutputURL: "http://nomad.systems:4646/v1/jobs",
		},
		{
			name:              "https address via path",
			inputConfig:       api.DefaultConfig(),
			inputPath:         "https://nomad.systems:4646/v1/jobs",
			expectedOutputURL: "https://nomad.systems:4646/v1/jobs",
		},
		{
			name:              "http address via path",
			inputConfig:       api.DefaultConfig(),
			inputPath:         "http://nomad.systems:4646/v1/jobs",
			expectedOutputURL: "http://nomad.systems:4646/v1/jobs",
		},
		{
			name: "https inferred by tls config",
			inputConfig: &api.Config{
				Address: "http://127.0.0.1:4646",
				TLSConfig: &api.TLSConfig{
					CAPath: "/path/to/nowhere",
				},
			},
			inputPath:         "/v1/jobs",
			expectedOutputURL: "https://127.0.0.1:4646/v1/jobs",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput, err := pathToURL(tc.inputConfig, tc.inputPath)
			must.NoError(t, err)
			must.NotNil(t, actualOutput)
			must.Eq(t, actualOutput.String(), tc.expectedOutputURL)
		})
	}
}

// TestOperatorAPICommand_ContentLength tests that requests have the proper
// ContentLength set.
//
// Don't run it in parallel as it modifies the package's Stdin variable.
func TestOperatorAPICommand_ContentLength(t *testing.T) {
	contentLength := make(chan int, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentLength <- int(r.ContentLength)
	}))
	defer ts.Close()

	// Setup a temp file to act as stdin.
	input := []byte("test-input")
	fakeStdin, err := os.CreateTemp("", "fake-stdin")
	require.NoError(t, err)
	defer os.Remove(fakeStdin.Name())

	_, err = fakeStdin.Write(input)
	require.NoError(t, err)
	_, err = fakeStdin.Seek(0, 0)
	require.NoError(t, err)

	// Override the package's Stdin variable for testing.
	Stdin = fakeStdin
	defer func() { Stdin = os.Stdin }()

	// Setup command.
	buf := bytes.NewBuffer(nil)
	ui := &cli.BasicUi{
		ErrorWriter: buf,
		Writer:      buf,
	}
	cmd := &OperatorAPICommand{Meta: Meta{Ui: ui}}

	// Assert that a request has the expected content length.
	exitCode := cmd.Run([]string{"-address=" + ts.URL, "/v1/jobs"})
	require.Zero(t, exitCode, buf.String())

	select {
	case l := <-contentLength:
		require.Equal(t, len(input), l)
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for request")
	}
}
