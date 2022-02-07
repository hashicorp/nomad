package api

import (
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/kr/pretty"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/stretchr/testify/assert"
)

func TestAgent_Self(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	// Get a handle on the Agent endpoints
	a := c.Agent()

	// Query the endpoint
	res, err := a.Self()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Check that we got a valid response
	if res.Member.Name == "" {
		t.Fatalf("bad member name in response: %#v", res)
	}

	// Local cache was populated
	if a.nodeName == "" || a.datacenter == "" || a.region == "" {
		t.Fatalf("cache should be populated, got: %#v", a)
	}
}

func TestAgent_NodeName(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	a := c.Agent()

	// Query the agent for the node name
	res, err := a.NodeName()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if res == "" {
		t.Fatalf("expected node name, got nothing")
	}
}

func TestAgent_Datacenter(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	a := c.Agent()

	// Query the agent for the datacenter
	dc, err := a.Datacenter()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if dc != "dc1" {
		t.Fatalf("expected dc1, got: %q", dc)
	}
}

func TestAgent_Join(t *testing.T) {
	t.Parallel()
	c1, s1 := makeClient(t, nil, nil)
	defer s1.Stop()
	a1 := c1.Agent()

	_, s2 := makeClient(t, nil, func(c *testutil.TestServerConfig) {
		c.Server.BootstrapExpect = 0
	})
	defer s2.Stop()

	// Attempting to join a nonexistent host returns error
	n, err := a1.Join("nope")
	if err == nil {
		t.Fatalf("expected error, got nothing")
	}
	if n != 0 {
		t.Fatalf("expected 0 nodes, got: %d", n)
	}

	// Returns correctly if join succeeds
	n, err = a1.Join(s2.SerfAddr)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 node, got: %d", n)
	}
}

func TestAgent_Members(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	a := c.Agent()

	// Query nomad for all the known members
	mem, err := a.Members()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Check that we got the expected result
	if n := len(mem.Members); n != 1 {
		t.Fatalf("expected 1 member, got: %d", n)
	}
	if m := mem.Members[0]; m.Name == "" || m.Addr == "" || m.Port == 0 {
		t.Fatalf("bad member: %#v", m)
	}
}

func TestAgent_ForceLeave(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	a := c.Agent()

	// Force-leave on a nonexistent node does not error
	if err := a.ForceLeave("nope"); err != nil {
		t.Fatalf("err: %s", err)
	}

	// TODO: test force-leave on an existing node
}

func (a *AgentMember) String() string {
	return "{Name: " + a.Name + " Region: " + a.Tags["region"] + " DC: " + a.Tags["dc"] + "}"
}

