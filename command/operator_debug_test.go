// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	consultest "github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	clienttest "github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/pointer"
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
			ui := cli.NewMockUi()
			cmd := &OperatorDebugCommand{Meta: Meta{Ui: ui}}

			code := cmd.Run(c.args)
			out := ui.OutputWriter.String()
			outerr := ui.ErrorWriter.String()

			assert.Equalf(t, c.expectedCode, code, "did not get expected exit code")

			if len(c.expectedOutputs) > 0 {
				if assert.NotEmpty(t, out, "command output was empty") {
					for _, expectedOutput := range c.expectedOutputs {
						assert.Contains(t, out, expectedOutput, "did not get expected output")
					}
				}
			} else {
				assert.Empty(t, out, "command output should have been empty")
			}

			if c.expectedError == "" {
				assert.Empty(t, outerr, "got unexpected error")
			} else {
				assert.Containsf(t, outerr, c.expectedError, "did not get expected error")
			}
		})
	}
}
func newClientAgentConfigFunc(region string, nodeClass string, srvRPCAddr string) func(*agent.Config) {
	if region == "" {
		region = "global"
	}

	return func(c *agent.Config) {
		c.Region = region
		c.Client.NodeClass = nodeClass
		c.Client.Servers = []string{srvRPCAddr}
		c.Client.Enabled = true
		c.Server.Enabled = false
	}
}

