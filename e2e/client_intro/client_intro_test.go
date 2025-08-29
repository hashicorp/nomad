// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package clientintro

import (
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/api"
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
	t.Run("testClientIntroEnforcementStrict", testClientIntroEnforcementStrict)
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
		"",
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
		"",
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
		wait.Timeout(30*time.Second),
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

func testClientIntroEnforcementStrict(t *testing.T) {

	// Find the Nomad binary that will be used for all Nomad agents in this
	// test.
	nomadBinary, err := discover.NomadExecutable()
	must.NoError(t, err)
	must.FileExists(t, nomadBinary)

	// Generate our server configuration file which sets the log level to error,
	// which ensures we include the client intro log lines.
	serverCallbackFn := func(c *execagent.AgentTemplateVars) {
		c.AgentName = "server-intro-" + uuid.Short()
		c.LogLevel = hclog.Error.String()
	}

	// Use our custom logger to capture the server output so we can inspect it
	// later.
	serverWriter := newCaptureLogger()

	extraCfg := `
server {
  client_introduction {
	enforcement = "strict"
  }
}`

	testServer, err := execagent.NewSingleModeAgent(
		nomadBinary,
		t.TempDir(),
		extraCfg,
		execagent.ModeServer,
		serverWriter,
		serverCallbackFn,
	)

	must.NoError(t, testServer.Start())
	t.Cleanup(func() { _ = testServer.Destroy() })

	//
	clientAgentName := "client-intro-" + uuid.Short()

	clientCallbackFn := func(c *execagent.AgentTemplateVars) {
		c.AgentName = clientAgentName
		c.LogLevel = hclog.Error.String()
		c.NodePool = "platform"
		c.RetryJoinAddrs = []string{"127.0.0.1" + ":" + strconv.Itoa(testServer.Vars.RPC)}
	}

	// Use our custom logger to capture the client output so we can inspect it
	// later.
	clientWriter := newCaptureLogger()

	testClient, err := execagent.NewSingleModeAgent(
		nomadBinary,
		t.TempDir(),
		"",
		execagent.ModeClient,
		clientWriter,
		clientCallbackFn,
	)
	must.NoError(t, err)
	must.NotNil(t, testClient)

	must.NoError(t, testClient.Start())
	t.Cleanup(func() { _ = testClient.Destroy() })

	// Wait for the client to log the expected error about being rejected due
	// to not having an intro token.
	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(func() error {

			// We need to lock the lines slice while we read it, as the client
			// agent is still running and writing to it.
			clientWriter.linesLock.RLock()
			defer clientWriter.linesLock.RUnlock()

			// Iterate the stored lines backwards, as the log line we are looking
			// for is likely to be towards the end of the output.
			for i := len(clientWriter.lines) - 1; i >= 0; i-- {
				if strings.Contains(
					clientWriter.lines[i],
					`client: error registering: error="rpc error: Permission denied"`,
				) {
					return nil
				}
			}
			return errors.New("did not find expected client intro log line")
		}),
		wait.Timeout(30*time.Second),
		wait.Gap(3*time.Second),
	))

	// We have confirmed the client in its current state cannot register with
	// the cluster, so destroy it.
	_ = testClient.Destroy()

	// The node has been rejected from registering which has been seen within
	// its logs, so now we need to check the server logs to ensure we saw the
	// expected error about the client joining without an intro token.
	must.SliceContainsFunc(
		t, serverWriter.lines,
		"[ERROR] nomad.client: node registration without introduction token",
		func(a string, b string) bool {
			return strings.Contains(a, b)
		},
	)

	// Get an API client, so we can create the introduction token that
	// the client will use.
	nomadClient, err := testServer.Client()
	must.NoError(t, err)
	must.NotNil(t, nomadClient)

	resp, _, err := nomadClient.ACLIdentity().CreateClientIntroductionToken(
		&api.ACLIdentityClientIntroductionTokenRequest{
			NodeName: clientAgentName,
			NodePool: "platform",
		},
		nil,
	)

	must.NoError(t, err)
	must.NotEq(t, "", resp.JWT)

	// Generate a new client agent, this time with the intro token, and start it.
	// It should be able to register successfully.
	newTestClient, err := execagent.NewSingleModeAgent(
		nomadBinary,
		t.TempDir(),
		"",
		execagent.ModeClient,
		clientWriter,
		clientCallbackFn,
	)
	must.NoError(t, err)
	must.NotNil(t, newTestClient)

	newTestClient.Cmd.Args = append(testClient.Cmd.Args, "-client-intro-token="+resp.JWT)

	must.NoError(t, newTestClient.Start())
	t.Cleanup(func() { _ = newTestClient.Destroy() })

	// Wait for the client to show up in the server's node list. We use the node
	// name as the identifier to check for since it's unique.
	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(func() error {
			nodeList, _, err := nomadClient.Nodes().List(nil)
			if err != nil {
				return err
			}
			for _, node := range nodeList {
				if node.Name == clientAgentName {
					return nil
				}
			}
			return errors.New("node not found")
		}),
		wait.Timeout(30*time.Second),
		wait.Gap(3*time.Second),
	))
}

// caputreLogger is a simple logger that captures log lines in memory and also
// writes them to stderr. It allows us to caputre output and inspect it for
// testing.
type captureLogger struct {
	lines     []string
	linesLock sync.RWMutex
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

	// Store the log line in memory for later inspection.
	c.linesLock.Lock()
	c.lines = append(c.lines, string(buf))
	c.linesLock.Unlock()

	return os.Stderr.Write(buf)
}
