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

func TestNodeIdentity_Get(t *testing.T) {
	ci.Parallel(t)

	// Create a test ACL server and client and perform our node identity get
	// tests against it.
	testACLServer, testServerToken, testACLServerCleanup := nomad.TestACLServer(t, nil)
	t.Cleanup(func() { testACLServerCleanup() })
	testutil.WaitForLeader(t, testACLServer.RPC)

	testACLClient, testACLClientCleanup := TestClient(t, func(c *config.Config) {
		c.ACLEnabled = true
		c.Servers = []string{testACLServer.GetConfig().RPCAddr.String()}
	})
	t.Cleanup(func() { _ = testACLClientCleanup() })
	testutil.WaitForClientStatusWithToken(
		t, testACLServer.RPC, testACLClient.NodeID(), testACLClient.Region(),
		structs.NodeStatusReady, testServerToken.SecretID,
	)

	t.Run("acl_denied", func(t *testing.T) {
		must.ErrorContains(
			t,
			testACLClient.ClientRPC(
				structs.NodeIdentityGetRPCMethod,
				&structs.NodeIdentityGetReq{},
				&structs.NodeIdentityGetResp{},
			),
			structs.ErrPermissionDenied.Error(),
		)
	})

	t.Run("acl_valid", func(t *testing.T) {

		aclPolicy := mock.NodePolicy(acl.PolicyRead)
		aclToken := mock.CreatePolicyAndToken(t, testACLServer.State(), 10, t.Name(), aclPolicy)

		req := structs.NodeIdentityGetReq{
			NodeID: testACLClient.NodeID(),
			QueryOptions: structs.QueryOptions{
				AuthToken: aclToken.SecretID,
			},
		}

		var resp structs.NodeIdentityGetResp

		must.NoError(
			t,
			testACLClient.ClientRPC(
				structs.NodeIdentityGetRPCMethod,
				&req,
				&resp,
			),
		)

		must.MapLen(t, 10, resp.Claims)

		must.MapContainsKeys(t, resp.Claims, []string{
			"aud",
			"exp",
			"jti",
			"nbf",
			"sub",
			"iat",
			"nomad_node_class",
			"nomad_node_datacenter",
			"nomad_node_id",
			"nomad_node_pool",
		})

		must.MapContainsValues(t, resp.Claims, []any{
			"nomadproject.io",
			testACLClient.NodeID(),
			testACLClient.Datacenter(),
			testACLClient.Node().NodeClass,
			testACLClient.Node().NodePool,
		})
	})

	// Create a test non-ACL server and client and perform our node identity get
	// tests against it.
	testServer, testServerCleanup := nomad.TestServer(t, nil)
	t.Cleanup(func() { testServerCleanup() })
	testutil.WaitForLeader(t, testServer.RPC)

	testClient, testClientCleanup := TestClient(t, func(c *config.Config) {
		c.Servers = []string{testServer.GetConfig().RPCAddr.String()}
	})
	t.Cleanup(func() { _ = testClientCleanup() })
	testutil.WaitForClient(t, testServer.RPC, testClient.NodeID(), testClient.Region())

	t.Run("non_acl_valid", func(t *testing.T) {

		req := structs.NodeIdentityGetReq{
			NodeID:       testACLClient.NodeID(),
			QueryOptions: structs.QueryOptions{},
		}

		var resp structs.NodeIdentityGetResp

		must.NoError(
			t,
			testClient.ClientRPC(
				structs.NodeIdentityGetRPCMethod,
				&req,
				&resp,
			),
		)

		must.MapLen(t, 10, resp.Claims)

		must.MapContainsKeys(t, resp.Claims, []string{
			"aud",
			"exp",
			"jti",
			"nbf",
			"sub",
			"iat",
			"nomad_node_class",
			"nomad_node_datacenter",
			"nomad_node_id",
			"nomad_node_pool",
		})

		must.MapContainsValues(t, resp.Claims, []any{
			"nomadproject.io",
			testClient.NodeID(),
			testClient.Datacenter(),
			testClient.Node().NodeClass,
			testClient.Node().NodePool,
		})
	})
}

func TestNodeIdentity_Renew(t *testing.T) {
	ci.Parallel(t)

	// Create a test ACL server and client and perform our node identity renewal
	// tests against it.
	testACLServer, testServerToken, testACLServerCleanup := nomad.TestACLServer(t, nil)
	t.Cleanup(func() { testACLServerCleanup() })
	testutil.WaitForLeader(t, testACLServer.RPC)

	testACLClient, testACLClientCleanup := TestClient(t, func(c *config.Config) {
		c.ACLEnabled = true
		c.Servers = []string{testACLServer.GetConfig().RPCAddr.String()}
	})
	t.Cleanup(func() { _ = testACLClientCleanup() })
	testutil.WaitForClientStatusWithToken(
		t, testACLServer.RPC, testACLClient.NodeID(), testACLClient.Region(),
		structs.NodeStatusReady, testServerToken.SecretID,
	)

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
		must.True(t, renewalVal)
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
	testutil.WaitForClient(t, testServer.RPC, testClient.NodeID(), testClient.Region())

	t.Run("non_acl_valid", func(t *testing.T) {
		must.NoError(
			t,
			testClient.ClientRPC(
				structs.NodeIdentityRenewRPCMethod,
				&structs.NodeIdentityRenewReq{
					NodeID:       testClient.NodeID(),
					QueryOptions: structs.QueryOptions{},
				},
				&structs.NodeIdentityRenewResp{},
			),
		)

		renewalVal := testClient.identityForceRenewal.Load()
		must.True(t, renewalVal)
	})
}
