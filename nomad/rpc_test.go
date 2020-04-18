package nomad

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/rpc"
	"os"
	"path"
	"testing"
	"time"

	"github.com/hashicorp/go-msgpack/codec"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/pool"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/tlsutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/testutil"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/yamux"
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
	// Write the Nomad RPC byte to set the mode
	conn.Write([]byte{byte(pool.RpcNomad)})
	return pool.NewClientCodec(conn)
}

func TestRPC_forwardLeader(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 2
	})
	defer cleanupS1()
	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 2
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

func TestRPC_getServer(t *testing.T) {
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

	// Lookup by name
	srv, err := s1.getServer("global", s2.serf.LocalMember().Name)
	require.NoError(t, err)

	require.Equal(t, srv.Name, s2.serf.LocalMember().Name)

	// Lookup by id
	srv, err = s2.getServer("global", s1.serf.LocalMember().Tags["id"])
	require.NoError(t, err)

	require.Equal(t, srv.Name, s1.serf.LocalMember().Name)
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

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 2
	})
	defer cleanupS1()
	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 2
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
		c.DataDir = path.Join(dir, "node1")
	})
	defer cleanupS1()

	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.Region = "regionFoo"
		c.BootstrapExpect = 2
		c.DevMode = false
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
	_, err = s2.Write([]byte{byte(pool.RpcStreaming)})
	require.Nil(err)

	_, err = s.streamingRpcImpl(s2, "Bogus")
	require.NotNil(err)
	require.Contains(err.Error(), "Bogus")
	require.True(structs.IsErrUnknownMethod(err))

}

// TestRPC_TLS_in_TLS asserts that trying to nest TLS connections fails.
func TestRPC_TLS_in_TLS(t *testing.T) {
	t.Parallel()

	const (
		cafile  = "../helper/tlsutil/testdata/ca.pem"
		foocert = "../helper/tlsutil/testdata/nomad-foo.pem"
		fookey  = "../helper/tlsutil/testdata/nomad-foo-key.pem"
	)

	s, cleanup := TestServer(t, func(c *Config) {
		c.TLSConfig = &config.TLSConfig{
			EnableRPC: true,
			CAFile:    cafile,
			CertFile:  foocert,
			KeyFile:   fookey,
		}
	})
	defer func() {
		cleanup()

		//TODO Avoid panics from logging during shutdown
		time.Sleep(1 * time.Second)
	}()

	conn, err := net.DialTimeout("tcp", s.config.RPCAddr.String(), time.Second)
	require.NoError(t, err)
	defer conn.Close()

	_, err = conn.Write([]byte{byte(pool.RpcTLS)})
	require.NoError(t, err)

	// Client TLS verification isn't necessary for
	// our assertions
	tlsConf, err := tlsutil.NewTLSConfiguration(s.config.TLSConfig, false, true)
	require.NoError(t, err)
	outTLSConf, err := tlsConf.OutgoingTLSConfig()
	require.NoError(t, err)
	outTLSConf.InsecureSkipVerify = true

	// Do initial handshake
	tlsConn := tls.Client(conn, outTLSConf)
	require.NoError(t, tlsConn.Handshake())
	conn = tlsConn

	// Try to create a nested TLS connection
	_, err = conn.Write([]byte{byte(pool.RpcTLS)})
	require.NoError(t, err)

	// Attempts at nested TLS connections should cause a disconnect
	buf := []byte{0}
	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	n, err := conn.Read(buf)
	require.Zero(t, n)
	require.Equal(t, io.EOF, err)
}

