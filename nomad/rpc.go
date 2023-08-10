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
	golog "log"
	"math/rand"
	"net"
	"net/rpc"
	"strings"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/go-connlimit"
	log "github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/pool"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/yamux"
)

const (
	// Warn if the Raft command is larger than this.
	// If it's over 1MB something is probably being abusive.
	raftWarnSize = 1024 * 1024

	// enqueueLimit caps how long we will wait to enqueue
	// a new Raft command. Something is probably wrong if this
	// value is ever reached. However, it prevents us from blocking
	// the requesting goroutine forever.
	enqueueLimit = 30 * time.Second
)

type rpcHandler struct {
	*Server

	// connLimiter is used to limit the number of RPC connections per
	// remote address. It is distinct from the HTTP connection limit.
	//
	// nil if limiting is disabled
	connLimiter *connlimit.Limiter
	connLimit   int

	// streamLimiter is used to limit the number of *streaming* RPC
	// connections per remote address. It is lower than the overall
	// connection limit to ensure their are free connections for Raft and
	// other RPCs.
	streamLimiter *connlimit.Limiter
	streamLimit   int

	logger   log.Logger
	gologger *golog.Logger
}

func newRpcHandler(s *Server) *rpcHandler {
	logger := s.logger.NamedIntercept("rpc")

	r := rpcHandler{
		Server:    s,
		connLimit: s.config.RPCMaxConnsPerClient,
		logger:    logger,
		gologger:  logger.StandardLoggerIntercept(&log.StandardLoggerOptions{InferLevels: true}),
	}

	// Setup connection limits
	if r.connLimit > 0 {
		r.connLimiter = connlimit.NewLimiter(connlimit.Config{
			MaxConnsPerClientIP: r.connLimit,
		})

		r.streamLimit = r.connLimit - config.LimitsNonStreamingConnsPerClient
		r.streamLimiter = connlimit.NewLimiter(connlimit.Config{
			MaxConnsPerClientIP: r.streamLimit,
		})
	}

	return &r
}

// RPCContext provides metadata about the RPC connection.
type RPCContext struct {
	// Conn exposes the raw connection.
	Conn net.Conn

	// Session exposes the multiplexed connection session.
	Session *yamux.Session

	// TLS marks whether the RPC is over a TLS based connection
	TLS bool

	// VerifiedChains is is the Verified certificates presented by the incoming
	// connection.
	VerifiedChains [][]*x509.Certificate

	// NodeID marks the NodeID that initiated the connection.
	NodeID string
}

// Certificate returns the first certificate available in the chain.
func (ctx *RPCContext) Certificate() *x509.Certificate {
	if ctx == nil || len(ctx.VerifiedChains) == 0 || len(ctx.VerifiedChains[0]) == 0 {
		return nil
	}

	return ctx.VerifiedChains[0][0]
}

// ValidateCertificateForName returns true if the RPC context certificate is valid
// for the given domain name.
func (ctx *RPCContext) ValidateCertificateForName(name string) error {
	if ctx == nil || !ctx.TLS {
		return nil
	}

	cert := ctx.Certificate()
	if cert == nil {
		return errors.New("missing certificate information")
	}

	validNames := []string{cert.Subject.CommonName}
	validNames = append(validNames, cert.DNSNames...)
	for _, valid := range validNames {
		if name == valid {
			return nil
		}
	}

	return fmt.Errorf("invalid certificate, %s not in %s", name, strings.Join(validNames, ","))
}

