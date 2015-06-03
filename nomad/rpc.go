package nomad

import (
	"crypto/tls"
	"io"
	"net"
	"strings"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/net-rpc-msgpackrpc"
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