// TestRPC_Limits_OK asserts that all valid limits combinations
// (tls/timeout/conns) work.
//
// Invalid limits are tested in command/agent/agent_test.go
func TestRPC_Limits_OK(t *testing.T) {
	t.Parallel()

	const (
		cafile   = "../helper/tlsutil/testdata/ca.pem"
		foocert  = "../helper/tlsutil/testdata/nomad-foo.pem"
		fookey   = "../helper/tlsutil/testdata/nomad-foo-key.pem"
		maxConns = 10 // limit must be < this for testing
	)

	cases := []struct {
		tls           bool
		timeout       time.Duration
		limit         int
		assertTimeout bool
		assertLimit   bool
	}{
		{
			tls:           false,
			timeout:       5 * time.Second,
			limit:         0,
			assertTimeout: true,
			assertLimit:   false,
		},
		{
			tls:           true,
			timeout:       5 * time.Second,
			limit:         0,
			assertTimeout: true,
			assertLimit:   false,
		},
		{
			tls:           false,
			timeout:       0,
			limit:         0,
			assertTimeout: false,
			assertLimit:   false,
		},
		{
			tls:           true,
			timeout:       0,
			limit:         0,
			assertTimeout: false,
			assertLimit:   false,
		},
		{
			tls:           false,
			timeout:       0,
			limit:         2,
			assertTimeout: false,
			assertLimit:   true,
		},
		{
			tls:           true,
			timeout:       0,
			limit:         2,
			assertTimeout: false,
			assertLimit:   true,
		},
		{
			tls:           false,
			timeout:       5 * time.Second,
			limit:         2,
			assertTimeout: true,
			assertLimit:   true,
		},
		{
			tls:           true,
			timeout:       5 * time.Second,
			limit:         2,
			assertTimeout: true,
			assertLimit:   true,
		},
	}

	assertTimeout := func(t *testing.T, s *Server, useTLS bool, timeout time.Duration) {
		// Increase timeout to detect timeouts
		clientTimeout := timeout + time.Second

		conn, err := net.DialTimeout("tcp", s.config.RPCAddr.String(), 1*time.Second)
		require.NoError(t, err)
		defer conn.Close()

		buf := []byte{0}
		readDeadline := time.Now().Add(clientTimeout)
		conn.SetReadDeadline(readDeadline)
		n, err := conn.Read(buf)
		require.Zero(t, n)
		if timeout == 0 {
			// Server should *not* have timed out.
			// Now() should always be after the client read deadline, but
			// isn't a sufficient assertion for correctness as slow tests
			// may cause this to be true even if the server timed out.
			now := time.Now()
			require.Truef(t, now.After(readDeadline),
				"Client read deadline (%s) should be in the past (before %s)", readDeadline, now)

			testutil.RequireDeadlineErr(t, err)
			return
		}

		// Server *should* have timed out (EOF)
		require.Equal(t, io.EOF, err)

		// Create a new connection to assert timeout doesn't
		// apply after first byte.
		conn, err = net.DialTimeout("tcp", s.config.RPCAddr.String(), time.Second)
		require.NoError(t, err)
		defer conn.Close()

		if useTLS {
			_, err := conn.Write([]byte{byte(pool.RpcTLS)})
			require.NoError(t, err)

			// Client TLS verification isn't necessary for
			// our assertions
			tlsConf, err := tlsutil.NewTLSConfiguration(s.config.TLSConfig, false, true)
			require.NoError(t, err)
			outTLSConf, err := tlsConf.OutgoingTLSConfig()
			require.NoError(t, err)
			outTLSConf.InsecureSkipVerify = true

			tlsConn := tls.Client(conn, outTLSConf)
			require.NoError(t, tlsConn.Handshake())

			conn = tlsConn
		}

		// Writing the Nomad RPC byte should be sufficient to
		// disable the handshake timeout
		n, err = conn.Write([]byte{byte(pool.RpcNomad)})
		require.NoError(t, err)
		require.Equal(t, 1, n)

		// Read should timeout due to client timeout, not
		// server's timeout
		readDeadline = time.Now().Add(clientTimeout)
		conn.SetReadDeadline(readDeadline)
		n, err = conn.Read(buf)
		require.Zero(t, n)
		testutil.RequireDeadlineErr(t, err)
	}

	assertNoLimit := func(t *testing.T, addr string) {
		var err error

		// Create max connections
		conns := make([]net.Conn, maxConns)
		errCh := make(chan error, maxConns)
		for i := 0; i < maxConns; i++ {
			conns[i], err = net.DialTimeout("tcp", addr, 1*time.Second)
			require.NoError(t, err)
			defer conns[i].Close()

			go func(i int) {
				buf := []byte{0}
				readDeadline := time.Now().Add(1 * time.Second)
				conns[i].SetReadDeadline(readDeadline)
				n, err := conns[i].Read(buf)
				if n > 0 {
					errCh <- fmt.Errorf("n > 0: %d", n)
					return
				}
				errCh <- err
			}(i)
		}

		// Now assert each error is a clientside read deadline error
		deadline := time.After(10 * time.Second)
		for i := 0; i < maxConns; i++ {
			select {
			case <-deadline:
				t.Fatalf("timed out waiting for conn error %d/%d", i+1, maxConns)
			case err := <-errCh:
				testutil.RequireDeadlineErr(t, err)
			}
		}
	}

	assertLimit := func(t *testing.T, addr string, limit int) {
		var err error

		// Create limit connections
		conns := make([]net.Conn, limit)
		errCh := make(chan error, limit)
		for i := range conns {
			conns[i], err = net.DialTimeout("tcp", addr, 1*time.Second)
			require.NoError(t, err)
			defer conns[i].Close()

			go func(i int) {
				buf := []byte{0}
				n, err := conns[i].Read(buf)
				if n > 0 {
					errCh <- fmt.Errorf("n > 0: %d", n)
					return
				}
				errCh <- err
			}(i)
		}

		// Assert a new connection is dropped
		conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
		require.NoError(t, err)
		defer conn.Close()

		buf := []byte{0}
		deadline := time.Now().Add(6 * time.Second)
		conn.SetReadDeadline(deadline)
		n, err := conn.Read(buf)
		require.Zero(t, n)
		require.Equal(t, io.EOF, err)

		// Assert existing connections are ok
	ERRCHECK:
		select {
		case err := <-errCh:
			t.Errorf("unexpected error from idle connection: (%T) %v", err, err)
			goto ERRCHECK
		default:
		}

		// Cleanup
		for _, conn := range conns {
			conn.Close()
		}
		for i := range conns {
			select {
			case err := <-errCh:
				require.Contains(t, err.Error(), "use of closed network connection")
			case <-time.After(10 * time.Second):
				t.Fatalf("timed out waiting for connection %d/%d to close", i, len(conns))
			}
		}
	}

	for i := range cases {
		tc := cases[i]
		name := fmt.Sprintf("%d-tls-%t-timeout-%s-limit-%v", i, tc.tls, tc.timeout, tc.limit)
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if tc.limit >= maxConns {
				t.Fatalf("test fixture failure: cannot assert limit (%d) >= max (%d)", tc.limit, maxConns)
			}
			if tc.assertTimeout && tc.timeout == 0 {
				t.Fatalf("test fixture failure: cannot assert timeout when no timeout set (0)")
			}

			s, cleanup := TestServer(t, func(c *Config) {
				if tc.tls {
					c.TLSConfig = &config.TLSConfig{
						EnableRPC: true,
						CAFile:    cafile,
						CertFile:  foocert,
						KeyFile:   fookey,
					}
				}
				c.RPCHandshakeTimeout = tc.timeout
				c.RPCMaxConnsPerClient = tc.limit
			})
			defer func() {
				cleanup()

				//TODO Avoid panics from logging during shutdown
				time.Sleep(1 * time.Second)
			}()

			assertTimeout(t, s, tc.tls, tc.timeout)

			if tc.assertLimit {
				// There's a race between assertTimeout(false) closing
				// its connection and the HTTP server noticing and
				// untracking it. Since there's no way to coordiante
				// when this occurs, sleeping is the only way to avoid
				// asserting limits before the timed out connection is
				// untracked.
				time.Sleep(1 * time.Second)

				assertLimit(t, s.config.RPCAddr.String(), tc.limit)
			} else {
				assertNoLimit(t, s.config.RPCAddr.String())
			}
		})
	}
}

