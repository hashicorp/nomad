// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test/must"
)

func TestNodeMeta_Apply(t *testing.T) {
	testutil.Parallel(t)

	cb := func(c *testutil.TestServerConfig) {
		c.DevMode = true
	}
	c, s := makeClient(t, nil, cb)
	defer s.Stop()

	nodeID := oneNodeFromNodeList(t, c.Nodes()).ID

	node, _, err := c.Nodes().Info(nodeID, nil)
	must.NoError(t, err)
	must.Greater(t, 1, len(node.Meta),
		must.Sprintf("expected more Node.Meta by default: %#v", node.Meta))

	metaResp, err := c.Nodes().Meta().Read(node.ID, nil)
	must.NoError(t, err)
	must.MapEq(t, node.Meta, metaResp.Meta)
	must.MapEq(t, node.Meta, metaResp.Static)
	must.MapEmpty(t, metaResp.Dynamic)

	staticKey := ""
	for staticKey = range node.Meta {
		break
	}

	req := &NodeMetaApplyRequest{
		NodeID: node.ID,
		Meta: map[string]*string{
			staticKey: nil,
			"foo":     pointerOf("bar"),
		},
	}

	metaResp, err = c.Nodes().Meta().Apply(req, nil)
	must.NoError(t, err)
	must.MapEq(t, req.Meta, metaResp.Dynamic)
	must.MapEq(t, node.Meta, metaResp.Static)
	must.MapNotContainsKey(t, metaResp.Meta, staticKey)
	must.Eq(t, "bar", metaResp.Meta["foo"])

	// Wait up to 10s (plus buffer) for node to re-register
	deadline := time.Now().Add(11 * time.Second)
	found := false
	for !found && time.Now().Before(deadline) {
		node, _, err = c.Nodes().Info(node.ID, nil)
		must.NoError(t, err)
		found = node.Meta["foo"] == "bar"
		time.Sleep(100 * time.Millisecond)
	}
	must.True(t, found)
}