// listen is used to listen for incoming RPC connections
func (r *rpcHandler) listen(ctx context.Context) {
	defer close(r.listenerCh)

	var acceptLoopDelay time.Duration
	for {
		select {
		case <-ctx.Done():
			r.logger.Info("closing server RPC connection")
			return
		default:
		}

		// Accept a connection
		conn, err := r.rpcListener.Accept()
		if err != nil {
			if r.shutdown {
				return
			}
			r.handleAcceptErr(ctx, err, &acceptLoopDelay)
			continue
		}
		// No error, reset loop delay
		acceptLoopDelay = 0

		// Apply per-connection limits (if enabled) *prior* to launching
		// goroutine to block further Accept()s until limits are checked.
		if r.connLimiter != nil {
			free, err := r.connLimiter.Accept(conn)
			if err != nil {
				r.logger.Error("rejecting client for exceeding maximum RPC connections",
					"remote_addr", conn.RemoteAddr(), "limit", r.connLimit)
				conn.Close()
				continue
			}

			// Wrap the connection so that conn.Close calls free() as well.
			// This is required for libraries like raft which handoff the
			// net.Conn to another goroutine and therefore can't be tracked
			// within this func.
			conn = connlimit.Wrap(conn, free)
		}

		go r.handleConn(ctx, conn, &RPCContext{Conn: conn})
		metrics.IncrCounter([]string{"nomad", "rpc", "accept_conn"}, 1)
	}
}

// handleAcceptErr sleeps to avoid spamming the log,
// with a maximum delay according to whether or not the error is temporary
func (r *rpcHandler) handleAcceptErr(ctx context.Context, err error, loopDelay *time.Duration) {
	const baseDelay = 5 * time.Millisecond
	const maxDelayPerm = 5 * time.Second
	const maxDelayTemp = 1 * time.Second

	if *loopDelay == 0 {
		*loopDelay = baseDelay
	} else {
		*loopDelay *= 2
	}

	temporaryError := false
	if ne, ok := err.(net.Error); ok && ne.Temporary() {
		temporaryError = true
	}

	if temporaryError && *loopDelay > maxDelayTemp {
		*loopDelay = maxDelayTemp
	} else if *loopDelay > maxDelayPerm {
		*loopDelay = maxDelayPerm
	}

	r.logger.Error("failed to accept RPC conn", "error", err, "delay", *loopDelay)

	select {
	case <-ctx.Done():
	case <-time.After(*loopDelay):
	}
}

