// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/rpc"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/go-sockaddr"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/ci"
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
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rpcClient is a test helper method to return a ClientCodec to use to make rpc
// calls to the passed server.
func rpcClient(t *testing.T, s *Server) rpc.ClientCodec {
	t.Helper()
	addr := s.config.RPCAddr
	conn, err := net.DialTimeout("tcp", addr.String(), time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// Write the Nomad RPC byte to set the mode
	conn.Write([]byte{byte(pool.RpcNomad)})
	return pool.NewClientCodec(conn)
}

// rpcClientWithTLS is a test helper method to return a ClientCodec to use to
// make RPC calls to the passed server via mTLS
func rpcClientWithTLS(t *testing.T, srv *Server, cfg *config.TLSConfig) rpc.ClientCodec {
	t.Helper()

	// configure TLS, ignoring client-side validation
	tlsConf, err := tlsutil.NewTLSConfiguration(cfg, true, true)
	must.NoError(t, err)
	outTLSConf, err := tlsConf.OutgoingTLSConfig()
	must.NoError(t, err)
	outTLSConf.InsecureSkipVerify = true

	// make the TCP connection
	conn, err := net.DialTimeout("tcp", srv.config.RPCAddr.String(), time.Second)

	// write the TLS byte to set the mode
	_, err = conn.Write([]byte{byte(pool.RpcTLS)})
	must.NoError(t, err)

	// connect w/ TLS
	tlsConn := tls.Client(conn, outTLSConf)
	must.NoError(t, tlsConn.Handshake())

	// write the Nomad RPC byte to set the mode
	_, err = tlsConn.Write([]byte{byte(pool.RpcNomad)})
	must.NoError(t, err)

	return pool.NewClientCodec(tlsConn)
}

func TestRPC_forwardLeader(t *testing.T) {
	ci.Parallel(t)

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
		err := s1.forwardLeader(remote, "Status.Ping", &structs.GenericRequest{}, &out)
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
		err := s2.forwardLeader(remote, "Status.Ping", &structs.GenericRequest{}, &out)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}
}

