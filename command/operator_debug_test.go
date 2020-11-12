package command

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDebugUtils(t *testing.T) {
	xs := argNodes("foo, bar")
	require.Equal(t, []string{"foo", "bar"}, xs)

	xs = argNodes("")
	require.Len(t, xs, 0)
	require.Empty(t, xs)

	// address calculation honors CONSUL_HTTP_SSL
	e := &external{addrVal: "http://127.0.0.1:8500", ssl: true}
	require.Equal(t, "https://127.0.0.1:8500", e.addr("foo"))

	e = &external{addrVal: "http://127.0.0.1:8500", ssl: false}
	require.Equal(t, "http://127.0.0.1:8500", e.addr("foo"))

	e = &external{addrVal: "127.0.0.1:8500", ssl: false}
	require.Equal(t, "http://127.0.0.1:8500", e.addr("foo"))

	e = &external{addrVal: "127.0.0.1:8500", ssl: true}
	require.Equal(t, "https://127.0.0.1:8500", e.addr("foo"))
}

func TestDebug_NodeClass(t *testing.T) {
	// Start test server and API client
	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	// Wait for leadership to establish
	testutil.WaitForLeader(t, srv.Agent.RPC)

	// Retrieve server RPC address to join clients
	srvRPCAddr := srv.GetConfig().AdvertiseAddrs.RPC
	t.Logf("[TEST] Leader started, srv.GetConfig().AdvertiseAddrs.RPC: %s", srvRPCAddr)

	// Setup Client 1 (nodeclass = clienta)
	agentConfFunc1 := func(c *agent.Config) {
		c.Region = "global"
		c.EnableDebug = true
		c.Server.Enabled = false
		c.Client.NodeClass = "clienta"
		c.Client.Enabled = true
		c.Client.Servers = []string{srvRPCAddr}
	}

	// Start Client 1
	client1 := agent.NewTestAgent(t, "client1", agentConfFunc1)
	defer client1.Shutdown()

	// Wait for the client to connect
	client1NodeID := client1.Agent.Client().NodeID()
	testutil.WaitForClient(t, srv.Agent.RPC, client1NodeID)
	t.Logf("[TEST] Client1 ready, id: %s", client1NodeID)

	// Setup Client 2 (nodeclass = clientb)
	agentConfFunc2 := func(c *agent.Config) {
		c.Region = "global"
		c.EnableDebug = true
		c.Server.Enabled = false
		c.Client.NodeClass = "clientb"
		c.Client.Enabled = true
		c.Client.Servers = []string{srvRPCAddr}
	}

	// Start Client 2
	client2 := agent.NewTestAgent(t, "client2", agentConfFunc2)
	defer client2.Shutdown()

	// Wait for the client to connect
	client2NodeID := client2.Agent.Client().NodeID()
	testutil.WaitForClient(t, srv.Agent.RPC, client2NodeID)
	t.Logf("[TEST] Client2 ready, id: %s", client2NodeID)

	// Setup Client 3 (nodeclass = clienta)
	agentConfFunc3 := func(c *agent.Config) {
		c.Server.Enabled = false
		c.EnableDebug = false
		c.Client.NodeClass = "clienta"
		c.Client.Servers = []string{srvRPCAddr}
	}

	// Start Client 3
	client3 := agent.NewTestAgent(t, "client3", agentConfFunc3)
	defer client3.Shutdown()

	// Wait for the client to connect
	client3NodeID := client3.Agent.Client().NodeID()
	testutil.WaitForClient(t, srv.Agent.RPC, client3NodeID)
	t.Logf("[TEST] Client3 ready, id: %s", client3NodeID)

	// Setup mock UI
	ui := cli.NewMockUi()
	cmd := &OperatorDebugCommand{Meta: Meta{Ui: ui}}

	// Debug on client - node class = "clienta"
	code := cmd.Run([]string{"-address", url, "-duration", "250ms", "-server-id", "all", "-node-id", "all", "-node-class", "clienta", "-max-nodes", "2"})

	assert.Equal(t, 0, code) // take note of failed return code, but continue to allow buffer content checks
	require.Empty(t, ui.ErrorWriter.String(), "errorwriter should be empty")
	require.Contains(t, ui.OutputWriter.String(), "Starting debugger")
	require.Contains(t, ui.OutputWriter.String(), "Node Class: clienta")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()
}