// handleConn is used to determine if this is a Raft or
// Nomad type RPC connection and invoke the correct handler
//
// **Cannot** use defer conn.Close in this method because the Raft handler uses
// the conn beyond the scope of this func.
func (r *rpcHandler) handleConn(ctx context.Context, conn net.Conn, rpcCtx *RPCContext) {
	// Limit how long an unauthenticated client can hold the connection
	// open before they send the magic byte.
	if !rpcCtx.TLS && r.config.RPCHandshakeTimeout > 0 {
		conn.SetDeadline(time.Now().Add(r.config.RPCHandshakeTimeout))
	}

	// Read a single byte
	buf := make([]byte, 1)
	if _, err := conn.Read(buf); err != nil {
		if err != io.EOF {
			r.logger.Error("failed to read first RPC byte", "error", err)
		}
		conn.Close()
		return
	}

	// Reset the deadline as we aren't sure what is expected next - it depends on
	// the protocol.
	if !rpcCtx.TLS && r.config.RPCHandshakeTimeout > 0 {
		conn.SetDeadline(time.Time{})
	}

	// Enforce TLS if EnableRPC is set
	if r.config.TLSConfig.EnableRPC && !rpcCtx.TLS && pool.RPCType(buf[0]) != pool.RpcTLS {
		if !r.config.TLSConfig.RPCUpgradeMode {
			r.logger.Warn("non-TLS connection attempted with RequireTLS set", "remote_addr", conn.RemoteAddr())
			conn.Close()
			return
		}
	}

	// Switch on the byte
	switch pool.RPCType(buf[0]) {
	case pool.RpcNomad:
		// Create an RPC Server and handle the request
		server := rpc.NewServer()
		r.setupRpcServer(server, rpcCtx)
		r.handleNomadConn(ctx, conn, server)

		// Remove any potential mapping between a NodeID to this connection and
		// close the underlying connection.
		r.removeNodeConn(rpcCtx)

	case pool.RpcRaft:
		metrics.IncrCounter([]string{"nomad", "rpc", "raft_handoff"}, 1)
		// Ensure that when TLS is configured, only certificates from `server.<region>.nomad` are accepted for Raft connections.
		if err := r.validateRaftTLS(rpcCtx); err != nil {
			conn.Close()
			return
		}
		r.raftLayer.Handoff(ctx, conn)

	case pool.RpcMultiplex:
		r.handleMultiplex(ctx, conn, rpcCtx)

	case pool.RpcTLS:
		if r.rpcTLS == nil {
			r.logger.Warn("TLS connection attempted, server not configured for TLS")
			conn.Close()
			return
		}

		// Don't allow malicious client to create TLS-in-TLS forever.
		if rpcCtx.TLS {
			r.logger.Error("TLS connection attempting to establish inner TLS connection", "remote_addr", conn.RemoteAddr())
			conn.Close()
			return
		}

		conn = tls.Server(conn, r.rpcTLS)

		// Force a handshake so we can get information about the TLS connection
		// state.
		tlsConn, ok := conn.(*tls.Conn)
		if !ok {
			r.logger.Error("expected TLS connection", "got", log.Fmt("%T", conn))
			conn.Close()
			return
		}

		// Enforce handshake timeout during TLS handshake to prevent
		// unauthenticated users from holding connections open
		// indefinitely.
		if r.config.RPCHandshakeTimeout > 0 {
			tlsConn.SetDeadline(time.Now().Add(r.config.RPCHandshakeTimeout))
		}

		if err := tlsConn.Handshake(); err != nil {
			r.logger.Warn("failed TLS handshake", "remote_addr", tlsConn.RemoteAddr(), "error", err)
			conn.Close()
			return
		}

		// Reset the deadline as unauthenticated users have now been rejected.
		if r.config.RPCHandshakeTimeout > 0 {
			tlsConn.SetDeadline(time.Time{})
		}

		// Update the connection context with the fact that the connection is
		// using TLS
		rpcCtx.TLS = true

		// Store the verified chains so they can be inspected later.
		state := tlsConn.ConnectionState()
		rpcCtx.VerifiedChains = state.VerifiedChains

		r.handleConn(ctx, conn, rpcCtx)

	case pool.RpcStreaming:
		// Apply a lower limit to streaming RPCs to avoid denial of
		// service by repeatedly starting streaming RPCs.
		//
		// TODO Remove once MultiplexV2 is used.
		if r.streamLimiter != nil {
			free, err := r.streamLimiter.Accept(conn)
			if err != nil {
				r.logger.Error("rejecting client for exceeding maximum streaming RPC connections",
					"remote_addr", conn.RemoteAddr(), "stream_limit", r.streamLimit)
				conn.Close()
				return
			}
			defer free()
		}
		r.handleStreamingConn(conn)

	case pool.RpcMultiplexV2:
		r.handleMultiplexV2(ctx, conn, rpcCtx)

	default:
		r.logger.Error("unrecognized RPC byte", "byte", buf[0])
		conn.Close()
		return
	}
}

// handleMultiplex is used to multiplex a single incoming connection
// using the Yamux multiplexer
func (r *rpcHandler) handleMultiplex(ctx context.Context, conn net.Conn, rpcCtx *RPCContext) {
	defer func() {
		// Remove any potential mapping between a NodeID to this connection and
		// close the underlying connection.
		r.removeNodeConn(rpcCtx)
		conn.Close()
	}()

	conf := yamux.DefaultConfig()
	conf.LogOutput = nil
	conf.Logger = r.gologger
	server, err := yamux.Server(conn, conf)
	if err != nil {
		r.logger.Error("multiplex failed to create yamux server", "error", err)
		return
	}

	// Update the context to store the yamux session
	rpcCtx.Session = server

	// Create the RPC server for this connection
	rpcServer := rpc.NewServer()
	r.setupRpcServer(rpcServer, rpcCtx)

	for {
		// stop handling connections if context was cancelled
		if ctx.Err() != nil {
			return
		}

		sub, err := server.Accept()
		if err != nil {
			if err != io.EOF {
				r.logger.Error("multiplex conn accept failed", "error", err)
			}
			return
		}
		go r.handleNomadConn(ctx, sub, rpcServer)
	}
}

