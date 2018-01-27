package nomad

import (
	"testing"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func TestServerWithNodeConn_NoPath(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	s2 := TestServer(t, func(c *Config) {
		c.DevDisableBootstrap = true
	})
	defer s2.Shutdown()
	TestJoin(t, s1, s2)
	testutil.WaitForLeader(t, s1.RPC)
	testutil.WaitForLeader(t, s2.RPC)

	nodeID := uuid.Generate()
	srv, err := s1.serverWithNodeConn(nodeID)
	require.Nil(srv)
	require.EqualError(err, structs.ErrNoNodeConn.Error())
}

func TestServerWithNodeConn_Path(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	s2 := TestServer(t, func(c *Config) {
		c.DevDisableBootstrap = true
	})
	defer s2.Shutdown()
	TestJoin(t, s1, s2)
	testutil.WaitForLeader(t, s1.RPC)
	testutil.WaitForLeader(t, s2.RPC)

	// Create a fake connection for the node on server 2
	nodeID := uuid.Generate()
	s2.addNodeConn(&RPCContext{
		NodeID: nodeID,
	})

	srv, err := s1.serverWithNodeConn(nodeID)
	require.NotNil(srv)
	require.Equal(srv.Addr.String(), s2.config.RPCAddr.String())
	require.Nil(err)
}

func TestServerWithNodeConn_Path_Newest(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	s2 := TestServer(t, func(c *Config) {
		c.DevDisableBootstrap = true
	})
	defer s2.Shutdown()
	s3 := TestServer(t, func(c *Config) {
		c.DevDisableBootstrap = true
	})
	defer s3.Shutdown()
	TestJoin(t, s1, s2, s3)
	testutil.WaitForLeader(t, s1.RPC)
	testutil.WaitForLeader(t, s2.RPC)
	testutil.WaitForLeader(t, s3.RPC)

	// Create a fake connection for the node on server 2 and 3
	nodeID := uuid.Generate()
	s2.addNodeConn(&RPCContext{
		NodeID: nodeID,
	})
	s3.addNodeConn(&RPCContext{
		NodeID: nodeID,
	})

	srv, err := s1.serverWithNodeConn(nodeID)
	require.NotNil(srv)
	require.Equal(srv.Addr.String(), s3.config.RPCAddr.String())
	require.Nil(err)
}

func TestServerWithNodeConn_PathAndErr(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	s2 := TestServer(t, func(c *Config) {
		c.DevDisableBootstrap = true
	})
	defer s2.Shutdown()
	s3 := TestServer(t, func(c *Config) {
		c.DevDisableBootstrap = true
	})
	defer s3.Shutdown()
	TestJoin(t, s1, s2, s3)
	testutil.WaitForLeader(t, s1.RPC)
	testutil.WaitForLeader(t, s2.RPC)
	testutil.WaitForLeader(t, s3.RPC)

	// Create a fake connection for the node on server 2
	nodeID := uuid.Generate()
	s2.addNodeConn(&RPCContext{
		NodeID: nodeID,
	})

	// Shutdown the RPC layer for server 3
	s3.rpcListener.Close()

	srv, err := s1.serverWithNodeConn(nodeID)
	require.NotNil(srv)
	require.Equal(srv.Addr.String(), s2.config.RPCAddr.String())
	require.Nil(err)
}

func TestServerWithNodeConn_NoPathAndErr(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	s2 := TestServer(t, func(c *Config) {
		c.DevDisableBootstrap = true
	})
	defer s2.Shutdown()
	s3 := TestServer(t, func(c *Config) {
		c.DevDisableBootstrap = true
	})
	defer s3.Shutdown()
	TestJoin(t, s1, s2, s3)
	testutil.WaitForLeader(t, s1.RPC)
	testutil.WaitForLeader(t, s2.RPC)
	testutil.WaitForLeader(t, s3.RPC)

	// Shutdown the RPC layer for server 3
	s3.rpcListener.Close()

	srv, err := s1.serverWithNodeConn(uuid.Generate())
	require.Nil(srv)
	require.NotNil(err)
	require.Contains(err.Error(), "failed querying")
}
