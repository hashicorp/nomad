// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"errors"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad"
	"github.com/hashicorp/nomad/nomad/structs"
	sconfig "github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
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

func Test_resolveServer(t *testing.T) {

	// note: we can't test a DNS name here without making an external DNS query,
	// which we don't want to do from CI
	testCases := []struct {
		name      string
		addr      string
		expect    string
		expectErr string
	}{
		{
			name:   "ipv6 no brackets",
			addr:   "2001:db8::1",
			expect: "[2001:db8::1]:4647",
		},
		{
			// expected bad result
			name:   "ambiguous ipv6 no brackets with port",
			addr:   "2001:db8::1:4647",
			expect: "[2001:db8::1:4647]:4647",
		},
		{
			name:   "ipv6 no port",
			addr:   "[2001:db8::1]",
			expect: "[2001:db8::1]:4647",
		},
		{
			name:   "ipv6 trailing port colon",
			addr:   "[2001:db8::1]:",
			expect: "[2001:db8::1]:4647",
		},
		{
			name:      "ipv6 malformed",
			addr:      "[2001:db8::1]:]",
			expectErr: "address [2001:db8::1]:]: unexpected ']' in address",
		},
		{
			name:   "ipv6 with port",
			addr:   "[2001:db8::1]:6647",
			expect: "[2001:db8::1]:6647",
		},
		{
			name:   "ipv4 no port",
			addr:   "192.168.1.117",
			expect: "192.168.1.117:4647",
		},
		{
			name:   "ipv4 trailing port colon",
			addr:   "192.168.1.117:",
			expect: "192.168.1.117:4647",
		},
		{
			name:   "ipv4 with port",
			addr:   "192.168.1.117:6647",
			expect: "192.168.1.117:6647",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			addr, err := resolveServer(tc.addr)
			if tc.expectErr != "" {
				must.Nil(t, addr)
				must.EqError(t, err, tc.expectErr)
			} else {
				must.NoError(t, err)
				must.Eq(t, tc.expect, addr.String())
			}
		})
	}

}

func TestRpc_RetryBlockTime(t *testing.T) {
	ci.Parallel(t)

	// Timeouts have to allow for multiple passes thru the recursive c.rpc
	// call. Unconfigurable internal timeouts prevent us from using a shorter
	// MaxQueryTime base for this test
	expectMaxQueryTime := time.Second
	rpcHoldTimeout := 5 * time.Second
	unblockTimeout := 7 * time.Second

	srv, cleanupSrv := nomad.TestServer(t, func(c *nomad.Config) {
		c.NumSchedulers = 0
		c.BootstrapExpect = 3 // we intentionally don't want a leader
	})
	t.Cleanup(func() { cleanupSrv() })

	c, cleanupC := TestClient(t, func(c *config.Config) {
		c.Servers = []string{srv.GetConfig().RPCAddr.String()}
		c.RPCHoldTimeout = rpcHoldTimeout
	})
	t.Cleanup(func() { cleanupC() })

	req := structs.NodeSpecificRequest{
		NodeID:   c.NodeID(),
		SecretID: c.secretNodeID(),
		QueryOptions: structs.QueryOptions{
			Region:        c.Region(),
			AuthToken:     c.secretNodeID(),
			MinQueryIndex: 10000, // some far-flung index we know won't exist yet
			MaxQueryTime:  expectMaxQueryTime,
		},
	}

	resp := structs.NodeClientAllocsResponse{}
	errCh := make(chan error)

	go func() {
		err := c.rpc("Node.GetClientAllocs", &req, &resp)
		errCh <- err
	}()

	// wait for the blocking query to run long enough for 2 passes thru,
	// including jitter
	select {
	case err := <-errCh:
		must.NoError(t, err)
	case <-time.After(unblockTimeout):
		cleanupC() // force unblock
	}

	must.Eq(t, expectMaxQueryTime, req.MaxQueryTime,
		must.Sprintf("MaxQueryTime was changed during retries but not reset"))
}
