// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"testing"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test/must"
)

func TestNodeIdentity_Get(t *testing.T) {
	testutil.Parallel(t)

	configCallback := func(c *testutil.TestServerConfig) { c.DevMode = true }
	testClient, testServer := makeClient(t, nil, configCallback)
	defer testServer.Stop()

	nodeID := oneNodeFromNodeList(t, testClient.Nodes()).ID

	req := NodeIdentityGetRequest{
		NodeID: nodeID,
	}

	resp, err := testClient.Nodes().Identity().Get(&req, nil)
	must.NoError(t, err)
	must.NotNil(t, resp)
	must.MapLen(t, 9, resp.Claims)
}

func TestNodeIdentity_Renew(t *testing.T) {
	testutil.Parallel(t)

	configCallback := func(c *testutil.TestServerConfig) { c.DevMode = true }
	testClient, testServer := makeClient(t, nil, configCallback)
	defer testServer.Stop()

	nodeID := oneNodeFromNodeList(t, testClient.Nodes()).ID

	req := NodeIdentityRenewRequest{
		NodeID: nodeID,
	}

	resp, err := testClient.Nodes().Identity().Renew(&req, nil)
	must.NoError(t, err)
	must.NotNil(t, resp)
}
