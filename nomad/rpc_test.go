package nomad

import (
	"context"
	"net"
	"net/rpc"
	"os"
	"path"
	"testing"
	"time"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/pool"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/testutil"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/yamux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ugorji/go/codec"
)

// rpcClient is a test helper method to return a ClientCodec to use to make rpc
// calls to the passed server.
func rpcClient(t *testing.T, s *Server) rpc.ClientCodec {
	addr := s.config.RPCAddr
	conn, err := net.DialTimeout("tcp", addr.String(), time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// Write the Nomad RPC byte to set the mode
	conn.Write([]byte{byte(pool.RpcNomad)})
	return pool.NewClientCodec(conn)
}

func TestRPC_forwardLeader(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.DevDisableBootstrap = true
	})
	defer cleanupS2()
	TestJoin(t, s1, s2)
	testutil.WaitForLeader(t, s1.RPC)
	testutil.WaitForLeader(t, s2.RPC)

	isLeader, remote := s1.getLeader()
	if !isLeader && remote == nil {
		t.Fatalf("missing leader")
	}

	if remote != nil {
		var out struct{}
		err := s1.forwardLeader(remote, "Status.Ping", struct{}{}, &out)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	isLeader, remote = s2.getLeader()
	if !isLeader && remote == nil {
		t.Fatalf("missing leader")
	}

	if remote != nil {
		var out struct{}
		err := s2.forwardLeader(remote, "Status.Ping", struct{}{}, &out)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}
}

func TestRPC_WaitForConsistentReads(t *testing.T) {
	t.Parallel()

	s1, cleanupS2 := TestServer(t, func(c *Config) {
		c.RPCHoldTimeout = 20 * time.Millisecond
	})
	defer cleanupS2()
	testutil.WaitForLeader(t, s1.RPC)

	isLeader, _ := s1.getLeader()
	require.True(t, isLeader)
	require.True(t, s1.isReadyForConsistentReads())

	s1.resetConsistentReadReady()
	require.False(t, s1.isReadyForConsistentReads())

	codec := rpcClient(t, s1)

	get := &structs.JobListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: "default",
		},
	}

	// check timeout while waiting for consistency
	var resp structs.JobListResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.List", get, &resp)
	require.Error(t, err)
	require.Contains(t, err.Error(), structs.ErrNotReadyForConsistentReads.Error())

	// check we wait and block
	go func() {
		time.Sleep(5 * time.Millisecond)
		s1.setConsistentReadReady()
	}()

	err = msgpackrpc.CallWithCodec(codec, "Job.List", get, &resp)
	require.NoError(t, err)

}

func TestRPC_forwardRegion(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.Region = "global"
	})
	defer cleanupS2()
	TestJoin(t, s1, s2)
	testutil.WaitForLeader(t, s1.RPC)
	testutil.WaitForLeader(t, s2.RPC)

	var out struct{}
	err := s1.forwardRegion("global", "Status.Ping", struct{}{}, &out)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = s2.forwardRegion("global", "Status.Ping", struct{}{}, &out)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestRPC_PlaintextRPCSucceedsWhenInUpgradeMode(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	const (
		cafile  = "../helper/tlsutil/testdata/ca.pem"
		foocert = "../helper/tlsutil/testdata/nomad-foo.pem"
		fookey  = "../helper/tlsutil/testdata/nomad-foo-key.pem"
	)
	dir := tmpDir(t)
	defer os.RemoveAll(dir)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.DataDir = path.Join(dir, "node1")
		c.TLSConfig = &config.TLSConfig{
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert,
			KeyFile:              fookey,
			RPCUpgradeMode:       true,
		}
	})
	defer cleanupS1()

	codec := rpcClient(t, s1)

	// Create the register request
	node := mock.Node()
	req := &structs.NodeRegisterRequest{
		Node:         node,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "Node.Register", req, &resp)
	assert.Nil(err)

	// Check that heartbeatTimers has the heartbeat ID
	_, ok := s1.heartbeatTimers[node.ID]
	assert.True(ok)
}