func TestDebug_NodeClass(t *testing.T) {

	// Start test server and API client
	srv, _, url := testServer(t, false, nil)

	// Wait for leadership to establish
	testutil.WaitForLeader(t, srv.Agent.RPC)

	// Retrieve server RPC address to join clients
	srvRPCAddr := srv.GetConfig().AdvertiseAddrs.RPC
	t.Logf("Leader started, srv.GetConfig().AdvertiseAddrs.RPC: %s", srvRPCAddr)

	// Start test clients
	testClient(t, "client1", newClientAgentConfigFunc("global", "classA", srvRPCAddr))
	testClient(t, "client2", newClientAgentConfigFunc("global", "classB", srvRPCAddr))
	testClient(t, "client3", newClientAgentConfigFunc("global", "classA", srvRPCAddr))

	// Setup test cases
	cases := testCases{
		{
			name:         "address=api, node-class=classA, max-nodes=2",
			args:         []string{"-address", url, "-duration", "250ms", "-interval", "250ms", "-server-id", "all", "-node-id", "all", "-node-class", "classA", "-max-nodes", "2"},
			expectedCode: 0,
			expectedOutputs: []string{
				"Servers: (1/1)",
				"Clients: (2/3)",
				"Max node count reached (2)",
				"Node Class: classA",
				"Created debug archive",
			},
			expectedError: "",
		},
		{
			name:         "address=api, node-class=classB, max-nodes=2",
			args:         []string{"-address", url, "-duration", "250ms", "-interval", "250ms", "-server-id", "all", "-node-id", "all", "-node-class", "classB", "-max-nodes", "2"},
			expectedCode: 0,
			expectedOutputs: []string{
				"Servers: (1/1)",
				"Clients: (1/3)",
				"Node Class: classB",
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

	// Wait for leadership to establish
	testutil.WaitForLeader(t, srv.Agent.RPC)

	// Retrieve server RPC address to join client
	srvRPCAddr := srv.GetConfig().AdvertiseAddrs.RPC
	t.Logf("Leader started, srv.GetConfig().AdvertiseAddrs.RPC: %s", srvRPCAddr)

	// Start client
	agent1, _, _ := testClient(t, "client1", newClientAgentConfigFunc("", "", srvRPCAddr))

	// Get API addresses
	addrServer := srv.HTTPAddr()
	addrClient1 := agent1.HTTPAddr()

	t.Logf("testAgent api address: %s", url)
	t.Logf("Server    api address: %s", addrServer)
	t.Logf("Client1   api address: %s", addrClient1)

	// Setup test cases
	var cases = testCases{
		{
			name:            "testAgent api server",
			args:            []string{"-address", url, "-duration", "250ms", "-interval", "250ms", "-server-id", "all", "-node-id", "all"},
			expectedCode:    0,
			expectedOutputs: []string{"Created debug archive"},
		},
		{
			name:            "server address",
			args:            []string{"-address", addrServer, "-duration", "250ms", "-interval", "250ms", "-server-id", "all", "-node-id", "all"},
			expectedCode:    0,
			expectedOutputs: []string{"Created debug archive"},
		},
		{
			name:            "client1 address - verify no SIGSEGV panic",
			args:            []string{"-address", addrClient1, "-duration", "250ms", "-interval", "250ms", "-server-id", "all", "-node-id", "all"},
			expectedCode:    0,
			expectedOutputs: []string{"Created debug archive"},
		},
	}

	runTestCases(t, cases)
}

func TestDebug_MultiRegion(t *testing.T) {

	region1 := "region1"
	region2 := "region2"

	// Start region1 server
	server1, _, addrServer1 := testServer(t, false, func(c *agent.Config) { c.Region = region1 })
	testutil.WaitForLeader(t, server1.Agent.RPC)
	rpcAddrServer1 := server1.GetConfig().AdvertiseAddrs.RPC
	t.Logf("%s: Leader started, HTTPAddr: %s, RPC: %s", region1, addrServer1, rpcAddrServer1)

	// Start region1 client
	agent1, _, addrClient1 := testClient(t, "client1", newClientAgentConfigFunc(region1, "", rpcAddrServer1))
	nodeIdClient1 := agent1.Agent.Client().NodeID()
	t.Logf("%s: Client1 started, ID: %s, HTTPAddr: %s", region1, nodeIdClient1, addrClient1)

	// Start region2 server
	server2, _, addrServer2 := testServer(t, false, func(c *agent.Config) { c.Region = region2 })
	testutil.WaitForLeader(t, server2.Agent.RPC)
	rpcAddrServer2 := server2.GetConfig().AdvertiseAddrs.RPC
	t.Logf("%s: Leader started, HTTPAddr: %s, RPC: %s", region2, addrServer2, rpcAddrServer2)

	// Start client2
	agent2, _, addrClient2 := testClient(t, "client2", newClientAgentConfigFunc(region2, "", rpcAddrServer2))
	nodeIdClient2 := agent2.Agent.Client().NodeID()
	t.Logf("%s: Client1 started, ID: %s, HTTPAddr: %s", region2, nodeIdClient2, addrClient2)

	t.Logf("Region: %s, Server1   api address: %s", region1, addrServer1)
	t.Logf("Region: %s, Client1   api address: %s", region1, addrClient1)
	t.Logf("Region: %s, Server2   api address: %s", region2, addrServer2)
	t.Logf("Region: %s, Client2   api address: %s", region2, addrClient2)

	// Setup test cases
	var cases = testCases{
		// Good
		{
			name:            "no region - all servers, all clients",
			args:            []string{"-address", addrServer1, "-duration", "250ms", "-interval", "250ms", "-server-id", "all", "-node-id", "all"},
			expectedCode:    0,
			expectedOutputs: []string{"Starting debugger"},
		},
		{
			name:         "region1 - server1 address",
			args:         []string{"-address", addrServer1, "-region", region1, "-duration", "50ms", "-interval", "50ms", "-server-id", "all", "-node-id", "all"},
			expectedCode: 0,
			expectedOutputs: []string{
				"Region: " + region1 + "\n",
				"Servers: (1/1) [TestDebug_MultiRegion.region1]",
				"Clients: (1/1) [" + nodeIdClient1 + "]",
				"Created debug archive",
			},
		},
		{
			name:         "region1 - client1 address",
			args:         []string{"-address", addrClient1, "-region", region1, "-duration", "50ms", "-interval", "50ms", "-server-id", "all", "-node-id", "all"},
			expectedCode: 0,
			expectedOutputs: []string{
				"Region: " + region1 + "\n",
				"Servers: (1/1) [TestDebug_MultiRegion.region1]",
				"Clients: (1/1) [" + nodeIdClient1 + "]",
				"Created debug archive",
			},
		},
		{
			name:         "region2 - server2 address",
			args:         []string{"-address", addrServer2, "-region", region2, "-duration", "50ms", "-interval", "50ms", "-server-id", "all", "-node-id", "all"},
			expectedCode: 0,
			expectedOutputs: []string{
				"Region: " + region2 + "\n",
				"Servers: (1/1) [TestDebug_MultiRegion.region2]",
				"Clients: (1/1) [" + nodeIdClient2 + "]",
				"Created debug archive",
			},
		},
		{
			name:         "region2 - client2 address",
			args:         []string{"-address", addrClient2, "-region", region2, "-duration", "50ms", "-interval", "50ms", "-server-id", "all", "-node-id", "all"},
			expectedCode: 0,
			expectedOutputs: []string{
				"Region: " + region2 + "\n",
				"Servers: (1/1) [TestDebug_MultiRegion.region2]",
				"Clients: (1/1) [" + nodeIdClient2 + "]",
				"Created debug archive",
			},
		},

		// Bad
		{
			name:          "invalid region - all servers, all clients",
			args:          []string{"-address", addrServer1, "-region", "never", "-duration", "50ms", "-interval", "50ms", "-server-id", "all", "-node-id", "all"},
			expectedCode:  1,
			expectedError: "500 (No path to region)",
		},
	}

	runTestCases(t, cases)
}

func TestDebug_SingleServer(t *testing.T) {

	srv, _, url := testServer(t, false, nil)
	testutil.WaitForLeader(t, srv.Agent.RPC)

	var cases = testCases{
		{
			name:         "address=api, server-id=leader",
			args:         []string{"-address", url, "-duration", "250ms", "-interval", "250ms", "-server-id", "leader"},
			expectedCode: 0,
			expectedOutputs: []string{
				"Servers: (1/1)",
				"Clients: (0/0)",
				"Created debug archive",
			},
			expectedError: "No node(s) with prefix",
		},
		{
			name:         "address=api, server-id=all",
			args:         []string{"-address", url, "-duration", "250ms", "-interval", "250ms", "-server-id", "all"},
			expectedCode: 0,
			expectedOutputs: []string{
				"Servers: (1/1)",
				"Clients: (0/0)",
				"Created debug archive",
			},
			expectedError: "No node(s) with prefix",
		},
	}

	runTestCases(t, cases)
}

func TestDebug_Failures(t *testing.T) {

	srv, _, url := testServer(t, false, nil)
	testutil.WaitForLeader(t, srv.Agent.RPC)

	var cases = testCases{
		{
			name:          "fails incorrect args",
			args:          []string{"some", "bad", "args"},
			expectedCode:  1,
			expectedError: "This command takes no arguments",
		},
		{
			name:          "Fails illegal node ids",
			args:          []string{"-node-id", "foo:bar"},
			expectedCode:  1,
			expectedError: "Error querying node info",
		},
		{
			name:          "Fails missing node ids",
			args:          []string{"-node-id", "abc,def", "-duration", "250ms", "-interval", "250ms"},
			expectedCode:  1,
			expectedError: "Error querying node info",
		},
		{
			name:          "Fails bad durations",
			args:          []string{"-duration", "foo"},
			expectedCode:  1,
			expectedError: "Error parsing duration: foo: time: invalid duration \"foo\""},
		{
			name:          "Fails bad intervals",
			args:          []string{"-interval", "bar"},
			expectedCode:  1,
			expectedError: "Error parsing interval: bar: time: invalid duration \"bar\"",
		},
		{
			name:          "Fails intervals greater than duration",
			args:          []string{"-duration", "5m", "-interval", "10m"},
			expectedCode:  1,
			expectedError: "Error parsing interval: 10m is greater than duration 5m",
		},
		{
			name:          "Fails bad pprof duration",
			args:          []string{"-pprof-duration", "baz"},
			expectedCode:  1,
			expectedError: "Error parsing pprof duration: baz: time: invalid duration \"baz\"",
		},
		{
			name:          "Fails bad pprof interval",
			args:          []string{"-pprof-interval", "bar"},
			expectedCode:  1,
			expectedError: "Error parsing pprof-interval: bar: time: invalid duration \"bar\"",
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
	testDir := t.TempDir()
	defer os.Remove(testDir)

	// Debug on the leader and all client nodes
	code := cmd.Run([]string{"-address", url, "-duration", "250ms", "-interval", "250ms", "-server-id", "leader", "-node-id", "all", "-output", testDir})
	assert.Equal(t, 0, code)

	// Bad plugin name should be escaped before it reaches the sandbox test
	require.NotContains(t, ui.ErrorWriter.String(), "file path escapes capture directory")
	require.Contains(t, ui.OutputWriter.String(), "Starting debugger")

	path := cmd.collectDir

	var pluginFiles []string
	for _, pluginName := range cases {
		pluginFile := fmt.Sprintf("csi-plugin-id-%s.json", helper.CleanFilename(pluginName, "_"))
		pluginFile = filepath.Join(path, intervalDir, "0000", pluginFile)
		pluginFiles = append(pluginFiles, pluginFile)
	}

	testutil.WaitForFiles(t, pluginFiles)
}

func buildPathSlice(path string, files []string) []string {
	paths := []string{}
	for _, file := range files {
		paths = append(paths, filepath.Join(path, file))
	}
	return paths
}

func TestDebug_CapturedFiles(t *testing.T) {
	srv, _, url := testServer(t, true, nil)
	testutil.WaitForLeader(t, srv.Agent.RPC)

	serverNodeName := srv.Config.NodeName
	region := srv.Config.Region
	serverName := fmt.Sprintf("%s.%s", serverNodeName, region)
	clientID := srv.Agent.Client().NodeID()
	testutil.WaitForClient(t, srv.Agent.Client().RPC, clientID, srv.Agent.Client().Region())

	t.Logf("serverName: %s, clientID, %s", serverName, clientID)

	// Setup file slices
	clusterFiles := []string{
		"agent-self.json",
		"members.json",
		"namespaces.json",
		"regions.json",
	}

	pprofFiles := []string{
		"allocs.prof",
		"goroutine-debug1.txt",
		"goroutine-debug2.txt",
		"goroutine.prof",
		"heap.prof",
		"profile_0000.prof",
		"threadcreate.prof",
		"trace.prof",
	}

	clientFiles := []string{
		"agent-host.json",
		"monitor.log",
	}
	clientFiles = append(clientFiles, pprofFiles...)

	serverFiles := []string{
		"agent-host.json",
		"monitor.log",
	}
	serverFiles = append(serverFiles, pprofFiles...)

	intervalFiles := []string{
		"allocations.json",
		"csi-plugins.json",
		"csi-volumes.json",
		"deployments.json",
		"evaluations.json",
		"jobs.json",
		"license.json",
		"metrics.json",
		"nodes.json",
		"operator-autopilot-health.json",
		"operator-raft.json",
		"operator-scheduler.json",
	}

	ui := cli.NewMockUi()
	cmd := &OperatorDebugCommand{Meta: Meta{Ui: ui}}
	testDir := t.TempDir()
	defer os.Remove(testDir)

	duration := 2 * time.Second
	interval := 750 * time.Millisecond
	waitTime := 2 * duration

	code := cmd.Run([]string{
		"-address", url,
		"-output", testDir,
		"-server-id", serverName,
		"-node-id", clientID,
		"-duration", duration.String(),
		"-interval", interval.String(),
	})

	// There should be no errors
	require.Empty(t, ui.ErrorWriter.String())
	require.Equal(t, 0, code)
	ui.ErrorWriter.Reset()

	// Verify cluster files
	clusterPaths := buildPathSlice(cmd.path(clusterDir), clusterFiles)
	t.Logf("Waiting for cluster files in path: %s", clusterDir)
	testutil.WaitForFilesUntil(t, clusterPaths, waitTime)

	// Verify client files
	clientPaths := buildPathSlice(cmd.path(clientDir, clientID), clientFiles)
	t.Logf("Waiting for client files in path: %s", clientDir)
	testutil.WaitForFilesUntil(t, clientPaths, waitTime)

	// Verify server files
	serverPaths := buildPathSlice(cmd.path(serverDir, serverName), serverFiles)
	t.Logf("Waiting for server files in path: %s", serverDir)
	testutil.WaitForFilesUntil(t, serverPaths, waitTime)

	// Verify interval 0000 files
	intervalPaths0 := buildPathSlice(cmd.path(intervalDir, "0000"), intervalFiles)
	t.Logf("Waiting for interval 0000 files in path: %s", intervalDir)
	testutil.WaitForFilesUntil(t, intervalPaths0, waitTime)

	// Verify interval 0001 files
	intervalPaths1 := buildPathSlice(cmd.path(intervalDir, "0001"), intervalFiles)
	t.Logf("Waiting for interval 0001 files in path: %s", intervalDir)
	testutil.WaitForFilesUntil(t, intervalPaths1, waitTime)
}

func TestDebug_ExistingOutput(t *testing.T) {
	ci.Parallel(t)

	ui := cli.NewMockUi()
	cmd := &OperatorDebugCommand{Meta: Meta{Ui: ui}}

	// Fails existing output
	format := "2006-01-02-150405Z"
	stamped := "nomad-debug-" + time.Now().UTC().Format(format)
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, stamped)
	os.MkdirAll(path, 0755)
	defer os.Remove(tempDir)

	code := cmd.Run([]string{"-output", tempDir, "-duration", "50ms", "-interval", "50ms"})
	require.Equal(t, 2, code)
}

func TestDebug_Fail_Pprof(t *testing.T) {

	// Setup agent config with debug endpoints disabled
	agentConfFunc := func(c *agent.Config) {
		c.EnableDebug = false
	}

	// Start test server and API client
	srv, _, url := testServer(t, false, agentConfFunc)

	// Wait for leadership to establish
	testutil.WaitForLeader(t, srv.Agent.RPC)

	// Setup mock UI
	ui := cli.NewMockUi()
	cmd := &OperatorDebugCommand{Meta: Meta{Ui: ui}}

	// Debug on server with endpoints disabled
	code := cmd.Run([]string{"-address", url, "-duration", "250ms", "-interval", "250ms", "-server-id", "all"})

	assert.Equal(t, 0, code) // Pprof failure isn't fatal
	require.Contains(t, ui.OutputWriter.String(), "Starting debugger")
	require.Contains(t, ui.ErrorWriter.String(), "Failed to retrieve pprof") // Should report pprof failure
	require.Contains(t, ui.ErrorWriter.String(), "Permission denied")        // Specifically permission denied
	require.Contains(t, ui.OutputWriter.String(), "Created debug archive")   // Archive should be generated anyway
}

// TestDebug_PprofVersionCheck asserts that only versions < 0.12.0 are
// filtered by the version constraint.
func TestDebug_PprofVersionCheck(t *testing.T) {
	cases := []struct {
		version string
		errMsg  string
	}{
		{"0.8.7", ""},
		{"0.11.1", "unsupported version=0.11.1 matches version filter >= 0.11.0, <= 0.11.2"},
		{"0.11.2", "unsupported version=0.11.2 matches version filter >= 0.11.0, <= 0.11.2"},
		{"0.11.2+ent", "unsupported version=0.11.2+ent matches version filter >= 0.11.0, <= 0.11.2"},
		{"0.11.3", ""},
		{"0.11.3+ent", ""},
		{"0.12.0", ""},
		{"1.3.0", ""},
		{"foo.bar", "error: Malformed version: foo.bar"},
	}

	for _, tc := range cases {
		t.Run(tc.version, func(t *testing.T) {
			err := checkVersion(tc.version, minimumVersionPprofConstraint)
			if tc.errMsg == "" {
				require.NoError(t, err, "expected no error from %s", tc.version)
			} else {
				require.EqualError(t, err, tc.errMsg)
			}
		})
	}
}

func TestDebug_StringToSlice(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		input    string
		expected []string
	}{
		{input: ",,", expected: []string(nil)},
		{input: "", expected: []string(nil)},
		{input: "foo, bar", expected: []string{"foo", "bar"}},
		{input: "  foo, bar ", expected: []string{"foo", "bar"}},
		{input: "foo,,bar", expected: []string{"foo", "bar"}},
	}
	for _, tc := range cases {
		out := stringToSlice(tc.input)
		require.Equal(t, tc.expected, out)
	}
}

func TestDebug_External(t *testing.T) {
	ci.Parallel(t)

	// address calculation honors CONSUL_HTTP_SSL
	// ssl: true - Correct alignment
	e := &external{addrVal: "https://127.0.0.1:8500", ssl: true}
	addr := e.addr("foo")
	require.Equal(t, "https://127.0.0.1:8500", addr)

	// ssl: true - protocol incorrect
	// NOTE: Address with protocol now overrides ssl flag
	e = &external{addrVal: "http://127.0.0.1:8500", ssl: true}
	addr = e.addr("foo")
	require.Equal(t, "http://127.0.0.1:8500", addr)

	// ssl: true - protocol missing
	e = &external{addrVal: "127.0.0.1:8500", ssl: true}
	addr = e.addr("foo")
	require.Equal(t, "https://127.0.0.1:8500", addr)

	// ssl: false - correct alignment
	e = &external{addrVal: "http://127.0.0.1:8500", ssl: false}
	addr = e.addr("foo")
	require.Equal(t, "http://127.0.0.1:8500", addr)

	// ssl: false - protocol incorrect
	// NOTE: Address with protocol now overrides ssl flag
	e = &external{addrVal: "https://127.0.0.1:8500", ssl: false}
	addr = e.addr("foo")
	require.Equal(t, "https://127.0.0.1:8500", addr)

	// ssl: false - protocol missing
	e = &external{addrVal: "127.0.0.1:8500", ssl: false}
	addr = e.addr("foo")
	require.Equal(t, "http://127.0.0.1:8500", addr)

	// Address through proxy might not have a port
	e = &external{addrVal: "https://127.0.0.1", ssl: true}
	addr = e.addr("foo")
	require.Equal(t, "https://127.0.0.1", addr)
}

func TestDebug_WriteBytes_Nil(t *testing.T) {
	ci.Parallel(t)

	var testDir, testFile, testPath string
	var testBytes []byte

	// Setup mock UI
	ui := cli.NewMockUi()
	cmd := &OperatorDebugCommand{Meta: Meta{Ui: ui}}

	testDir = t.TempDir()
	defer os.Remove(testDir)
	cmd.collectDir = testDir

	testFile = "test_nil.json"
	testPath = filepath.Join(testDir, testFile)

	// Write nil file at top level of collect directory
	err := cmd.writeBytes("", testFile, testBytes)
	require.NoError(t, err)
	require.FileExists(t, testPath)
}

func TestDebug_WriteBytes_PathEscapesSandbox(t *testing.T) {
	ci.Parallel(t)

	var testDir, testFile string
	var testBytes []byte

	testDir = t.TempDir()
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

func TestDebug_CollectConsul(t *testing.T) {
	ci.Parallel(t)
	if testing.Short() {
		t.Skip("-short set; skipping")
	}

	// Skip test if Consul binary cannot be found
	clienttest.RequireConsul(t)

	// Create an embedded Consul server
	testconsul, err := consultest.NewTestServerConfigT(t, func(c *consultest.TestServerConfig) {
		c.Peering = nil // fix for older versions of Consul (<1.13.0) that don't support peering
		// If -v wasn't specified squelch consul logging
		if !testing.Verbose() {
			c.Stdout = io.Discard
			c.Stderr = io.Discard
		}
	})
	require.NoError(t, err)
	if err != nil {
		t.Fatalf("error starting test consul server: %v", err)
	}
	defer testconsul.Stop()

	consulConfig := consulapi.DefaultConfig()
	consulConfig.Address = testconsul.HTTPAddr

	// Setup mock UI
	ui := cli.NewMockUi()
	c := &OperatorDebugCommand{Meta: Meta{Ui: ui}}

	// Setup Consul *external
	ce := &external{}
	ce.setAddr(consulConfig.Address)
	if ce.ssl {
		ce.tls = &api.TLSConfig{}
	}

	// Set global client
	c.consul = ce

	// Setup capture directory
	testDir := t.TempDir()
	defer os.Remove(testDir)
	c.collectDir = testDir

	// Collect data from Consul into folder "test"
	c.collectConsul("test")

	require.Empty(t, ui.ErrorWriter.String())
	require.FileExists(t, filepath.Join(testDir, "test", "consul-agent-host.json"))
	require.FileExists(t, filepath.Join(testDir, "test", "consul-agent-members.json"))
	require.FileExists(t, filepath.Join(testDir, "test", "consul-agent-metrics.json"))
	require.FileExists(t, filepath.Join(testDir, "test", "consul-leader.json"))
}

func TestDebug_CollectVault(t *testing.T) {
	ci.Parallel(t)
	if testing.Short() {
		t.Skip("-short set; skipping")
	}

	// Skip test if Consul binary cannot be found
	clienttest.RequireVault(t)

	// Create a Vault server
	v := testutil.NewTestVault(t)
	defer v.Stop()

	// Setup mock UI
	ui := cli.NewMockUi()
	c := &OperatorDebugCommand{Meta: Meta{Ui: ui}}

	// Setup Vault *external
	ve := &external{}
	ve.tokenVal = v.RootToken
	ve.setAddr(v.HTTPAddr)
	if ve.ssl {
		ve.tls = &api.TLSConfig{}
	}

	// Set global client
	c.vault = ve

	// Set capture directory
	testDir := t.TempDir()
	defer os.Remove(testDir)
	c.collectDir = testDir

	// Collect data from Vault
	err := c.collectVault("test", "")

	require.NoError(t, err)
	require.Empty(t, ui.ErrorWriter.String())

	require.FileExists(t, filepath.Join(testDir, "test", "vault-sys-health.json"))
}

// TestDebug_RedirectError asserts that redirect errors are detected so they
// can be translated into more understandable output.
func TestDebug_RedirectError(t *testing.T) {
	ci.Parallel(t)

	// Create a test server that always returns the error many versions of
	// Nomad return instead of a 404 for unknown paths.
	// 1st request redirects to /ui/
	// 2nd request returns UI's HTML
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.String(), "/ui/") {
			fmt.Fprintln(w, `<html>Fake UI HTML</html>`)
			return
		}

		w.Header().Set("Location", "/ui/")
		w.WriteHeader(http.StatusTemporaryRedirect)
		fmt.Fprintln(w, `<a href="/ui/">Temporary Redirect</a>.`)
	}))
	defer ts.Close()

	config := api.DefaultConfig()
	config.Address = ts.URL
	client, err := api.NewClient(config)
	require.NoError(t, err)

	resp, err := client.Agent().Host("abc", "", nil)
	assert.Nil(t, resp)
	assert.True(t, isRedirectError(err), err.Error())
}

