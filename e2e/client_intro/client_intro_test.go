// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package clientintro

import (
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/e2e/execagent"
	"github.com/hashicorp/nomad/helper/discover"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

const (
	envGate = "NOMAD_E2E_CLIENT_INTRO"
)

func TestClientIntro(t *testing.T) {

	// This test is gated behind an environment variable so it does not run when
	// the e2e suite is triggered by CI.
	if os.Getenv(envGate) != "1" {
		t.Skip(envGate + " is not set; skipping")
	}

	t.Run("testClientIntroEnforcementWarn", testClientIntroEnforcementWarn)
}

func testClientIntroEnforcementWarn(t *testing.T) {

	// Find the Nomad binary that will be used for all Nomad agents in this
	// test.
	nomadBinary, err := discover.NomadExecutable()
	must.NoError(t, err)
	must.FileExists(t, nomadBinary)

	// Generate our server configuration file which sets the log level to warn,
	// which ensures we include the client intro log lines.
	serverCallbackFn := func(c *execagent.AgentTemplateVars) {
		c.AgentName = "server-intro-" + uuid.Short()
		c.LogLevel = hclog.Warn.String()
	}

	// Use our custom logger to capture the server output so we can inspect it
	// later.
	serverWriter := newCaptureLogger()

	testServer, err := execagent.NewSingleModeAgent(
		nomadBinary,
		t.TempDir(),
		execagent.ModeServer,
		serverWriter,
		serverCallbackFn,
	)

	must.NoError(t, testServer.Start())
	t.Cleanup(func() { _ = testServer.Destroy() })

	clientCallbackFn := func(c *execagent.AgentTemplateVars) {
		c.AgentName = "client-intro-" + uuid.Short()
		c.LogLevel = hclog.Warn.String()
		c.RetryJoinAddrs = []string{"127.0.0.1" + ":" + strconv.Itoa(testServer.Vars.RPC)}
	}

	clientWriter := io.Writer(os.Stderr)

	testClient, err := execagent.NewSingleModeAgent(
		nomadBinary,
		t.TempDir(),
		execagent.ModeClient,
		clientWriter,
		clientCallbackFn,
	)
	must.NoError(t, err)
	must.NotNil(t, testClient)

	must.NoError(t, testClient.Start())
	t.Cleanup(func() { _ = testClient.Destroy() })

	// Create a Nomad API client to talk to the server. Do it here, so we only
	// do this once.
	nomadClient, err := testServer.Client()
	must.NoError(t, err)
	must.NotNil(t, nomadClient)

	// Wait for the client to show up in the server's node list. We use the node
	// name as the identifier to check for since it's unique.
	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(func() error {
			nodeList, _, err := nomadClient.Nodes().List(nil)
			if err != nil {
				return err
			}
			for _, node := range nodeList {
				if node.Name == testClient.Vars.AgentName {
					return nil
				}
			}
			return errors.New("node not found")
		}),
		wait.Timeout(20*time.Second),
		wait.Gap(3*time.Second),
	))

	// If we reached this point, we are going to be reading the log lines, so
	// we can now destroy the agents to avoid race conditions.
	_ = testServer.Destroy()
	_ = testClient.Destroy()

	// The node has joined the cluster, now we need to check the server logs to
	// ensurewe saw the expected warning about the client joining without an
	// intro token.
	must.SliceContainsFunc(
		t, serverWriter.lines,
		"[WARN]  nomad.client: node registration without introduction token",
		func(a string, b string) bool {
			return strings.Contains(a, b)
		},
	)
}

// caputreLogger is a simple logger that captures log lines in memory and also
// writes them to stderr. It allows us to caputre output and inspect it for
// testing.
type captureLogger struct {
	lines []string
}

func newCaptureLogger() *captureLogger {
	return &captureLogger{
		lines: make([]string, 0),
	}
}

func (c *captureLogger) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	totalLength := len(p)
	buf := make([]byte, 0, totalLength)
	buf = append(buf, p...)

	c.lines = append(c.lines, string(buf))

	return os.Stderr.Write(buf)
}