func TestRPC_PlaintextRPCFailsWhenNotInUpgradeMode(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	const (
		cafile  = "../helper/tlsutil/testdata/ca.pem"
		foocert = "../helper/tlsutil/testdata/nomad-foo.pem"
		fookey  = "../helper/tlsutil/testdata/nomad-foo-key.pem"
	)
	dir := tmpDir(t)
	defer os.RemoveAll(dir)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.DataDir = path.Join(dir, "node1")
		c.TLSConfig = &config.TLSConfig{
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert,
			KeyFile:              fookey,
		}
	})
	defer cleanupS1()

	codec := rpcClient(t, s1)

	node := mock.Node()
	req := &structs.NodeRegisterRequest{
		Node:         node,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "Node.Register", req, &resp)
	assert.NotNil(err)
}

func TestRPC_streamingRpcConn_badMethod(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.DevDisableBootstrap = true
	})
	defer cleanupS2()
	TestJoin(t, s1, s2)
	testutil.WaitForLeader(t, s1.RPC)
	testutil.WaitForLeader(t, s2.RPC)

	s1.peerLock.RLock()
	ok, parts := isNomadServer(s2.LocalMember())
	require.True(ok)
	server := s1.localPeers[raft.ServerAddress(parts.Addr.String())]
	require.NotNil(server)
	s1.peerLock.RUnlock()

	conn, err := s1.streamingRpc(server, "Bogus")
	require.Nil(conn)
	require.NotNil(err)
	require.Contains(err.Error(), "Bogus")
	require.True(structs.IsErrUnknownMethod(err))
}

func TestRPC_streamingRpcConn_badMethod_TLS(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	const (
		cafile  = "../helper/tlsutil/testdata/ca.pem"
		foocert = "../helper/tlsutil/testdata/nomad-foo.pem"
		fookey  = "../helper/tlsutil/testdata/nomad-foo-key.pem"
	)
	dir := tmpDir(t)
	defer os.RemoveAll(dir)
	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.Region = "regionFoo"
		c.BootstrapExpect = 2
		c.DevMode = false
		c.DevDisableBootstrap = true
		c.DataDir = path.Join(dir, "node1")
		c.TLSConfig = &config.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert,
			KeyFile:              fookey,
		}
	})
	defer cleanupS1()

	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.Region = "regionFoo"
		c.BootstrapExpect = 2
		c.DevMode = false
		c.DevDisableBootstrap = true
		c.DataDir = path.Join(dir, "node2")
		c.TLSConfig = &config.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert,
			KeyFile:              fookey,
		}
	})
	defer cleanupS2()

	TestJoin(t, s1, s2)
	testutil.WaitForLeader(t, s1.RPC)

	s1.peerLock.RLock()
	ok, parts := isNomadServer(s2.LocalMember())
	require.True(ok)
	server := s1.localPeers[raft.ServerAddress(parts.Addr.String())]
	require.NotNil(server)
	s1.peerLock.RUnlock()

	conn, err := s1.streamingRpc(server, "Bogus")
	require.Nil(conn)
	require.NotNil(err)
	require.Contains(err.Error(), "Bogus")
	require.True(structs.IsErrUnknownMethod(err))
}

func TestRPC_streamingRpcConn_goodMethod_Plaintext(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	dir := tmpDir(t)
	defer os.RemoveAll(dir)
	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.Region = "regionFoo"
		c.BootstrapExpect = 2
		c.DevMode = false
		c.DevDisableBootstrap = true
		c.DataDir = path.Join(dir, "node1")
	})
	defer cleanupS1()

	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.Region = "regionFoo"
		c.BootstrapExpect = 2
		c.DevMode = false
		c.DevDisableBootstrap = true
		c.DataDir = path.Join(dir, "node2")
	})
	defer cleanupS2()

	TestJoin(t, s1, s2)
	testutil.WaitForLeader(t, s1.RPC)

	s1.peerLock.RLock()
	ok, parts := isNomadServer(s2.LocalMember())
	require.True(ok)
	server := s1.localPeers[raft.ServerAddress(parts.Addr.String())]
	require.NotNil(server)
	s1.peerLock.RUnlock()

	conn, err := s1.streamingRpc(server, "FileSystem.Logs")
	require.NotNil(conn)
	require.NoError(err)

	decoder := codec.NewDecoder(conn, structs.MsgpackHandle)
	encoder := codec.NewEncoder(conn, structs.MsgpackHandle)

	allocID := uuid.Generate()
	require.NoError(encoder.Encode(cstructs.FsStreamRequest{
		AllocID: allocID,
		QueryOptions: structs.QueryOptions{
			Region: "regionFoo",
		},
	}))

	var result cstructs.StreamErrWrapper
	require.NoError(decoder.Decode(&result))
	require.Empty(result.Payload)
	require.True(structs.IsErrUnknownAllocation(result.Error))
}