func TestRPC_WaitForConsistentReads(t *testing.T) {
	ci.Parallel(t)

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
	ci.Parallel(t)

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
	err := s1.forwardRegion("global", "Status.Ping", &structs.GenericRequest{}, &out)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = s2.forwardRegion("global", "Status.Ping", &structs.GenericRequest{}, &out)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestRPC_getServer(t *testing.T) {
	ci.Parallel(t)

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
	ci.Parallel(t)
	assert := assert.New(t)

	const (
		cafile  = "../helper/tlsutil/testdata/nomad-agent-ca.pem"
		foocert = "../helper/tlsutil/testdata/regionFoo-client-nomad.pem"
		fookey  = "../helper/tlsutil/testdata/regionFoo-client-nomad-key.pem"
	)
	dir := t.TempDir()

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
	ci.Parallel(t)
	assert := assert.New(t)

	const (
		cafile  = "../helper/tlsutil/testdata/nomad-agent-ca.pem"
		foocert = "../helper/tlsutil/testdata/regionFoo-client-nomad.pem"
		fookey  = "../helper/tlsutil/testdata/regionFoo-client-nomad-key.pem"
	)
	dir := t.TempDir()

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
	ci.Parallel(t)
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
	ci.Parallel(t)
	require := require.New(t)

	const (
		cafile  = "../helper/tlsutil/testdata/nomad-agent-ca.pem"
		foocert = "../helper/tlsutil/testdata/regionFoo-server-nomad.pem"
		fookey  = "../helper/tlsutil/testdata/regionFoo-server-nomad-key.pem"
	)
	dir := t.TempDir()
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
	ci.Parallel(t)
	require := require.New(t)
	dir := t.TempDir()
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
	ci.Parallel(t)
	require := require.New(t)

	const (
		cafile  = "../helper/tlsutil/testdata/nomad-agent-ca.pem"
		foocert = "../helper/tlsutil/testdata/regionFoo-server-nomad.pem"
		fookey  = "../helper/tlsutil/testdata/regionFoo-server-nomad-key.pem"
	)
	dir := t.TempDir()
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
	ci.Parallel(t)
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
	ci.Parallel(t)

	const (
		cafile  = "../helper/tlsutil/testdata/nomad-agent-ca.pem"
		foocert = "../helper/tlsutil/testdata/regionFoo-client-nomad.pem"
		fookey  = "../helper/tlsutil/testdata/regionFoo-client-nomad-key.pem"
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
	ci.Parallel(t)

	const (
		cafile   = "../helper/tlsutil/testdata/nomad-agent-ca.pem"
		foocert  = "../helper/tlsutil/testdata/regionFoo-client-nomad.pem"
		fookey   = "../helper/tlsutil/testdata/regionFoo-client-nomad-key.pem"
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

			require.Truef(t, errors.Is(err, os.ErrDeadlineExceeded),
				"error does not wrap os.ErrDeadlineExceeded: (%T) %v", err, err)

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
		require.Truef(t, errors.Is(err, os.ErrDeadlineExceeded),
			"error does not wrap os.ErrDeadlineExceeded: (%T) %v", err, err)
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
				require.Truef(t, errors.Is(err, os.ErrDeadlineExceeded),
					"error does not wrap os.ErrDeadlineExceeded: (%T) %v", err, err)
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
			ci.Parallel(t)

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

				// Bind the server to a private IP so that Autopilot's
				// StatsFetcher requests come from a different IP than the test
				// requests, otherwise they would interfere with the connection
				// rate limiter since limits are imposed by IP address.
				ip, err := sockaddr.GetPrivateIP()
				require.NoError(t, err)
				c.RPCAddr.IP = []byte(ip)
				c.SerfConfig.MemberlistConfig.BindAddr = ip
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
	ci.Parallel(t)

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
	require.Truef(t, errors.Is(err, os.ErrDeadlineExceeded),
		"error does not wrap os.ErrDeadlineExceeded: (%T) %v", err, err)

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

		require.True(t, errors.Is(err, os.ErrDeadlineExceeded))
		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})
}

func TestRPC_TLS_Enforcement_Raft(t *testing.T) {
	ci.Parallel(t)

	defer func() {
		//TODO Avoid panics from logging during shutdown
		time.Sleep(1 * time.Second)
	}()

	tlsHelper := newTLSTestHelper(t)
	defer tlsHelper.cleanup()

	// When VerifyServerHostname is enabled:
	// Only local servers can connect to the Raft layer
	cases := []struct {
		name    string
		cn      string
		canRaft bool
	}{
		{
			name:    "local server",
			cn:      "server.global.nomad",
			canRaft: true,
		},
		{
			name:    "local client",
			cn:      "client.global.nomad",
			canRaft: false,
		},
		{
			name:    "other region server",
			cn:      "server.other.nomad",
			canRaft: false,
		},
		{
			name:    "other region client",
			cn:      "client.other.nomad",
			canRaft: false,
		},
		{
			name:    "irrelevant cert",
			cn:      "nomad.example.com",
			canRaft: false,
		},
		{
			name:    "globs",
			cn:      "*.global.nomad",
			canRaft: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			certPath := tlsHelper.newCert(t, tc.cn)

			cfg := &config.TLSConfig{
				EnableRPC:            true,
				VerifyServerHostname: true,
				CAFile:               filepath.Join(tlsHelper.dir, "ca.pem"),
				CertFile:             certPath + ".pem",
				KeyFile:              certPath + ".key",
			}

			t.Run("Raft RPC: verify_hostname=true", func(t *testing.T) {
				err := tlsHelper.raftRPC(t, tlsHelper.mtlsServer1, cfg)

				// the expected error depends on location of failure.
				// We expect "bad certificate" if connection fails during handshake,
				// or EOF when connection is closed after RaftRPC byte.
				if tc.canRaft {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
					require.Regexp(t, "(bad certificate|EOF)", err.Error())
				}
			})
			t.Run("Raft RPC: verify_hostname=false", func(t *testing.T) {
				err := tlsHelper.raftRPC(t, tlsHelper.nonVerifyServer, cfg)
				require.NoError(t, err)
			})
		})
	}
}

