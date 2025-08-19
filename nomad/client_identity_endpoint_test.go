// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
)

func TestNodeIdentity_Get_Forward(t *testing.T) {
	ci.Parallel(t)

	servers := []*Server{}
	for range 3 {
		s, cleanup := TestServer(t, func(c *Config) {
			c.BootstrapExpect = 3
			c.NumSchedulers = 0
		})
		t.Cleanup(cleanup)
		servers = append(servers, s)
	}

	TestJoin(t, servers...)
	leader := testutil.WaitForLeaders(t, servers[0].RPC, servers[1].RPC, servers[2].RPC)

	followers := []string{}
	for _, s := range servers {
		if addr := s.config.RPCAddr.String(); addr != leader {
			followers = append(followers, addr)
		}
	}
	t.Logf("leader=%s followers=%q", leader, followers)

	clients := make([]*client.Client, 4)

	for i := range 4 {
		c, cleanup := client.TestClient(t, func(c *config.Config) {
			c.Servers = followers
		})
		t.Cleanup(func() { _ = cleanup() })
		clients[i] = c
	}
	for _, c := range clients {
		testutil.WaitForClient(t, servers[0].RPC, c.NodeID(), c.Region())
	}

	agentRPCs := []func(string, any, any) error{}
	nodeIDs := make([]string, 0, len(clients))

	// Build list of agents and node IDs
	for _, s := range servers {
		agentRPCs = append(agentRPCs, s.RPC)
	}

	for _, c := range clients {
		agentRPCs = append(agentRPCs, c.RPC)
		nodeIDs = append(nodeIDs, c.NodeID())
	}

	// Iterate through all the agent RPCs to ensure that the renew RPC will
	// succeed, no matter which agent we connect to.
	for _, agentRPC := range agentRPCs {
		for _, nodeID := range nodeIDs {
			args := &structs.NodeIdentityGetReq{
				NodeID: nodeID,
				QueryOptions: structs.QueryOptions{
					Region: clients[0].Region(),
				},
			}
			must.NoError(t,
				agentRPC(structs.NodeIdentityGetRPCMethod,
					args,
					&structs.NodeIdentityGetResp{},
				),
			)
		}
	}
}

func TestNodeIdentity_Renew_Forward(t *testing.T) {
	ci.Parallel(t)

	servers := []*Server{}
	for i := 0; i < 3; i++ {
		s, cleanup := TestServer(t, func(c *Config) {
			c.BootstrapExpect = 3
			c.NumSchedulers = 0
		})
		t.Cleanup(cleanup)
		servers = append(servers, s)
	}

	TestJoin(t, servers...)
	leader := testutil.WaitForLeaders(t, servers[0].RPC, servers[1].RPC, servers[2].RPC)

	followers := []string{}
	for _, s := range servers {
		if addr := s.config.RPCAddr.String(); addr != leader {
			followers = append(followers, addr)
		}
	}
	t.Logf("leader=%s followers=%q", leader, followers)

	clients := make([]*client.Client, 4)

	for i := 0; i < 4; i++ {
		c, cleanup := client.TestClient(t, func(c *config.Config) {
			c.Servers = followers
		})
		t.Cleanup(func() { _ = cleanup() })
		clients[i] = c
	}
	for _, c := range clients {
		testutil.WaitForClient(t, servers[0].RPC, c.NodeID(), c.Region())
	}

	agentRPCs := []func(string, any, any) error{}
	nodeIDs := make([]string, 0, len(clients))

	// Build list of agents and node IDs
	for _, s := range servers {
		agentRPCs = append(agentRPCs, s.RPC)
	}

	for _, c := range clients {
		agentRPCs = append(agentRPCs, c.RPC)
		nodeIDs = append(nodeIDs, c.NodeID())
	}

	// Iterate through all the agent RPCs to ensure that the renew RPC will
	// succeed, no matter which agent we connect to.
	for _, agentRPC := range agentRPCs {
		for _, nodeID := range nodeIDs {
			args := &structs.NodeIdentityRenewReq{
				NodeID: nodeID,
				QueryOptions: structs.QueryOptions{
					Region: clients[0].Region(),
				},
			}
			must.NoError(t,
				agentRPC(structs.NodeIdentityRenewRPCMethod,
					args,
					&structs.NodeIdentityRenewResp{},
				),
			)
		}
	}
}