func TestRPC_streamingRpcConn_goodMethod_TLS(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	const (
		cafile  = "../helper/tlsutil/testdata/ca.pem"
		foocert = "../helper/tlsutil/testdata/nomad-foo.pem"
		fookey  = "../helper/tlsutil/testdata/nomad-foo-key.pem"
	)
	dir := tmpDir(t)
	defer os.RemoveAll(dir)
	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.Region = "regionFoo"
		c.BootstrapExpect = 2
		c.DevMode = false
		c.DevDisableBootstrap = true
		c.DataDir = path.Join(dir, "node1")
		c.TLSConfig = &config.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert,
			KeyFile:              fookey,
		}
	})
	defer cleanupS1()

	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.Region = "regionFoo"
		c.BootstrapExpect = 2
		c.DevMode = false
		c.DevDisableBootstrap = true
		c.DataDir = path.Join(dir, "node2")
		c.TLSConfig = &config.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert,
			KeyFile:              fookey,
		}
	})
	defer cleanupS2()

	TestJoin(t, s1, s2)
	testutil.WaitForLeader(t, s1.RPC)

	s1.peerLock.RLock()
	ok, parts := isNomadServer(s2.LocalMember())
	require.True(ok)
	server := s1.localPeers[raft.ServerAddress(parts.Addr.String())]
	require.NotNil(server)
	s1.peerLock.RUnlock()

	conn, err := s1.streamingRpc(server, "FileSystem.Logs")
	require.NotNil(conn)
	require.NoError(err)

	decoder := codec.NewDecoder(conn, structs.MsgpackHandle)
	encoder := codec.NewEncoder(conn, structs.MsgpackHandle)

	allocID := uuid.Generate()
	require.NoError(encoder.Encode(cstructs.FsStreamRequest{
		AllocID: allocID,
		QueryOptions: structs.QueryOptions{
			Region: "regionFoo",
		},
	}))

	var result cstructs.StreamErrWrapper
	require.NoError(decoder.Decode(&result))
	require.Empty(result.Payload)
	require.True(structs.IsErrUnknownAllocation(result.Error))
}

// COMPAT: Remove in 0.10
// This is a very low level test to assert that the V2 handling works. It is
// making manual RPC calls since no helpers exist at this point since we are
// only implementing support for v2 but not using it yet. In the future we can
// switch the conn pool to establishing v2 connections and we can deprecate this
// test.
func TestRPC_handleMultiplexV2(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	s, cleanupS := TestServer(t, nil)
	defer cleanupS()
	testutil.WaitForLeader(t, s.RPC)

	p1, p2 := net.Pipe()
	defer p1.Close()
	defer p2.Close()

	// Start the handler
	doneCh := make(chan struct{})
	go func() {
		s.handleConn(context.Background(), p2, &RPCContext{Conn: p2})
		close(doneCh)
	}()

	// Establish the MultiplexV2 connection
	_, err := p1.Write([]byte{byte(pool.RpcMultiplexV2)})
	require.Nil(err)

	// Make two streams
	conf := yamux.DefaultConfig()
	conf.LogOutput = nil
	conf.Logger = testlog.Logger(t)
	session, err := yamux.Client(p1, conf)
	require.Nil(err)

	s1, err := session.Open()
	require.Nil(err)
	defer s1.Close()

	s2, err := session.Open()
	require.Nil(err)
	defer s2.Close()

	// Make an RPC
	_, err = s1.Write([]byte{byte(pool.RpcNomad)})
	require.Nil(err)

	args := &structs.GenericRequest{}
	var l string
	err = msgpackrpc.CallWithCodec(pool.NewClientCodec(s1), "Status.Leader", args, &l)
	require.Nil(err)
	require.NotEmpty(l)

	// Make a streaming RPC
	_, err = s.streamingRpcImpl(s2, s.Region(), "Bogus")
	require.NotNil(err)
	require.Contains(err.Error(), "Bogus")
	require.True(structs.IsErrUnknownMethod(err))

}
