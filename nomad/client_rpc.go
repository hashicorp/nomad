package nomad

import (
	"fmt"
	"net"
	"time"

	multierror "github.com/hashicorp/go-multierror"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/helper/pool"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/yamux"
	"github.com/ugorji/go/codec"
)

// nodeConnState is used to track connection information about a Nomad Client.
type nodeConnState struct {
	// Session holds the multiplexed yamux Session for dialing back.
	Session *yamux.Session

	// Established is when the connection was established.
	Established time.Time

	// Ctx is the full RPC context
	Ctx *RPCContext
}

// getNodeConn returns the connection to the given node and whether it exists.
func (s *Server) getNodeConn(nodeID string) (*nodeConnState, bool) {
	s.nodeConnsLock.RLock()
	defer s.nodeConnsLock.RUnlock()
	state, ok := s.nodeConns[nodeID]
	return state, ok
}

// connectedNodes returns the set of nodes we have a connection with.
func (s *Server) connectedNodes() map[string]time.Time {
	s.nodeConnsLock.RLock()
	defer s.nodeConnsLock.RUnlock()
	nodes := make(map[string]time.Time, len(s.nodeConns))
	for nodeID, state := range s.nodeConns {
		nodes[nodeID] = state.Established
	}
	return nodes
}

// addNodeConn adds the mapping between a node and its session.
func (s *Server) addNodeConn(ctx *RPCContext) {
	// Hotpath the no-op
	if ctx == nil || ctx.NodeID == "" {
		return
	}

	s.nodeConnsLock.Lock()
	defer s.nodeConnsLock.Unlock()
	s.nodeConns[ctx.NodeID] = &nodeConnState{
		Session:     ctx.Session,
		Established: time.Now(),
		Ctx:         ctx,
	}
}

// removeNodeConn removes the mapping between a node and its session.
func (s *Server) removeNodeConn(ctx *RPCContext) {
	// Hotpath the no-op
	if ctx == nil || ctx.NodeID == "" {
		return
	}

	s.nodeConnsLock.Lock()
	defer s.nodeConnsLock.Unlock()
	state, ok := s.nodeConns[ctx.NodeID]
	if !ok {
		return
	}

	// It is important that we check that the connection being removed is the
	// actual stored connection for the client. It is possible for the client to
	// dial various addresses that all route to the same server. The most common
	// case for this is the original address the client uses to connect to the
	// server differs from the advertised address sent by the heartbeat.
	if state.Ctx.Conn.LocalAddr().String() == ctx.Conn.LocalAddr().String() &&
		state.Ctx.Conn.RemoteAddr().String() == ctx.Conn.RemoteAddr().String() {
		delete(s.nodeConns, ctx.NodeID)
	}
}

// serverWithNodeConn is used to determine which remote server has the most
// recent connection to the given node. The local server is not queried.
// ErrNoNodeConn is returned if all local peers could be queried but did not
// have a connection to the node. Otherwise if a connection could not be found
// and there were RPC errors, an error is returned.
func (s *Server) serverWithNodeConn(nodeID, region string) (*serverParts, error) {
	// We skip ourselves.
	selfAddr := s.LocalMember().Addr.String()

	// Build the request
	req := &structs.NodeSpecificRequest{
		NodeID: nodeID,
		QueryOptions: structs.QueryOptions{
			Region: s.config.Region,
		},
	}

	// Select the list of servers to check based on what region we are querying
	s.peerLock.RLock()

	var rawTargets []*serverParts
	if region == s.Region() {
		rawTargets = make([]*serverParts, 0, len(s.localPeers))
		for _, srv := range s.localPeers {
			rawTargets = append(rawTargets, srv)
		}
	} else {
		peers, ok := s.peers[region]
		if !ok {
			s.peerLock.RUnlock()
			return nil, structs.ErrNoRegionPath
		}
		rawTargets = peers
	}

	targets := make([]*serverParts, 0, len(rawTargets))
	for _, target := range rawTargets {
		targets = append(targets, target.Copy())
	}
	s.peerLock.RUnlock()

	// connections is used to store the servers that have connections to the
	// requested node.
	var mostRecentServer *serverParts
	var mostRecent time.Time

	var rpcErr multierror.Error
	for _, server := range targets {
		if server.Addr.String() == selfAddr {
			continue
		}

		// Make the RPC
		var resp structs.NodeConnQueryResponse
		err := s.connPool.RPC(s.config.Region, server.Addr, server.MajorVersion,
			"Status.HasNodeConn", &req, &resp)
		if err != nil {
			multierror.Append(&rpcErr, fmt.Errorf("failed querying server %q: %v", server.Addr.String(), err))
			continue
		}

		if resp.Connected && resp.Established.After(mostRecent) {
			mostRecentServer = server
			mostRecent = resp.Established
		}
	}

	// Return an error if there is no route to the node.
	if mostRecentServer == nil {
		if err := rpcErr.ErrorOrNil(); err != nil {
			return nil, err
		}

		return nil, structs.ErrNoNodeConn
	}

	return mostRecentServer, nil
}

// NodeRpc is used to make an RPC call to a node. The method takes the
// Yamux session for the node and the method to be called.
func NodeRpc(session *yamux.Session, method string, args, reply interface{}) error {
	// Open a new session
	stream, err := session.Open()
	if err != nil {
		return err
	}
	defer stream.Close()

	// Write the RpcNomad byte to set the mode
	if _, err := stream.Write([]byte{byte(pool.RpcNomad)}); err != nil {
		stream.Close()
		return err
	}

	// Make the RPC
	err = msgpackrpc.CallWithCodec(pool.NewClientCodec(stream), method, args, reply)
	if err != nil {
		return err
	}

	return nil
}

// NodeStreamingRpc is used to make a streaming RPC call to a node. The method
// takes the Yamux session for the node and the method to be called. It conducts
// the initial handshake and returns a connection to be used or an error. It is
// the callers responsibility to close the connection if there is no error.
func NodeStreamingRpc(session *yamux.Session, method string) (net.Conn, error) {
	// Open a new session
	stream, err := session.Open()
	if err != nil {
		return nil, err
	}

	// Write the RpcNomad byte to set the mode
	if _, err := stream.Write([]byte{byte(pool.RpcStreaming)}); err != nil {
		stream.Close()
		return nil, err
	}

	// Send the header
	encoder := codec.NewEncoder(stream, structs.MsgpackHandle)
	header := structs.StreamingRpcHeader{
		Method: method,
	}
	if err := encoder.Encode(header); err != nil {
		stream.Close()
		return nil, err
	}

	return stream, nil
}

// findNodeConnAndForward is a helper for finding the server with a connection
// to the given node and forwarding the RPC to the correct server. This does not
// work for streaming RPCs.
func findNodeConnAndForward(srv *Server, snap *state.StateSnapshot,
	nodeID, method string, args, reply interface{}) error {

	node, err := snap.NodeByID(nil, nodeID)
	if err != nil {
		return err
	}

	if node == nil {
		return fmt.Errorf("Unknown node %q", nodeID)
	}

	// Determine the Server that has a connection to the node.
	srvWithConn, err := srv.serverWithNodeConn(nodeID, srv.Region())
	if err != nil {
		return err
	}

	if srvWithConn == nil {
		return structs.ErrNoNodeConn
	}

	return srv.forwardServer(srvWithConn, method, args, reply)
}