// handleNomadConn is used to service a single Nomad RPC connection
func (r *rpcHandler) handleNomadConn(ctx context.Context, conn net.Conn, server *rpc.Server) {
	defer conn.Close()
	rpcCodec := pool.NewServerCodec(conn)
	for {
		select {
		case <-ctx.Done():
			r.logger.Info("closing server RPC connection")
			return
		case <-r.shutdownCh:
			return
		default:
		}

		if err := server.ServeRequest(rpcCodec); err != nil {
			if err != io.EOF && !strings.Contains(err.Error(), "closed") {
				r.logger.Error("RPC error", "error", err, "connection", conn)
				metrics.IncrCounter([]string{"nomad", "rpc", "request_error"}, 1)
			}
			return
		}
		metrics.IncrCounter([]string{"nomad", "rpc", "request"}, 1)
	}
}

// handleStreamingConn is used to handle a single Streaming Nomad RPC connection.
func (r *rpcHandler) handleStreamingConn(conn net.Conn) {
	defer conn.Close()

	// Decode the header
	var header structs.StreamingRpcHeader
	decoder := codec.NewDecoder(conn, structs.MsgpackHandle)
	if err := decoder.Decode(&header); err != nil {
		if err != io.EOF && !strings.Contains(err.Error(), "closed") {
			r.logger.Error("streaming RPC error", "error", err, "connection", conn)
			metrics.IncrCounter([]string{"nomad", "streaming_rpc", "request_error"}, 1)
		}

		return
	}

	ack := structs.StreamingRpcAck{}
	handler, err := r.streamingRpcs.GetHandler(header.Method)
	if err != nil {
		r.logger.Error("streaming RPC error", "error", err, "connection", conn)
		metrics.IncrCounter([]string{"nomad", "streaming_rpc", "request_error"}, 1)
		ack.Error = err.Error()
	}

	// Send the acknowledgement
	encoder := codec.NewEncoder(conn, structs.MsgpackHandle)
	if err := encoder.Encode(ack); err != nil {
		conn.Close()
		return
	}

	if ack.Error != "" {
		return
	}

	// Invoke the handler
	metrics.IncrCounter([]string{"nomad", "streaming_rpc", "request"}, 1)
	handler(conn)
}

// handleMultiplexV2 is used to multiplex a single incoming connection
// using the Yamux multiplexer. Version 2 handling allows a single connection to
// switch streams between regulars RPCs and Streaming RPCs.
func (r *rpcHandler) handleMultiplexV2(ctx context.Context, conn net.Conn, rpcCtx *RPCContext) {
	defer func() {
		// Remove any potential mapping between a NodeID to this connection and
		// close the underlying connection.
		r.removeNodeConn(rpcCtx)
		conn.Close()
	}()

	conf := yamux.DefaultConfig()
	conf.LogOutput = nil
	conf.Logger = r.gologger
	server, err := yamux.Server(conn, conf)
	if err != nil {
		r.logger.Error("multiplex_v2 failed to create yamux server", "error", err)
		return
	}

	// Update the context to store the yamux session
	rpcCtx.Session = server

	// Create the RPC server for this connection
	rpcServer := rpc.NewServer()
	r.setupRpcServer(rpcServer, rpcCtx)

	for {
		// stop handling connections if context was cancelled
		if ctx.Err() != nil {
			return
		}

		// Accept a new stream
		sub, err := server.Accept()
		if err != nil {
			if err != io.EOF {
				r.logger.Error("multiplex_v2 conn accept failed", "error", err)
			}
			return
		}

		// Read a single byte
		buf := make([]byte, 1)
		if _, err := sub.Read(buf); err != nil {
			if err != io.EOF {
				r.logger.Error("multiplex_v2 failed to read first byte", "error", err)
			}
			return
		}

		// Determine which handler to use
		switch pool.RPCType(buf[0]) {
		case pool.RpcNomad:
			go r.handleNomadConn(ctx, sub, rpcServer)
		case pool.RpcStreaming:
			go r.handleStreamingConn(sub)

		default:
			r.logger.Error("multiplex_v2 unrecognized first RPC byte", "byte", buf[0])
			return
		}
	}

}

