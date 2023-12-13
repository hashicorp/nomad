// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"errors"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad"
	"github.com/hashicorp/nomad/nomad/structs"
	sconfig "github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func TestRpc_streamingRpcConn_badEndpoint(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	s1, cleanupS1 := nomad.TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	c, cleanupC := TestClient(t, func(c *config.Config) {
		c.Servers = []string{s1.GetConfig().RPCAddr.String()}
	})
	defer cleanupC()

	// Wait for the client to connect
	testutil.WaitForResult(func() (bool, error) {
		node, err := s1.State().NodeByID(nil, c.NodeID())
		if err != nil {
			return false, err
		}
		if node == nil {
			return false, errors.New("no node")
		}

		return node.Status == structs.NodeStatusReady, errors.New("wrong status")
	}, func(err error) {
		t.Fatalf("should have a clients")
	})

	// Get the server
	server := c.servers.FindServer()
	require.NotNil(server)

	conn, err := c.streamingRpcConn(server, "Bogus")
	require.Nil(conn)
	require.NotNil(err)
	require.Contains(err.Error(), "Unknown rpc method: \"Bogus\"")
}

func TestRpc_streamingRpcConn_badEndpoint_TLS(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	const (
		cafile        = "../helper/tlsutil/testdata/nomad-agent-ca.pem"
		fooservercert = "../helper/tlsutil/testdata/regionFoo-server-nomad.pem"
		fooserverkey  = "../helper/tlsutil/testdata/regionFoo-server-nomad-key.pem"
	)

	s1, cleanupS1 := nomad.TestServer(t, func(c *nomad.Config) {
		c.Region = "regionFoo"
		c.BootstrapExpect = 1
		c.TLSConfig = &sconfig.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             fooservercert,
			KeyFile:              fooserverkey,
		}
	})
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	c, cleanupC := TestClient(t, func(c *config.Config) {
		c.Region = "regionFoo"
		c.Servers = []string{s1.GetConfig().RPCAddr.String()}
		c.TLSConfig = &sconfig.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             fooservercert,
			KeyFile:              fooserverkey,
		}
	})
	defer cleanupC()

	// Wait for the client to connect
	testutil.WaitForResult(func() (bool, error) {
		node, err := s1.State().NodeByID(nil, c.NodeID())
		if err != nil {
			return false, err
		}
		if node == nil {
			return false, errors.New("no node")
		}

		return node.Status == structs.NodeStatusReady, errors.New("wrong status")
	}, func(err error) {
		t.Fatalf("should have a clients")
	})

	// Get the server
	server := c.servers.FindServer()
	require.NotNil(server)

	conn, err := c.streamingRpcConn(server, "Bogus")
	require.Nil(conn)
	require.NotNil(err)
	require.Contains(err.Error(), "Unknown rpc method: \"Bogus\"")
}