func TestAgents_Sort(t *testing.T) {
	t.Parallel()
	var sortTests = []struct {
		in  []*AgentMember
		out []*AgentMember
	}{
		{
			[]*AgentMember{
				{Name: "nomad-2.vac.us-east",
					Tags: map[string]string{"region": "us-east", "dc": "us-east-1c"}},
				{Name: "nomad-1.global",
					Tags: map[string]string{"region": "global", "dc": "dc1"}},
				{Name: "nomad-1.vac.us-east",
					Tags: map[string]string{"region": "us-east", "dc": "us-east-1c"}},
			},
			[]*AgentMember{
				{Name: "nomad-1.global",
					Tags: map[string]string{"region": "global", "dc": "dc1"}},
				{Name: "nomad-1.vac.us-east",
					Tags: map[string]string{"region": "us-east", "dc": "us-east-1c"}},
				{Name: "nomad-2.vac.us-east",
					Tags: map[string]string{"region": "us-east", "dc": "us-east-1c"}},
			},
		},
		{
			[]*AgentMember{
				{Name: "nomad-02.tam.us-east",
					Tags: map[string]string{"region": "us-east", "dc": "tampa"}},
				{Name: "nomad-02.pal.us-west",
					Tags: map[string]string{"region": "us-west", "dc": "palo_alto"}},
				{Name: "nomad-01.pal.us-west",
					Tags: map[string]string{"region": "us-west", "dc": "palo_alto"}},
				{Name: "nomad-01.tam.us-east",
					Tags: map[string]string{"region": "us-east", "dc": "tampa"}},
			},
			[]*AgentMember{
				{Name: "nomad-01.tam.us-east",
					Tags: map[string]string{"region": "us-east", "dc": "tampa"}},
				{Name: "nomad-02.tam.us-east",
					Tags: map[string]string{"region": "us-east", "dc": "tampa"}},
				{Name: "nomad-01.pal.us-west",
					Tags: map[string]string{"region": "us-west", "dc": "palo_alto"}},
				{Name: "nomad-02.pal.us-west",
					Tags: map[string]string{"region": "us-west", "dc": "palo_alto"}},
			},
		},
		{
			[]*AgentMember{
				{Name: "nomad-02.tam.us-east",
					Tags: map[string]string{"region": "us-east", "dc": "tampa"}},
				{Name: "nomad-02.ams.europe",
					Tags: map[string]string{"region": "europe", "dc": "amsterdam"}},
				{Name: "nomad-01.tam.us-east",
					Tags: map[string]string{"region": "us-east", "dc": "tampa"}},
				{Name: "nomad-01.ams.europe",
					Tags: map[string]string{"region": "europe", "dc": "amsterdam"}},
			},
			[]*AgentMember{
				{Name: "nomad-01.ams.europe",
					Tags: map[string]string{"region": "europe", "dc": "amsterdam"}},
				{Name: "nomad-02.ams.europe",
					Tags: map[string]string{"region": "europe", "dc": "amsterdam"}},
				{Name: "nomad-01.tam.us-east",
					Tags: map[string]string{"region": "us-east", "dc": "tampa"}},
				{Name: "nomad-02.tam.us-east",
					Tags: map[string]string{"region": "us-east", "dc": "tampa"}},
			},
		},
		{
			[]*AgentMember{
				{Name: "nomad-02.ber.europe",
					Tags: map[string]string{"region": "europe", "dc": "berlin"}},
				{Name: "nomad-02.ams.europe",
					Tags: map[string]string{"region": "europe", "dc": "amsterdam"}},
				{Name: "nomad-01.ams.europe",
					Tags: map[string]string{"region": "europe", "dc": "amsterdam"}},
				{Name: "nomad-01.ber.europe",
					Tags: map[string]string{"region": "europe", "dc": "berlin"}},
			},
			[]*AgentMember{
				{Name: "nomad-01.ams.europe",
					Tags: map[string]string{"region": "europe", "dc": "amsterdam"}},
				{Name: "nomad-02.ams.europe",
					Tags: map[string]string{"region": "europe", "dc": "amsterdam"}},
				{Name: "nomad-01.ber.europe",
					Tags: map[string]string{"region": "europe", "dc": "berlin"}},
				{Name: "nomad-02.ber.europe",
					Tags: map[string]string{"region": "europe", "dc": "berlin"}},
			},
		},
		{
			[]*AgentMember{
				{Name: "nomad-1.global"},
				{Name: "nomad-3.global"},
				{Name: "nomad-2.global"},
			},
			[]*AgentMember{
				{Name: "nomad-1.global"},
				{Name: "nomad-2.global"},
				{Name: "nomad-3.global"},
			},
		},
	}
	for _, tt := range sortTests {
		sort.Sort(AgentMembersNameSort(tt.in))
		if !reflect.DeepEqual(tt.in, tt.out) {
			t.Errorf("\nexpected: %s\nget     : %s", tt.in, tt.out)
		}
	}
}

func TestAgent_Health(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	a := c.Agent()

	health, err := a.Health()
	assert.Nil(err)
	assert.True(health.Server.Ok)
}