func TestRPC_TLS_Enforcement_RPC(t *testing.T) {
	ci.Parallel(t)

	tlsHelper := newTLSTestHelper(t)
	t.Cleanup(tlsHelper.cleanup)

	standardRPCs := map[string]any{
		"Status.Ping": &structs.GenericRequest{},
	}

	localServersOnlyRPCs := map[string]any{
		"Eval.Update": &structs.EvalUpdateRequest{
			WriteRequest: structs.WriteRequest{Region: "global"},
		},
		"Eval.Ack": &structs.EvalAckRequest{
			WriteRequest: structs.WriteRequest{Region: "global"},
		},
		"Eval.Nack": &structs.EvalAckRequest{
			WriteRequest: structs.WriteRequest{Region: "global"},
		},
		"Eval.Dequeue": &structs.EvalDequeueRequest{
			WriteRequest: structs.WriteRequest{Region: "global"},
		},
		"Eval.Create": &structs.EvalUpdateRequest{
			WriteRequest: structs.WriteRequest{Region: "global"},
		},
		"Eval.Reblock": &structs.EvalUpdateRequest{
			WriteRequest: structs.WriteRequest{Region: "global"},
		},
		"Eval.Reap": &structs.EvalReapRequest{
			WriteRequest: structs.WriteRequest{Region: "global"},
		},
		"Plan.Submit": &structs.PlanRequest{
			WriteRequest: structs.WriteRequest{Region: "global"},
		},
		"Deployment.Reap": &structs.DeploymentDeleteRequest{
			WriteRequest: structs.WriteRequest{Region: "global"},
		},
	}

	localClientsOnlyRPCs := map[string]any{
		"Alloc.GetAllocs": &structs.AllocsGetRequest{
			QueryOptions: structs.QueryOptions{Region: "global"},
		},
		"Node.EmitEvents": &structs.EmitNodeEventsRequest{
			WriteRequest: structs.WriteRequest{Region: "global"},
		},
		"Node.UpdateAlloc": &structs.AllocUpdateRequest{
			WriteRequest: structs.WriteRequest{Region: "global"},
		},
		"ServiceRegistration.Upsert": &structs.ServiceRegistrationUpsertRequest{
			WriteRequest: structs.WriteRequest{Region: "global"},
		},
	}

	// When VerifyServerHostname is enabled:
	// All servers can make RPC requests
	// Only local clients can make RPC requests
	// Some endpoints can only be called server -> server
	// Some endpoints can only be called client -> server
	cases := []struct {
		name   string
		cn     string
		rpcs   map[string]any
		canRPC bool
	}{
		// Local server.
		{
			name:   "local server/standard rpc",
			cn:     "server.global.nomad",
			rpcs:   standardRPCs,
			canRPC: true,
		},
		{
			name:   "local server/servers only rpc",
			cn:     "server.global.nomad",
			rpcs:   localServersOnlyRPCs,
			canRPC: true,
		},
		{
			name:   "local server/clients only rpc",
			cn:     "server.global.nomad",
			rpcs:   localClientsOnlyRPCs,
			canRPC: true,
		},
		// Local client.
		{
			name:   "local client/standard rpc",
			cn:     "client.global.nomad",
			rpcs:   standardRPCs,
			canRPC: true,
		},
		{
			name:   "local client/servers only rpc",
			cn:     "client.global.nomad",
			rpcs:   localServersOnlyRPCs,
			canRPC: false,
		},
		{
			name:   "local client/clients only rpc",
			cn:     "client.global.nomad",
			rpcs:   localClientsOnlyRPCs,
			canRPC: true,
		},
		// Other region server.
		{
			name:   "other region server/standard rpc",
			cn:     "server.other.nomad",
			rpcs:   standardRPCs,
			canRPC: true,
		},
		{
			name:   "other region server/servers only rpc",
			cn:     "server.other.nomad",
			rpcs:   localServersOnlyRPCs,
			canRPC: false,
		},
		{
			name:   "other region server/clients only rpc",
			cn:     "server.other.nomad",
			rpcs:   localClientsOnlyRPCs,
			canRPC: false,
		},
		// Other region client.
		{
			name:   "other region client/standard rpc",
			cn:     "client.other.nomad",
			rpcs:   standardRPCs,
			canRPC: false,
		},
		{
			name:   "other region client/servers only rpc",
			cn:     "client.other.nomad",
			rpcs:   localServersOnlyRPCs,
			canRPC: false,
		},
		{
			name:   "other region client/clients only rpc",
			cn:     "client.other.nomad",
			rpcs:   localClientsOnlyRPCs,
			canRPC: false,
		},
		// Wrong certs.
		{
			name:   "irrelevant cert",
			cn:     "nomad.example.com",
			rpcs:   standardRPCs,
			canRPC: false,
		},
		{
			name:   "globs",
			cn:     "*.global.nomad",
			rpcs:   standardRPCs,
			canRPC: false,
		},
		{},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			certPath := tlsHelper.newCert(t, tc.cn)

			cfg := &config.TLSConfig{
				EnableRPC:            true,
				VerifyServerHostname: true,
				CAFile:               filepath.Join(tlsHelper.dir, "ca.pem"),
				CertFile:             certPath + ".pem",
				KeyFile:              certPath + ".key",
			}

			for method, arg := range tc.rpcs {
				for _, srv := range []*Server{tlsHelper.mtlsServer1, tlsHelper.mtlsServer2} {
					name := fmt.Sprintf("nomad RPC: rpc=%s verify_hostname=true leader=%v", method, srv.IsLeader())
					t.Run(name, func(t *testing.T) {
						err := tlsHelper.nomadRPC(t, srv, cfg, method, arg)

						if tc.canRPC {
							if err != nil {
								// note: lots of these RPCs will return
								// validation errors after connection b/c we're
								// focusing on testing TLS here
								must.StrNotContains(t, err.Error(), "certificate")
							}
						} else {
							// We expect "bad certificate" for these failures,
							// but locally the error can return before the error
							// message bytes have been received, in which case
							// we immediately write on the pipe that was just
							// closed by the client
							must.Error(t, err)
							must.RegexMatch(t,
								regexp.MustCompile("(certificate|broken pipe)"), err.Error())
						}
					})
				}

				t.Run(fmt.Sprintf("nomad RPC: rpc=%s verify_hostname=false", method), func(t *testing.T) {
					err := tlsHelper.nomadRPC(t, tlsHelper.nonVerifyServer, cfg, method, arg)
					if err != nil {
						must.StrNotContains(t, "certificate", err.Error())
					}
				})
			}
		})
	}
}

