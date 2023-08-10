// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
)

// TestNodeMeta_Forward asserts that Client RPCs do not result in infinite
// loops. For example in a cluster with 1 Leader, 2 Followers, and a Node
// connected to Follower 1:
//
// If a NodeMeta.Apply RPC with AllowStale=false is received by Follower 1, it
// will honor AllowStale=false and forward the request to the Leader.
//
// The Leader will accept the RPC, notice that Follower 1 has a connection to
// the Node, and the Leader will send the request back to Follower 1.
//
// Follower 1, ever respectful of AllowStale=false, will forward it back to the
// Leader.
//
// The Leader, being unable to forward to the Node, will send it back to
// Follower 1.
//
// This argument will continue until one of the Servers runs out of memory or
// patience and stomps away in anger (crashes). Like any good argument the
// ending is never pretty as the Servers will suffer CPU starvation and
// potentially Raft flapping before anyone actually OOMs.
//
// See https://github.com/hashicorp/nomad/issues/16517 for details.
//
// If test fails it will do so spectacularly by consuming all available CPU and
// potentially all available memory. Running it in a VM or container is
// suggested.
func TestNodeMeta_Forward(t *testing.T) {
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

	clients := []*client.Client{}
	for i := 0; i < 4; i++ {
		c, cleanup := client.TestClient(t, func(c *config.Config) {
			// Clients will rebalance across all servers, but try to get them to use
			// followers to ensure we don't hit the loop in #16517
			c.Servers = followers
		})
		defer cleanup()
		clients = append(clients, c)
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

	region := clients[0].Region()

	// Apply metadata to every client through every agent to ensure forwarding
	// always works regardless of path taken.
	for _, rpc := range agentRPCs {
		for _, nodeID := range nodeIDs {
			args := &structs.NodeMetaApplyRequest{
				// Intentionally don't set QueryOptions.AllowStale to exercise #16517
				QueryOptions: structs.QueryOptions{
					Region: region,
				},
				NodeID: nodeID,
				Meta:   map[string]*string{"testing": pointer.Of("123")},
			}
			reply := &structs.NodeMetaResponse{}
			must.NoError(t, rpc("NodeMeta.Apply", args, reply))
			must.MapNotEmpty(t, reply.Meta)
		}
	}

	for _, rpc := range agentRPCs {
		for _, nodeID := range nodeIDs {
			args := &structs.NodeSpecificRequest{
				// Intentionally don't set QueryOptions.AllowStale to exercise #16517
				QueryOptions: structs.QueryOptions{
					Region: region,
				},
				NodeID: nodeID,
			}
			reply := &structs.NodeMetaResponse{}
			must.NoError(t, rpc("NodeMeta.Read", args, reply))
			must.MapNotEmpty(t, reply.Meta)
			must.Eq(t, reply.Meta["testing"], "123")
			must.MapNotEmpty(t, reply.Dynamic)
			must.Eq(t, *reply.Dynamic["testing"], "123")
		}
	}
}