// TestAgent_MonitorWithNode tests the Monitor endpoint
// passing in a log level and node ie, which tests monitor
// functionality for a specific client node
func TestAgent_MonitorWithNode(t *testing.T) {
	t.Parallel()
	rpcPort := 0
	c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) {
		rpcPort = c.Ports.RPC
		c.Client = &testutil.ClientConfig{
			Enabled: true,
		}
	})
	defer s.Stop()

	require.NoError(t, c.Agent().SetServers([]string{fmt.Sprintf("127.0.0.1:%d", rpcPort)}))

	agent := c.Agent()

	index := uint64(0)
	var node *NodeListStub
	// grab a node
	testutil.WaitForResult(func() (bool, error) {
		nodes, qm, err := c.Nodes().List(&QueryOptions{WaitIndex: index})
		if err != nil {
			return false, err
		}
		index = qm.LastIndex
		if len(nodes) != 1 {
			return false, fmt.Errorf("expected 1 node but found: %s", pretty.Sprint(nodes))
		}
		if nodes[0].Status != "ready" {
			return false, fmt.Errorf("node not ready: %s", nodes[0].Status)
		}
		node = nodes[0]
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	doneCh := make(chan struct{})
	q := &QueryOptions{
		Params: map[string]string{
			"log_level": "debug",
			"node_id":   node.ID,
		},
	}

	frames, errCh := agent.Monitor(doneCh, q)
	defer close(doneCh)

	// make a request to generate some logs
	_, err := agent.NodeName()
	require.NoError(t, err)

	// Wait for a log message
OUTER:
	for {
		select {
		case f := <-frames:
			if strings.Contains(string(f.Data), "[DEBUG]") {
				break OUTER
			}
		case err := <-errCh:
			t.Errorf("Error: %v", err)
		case <-time.After(2 * time.Second):
			require.Fail(t, "failed to get a DEBUG log message")
		}
	}
}

// TestAgent_Monitor tests the Monitor endpoint
// passing in only a log level, which tests the servers
// monitor functionality
func TestAgent_Monitor(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	agent := c.Agent()

	q := &QueryOptions{
		Params: map[string]string{
			"log_level": "debug",
		},
	}

	doneCh := make(chan struct{})
	frames, errCh := agent.Monitor(doneCh, q)
	defer close(doneCh)

	// make a request to generate some logs
	_, err := agent.Region()
	require.NoError(t, err)

	// Wait for a log message
OUTER:
	for {
		select {
		case log := <-frames:
			if log == nil {
				continue
			}
			if strings.Contains(string(log.Data), "[DEBUG]") {
				break OUTER
			}
		case err := <-errCh:
			t.Fatalf("error: %v", err)
		case <-time.After(2 * time.Second):
			require.Fail(t, "failed to get a DEBUG log message")
		}
	}
}

func TestAgentCPUProfile(t *testing.T) {
	t.Parallel()

	c, s, token := makeACLClient(t, nil, nil)
	defer s.Stop()

	agent := c.Agent()

	q := &QueryOptions{
		AuthToken: token.SecretID,
	}

	// Valid local request
	{
		opts := PprofOptions{
			Seconds: 1,
		}
		resp, err := agent.CPUProfile(opts, q)
		require.NoError(t, err)
		require.NotNil(t, resp)
	}

	// Invalid server request
	{
		opts := PprofOptions{
			Seconds:  1,
			ServerID: "unknown.global",
		}
		resp, err := agent.CPUProfile(opts, q)
		require.Error(t, err)
		require.Contains(t, err.Error(), "500 (unknown Nomad server unknown.global)")
		require.Nil(t, resp)
	}

}

func TestAgentTrace(t *testing.T) {
	t.Parallel()

	c, s, token := makeACLClient(t, nil, nil)
	defer s.Stop()

	agent := c.Agent()

	q := &QueryOptions{
		AuthToken: token.SecretID,
	}

	resp, err := agent.Trace(PprofOptions{}, q)
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestAgentProfile(t *testing.T) {
	t.Parallel()

	c, s, token := makeACLClient(t, nil, nil)
	defer s.Stop()

	agent := c.Agent()

	q := &QueryOptions{
		AuthToken: token.SecretID,
	}

	{
		resp, err := agent.Lookup("heap", PprofOptions{}, q)
		require.NoError(t, err)
		require.NotNil(t, resp)
	}

	// unknown profile
	{
		resp, err := agent.Lookup("invalid", PprofOptions{}, q)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Unexpected response code: 404")
		require.Nil(t, resp)
	}
}

func TestAgent_SchedulerWorkerConfig(t *testing.T) {
	t.Parallel()

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	a := c.Agent()

	config, err := a.GetSchedulerWorkerConfig(nil)
	require.NoError(t, err)
	require.NotNil(t, config)
	newConfig := SchedulerWorkerPoolArgs{NumSchedulers: 0, EnabledSchedulers: []string{"_core", "system"}}
	resp, err := a.SetSchedulerWorkerConfig(newConfig, nil)
	require.NoError(t, err)
	assert.NotEqual(t, config, resp)
}

func TestAgent_SchedulerWorkerConfig_BadRequest(t *testing.T) {
	t.Parallel()

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	a := c.Agent()

	config, err := a.GetSchedulerWorkerConfig(nil)
	require.NoError(t, err)
	require.NotNil(t, config)
	newConfig := SchedulerWorkerPoolArgs{NumSchedulers: -1, EnabledSchedulers: []string{"_core", "system"}}
	_, err = a.SetSchedulerWorkerConfig(newConfig, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), fmt.Sprintf("%v (%s)", http.StatusBadRequest, "Invalid request"))
}

func TestAgent_SchedulerWorkersInfo(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	a := c.Agent()

	info, err := a.GetSchedulerWorkersInfo(nil)
	require.NoError(t, err)
	require.NotNil(t, info)
	defaultSchedulers := []string{"batch", "system", "sysbatch", "service", "_core"}
	for _, worker := range info.Schedulers {
		require.ElementsMatch(t, defaultSchedulers, worker.EnabledSchedulers)
	}
}
