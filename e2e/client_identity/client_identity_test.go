// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package clientidentity

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/shoenig/test/must"
)

func TestClientIdentity(t *testing.T) {
	nomad := e2eutil.NomadClient(t)

	e2eutil.WaitForLeader(t, nomad)
	e2eutil.WaitForNodesReady(t, nomad, 1)

	t.Run("testClientIdentity", testClientIdentity)
}

func testClientIdentity(t *testing.T) {

	nomad := e2eutil.NomadClient(t)

	// Get the list of regions which should include a single entry, so we can
	// use it to validate the node identity claims.
	regionList, err := nomad.Regions().List()
	must.NoError(t, err)
	must.Len(t, 1, regionList)

	nodeList, _, err := nomad.Nodes().List(nil)
	must.NoError(t, err)
	must.Greater(t, 0, len(nodeList))

	// Create a context with a timeout to avoid waiting indefinitely for the
	// client to renew its identity. 40 seconds should be more than enough no
	// matter how long each client has until its next heartbeat.
	ctx, cancel := context.WithTimeout(t.Context(), 40*time.Second)
	defer cancel()

	// Use a wait group which allows us to trigger the identity renewal for all
	// nodes in parallel, speeding up the test completion time.
	wg := new(sync.WaitGroup)

	for _, node := range nodeList {

		// Perform an initial identity get request and validate the claims
		// before asking the client to renew its identity.
		nodeIdentityResp, err := nomad.Nodes().Identity().Get(
			&api.NodeIdentityGetRequest{
				NodeID: node.ID,
			},
			nil)
		must.NoError(t, err)
		must.MapNotEmpty(t, nodeIdentityResp.Claims)

		assertNodeIdentityClaims(t, regionList[0], node, nodeIdentityResp.Claims)

		wg.Add(1)

		go func(wg *sync.WaitGroup) {
			defer wg.Done()
			testClientIdentityRenew(t, ctx, nomad, node, nodeIdentityResp.Claims["jti"].(string), regionList[0])
		}(wg)
	}

	// Wait for all the go routines to complete.
	wg.Wait()
}

func testClientIdentityRenew(
	t *testing.T,
	ctx context.Context,
	client *api.Client,
	nodeStub *api.NodeListStub,
	jwtID, region string) {

	renewResp, err := client.Nodes().Identity().Renew(
		&api.NodeIdentityRenewRequest{NodeID: nodeStub.ID},
		nil,
	)
	must.NoError(t, err)
	must.NotNil(t, renewResp)

	// Use a ticker, so we can poll the client to view it's identity claims
	// until it has renewed.
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	// Wait for the client to renew its identity, or the context to timeout. If
	// the context times out then the test will fail and indicate the node did
	// not renew its identity within the expected time.
	for {
		select {
		case <-ctx.Done():
			t.Fatalf("timeout reached waiting for client %q to renew identity", nodeStub.ID)
			return
		case <-ticker.C:
			nodeIdentityResp, err := client.Nodes().Identity().Get(
				&api.NodeIdentityGetRequest{NodeID: nodeStub.ID},
				nil)
			must.NoError(t, err)
			must.MapNotEmpty(t, nodeIdentityResp.Claims)

			// If the jwtID has not changed, then continue to wait until the
			// client performs its heartbeat to refresh the identity.
			if nodeIdentityResp.Claims["jti"].(string) == jwtID {
				continue
			}

			assertNodeIdentityClaims(t, region, nodeStub, nodeIdentityResp.Claims)
			return
		}
	}
}

func assertNodeIdentityClaims(t *testing.T, region string, node *api.NodeListStub, claims map[string]any) {

	// Assert the Nomad node specific claims.
	must.Eq(t, node.ID, claims["nomad_node_id"].(string))
	must.Eq(t, node.Datacenter, claims["nomad_node_datacenter"].(string))
	must.Eq(t, node.NodePool, claims["nomad_node_pool"].(string))

	// Check the Nomad specific generic claims.
	must.Eq(t, "nomadproject.io", claims["aud"].(string))
	must.Eq(t, fmt.Sprintf("node:%s:%s:%s:default", region, node.NodePool, node.ID), claims["sub"].(string))

	// Check the standard claims that should be present. It's tricky to perform
	// exact matches on these as they are time based or generated values, so we
	// just check they are present and of the correct type.
	must.NotEq(t, "", claims["jti"].(string))
	must.Greater(t, 0, claims["iat"].(float64))
	must.Greater(t, 0, claims["nbf"].(float64))
	must.Greater(t, 0, claims["exp"].(float64))
}