// forward is used to forward to a remote region or to forward to the local leader
// Returns a bool of if forwarding was performed, as well as any error
func (r *rpcHandler) forward(method string, info structs.RPCInfo, args interface{}, reply interface{}) (bool, error) {
	region := info.RequestRegion()
	if region == "" {
		return true, fmt.Errorf("missing region for target RPC")
	}

	// Handle region forwarding
	if region != r.config.Region {
		// Mark that we are forwarding the RPC
		info.SetForwarded()
		err := r.forwardRegion(region, method, args, reply)
		return true, err
	}

	// Check if we can allow a stale read
	if info.IsRead() && info.AllowStaleRead() {
		return false, nil
	}

	remoteServer, err := r.getLeaderForRPC()
	if err != nil {
		return true, err
	}

	// we are the leader
	if remoteServer == nil {
		return false, nil
	}

	// forward to leader
	info.SetForwarded()
	err = r.forwardLeader(remoteServer, method, args, reply)
	return true, err
}

// getLeaderForRPC returns the server info of the currently known leader, or
// nil if this server is the current leader.  If the local server is the leader
// it blocks until it is ready to handle consistent RPC invocations.  If leader
// is not known or consistency isn't guaranteed, an error is returned.
func (r *rpcHandler) getLeaderForRPC() (*serverParts, error) {
	var firstCheck time.Time

CHECK_LEADER:
	// Find the leader
	isLeader, remoteServer := r.getLeader()

	// Handle the case we are the leader
	if isLeader && r.Server.isReadyForConsistentReads() {
		return nil, nil
	}

	// Handle the case of a known leader
	if remoteServer != nil {
		return remoteServer, nil
	}

	// Gate the request until there is a leader
	if firstCheck.IsZero() {
		firstCheck = time.Now()
	}
	if time.Since(firstCheck) < r.config.RPCHoldTimeout {
		jitter := helper.RandomStagger(r.config.RPCHoldTimeout / structs.JitterFraction)
		select {
		case <-time.After(jitter):
			goto CHECK_LEADER
		case <-r.shutdownCh:
		}
	}

	// hold time exceeeded without being ready to respond
	if isLeader {
		return nil, structs.ErrNotReadyForConsistentReads
	}

	return nil, structs.ErrNoLeader

}

// getLeader returns if the current node is the leader, and if not
// then it returns the leader which is potentially nil if the cluster
// has not yet elected a leader.
func (s *Server) getLeader() (bool, *serverParts) {
	// Check if we are the leader
	if s.IsLeader() {
		return true, nil
	}

	// Get the leader
	leader := s.raft.Leader()
	if leader == "" {
		return false, nil
	}

	// Lookup the server
	s.peerLock.RLock()
	server := s.localPeers[leader]
	s.peerLock.RUnlock()

	// Server could be nil
	return false, server
}

// forwardLeader is used to forward an RPC call to the leader, or fail if no leader
func (r *rpcHandler) forwardLeader(server *serverParts, method string, args interface{}, reply interface{}) error {
	// Handle a missing server
	if server == nil {
		return structs.ErrNoLeader
	}
	return r.connPool.RPC(r.config.Region, server.Addr, method, args, reply)
}

// forwardServer is used to forward an RPC call to a particular server
func (r *rpcHandler) forwardServer(server *serverParts, method string, args interface{}, reply interface{}) error {
	// Handle a missing server
	if server == nil {
		return errors.New("must be given a valid server address")
	}
	return r.connPool.RPC(r.config.Region, server.Addr, method, args, reply)
}