type tlsTestHelper struct {
	dir    string
	nodeID int

	mtlsServer1            *Server
	mtlsServerCleanup1     func()
	mtlsServer2            *Server
	mtlsServerCleanup2     func()
	nonVerifyServer        *Server
	nonVerifyServerCleanup func()

	caPEM      string
	pk         string
	serverCert string
}

func newTLSTestHelper(t *testing.T) tlsTestHelper {
	var err error

	h := tlsTestHelper{
		dir:    t.TempDir(),
		nodeID: 1,
	}

	// Generate CA certificate and write it to disk.
	h.caPEM, h.pk, err = tlsutil.GenerateCA(tlsutil.CAOpts{
		Name:               "Nomad CA",
		Country:            "ZZ",
		Days:               5,
		Organization:       "CustOrgUnit",
		OrganizationalUnit: "CustOrgUnit",
	})
	must.NoError(t, err)

	err = os.WriteFile(filepath.Join(h.dir, "ca.pem"), []byte(h.caPEM), 0600)
	must.NoError(t, err)

	// Generate servers and their certificate.
	h.serverCert = h.newCert(t, "server.global.nomad")

	makeServer := func(bootstrapExpect int, verifyServerHostname bool) (*Server, func()) {
		return TestServer(t, func(c *Config) {
			c.NumSchedulers = 0
			c.BootstrapExpect = bootstrapExpect
			c.TLSConfig = &config.TLSConfig{
				EnableRPC:            true,
				VerifyServerHostname: verifyServerHostname,
				CAFile:               filepath.Join(h.dir, "ca.pem"),
				CertFile:             h.serverCert + ".pem",
				KeyFile:              h.serverCert + ".key",
			}
		})
	}

	h.mtlsServer1, h.mtlsServerCleanup1 = makeServer(3, true)
	h.mtlsServer2, h.mtlsServerCleanup2 = makeServer(3, true)
	h.nonVerifyServer, h.nonVerifyServerCleanup = makeServer(3, false)

	TestJoin(t, h.mtlsServer1, h.mtlsServer2, h.nonVerifyServer)
	testutil.WaitForLeaders(t, h.mtlsServer1.RPC, h.mtlsServer2.RPC, h.nonVerifyServer.RPC)
	return h
}