// TestDebug_StaleLeadership verifies that APIs that are required to
// complete a debug run have their query options configured with the
// -stale flag
func TestDebug_StaleLeadership(t *testing.T) {

	srv, _, url := testServerWithoutLeader(t, false, nil)
	addrServer := srv.HTTPAddr()

	t.Logf("testAgent api address: %s", url)
	t.Logf("Server    api address: %s", addrServer)

	var cases = testCases{
		{
			name: "no leader without stale flag",
			args: []string{"-address", addrServer,
				"-duration", "250ms", "-interval", "250ms",
				"-server-id", "all", "-node-id", "all"},
			expectedCode:  1,
			expectedError: "No cluster leader",
		},
		{
			name: "no leader with stale flag",
			args: []string{
				"-address", addrServer,
				"-duration", "250ms", "-interval", "250ms",
				"-server-id", "all", "-node-id", "all",
				"-stale"},
			expectedCode:    0,
			expectedOutputs: []string{"Created debug archive"},
			expectedError:   "No node(s) with prefix", // still exits 0
		},
	}

	runTestCases(t, cases)
}

func testServerWithoutLeader(t *testing.T, runClient bool, cb func(*agent.Config)) (*agent.TestAgent, *api.Client, string) {
	// Make a new test server
	a := agent.NewTestAgent(t, t.Name(), func(config *agent.Config) {
		config.Client.Enabled = runClient
		config.Server.Enabled = true
		config.Server.NumSchedulers = pointer.Of(0)
		config.Server.BootstrapExpect = 3

		if cb != nil {
			cb(config)
		}
	})
	t.Cleanup(func() { a.Shutdown() })

	c := a.Client()
	return a, c, a.HTTPAddr()
}