func (r *rpcHandler) findRegionServer(region string) (*serverParts, error) {
	r.peerLock.RLock()
	defer r.peerLock.RUnlock()

	servers := r.peers[region]
	if len(servers) == 0 {
		r.logger.Warn("no path found to region", "region", region)
		return nil, structs.ErrNoRegionPath
	}

	// Select a random addr
	offset := rand.Intn(len(servers))
	return servers[offset], nil
}

// forwardRegion is used to forward an RPC call to a remote region, or fail if no servers
func (r *rpcHandler) forwardRegion(region, method string, args interface{}, reply interface{}) error {
	server, err := r.findRegionServer(region)
	if err != nil {
		return err
	}

	// Forward to remote Nomad
	metrics.IncrCounter([]string{"nomad", "rpc", "cross-region", region}, 1)
	return r.connPool.RPC(region, server.Addr, method, args, reply)
}

func (r *rpcHandler) getServer(region, serverID string) (*serverParts, error) {
	// Bail if we can't find any servers
	r.peerLock.RLock()
	defer r.peerLock.RUnlock()

	servers := r.peers[region]
	if len(servers) == 0 {
		r.logger.Warn("no path found to region", "region", region)
		return nil, structs.ErrNoRegionPath
	}

	// Lookup server by id or name
	for _, server := range servers {
		if server.Name == serverID || server.ID == serverID {
			return server, nil
		}
	}

	return nil, fmt.Errorf("unknown Nomad server %s", serverID)
}

// streamingRpc creates a connection to the given server and conducts the
// initial handshake, returning the connection or an error. It is the callers
// responsibility to close the connection if there is no returned error.
func (r *rpcHandler) streamingRpc(server *serverParts, method string) (net.Conn, error) {
	c, err := r.connPool.StreamingRPC(r.config.Region, server.Addr)
	if err != nil {
		return nil, err
	}

	return r.streamingRpcImpl(c, method)
}

// streamingRpcImpl takes a pre-established connection to a server and conducts
// the handshake to establish a streaming RPC for the given method. If an error
// is returned, the underlying connection has been closed. Otherwise it is
// assumed that the connection has been hijacked by the RPC method.
func (r *rpcHandler) streamingRpcImpl(conn net.Conn, method string) (net.Conn, error) {

	// Send the header
	encoder := codec.NewEncoder(conn, structs.MsgpackHandle)
	decoder := codec.NewDecoder(conn, structs.MsgpackHandle)
	header := structs.StreamingRpcHeader{
		Method: method,
	}
	if err := encoder.Encode(header); err != nil {
		conn.Close()
		return nil, err
	}

	// Wait for the acknowledgement
	var ack structs.StreamingRpcAck
	if err := decoder.Decode(&ack); err != nil {
		conn.Close()
		return nil, err
	}

	if ack.Error != "" {
		conn.Close()
		return nil, errors.New(ack.Error)
	}

	return conn, nil
}

// raftApplyFuture is used to encode a message, run it through raft, and return the Raft future.
func (s *Server) raftApplyFuture(t structs.MessageType, msg interface{}) (raft.ApplyFuture, error) {
	buf, err := structs.Encode(t, msg)
	if err != nil {
		return nil, fmt.Errorf("Failed to encode request: %v", err)
	}

	// Warn if the command is very large
	if n := len(buf); n > raftWarnSize {
		s.logger.Warn("attempting to apply large raft entry", "raft_type", t, "bytes", n)
	}

	future := s.raft.Apply(buf, enqueueLimit)
	return future, nil
}

// raftApplyFn is the function signature for applying a msg to Raft
type raftApplyFn func(t structs.MessageType, msg interface{}) (interface{}, uint64, error)

// raftApply is used to encode a message, run it through raft, and return the
// FSM response along with any errors. If the FSM.Apply response is an error it
// will be returned as the error return value with a nil response.
func (s *Server) raftApply(t structs.MessageType, msg any) (any, uint64, error) {
	future, err := s.raftApplyFuture(t, msg)
	if err != nil {
		return nil, 0, err
	}
	if err := future.Error(); err != nil {
		return nil, 0, err
	}
	resp := future.Response()
	if err, ok := resp.(error); ok && err != nil {
		return nil, future.Index(), err
	}
	return resp, future.Index(), nil
}

