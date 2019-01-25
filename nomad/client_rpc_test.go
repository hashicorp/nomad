package nomad

import (
	"net"
	"testing"

	"github.com/hashicorp/nomad/client"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

type namedConnWrapper struct {
	net.Conn
	name string
}

type namedAddr string

func (n namedAddr) String() string  { return string(n) }
func (n namedAddr) Network() string { return string(n) }

func (n namedConnWrapper) LocalAddr() net.Addr {
	return namedAddr(n.name)
}

func TestServer_removeNodeConn_differentAddrs(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	p1, p2 := net.Pipe()
	w1 := namedConnWrapper{
		Conn: p1,
		name: "a",
	}
	w2 := namedConnWrapper{
		Conn: p2,
		name: "b",
	}

	// Add the connections
	nodeID := uuid.Generate()
	ctx1 := &RPCContext{
		Conn:   w1,
		NodeID: nodeID,
	}
	ctx2 := &RPCContext{
		Conn:   w2,
		NodeID: nodeID,
	}

	s1.addNodeConn(ctx1)
	s1.addNodeConn(ctx2)
	require.Len(s1.connectedNodes(), 1)
	require.Len(s1.nodeConns[nodeID], 2)

	// Check that the value is the second conn.
	state, ok := s1.getNodeConn(nodeID)
	require.True(ok)
	require.Equal(state.Ctx.Conn.LocalAddr().String(), w2.name)

	// Delete the first
	s1.removeNodeConn(ctx1)
	require.Len(s1.connectedNodes(), 1)
	require.Len(s1.nodeConns[nodeID], 1)

	// Check that the value is the second conn.
	state, ok = s1.getNodeConn(nodeID)
	require.True(ok)
	require.Equal(state.Ctx.Conn.LocalAddr().String(), w2.name)

	// Delete the second
	s1.removeNodeConn(ctx2)
	require.Len(s1.connectedNodes(), 0)

	_, ok = s1.getNodeConn(nodeID)
	require.False(ok)
}

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
	srv, err := s1.serverWithNodeConn(nodeID, s1.Region())
	require.Nil(srv)
	require.EqualError(err, structs.ErrNoNodeConn.Error())
}

func TestServerWithNodeConn_NoPath_Region(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	nodeID := uuid.Generate()
	srv, err := s1.serverWithNodeConn(nodeID, "fake-region")
	require.Nil(srv)
	require.EqualError(err, structs.ErrNoRegionPath.Error())
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

	srv, err := s1.serverWithNodeConn(nodeID, s1.Region())
	require.NotNil(srv)
	require.Equal(srv.Addr.String(), s2.config.RPCAddr.String())
	require.Nil(err)
}

func TestServerWithNodeConn_Path_Region(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	s2 := TestServer(t, func(c *Config) {
		c.Region = "two"
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

	srv, err := s1.serverWithNodeConn(nodeID, s2.Region())
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

	srv, err := s1.serverWithNodeConn(nodeID, s1.Region())
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

	srv, err := s1.serverWithNodeConn(nodeID, s1.Region())
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

	srv, err := s1.serverWithNodeConn(uuid.Generate(), s1.Region())
	require.Nil(srv)
	require.NotNil(err)
	require.Contains(err.Error(), "failed querying")
}

func TestNodeStreamingRpc_badEndpoint(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	c, cleanup := client.TestClient(t, func(c *config.Config) {
		c.Servers = []string{s1.config.RPCAddr.String()}
	})
	defer cleanup()

	// Wait for the client to connect
	testutil.WaitForResult(func() (bool, error) {
		nodes := s1.connectedNodes()
		return len(nodes) == 1, nil
	}, func(err error) {
		t.Fatalf("should have a clients")
	})

	state, ok := s1.getNodeConn(c.NodeID())
	require.True(ok)

	conn, err := NodeStreamingRpc(state.Session, "Bogus")
	require.Nil(conn)
	require.NotNil(err)
	require.Contains(err.Error(), "Bogus")
	require.True(structs.IsErrUnknownMethod(err))
}