// testOutput is used to receive test output from a channel
type testOutput struct {
	name   string
	code   int
	output string
	error  string
}

func TestDebug_EventStream_TopicsFromString(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name      string
		topicList string
		want      map[api.Topic][]string
	}{
		{
			name:      "topics = all",
			topicList: "all",
			want:      allTopics(),
		},
		{
			name:      "topics = none",
			topicList: "none",
			want:      nil,
		},
		{
			name:      "two topics",
			topicList: "Deployment,Job",
			want: map[api.Topic][]string{
				"Deployment": {"*"},
				"Job":        {"*"},
			},
		},
		{
			name:      "multiple topics and filters (using api const)",
			topicList: "Evaluation:example,Job:*,Node:*",
			want: map[api.Topic][]string{
				api.TopicEvaluation: {"example"},
				api.TopicJob:        {"*"},
				api.TopicNode:       {"*"},
			},
		},
		{
			name:      "capitalize topics",
			topicList: "evaluation:example,job:*,node:*",
			want: map[api.Topic][]string{
				api.TopicEvaluation: {"example"},
				api.TopicJob:        {"*"},
				api.TopicNode:       {"*"},
			},
		},
		{
			name:      "all topics for filterKey",
			topicList: "*:example",
			want: map[api.Topic][]string{
				"*": {"example"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := topicsFromString(tc.topicList)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestDebug_EventStream(t *testing.T) {
	ci.Parallel(t)

	// TODO dmay: specify output directory to allow inspection of eventstream.json
	// TODO dmay: require specific events in the eventstream.json file(s)
	// TODO dmay: scenario where no events are expected, verify "No events captured"
	// TODO dmay: verify event topic filtering only includes expected events

	start := time.Now()

	// Start test server
	srv, client, url := testServer(t, true, nil)
	t.Logf("%s: test server started, waiting for leadership to establish\n", time.Since(start))

	// Ensure leader is ready
	testutil.WaitForLeader(t, srv.Agent.RPC)
	t.Logf("%s: Leadership established\n", time.Since(start))

	// Setup mock UI
	ui := cli.NewMockUi()
	cmd := &OperatorDebugCommand{Meta: Meta{Ui: ui}}

	// Return command output back to the main test goroutine
	chOutput := make(chan testOutput)

	// Set duration for capture
	duration := 5 * time.Second
	// Fail with timeout if duration is exceeded by 5 seconds
	timeout := duration + 5*time.Second

	// Run debug in a goroutine so we can start the capture before we run the test job
	t.Logf("%s: Starting nomad operator debug in goroutine\n", time.Since(start))
	go func() {
		code := cmd.Run([]string{"-address", url, "-duration", duration.String(), "-interval", "5s", "-event-topic", "Job:*"})
		assert.Equal(t, 0, code)

		chOutput <- testOutput{
			name:   "yo",
			code:   code,
			output: ui.OutputWriter.String(),
			error:  ui.ErrorWriter.String(),
		}
	}()

	// Start test job
	t.Logf("%s: Running test job\n", time.Since(start))
	job := testJob("event_stream_test")
	resp, _, err := client.Jobs().Register(job, nil)
	t.Logf("%s: Test job started\n", time.Since(start))

	// Ensure job registered
	require.NoError(t, err)

	// Wait for the job to complete
	if code := waitForSuccess(ui, client, fullId, t, resp.EvalID); code != 0 {
		switch code {
		case 1:
			t.Fatalf("status code 1: All other failures (API connectivity, internal errors, etc)\n")
		case 2:
			t.Fatalf("status code 2: Problem scheduling job (impossible constraints, resources exhausted, etc)\n")
		default:
			t.Fatalf("status code non zero saw %d\n", code)
		}
	}
	t.Logf("%s: test job is complete, eval id: %s\n", time.Since(start), resp.EvalID)

	// Capture the output struct from nomad operator debug goroutine
	var testOut testOutput
	select {
	case testOut = <-chOutput:
		t.Logf("%s: goroutine is complete", time.Since(start))
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for event stream event (duration: %s, timeout: %s", duration, timeout)
	}

	t.Logf("Values from struct -- code: %d, len(out): %d, len(outerr): %d\n", testOut.code, len(testOut.output), len(testOut.error))

	require.Empty(t, testOut.error)

	archive := extractArchiveName(testOut.output)
	require.NotEmpty(t, archive)
	fmt.Println(archive)

	// TODO dmay: verify evenstream.json output file contains expected content
}

// extractArchiveName searches string s for the archive filename
func extractArchiveName(captureOutput string) string {
	file := ""

	r := regexp.MustCompile(`Created debug archive: (.+)?\n`)
	res := r.FindStringSubmatch(captureOutput)
	// If found, there will be 2 elements, where element [1] is the desired text from the submatch
	if len(res) == 2 {
		file = res[1]
	}

	return file
}
