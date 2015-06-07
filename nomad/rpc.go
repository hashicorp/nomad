package nomad

import (
	"crypto/tls"
	"fmt"
	"io"
	"math/rand"
	"net"
	"strings"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/yamux"
)

type RPCType byte

const (
	rpcNomad     RPCType = 0x01
	rpcRaft              = 0x02
	rpcMultiplex         = 0x03
	rpcTLS               = 0x04
)

const (
	// rpcHTTPSMagic is used to detect an incoming HTTPS
	// request. TLS starts with the 0x16 magic byte.
	rpcHTTPSMagic = 0x16

	// rpcHTTPMagic is used to detect an incoming HTTP
	// request. The request starts with 'HTTP'
	rpcHTTPMagic = 0x48
)

const (
	// maxQueryTime is used to bound the limit of a blocking query
	maxQueryTime = 300 * time.Second

	// defaultQueryTime is the amount of time we block waiting for a change
	// if no time is specified. Previously we would wait the maxQueryTime.
	defaultQueryTime = 300 * time.Second

	// jitterFraction is a the limit to the amount of jitter we apply
	// to a user specified MaxQueryTime. We divide the specified time by
	// the fraction. So 16 == 6.25% limit of jitter
	jitterFraction = 16

	// Warn if the Raft command is larger than this.
	// If it's over 1MB something is probably being abusive.
	raftWarnSize = 1024 * 1024

	// enqueueLimit caps how long we will wait to enqueue
	// a new Raft command. Something is probably wrong if this
	// value is ever reached. However, it prevents us from blocking
	// the requesting goroutine forever.
	enqueueLimit = 30 * time.Second
)

// listen is used to listen for incoming RPC connections
func (s *Server) listen() {
	for {
		// Accept a connection
		conn, err := s.rpcListener.Accept()
		if err != nil {
			if s.shutdown {
				return
			}
			s.logger.Printf("[ERR] nomad.rpc: failed to accept RPC conn: %v", err)
			continue
		}

		go s.handleConn(conn, false)
		metrics.IncrCounter([]string{"nomad", "rpc", "accept_conn"}, 1)
	}
}

// handleConn is used to determine if this is a Raft or
// Nomad type RPC connection and invoke the correct handler
func (s *Server) handleConn(conn net.Conn, isTLS bool) {
	// Read a single byte
	buf := make([]byte, 1)
	if _, err := conn.Read(buf); err != nil {
		if err != io.EOF {
			s.logger.Printf("[ERR] nomad.rpc: failed to read byte: %v", err)
		}
		conn.Close()
		return
	}

	// Enforce TLS if VerifyIncoming is set
	if s.config.RequireTLS && !isTLS && RPCType(buf[0]) != rpcTLS {
		s.logger.Printf("[WARN] nomad.rpc: Non-TLS connection attempted with RequireTLS set")
		conn.Close()
		return
	}

	// Switch on the byte
	switch RPCType(buf[0]) {
	case rpcNomad:
		s.handleNomadConn(conn)

	case rpcRaft:
		metrics.IncrCounter([]string{"nomad", "rpc", "raft_handoff"}, 1)
		s.raftLayer.Handoff(conn)

	case rpcMultiplex:
		s.handleMultiplex(conn)

	case rpcTLS:
		if s.rpcTLS == nil {
			s.logger.Printf("[WARN] nomad.rpc: TLS connection attempted, server not configured for TLS")
			conn.Close()
			return
		}
		conn = tls.Server(conn, s.rpcTLS)
		s.handleConn(conn, true)

	default:
		s.logger.Printf("[ERR] nomad.rpc: unrecognized RPC byte: %v", buf[0])
		conn.Close()
		return
	}
}

