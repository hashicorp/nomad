package client

import (
	"errors"
	"testing"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func TestRpc_streamingRpcConn_badEndpoint(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	s1 := nomad.TestServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	c := TestClient(t, func(c *config.Config) {
		c.Servers = []string{s1.GetConfig().RPCAddr.String()}
	})
	defer c.Shutdown()

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
	require.Contains(err.Error(), "unknown rpc method: \"Bogus\"")
}
