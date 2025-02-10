// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/shoenig/test/must"
)

func TestHTTP_rpcHandlerForAlloc(t *testing.T) {
	ci.Parallel(t)

	agent := NewTestAgent(t, t.Name(), nil)
	defer agent.Shutdown()

	a := mockFSAlloc(agent.client.NodeID(), nil)
	addAllocToClient(agent, a, terminalClientAlloc)

	// Case 1: Client has allocation
	// Outcome: Use local client
	lc, rc, s := agent.Server.rpcHandlerForAlloc(a.ID)
	must.True(t, lc)
	must.False(t, rc)
	must.False(t, s)

	// Case 2: Client doesn't have allocation and there is a server
	// Outcome: Use server
	lc, rc, s = agent.Server.rpcHandlerForAlloc(uuid.Generate())
	must.False(t, lc)
	must.False(t, rc)
	must.True(t, s)

	// Case 3: Client doesn't have allocation and there is no server
	// Outcome: Use client RPC to server
	srv := agent.server
	agent.server = nil
	lc, rc, s = agent.Server.rpcHandlerForAlloc(uuid.Generate())
	must.False(t, lc)
	must.True(t, rc)
	must.False(t, s)
	agent.server = srv

	// Case 4: No client
	// Outcome: Use server
	client := agent.client
	agent.client = nil
	lc, rc, s = agent.Server.rpcHandlerForAlloc(uuid.Generate())
	must.False(t, lc)
	must.False(t, rc)
	must.True(t, s)
	agent.client = client
}

func TestHTTP_rpcHandlerForNode(t *testing.T) {
	ci.Parallel(t)

	agent := NewTestAgent(t, t.Name(), nil)
	defer agent.Shutdown()

	cID := agent.client.NodeID()

	// Case 1: Node running, no node ID given
	// Outcome: Use local node
	lc, rc, s := agent.Server.rpcHandlerForNode("")
	must.True(t, lc)
	must.False(t, rc)
	must.False(t, s)

	// Case 2: Node running, it's ID given
	// Outcome: Use local node
	lc, rc, s = agent.Server.rpcHandlerForNode(cID)
	must.True(t, lc)
	must.False(t, rc)
	must.False(t, s)

	// Case 3: Local node but wrong ID and there is no server
	// Outcome: Use client RPC to server
	srv := agent.server
	agent.server = nil
	lc, rc, s = agent.Server.rpcHandlerForNode(uuid.Generate())
	must.False(t, lc)
	must.True(t, rc)
	must.False(t, s)
	agent.server = srv

	// Case 4: No client
	// Outcome: Use server
	client := agent.client
	agent.client = nil
	lc, rc, s = agent.Server.rpcHandlerForNode(uuid.Generate())
	must.False(t, lc)
	must.False(t, rc)
	must.True(t, s)
	agent.client = client
}