// handleMultiplex is used to multiplex a single incoming connection
// using the Yamux multiplexer
func (s *Server) handleMultiplex(conn net.Conn) {
	defer conn.Close()
	conf := yamux.DefaultConfig()
	conf.LogOutput = s.config.LogOutput
	server, _ := yamux.Server(conn, conf)
	for {
		sub, err := server.Accept()
		if err != nil {
			if err != io.EOF {
				s.logger.Printf("[ERR] nomad.rpc: multiplex conn accept failed: %v", err)
			}
			return
		}
		go s.handleNomadConn(sub)
	}
}

// handleNomadConn is used to service a single Nomad RPC connection
func (s *Server) handleNomadConn(conn net.Conn) {
	defer conn.Close()
	rpcCodec := msgpackrpc.NewServerCodec(conn)
	for {
		select {
		case <-s.shutdownCh:
			return
		default:
		}

		if err := s.rpcServer.ServeRequest(rpcCodec); err != nil {
			if err != io.EOF && !strings.Contains(err.Error(), "closed") {
				s.logger.Printf("[ERR] nomad.rpc: RPC error: %v (%v)", err, conn)
				metrics.IncrCounter([]string{"nomad", "rpc", "request_error"}, 1)
			}
			return
		}
		metrics.IncrCounter([]string{"nomad", "rpc", "request"}, 1)
	}
}

// forward is used to forward to a remote region or to forward to the local leader
// Returns a bool of if forwarding was performed, as well as any error
func (s *Server) forward(method string, info structs.RPCInfo, args interface{}, reply interface{}) (bool, error) {
	// Handle region forwarding
	region := info.RequestRegion()
	if region != s.config.Region {
		err := s.forwardRegion(method, region, args, reply)
		return true, err
	}

	// Check if we can allow a stale read
	if info.IsRead() && info.AllowStaleRead() {
		return false, nil
	}

	// Handle leader forwarding
	if !s.IsLeader() {
		err := s.forwardLeader(method, args, reply)
		return true, err
	}
	return false, nil
}

// forwardLeader is used to forward an RPC call to the leader, or fail if no leader
func (s *Server) forwardLeader(method string, args interface{}, reply interface{}) error {
	// Get the leader
	leader := s.raft.Leader()
	if leader == "" {
		return structs.ErrNoLeader
	}

	// Lookup the server
	s.peerLock.RLock()
	server := s.localPeers[leader]
	s.peerLock.RUnlock()

	// Handle a missing server
	if server == nil {
		return structs.ErrNoLeader
	}
	return s.connPool.RPC(s.config.Region, server.Addr, server.Version, method, args, reply)
}

// forwardRegion is used to forward an RPC call to a remote region, or fail if no servers
func (s *Server) forwardRegion(method, region string, args interface{}, reply interface{}) error {
	// Bail if we can't find any servers
	s.peerLock.RLock()
	servers := s.peers[region]
	if len(servers) == 0 {
		s.peerLock.RUnlock()
		s.logger.Printf("[WARN] nomad.rpc: RPC request for region '%s', no path found",
			region)
		return structs.ErrNoRegionPath
	}

	// Select a random addr
	offset := rand.Int31() % int32(len(servers))
	server := servers[offset]
	s.peerLock.RUnlock()

	// Forward to remote Nomad
	metrics.IncrCounter([]string{"nomad", "rpc", "cross-region", region}, 1)
	return s.connPool.RPC(region, server.Addr, server.Version, method, args, reply)
}

// raftApply is used to encode a message, run it through raft, and return
// the FSM response along with any errors
func (s *Server) raftApply(t structs.MessageType, msg interface{}) (interface{}, error) {
	buf, err := structs.Encode(t, msg)
	if err != nil {
		return nil, fmt.Errorf("Failed to encode request: %v", err)
	}

	// Warn if the command is very large
	if n := len(buf); n > raftWarnSize {
		s.logger.Printf("[WARN] nomad: Attempting to apply large raft entry (type %d) (%d bytes)", t, n)
	}

	future := s.raft.Apply(buf, enqueueLimit)
	if err := future.Error(); err != nil {
		return nil, err
	}

	return future.Response(), nil
}
