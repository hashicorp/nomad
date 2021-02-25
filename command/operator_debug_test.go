package command

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// NOTE: most of these tests cannot be run in parallel

type testCase struct {
	name            string
	args            []string
	expectedCode    int
	expectedOutputs []string
	expectedError   string
}

type testCases []testCase

func runTestCases(t *testing.T, cases testCases) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Setup mock UI
			ui := cli.NewMockUi()
			cmd := &OperatorDebugCommand{Meta: Meta{Ui: ui}}

			// Run test case
			code := cmd.Run(c.args)
			out := ui.OutputWriter.String()
			outerr := ui.ErrorWriter.String()

			// Verify case expectations
			require.Equalf(t, code, c.expectedCode, "expected exit code %d, got: %d: %s", c.expectedCode, code, outerr)
			for _, expectedOutput := range c.expectedOutputs {
				require.Contains(t, out, expectedOutput, "expected output %q, got %q", expectedOutput, out)
			}
			require.Containsf(t, outerr, c.expectedError, "expected error %q, got %q", c.expectedError, outerr)
		})
	}
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

	// Setup client 1 (nodeclass = clienta)
	agentConfFunc1 := func(c *agent.Config) {
		c.Region = "global"
		c.Server.Enabled = false
		c.Client.NodeClass = "clienta"
		c.Client.Enabled = true
		c.Client.Servers = []string{srvRPCAddr}
	}

	// Start client 1
	client1 := agent.NewTestAgent(t, "client1", agentConfFunc1)
	defer client1.Shutdown()

	// Wait for client1 to connect
	client1NodeID := client1.Agent.Client().NodeID()
	testutil.WaitForClient(t, srv.Agent.RPC, client1NodeID)
	t.Logf("[TEST] Client1 ready, id: %s", client1NodeID)

	// Setup client 2 (nodeclass = clientb)
	agentConfFunc2 := func(c *agent.Config) {
		c.Region = "global"
		c.Server.Enabled = false
		c.Client.NodeClass = "clientb"
		c.Client.Enabled = true
		c.Client.Servers = []string{srvRPCAddr}
	}

	// Start client 2
	client2 := agent.NewTestAgent(t, "client2", agentConfFunc2)
	defer client2.Shutdown()

	// Wait for client2 to connect
	client2NodeID := client2.Agent.Client().NodeID()
	testutil.WaitForClient(t, srv.Agent.RPC, client2NodeID)
	t.Logf("[TEST] Client2 ready, id: %s", client2NodeID)

	// Setup client 3 (nodeclass = clienta)
	agentConfFunc3 := func(c *agent.Config) {
		c.Server.Enabled = false
		c.Client.NodeClass = "clienta"
		c.Client.Servers = []string{srvRPCAddr}
	}

	// Start client 3
	client3 := agent.NewTestAgent(t, "client3", agentConfFunc3)
	defer client3.Shutdown()

	// Wait for client3 to connect
	client3NodeID := client3.Agent.Client().NodeID()
	testutil.WaitForClient(t, srv.Agent.RPC, client3NodeID)
	t.Logf("[TEST] Client3 ready, id: %s", client3NodeID)

	// Setup test cases
	cases := testCases{
		{
			name:         "address=api, node-class=clienta, max-nodes=2",
			args:         []string{"-address", url, "-duration", "250ms", "-server-id", "all", "-node-id", "all", "-node-class", "clienta", "-max-nodes", "2"},
			expectedCode: 0,
			expectedOutputs: []string{
				"Servers: (1/1)",
				"Clients: (2/3)",
				"Max node count reached (2)",
				"Node Class: clienta",
				"Created debug archive",
			},
			expectedError: "",
		},
		{
			name:         "address=api, node-class=clientb, max-nodes=2",
			args:         []string{"-address", url, "-duration", "250ms", "-server-id", "all", "-node-id", "all", "-node-class", "clientb", "-max-nodes", "2"},
			expectedCode: 0,
			expectedOutputs: []string{
				"Servers: (1/1)",
				"Clients: (1/3)",
				"Node Class: clientb",
				"Created debug archive",
			},
			expectedError: "",
		},
	}

	runTestCases(t, cases)
}

