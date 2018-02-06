package nomad

import (
	"net"
	"net/rpc"
	"os"
	"path"
	"testing"
	"time"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/helper/pool"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/testutil"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rpcClient is a test helper method to return a ClientCodec to use to make rpc
// calls to the passed server.
func rpcClient(t *testing.T, s *Server) rpc.ClientCodec {
	addr := s.config.RPCAddr
	conn, err := net.DialTimeout("tcp", addr.String(), time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// Write the Consul RPC byte to set the mode
	conn.Write([]byte{byte(pool.RpcNomad)})
	return pool.NewClientCodec(conn)
}

func TestRPC_forwardLeader(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	s2 := TestServer(t, func(c *Config) {
		c.DevDisableBootstrap = true
	})
	defer s2.Shutdown()
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

func TestRPC_forwardRegion(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	s2 := TestServer(t, func(c *Config) {
		c.Region = "region2"
	})
	defer s2.Shutdown()
	TestJoin(t, s1, s2)
	testutil.WaitForLeader(t, s1.RPC)
	testutil.WaitForLeader(t, s2.RPC)

	var out struct{}
	err := s1.forwardRegion("region2", "Status.Ping", struct{}{}, &out)
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

	s1 := TestServer(t, func(c *Config) {
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
	defer s1.Shutdown()

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

	s1 := TestServer(t, func(c *Config) {
		c.DataDir = path.Join(dir, "node1")
		c.TLSConfig = &config.TLSConfig{
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert,
			KeyFile:              fookey,
		}
	})
	defer s1.Shutdown()

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

	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	s2 := TestServer(t, func(c *Config) {
		c.DevDisableBootstrap = true
	})
	defer s2.Shutdown()
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
	require.Contains(err.Error(), "unknown rpc method: \"Bogus\"")
}