// setQueryMeta is used to populate the QueryMeta data for an RPC call
func (r *rpcHandler) setQueryMeta(m *structs.QueryMeta) {
	if r.IsLeader() {
		m.LastContact = 0
		m.KnownLeader = true
	} else {
		m.LastContact = time.Since(r.raft.LastContact())
		m.KnownLeader = (r.raft.Leader() != "")
	}
}

// queryFn is used to perform a query operation. If a re-query is needed, the
// passed-in watch set will be used to block for changes. The passed-in state
// store should be used (vs. calling fsm.State()) since the given state store
// will be correctly watched for changes if the state store is restored from
// a snapshot.
type queryFn func(memdb.WatchSet, *state.StateStore) error

// blockingOptions is used to parameterize blockingRPC
type blockingOptions struct {
	queryOpts *structs.QueryOptions
	queryMeta *structs.QueryMeta
	run       queryFn
}

// blockingRPC is used for queries that need to wait for a
// minimum index. This is used to block and wait for changes.
func (r *rpcHandler) blockingRPC(opts *blockingOptions) error {
	ctx := context.Background()
	var cancel context.CancelFunc
	var state *state.StateStore

	// Fast path non-blocking
	if opts.queryOpts.MinQueryIndex == 0 {
		goto RUN_QUERY
	}

	opts.queryOpts.MaxQueryTime = opts.queryOpts.TimeToBlock()

	// Apply a small amount of jitter to the request
	opts.queryOpts.MaxQueryTime += helper.RandomStagger(opts.queryOpts.MaxQueryTime / structs.JitterFraction)

	// Setup a query timeout
	ctx, cancel = context.WithTimeout(context.Background(), opts.queryOpts.MaxQueryTime)
	defer cancel()

RUN_QUERY:
	// Update the query meta data
	r.setQueryMeta(opts.queryMeta)

	// Increment the rpc query counter
	metrics.IncrCounter([]string{"nomad", "rpc", "query"}, 1)

	// We capture the state store and its abandon channel but pass a snapshot to
	// the blocking query function. We operate on the snapshot to allow separate
	// calls to the state store not all wrapped within the same transaction.
	state = r.fsm.State()
	abandonCh := state.AbandonCh()
	snap, _ := state.Snapshot()
	stateSnap := &snap.StateStore

	// We can skip all watch tracking if this isn't a blocking query.
	var ws memdb.WatchSet
	if opts.queryOpts.MinQueryIndex > 0 {
		ws = memdb.NewWatchSet()

		// This channel will be closed if a snapshot is restored and the
		// whole state store is abandoned.
		ws.Add(abandonCh)
	}

	// Block up to the timeout if we didn't see anything fresh.
	err := opts.run(ws, stateSnap)

	// Check for minimum query time
	if err == nil && opts.queryOpts.MinQueryIndex > 0 && opts.queryMeta.Index <= opts.queryOpts.MinQueryIndex {
		if err := ws.WatchCtx(ctx); err == nil {
			goto RUN_QUERY
		}
	}
	return err
}

func (r *rpcHandler) validateRaftTLS(rpcCtx *RPCContext) error {
	// TLS is not configured or not to be enforced
	tlsConf := r.config.TLSConfig
	if !tlsConf.EnableRPC || !tlsConf.VerifyServerHostname || tlsConf.RPCUpgradeMode {
		return nil
	}

	// check that `server.<region>.nomad` is present in cert
	expected := "server." + r.Region() + ".nomad"
	err := rpcCtx.ValidateCertificateForName(expected)
	if err != nil {
		cert := rpcCtx.Certificate()
		if cert != nil {
			err = fmt.Errorf("request certificate is only valid for %s: %v", cert.DNSNames, err)
		}

		return fmt.Errorf("unauthorized raft connection from %s: %v", rpcCtx.Conn.RemoteAddr(), err)
	}

	// Certificate is valid for the expected name
	return nil
}