func TestDebug_ClientToServer(t *testing.T) {
	// Start test server and API client
	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	// Wait for leadership to establish
	testutil.WaitForLeader(t, srv.Agent.RPC)

	// Retrieve server RPC address to join client
	srvRPCAddr := srv.GetConfig().AdvertiseAddrs.RPC
	t.Logf("[TEST] Leader started, srv.GetConfig().AdvertiseAddrs.RPC: %s", srvRPCAddr)

	// Setup client 1 (nodeclass = clienta)
	agentConfFunc1 := func(c *agent.Config) {
		c.Region = "global"
		c.Server.Enabled = false
		c.Client.NodeClass = "clienta"
		c.Client.Enabled = true
		c.Client.Servers = []string{srvRPCAddr}
	}

	// Start client 1
	client1 := agent.NewTestAgent(t, "client1", agentConfFunc1)
	defer client1.Shutdown()

	// Wait for client 1 to connect
	client1NodeID := client1.Agent.Client().NodeID()
	testutil.WaitForClient(t, srv.Agent.RPC, client1NodeID)
	t.Logf("[TEST] Client1 ready, id: %s", client1NodeID)

	// Get API addresses
	addrServer := srv.HTTPAddr()
	addrClient1 := client1.HTTPAddr()

	t.Logf("[TEST] testAgent api address: %s", url)
	t.Logf("[TEST] Server    api address: %s", addrServer)
	t.Logf("[TEST] Client1   api address: %s", addrClient1)

	// Setup test cases
	var cases = testCases{
		{
			name:            "testAgent api server",
			args:            []string{"-address", url, "-duration", "250ms", "-server-id", "all", "-node-id", "all"},
			expectedCode:    0,
			expectedOutputs: []string{"Created debug archive"},
		},
		{
			name:            "server address",
			args:            []string{"-address", addrServer, "-duration", "250ms", "-server-id", "all", "-node-id", "all"},
			expectedCode:    0,
			expectedOutputs: []string{"Created debug archive"},
		},
		{
			name:            "client1 address - verify no SIGSEGV panic",
			args:            []string{"-address", addrClient1, "-duration", "250ms", "-server-id", "all", "-node-id", "all"},
			expectedCode:    0,
			expectedOutputs: []string{"Created debug archive"},
		},
	}

	runTestCases(t, cases)
}

func TestDebug_SingleServer(t *testing.T) {
	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()
	testutil.WaitForLeader(t, srv.Agent.RPC)

	var cases = testCases{
		{
			name:         "address=api, server-id=leader",
			args:         []string{"-address", url, "-duration", "250ms", "-server-id", "leader"},
			expectedCode: 0,
			expectedOutputs: []string{
				"Servers: (1/1)",
				"Clients: (0/0)",
				"Created debug archive",
			},
			expectedError: "",
		},
		{
			name:         "address=api, server-id=all",
			args:         []string{"-address", url, "-duration", "250ms", "-server-id", "all"},
			expectedCode: 0,
			expectedOutputs: []string{
				"Servers: (1/1)",
				"Clients: (0/0)",
				"Created debug archive",
			},
			expectedError: "",
		},
	}

	runTestCases(t, cases)
}

func TestDebug_Failures(t *testing.T) {
	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()
	testutil.WaitForLeader(t, srv.Agent.RPC)

	var cases = testCases{
		{
			name:         "fails incorrect args",
			args:         []string{"some", "bad", "args"},
			expectedCode: 1,
		},
		{
			name:         "Fails illegal node ids",
			args:         []string{"-node-id", "foo:bar"},
			expectedCode: 1,
		},
		{
			name:         "Fails missing node ids",
			args:         []string{"-node-id", "abc,def", "-duration", "250ms"},
			expectedCode: 1,
		},
		{
			name:         "Fails bad durations",
			args:         []string{"-duration", "foo"},
			expectedCode: 1,
		},
		{
			name:         "Fails bad intervals",
			args:         []string{"-interval", "bar"},
			expectedCode: 1,
		},
		{
			name:          "Fails bad address",
			args:          []string{"-address", url + "bogus"},
			expectedCode:  1,
			expectedError: "invalid address",
		},
	}

	runTestCases(t, cases)
}

func TestDebug_Bad_CSIPlugin_Names(t *testing.T) {
	// Start test server and API client
	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	// Wait for leadership to establish
	testutil.WaitForLeader(t, srv.Agent.RPC)

	cases := []string{
		"aws/ebs",
		"gcp-*-1",
	}
	for _, pluginName := range cases {
		cleanup := state.CreateTestCSIPlugin(srv.Agent.Server().State(), pluginName)
		defer cleanup()
	}

	// Setup mock UI
	ui := cli.NewMockUi()
	cmd := &OperatorDebugCommand{Meta: Meta{Ui: ui}}

	// Debug on the leader and all client nodes
	code := cmd.Run([]string{"-address", url, "-duration", "250ms", "-server-id", "leader", "-node-id", "all", "-output", os.TempDir()})
	assert.Equal(t, 0, code)

	// Bad plugin name should be escaped before it reaches the sandbox test
	require.NotContains(t, ui.ErrorWriter.String(), "file path escapes capture directory")
	require.Contains(t, ui.OutputWriter.String(), "Starting debugger")

	path := cmd.collectDir
	defer os.Remove(path)

	var pluginFiles []string
	for _, pluginName := range cases {
		pluginFile := fmt.Sprintf("csi-plugin-id-%s.json", helper.CleanFilename(pluginName, "_"))
		pluginFile = filepath.Join(path, "nomad", "0000", pluginFile)
		pluginFiles = append(pluginFiles, pluginFile)
	}

	testutil.WaitForFiles(t, pluginFiles)
}