// TestRPC_Limits_Streaming asserts that the streaming RPC limit is lower than
// the overall connection limit to prevent DOS via server-routed streaming API
// calls.
func TestRPC_Limits_Streaming(t *testing.T) {
	t.Parallel()

	s, cleanup := TestServer(t, func(c *Config) {
		limits := config.DefaultLimits()
		c.RPCMaxConnsPerClient = *limits.RPCMaxConnsPerClient
	})
	defer func() {
		cleanup()

		//TODO Avoid panics from logging during shutdown
		time.Sleep(1 * time.Second)
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)

	// Create a streaming connection
	dialStreamer := func() net.Conn {
		conn, err := net.DialTimeout("tcp", s.config.RPCAddr.String(), 1*time.Second)
		require.NoError(t, err)

		_, err = conn.Write([]byte{byte(pool.RpcStreaming)})
		require.NoError(t, err)
		return conn
	}

	// Create up to the limit streaming connections
	streamers := make([]net.Conn, s.config.RPCMaxConnsPerClient-config.LimitsNonStreamingConnsPerClient)
	for i := range streamers {
		streamers[i] = dialStreamer()

		go func(i int) {
			// Streamer should never die until test exits
			buf := []byte{0}
			_, err := streamers[i].Read(buf)
			if ctx.Err() != nil {
				// Error is expected when test finishes
				return
			}

			t.Logf("connection %d died with error: (%T) %v", i, err, err)

			// Send unexpected errors back
			if err != nil {
				select {
				case errCh <- err:
				case <-ctx.Done():
				default:
					// Only send first error
				}
			}
		}(i)
	}

	defer func() {
		cancel()
		for _, conn := range streamers {
			conn.Close()
		}
	}()

	// Assert no streamer errors have occurred
	select {
	case err := <-errCh:
		t.Fatalf("unexpected error from blocking streaming RPCs: (%T) %v", err, err)
	case <-time.After(500 * time.Millisecond):
		// Ok! No connections were rejected immediately.
	}

	// Assert subsequent streaming RPC are rejected
	conn := dialStreamer()
	t.Logf("expect connection to be rejected due to limit")
	buf := []byte{0}
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, err := conn.Read(buf)
	require.Equalf(t, io.EOF, err, "expected io.EOF but found: (%T) %v", err, err)

	// Assert no streamer errors have occurred
	select {
	case err := <-errCh:
		t.Fatalf("unexpected error from blocking streaming RPCs: %v", err)
	default:
	}

	// Subsequent non-streaming RPC should be OK
	conn, err = net.DialTimeout("tcp", s.config.RPCAddr.String(), 1*time.Second)
	require.NoError(t, err)
	_, err = conn.Write([]byte{byte(pool.RpcNomad)})
	require.NoError(t, err)

	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	_, err = conn.Read(buf)
	testutil.RequireDeadlineErr(t, err)

	// Close 1 streamer and assert another is allowed
	t.Logf("expect streaming connection 0 to exit with error")
	streamers[0].Close()
	<-errCh

	// Assert that new connections are allowed.
	// Due to the distributed nature here, server may not immediately recognize
	// the connection closure, so first attempts may be rejections (i.e. EOF)
	// but the first non-EOF request must be a read-deadline error
	testutil.WaitForResult(func() (bool, error) {
		conn = dialStreamer()
		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		_, err = conn.Read(buf)
		if err == io.EOF {
			return false, fmt.Errorf("connection was rejected")
		}

		testutil.RequireDeadlineErr(t, err)
		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})
}
