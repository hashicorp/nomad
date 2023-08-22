// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package apitests

import (
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/testutil"
)

type configCallback func(c *api.Config)

// seen is used to track which tests we have already marked as parallel
var seen map[*testing.T]struct{}

func init() {
	seen = make(map[*testing.T]struct{})
}

func makeACLClient(t *testing.T, cb1 configCallback,
	cb2 testutil.ServerConfigCallback) (*api.Client, *testutil.TestServer, *api.ACLToken) {
	client, server := makeClient(t, cb1, func(c *testutil.TestServerConfig) {
		c.ACL.Enabled = true
		if cb2 != nil {
			cb2(c)
		}
	})

	// Get the root token
	root, _, err := client.ACLTokens().BootstrapOpts("", nil)
	if err != nil {
		t.Fatalf("failed to bootstrap ACLs: %v", err)
	}
	client.SetSecretID(root.SecretID)
	return client, server, root
}

func makeClient(t *testing.T, cb1 configCallback,
	cb2 testutil.ServerConfigCallback) (*api.Client, *testutil.TestServer) {
	// Make client config
	conf := api.DefaultConfig()
	if cb1 != nil {
		cb1(conf)
	}

	// Create server
	server := testutil.NewTestServer(t, cb2)
	conf.Address = "http://" + server.HTTPAddr

	// Create client
	client, err := api.NewClient(conf)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	return client, server
}
