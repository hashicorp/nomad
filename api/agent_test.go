// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test/must"
)

func TestAgent_Self(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	// Get a handle on the Agent endpoints
	a := c.Agent()

	// Query the endpoint
	res, err := a.Self()
	must.NoError(t, err)

	// Check that we got a valid response
	must.NotEq(t, "", res.Member.Name, must.Sprint("missing member name"))

	// Local cache was populated
	must.NotEq(t, "", a.nodeName, must.Sprint("cache should be populated"))
	must.NotEq(t, "", a.datacenter, must.Sprint("cache should be populated"))
	must.NotEq(t, "", a.region, must.Sprint("cache should be populated"))
}

func TestAgent_NodeName(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	a := c.Agent()

	// Query the agent for the node name
	nodeName, err := a.NodeName()
	must.NoError(t, err)
	must.NotEq(t, "", nodeName)
}

func TestAgent_Datacenter(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	a := c.Agent()

	// Query the agent for the datacenter
	dc, err := a.Datacenter()
	must.NoError(t, err)
	must.Eq(t, "dc1", dc)
}

func TestAgent_Join(t *testing.T) {
	testutil.Parallel(t)

	c1, s1 := makeClient(t, nil, nil)
	defer s1.Stop()
	a1 := c1.Agent()

	_, s2 := makeClient(t, nil, func(c *testutil.TestServerConfig) {
		c.Server.BootstrapExpect = 0
	})
	defer s2.Stop()

	// Attempting to join a nonexistent host returns error
	n, err := a1.Join("nope")
	must.Error(t, err)
	must.Zero(t, 0, must.Sprint("should be zero errors"))

	// Returns correctly if join succeeds
	n, err = a1.Join(s2.SerfAddr)
	must.NoError(t, err)
	must.One(t, n)
}

func TestAgent_Members(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	a := c.Agent()

	// Query nomad for all the known members
	mem, err := a.Members()
	must.NoError(t, err)

	// Check that we got the expected result
	must.Len(t, 1, mem.Members)
	must.NotEq(t, "", mem.Members[0].Name)
	must.NotEq(t, "", mem.Members[0].Addr)
	must.NotEq(t, 0, mem.Members[0].Port)
}

func TestAgent_ForceLeave(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	a := c.Agent()

	// Force-leave on a nonexistent node does not error
	err := a.ForceLeave("nope")
	must.NoError(t, err)

	// TODO: test force-leave on an existing node
}

func (a *AgentMember) String() string {
	return "{Name: " + a.Name + " Region: " + a.Tags["region"] + " DC: " + a.Tags["dc"] + "}"
}

func TestAgents_Sort(t *testing.T) {
	testutil.Parallel(t)

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
		must.Eq(t, tt.in, tt.out)
	}
}

func TestAgent_Health(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	a := c.Agent()

	health, err := a.Health()
	must.NoError(t, err)
	must.True(t, health.Server.Ok)
}

// TestAgent_MonitorWithNode tests the Monitor endpoint
// passing in a log level and node ie, which tests monitor
// functionality for a specific client node
func TestAgent_MonitorWithNode(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) {
		c.DevMode = true
	})
	defer s.Stop()

	agent := c.Agent()
	node := oneNodeFromNodeList(t, c.Nodes())

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
	must.NoError(t, err)

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
			t.Fatal("failed to get a DEBUG log message")
		}
	}
}

// TestAgent_Monitor tests the Monitor endpoint
// passing in only a log level, which tests the servers
// monitor functionality
func TestAgent_Monitor(t *testing.T) {
	testutil.Parallel(t)

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
	must.NoError(t, err)

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
			must.Unreachable(t, must.Sprint("failed to get DEBUG log message"))
		}
	}
}

func TestAgentCPUProfile(t *testing.T) {
	testutil.Parallel(t)

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
		must.NoError(t, err)
		must.NotNil(t, resp)
	}

	// Invalid server request
	{
		opts := PprofOptions{
			Seconds:  1,
			ServerID: "unknown.global",
		}
		resp, err := agent.CPUProfile(opts, q)
		must.Error(t, err)
		must.ErrorContains(t, err, "500 (unknown Nomad server unknown.global)")
		must.Nil(t, resp)
	}

}

func TestAgentTrace(t *testing.T) {
	testutil.Parallel(t)

	c, s, token := makeACLClient(t, nil, nil)
	defer s.Stop()

	agent := c.Agent()

	q := &QueryOptions{
		AuthToken: token.SecretID,
	}

	resp, err := agent.Trace(PprofOptions{}, q)
	must.NoError(t, err)
	must.NotNil(t, resp)
}

func TestAgentProfile(t *testing.T) {
	testutil.Parallel(t)

	c, s, token := makeACLClient(t, nil, nil)
	defer s.Stop()

	agent := c.Agent()

	q := &QueryOptions{
		AuthToken: token.SecretID,
	}

	{
		resp, err := agent.Lookup("heap", PprofOptions{}, q)
		must.NoError(t, err)
		must.NotNil(t, resp)
	}

	// unknown profile
	{
		resp, err := agent.Lookup("invalid", PprofOptions{}, q)
		must.Error(t, err)
		must.ErrorContains(t, err, "Unexpected response code: 404")
		must.Nil(t, resp)
	}
}

func TestAgent_SchedulerWorkerConfig(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	a := c.Agent()

	config, err := a.GetSchedulerWorkerConfig(nil)
	must.NoError(t, err)
	must.NotNil(t, config)
	newConfig := SchedulerWorkerPoolArgs{NumSchedulers: 0, EnabledSchedulers: []string{"_core", "system"}}
	resp, err := a.SetSchedulerWorkerConfig(newConfig, nil)
	must.NoError(t, err)
	must.NotEq(t, config, resp)
}

func TestAgent_SchedulerWorkerConfig_BadRequest(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	a := c.Agent()

	config, err := a.GetSchedulerWorkerConfig(nil)
	must.NoError(t, err)
	must.NotNil(t, config)
	newConfig := SchedulerWorkerPoolArgs{NumSchedulers: -1, EnabledSchedulers: []string{"_core", "system"}}
	_, err = a.SetSchedulerWorkerConfig(newConfig, nil)
	must.Error(t, err)
	must.ErrorContains(t, err, "400 (Invalid request)")
}

func TestAgent_SchedulerWorkersInfo(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	a := c.Agent()

	info, err := a.GetSchedulerWorkersInfo(nil)
	must.NoError(t, err)
	must.NotNil(t, info)
	defaultSchedulers := []string{"batch", "system", "sysbatch", "service", "_core"}
	for _, worker := range info.Schedulers {
		must.SliceContainsAll(t, defaultSchedulers, worker.EnabledSchedulers)
	}
}