func TestDebug_CapturedFiles(t *testing.T) {
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

func TestDebug_ExistingOutput(t *testing.T) {
	ui := cli.NewMockUi()
	cmd := &OperatorDebugCommand{Meta: Meta{Ui: ui}}

	// Fails existing output
	format := "2006-01-02-150405Z"
	stamped := "nomad-debug-" + time.Now().UTC().Format(format)
	path := filepath.Join(os.TempDir(), stamped)
	os.MkdirAll(path, 0755)
	defer os.Remove(path)

	code := cmd.Run([]string{"-output", os.TempDir(), "-duration", "50ms"})
	require.Equal(t, 2, code)
}

func TestDebug_Fail_Pprof(t *testing.T) {
	// Setup agent config with debug endpoints disabled
	agentConfFunc := func(c *agent.Config) {
		c.EnableDebug = false
	}

	// Start test server and API client
	srv, _, url := testServer(t, false, agentConfFunc)
	defer srv.Shutdown()

	// Wait for leadership to establish
	testutil.WaitForLeader(t, srv.Agent.RPC)

	// Setup mock UI
	ui := cli.NewMockUi()
	cmd := &OperatorDebugCommand{Meta: Meta{Ui: ui}}

	// Debug on client - node class = "clienta"
	code := cmd.Run([]string{"-address", url, "-duration", "250ms", "-server-id", "all"})

	assert.Equal(t, 0, code) // Pprof failure isn't fatal
	require.Contains(t, ui.OutputWriter.String(), "Starting debugger")
	require.Contains(t, ui.ErrorWriter.String(), "Failed to retrieve pprof") // Should report pprof failure
	require.Contains(t, ui.ErrorWriter.String(), "Permission denied")        // Specifically permission denied
	require.Contains(t, ui.OutputWriter.String(), "Created debug archive")   // Archive should be generated anyway
}

func TestDebug_Utils(t *testing.T) {
	t.Parallel()

	xs := argNodes("foo, bar")
	require.Equal(t, []string{"foo", "bar"}, xs)

	xs = argNodes("")
	require.Len(t, xs, 0)
	require.Empty(t, xs)

	// address calculation honors CONSUL_HTTP_SSL
	// ssl: true - Correct alignment
	e := &external{addrVal: "https://127.0.0.1:8500", ssl: true}
	addr := e.addr("foo")
	require.Equal(t, "https://127.0.0.1:8500", addr)

	// ssl: true - protocol incorrect
	e = &external{addrVal: "http://127.0.0.1:8500", ssl: true}
	addr = e.addr("foo")
	require.Equal(t, "https://127.0.0.1:8500", addr)

	// ssl: true - protocol missing
	e = &external{addrVal: "127.0.0.1:8500", ssl: true}
	addr = e.addr("foo")
	require.Equal(t, "https://127.0.0.1:8500", addr)

	// ssl: false - correct alignment
	e = &external{addrVal: "http://127.0.0.1:8500", ssl: false}
	addr = e.addr("foo")
	require.Equal(t, "http://127.0.0.1:8500", addr)

	// ssl: false - protocol incorrect
	e = &external{addrVal: "https://127.0.0.1:8500", ssl: false}
	addr = e.addr("foo")
	require.Equal(t, "http://127.0.0.1:8500", addr)

	// ssl: false - protocol missing
	e = &external{addrVal: "127.0.0.1:8500", ssl: false}
	addr = e.addr("foo")
	require.Equal(t, "http://127.0.0.1:8500", addr)
}

func TestDebug_WriteBytes_Nil(t *testing.T) {
	t.Parallel()

	var testDir, testFile, testPath string
	var testBytes []byte

	// Setup mock UI
	ui := cli.NewMockUi()
	cmd := &OperatorDebugCommand{Meta: Meta{Ui: ui}}

	testDir = os.TempDir()
	cmd.collectDir = testDir

	testFile = "test_nil.json"
	testPath = filepath.Join(testDir, testFile)
	defer os.Remove(testPath)

	// Write nil file at top level of collect directory
	err := cmd.writeBytes("", testFile, testBytes)
	require.NoError(t, err)
	require.FileExists(t, testPath)
}

func TestDebug_WriteBytes_PathEscapesSandbox(t *testing.T) {
	t.Parallel()

	var testDir, testFile string
	var testBytes []byte

	testDir = os.TempDir()
	defer os.Remove(testDir)

	testFile = "testing.json"
	testPath := filepath.Join(testDir, testFile)
	defer os.Remove(testPath)

	// Setup mock UI
	ui := cli.NewMockUi()
	cmd := &OperatorDebugCommand{Meta: Meta{Ui: ui}}

	// Empty collectDir will always appear to be escaped
	cmd.collectDir = ""
	err := cmd.writeBytes(testDir, testFile, testBytes)
	require.Error(t, err)
}
