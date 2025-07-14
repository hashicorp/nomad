// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"testing"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
)

func TestNodeIdentity_Renew(t *testing.T) {
	ci.Parallel(t)

	// Create a test ACL server and client and perform our node identity renewal
	// tests against it.
	testACLServer, _, testACLServerCleanup := nomad.TestACLServer(t, nil)
	t.Cleanup(func() { testACLServerCleanup() })
	testutil.WaitForLeader(t, testACLServer.RPC)

	testACLClient, testACLClientCleanup := TestClient(t, func(c *config.Config) {
		c.ACLEnabled = true
		c.Servers = []string{testACLServer.GetConfig().RPCAddr.String()}
	})
	t.Cleanup(func() { _ = testACLClientCleanup() })

	t.Run("acl_denied", func(t *testing.T) {
		must.ErrorContains(
			t,
			testACLClient.ClientRPC(
				structs.NodeIdentityRenewRPCMethod,
				&structs.NodeIdentityRenewReq{},
				&structs.NodeIdentityRenewResp{},
			),
			structs.ErrPermissionDenied.Error(),
		)
	})

	t.Run("acl_valid", func(t *testing.T) {

		aclPolicy := mock.NodePolicy(acl.PolicyWrite)
		aclToken := mock.CreatePolicyAndToken(t, testACLServer.State(), 10, t.Name(), aclPolicy)

		req := structs.NodeIdentityRenewReq{
			NodeID: testACLClient.NodeID(),
			QueryOptions: structs.QueryOptions{
				AuthToken: aclToken.SecretID,
			},
		}

		must.NoError(
			t,
			testACLClient.ClientRPC(
				structs.NodeIdentityRenewRPCMethod,
				&req,
				&structs.NodeIdentityRenewResp{},
			),
		)

		renewalVal := testACLClient.identityForceRenewal.Load()
		must.True(t, renewalVal.(bool))
	})

	// Create a test non-ACL server and client and perform our node identity
	// renewal tests against it.
	testServer, testServerCleanup := nomad.TestServer(t, nil)
	t.Cleanup(func() { testServerCleanup() })
	testutil.WaitForLeader(t, testServer.RPC)

	testClient, testClientCleanup := TestClient(t, func(c *config.Config) {
		c.Servers = []string{testServer.GetConfig().RPCAddr.String()}
	})
	t.Cleanup(func() { _ = testClientCleanup() })

	t.Run("acl_denied", func(t *testing.T) {
		must.NoError(
			t,
			testClient.ClientRPC(
				structs.NodeIdentityRenewRPCMethod,
				&structs.NodeIdentityRenewReq{},
				&structs.NodeIdentityRenewResp{},
			),
		)

		renewalVal := testACLClient.identityForceRenewal.Load()
		must.True(t, renewalVal.(bool))
	})
}