func TestDebugSuccesses(t *testing.T) {
	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()
	testutil.WaitForLeader(t, srv.Agent.RPC)

	ui := cli.NewMockUi()
	cmd := &OperatorDebugCommand{Meta: Meta{Ui: ui}}

	// NOTE -- duration must be shorter than default 2m to prevent testify from timing out

	// Debug on the leader
	code := cmd.Run([]string{"-address", url, "-duration", "250ms", "-server-id", "leader"})
	assert.Equal(t, 0, code) // take note of failed return code, but continue to see why
	assert.Empty(t, ui.ErrorWriter.String(), "errorwriter should be empty")
	require.Contains(t, ui.OutputWriter.String(), "Starting debugger")
	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Debug on all servers
	code = cmd.Run([]string{"-address", url, "-duration", "250ms", "-server-id", "all"})
	assert.Equal(t, 0, code)
	require.Empty(t, ui.ErrorWriter.String(), "errorwriter should be empty")
	require.Contains(t, ui.OutputWriter.String(), "Starting debugger")
	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()
}

func TestDebugFails(t *testing.T) {
	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()
	testutil.WaitForLeader(t, srv.Agent.RPC)

	ui := cli.NewMockUi()
	cmd := &OperatorDebugCommand{Meta: Meta{Ui: ui}}

	// Fails incorrect args
	code := cmd.Run([]string{"some", "bad", "args"})
	require.Equal(t, 1, code)
	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Fails illegal node ids
	code = cmd.Run([]string{"-node-id", "foo:bar"})
	require.Equal(t, 1, code)
	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Fails missing node ids
	code = cmd.Run([]string{"-node-id", "abc,def", "-duration", "250ms"})
	require.Equal(t, 1, code)
	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Fails bad durations
	code = cmd.Run([]string{"-duration", "foo"})
	require.Equal(t, 1, code)
	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Fails bad durations
	code = cmd.Run([]string{"-interval", "bar"})
	require.Equal(t, 1, code)
	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Fails existing output
	format := "2006-01-02-150405Z"
	stamped := "nomad-debug-" + time.Now().UTC().Format(format)
	path := filepath.Join(os.TempDir(), stamped)
	os.MkdirAll(path, 0755)
	defer os.Remove(path)
	// short duration to prevent timeout
	code = cmd.Run([]string{"-output", os.TempDir(), "-duration", "50ms"})
	require.Equal(t, 2, code)
	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Fails bad address
	code = cmd.Run([]string{"-address", url + "bogus"})
	assert.Equal(t, 1, code) // take note of failed return code, but continue to see why in the OutputWriter
	require.NotContains(t, ui.OutputWriter.String(), "Starting debugger")
	require.Contains(t, ui.ErrorWriter.String(), "invalid address")
	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()
}

func TestDebugCapturedFiles(t *testing.T) {
	// NOTE: pprof tracing/profiling cannot be run in parallel

	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()
	testutil.WaitForLeader(t, srv.Agent.RPC)

	ui := cli.NewMockUi()
	cmd := &OperatorDebugCommand{Meta: Meta{Ui: ui}}

	code := cmd.Run([]string{
		"-address", url,
		"-output", os.TempDir(),
		"-server-id", "leader",
		"-duration", "1300ms",
		"-interval", "600ms",
	})

	path := cmd.collectDir
	defer os.Remove(path)

	require.Empty(t, ui.ErrorWriter.String())
	require.Equal(t, 0, code)
	ui.ErrorWriter.Reset()

	serverFiles := []string{
		// Version is always captured
		filepath.Join(path, "version", "agent-self.json"),

		// Consul and Vault contain results or errors
		filepath.Join(path, "version", "consul-agent-self.json"),
		filepath.Join(path, "version", "vault-sys-health.json"),

		// Monitor files are only created when selected
		filepath.Join(path, "server", "leader", "monitor.log"),
		filepath.Join(path, "server", "leader", "profile.prof"),
		filepath.Join(path, "server", "leader", "trace.prof"),
		filepath.Join(path, "server", "leader", "goroutine.prof"),
		filepath.Join(path, "server", "leader", "goroutine-debug1.txt"),
		filepath.Join(path, "server", "leader", "goroutine-debug2.txt"),

		// Multiple snapshots are collected, 00 is always created
		filepath.Join(path, "nomad", "0000", "jobs.json"),
		filepath.Join(path, "nomad", "0000", "nodes.json"),
		filepath.Join(path, "nomad", "0000", "metrics.json"),

		// Multiple snapshots are collected, 01 requires two intervals
		filepath.Join(path, "nomad", "0001", "jobs.json"),
		filepath.Join(path, "nomad", "0001", "nodes.json"),
		filepath.Join(path, "nomad", "0001", "metrics.json"),
	}

	testutil.WaitForFiles(t, serverFiles)
}