func (h tlsTestHelper) cleanup() {
	h.mtlsServerCleanup1()
	h.mtlsServerCleanup2()
	h.nonVerifyServerCleanup()
}

func (h tlsTestHelper) newCert(t *testing.T, name string) string {
	t.Helper()

	node := fmt.Sprintf("node%d", h.nodeID)
	h.nodeID++
	signer, err := tlsutil.ParseSigner(h.pk)
	require.NoError(t, err)

	pem, key, err := tlsutil.GenerateCert(tlsutil.CertOpts{
		Signer:      signer,
		CA:          h.caPEM,
		Name:        name,
		Days:        5,
		DNSNames:    []string{node + "." + name, name, "localhost"},
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	})
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(h.dir, node+"-"+name+".pem"), []byte(pem), 0600)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(h.dir, node+"-"+name+".key"), []byte(key), 0600)
	require.NoError(t, err)

	return filepath.Join(h.dir, node+"-"+name)
}

func (h tlsTestHelper) connect(t *testing.T, s *Server, c *config.TLSConfig) net.Conn {
	t.Helper()
	conn, err := net.DialTimeout("tcp", s.config.RPCAddr.String(), time.Second)
	must.NoError(t, err)

	// configure TLS
	_, err = conn.Write([]byte{byte(pool.RpcTLS)})
	must.NoError(t, err)

	// Client TLS verification isn't necessary for
	// our assertions
	tlsConf, err := tlsutil.NewTLSConfiguration(c, true, true)
	must.NoError(t, err)
	outTLSConf, err := tlsConf.OutgoingTLSConfig()
	must.NoError(t, err)
	outTLSConf.InsecureSkipVerify = true

	tlsConn := tls.Client(conn, outTLSConf)
	must.NoError(t, tlsConn.Handshake())

	return tlsConn
}

func (h tlsTestHelper) nomadRPC(t *testing.T, s *Server, c *config.TLSConfig, method string, arg interface{}) error {
	t.Helper()
	conn := h.connect(t, s, c)
	defer conn.Close()
	_, err := conn.Write([]byte{byte(pool.RpcNomad)})
	must.NoError(t, err)

	codec := pool.NewClientCodec(conn)

	var out struct{}
	return msgpackrpc.CallWithCodec(codec, method, arg, &out)
}

func (h tlsTestHelper) raftRPC(t *testing.T, s *Server, c *config.TLSConfig) error {
	conn := h.connect(t, s, c)
	defer conn.Close()

	_, err := conn.Write([]byte{byte(pool.RpcRaft)})
	require.NoError(t, err)

	_, err = doRaftRPC(conn, s.config.NodeName)
	return err
}

func doRaftRPC(conn net.Conn, leader string) (*raft.AppendEntriesResponse, error) {
	req := raft.AppendEntriesRequest{
		RPCHeader:         raft.RPCHeader{ProtocolVersion: 3},
		Term:              0,
		Leader:            []byte(leader),
		PrevLogEntry:      0,
		PrevLogTerm:       0xc,
		LeaderCommitIndex: 50,
	}

	enc := codec.NewEncoder(conn, &codec.MsgpackHandle{})
	dec := codec.NewDecoder(conn, &codec.MsgpackHandle{})

	const rpcAppendEntries = 0
	if _, err := conn.Write([]byte{rpcAppendEntries}); err != nil {
		return nil, fmt.Errorf("failed to write raft-RPC byte: %w", err)
	}

	if err := enc.Encode(req); err != nil {
		return nil, fmt.Errorf("failed to send append entries RPC: %w", err)
	}

	var rpcError string
	var resp raft.AppendEntriesResponse
	if err := dec.Decode(&rpcError); err != nil {
		return nil, fmt.Errorf("failed to decode response error: %w", err)
	}
	if rpcError != "" {
		return nil, fmt.Errorf("rpc error: %v", rpcError)
	}
	if err := dec.Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &resp, nil
}
