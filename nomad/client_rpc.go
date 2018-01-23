package nomad

import (
	"fmt"
	"time"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/yamux"
)

// nodeConnState is used to track connection information about a Nomad Client.
type nodeConnState struct {
	// Session holds the multiplexed yamux Session for dialing back.
	Session *yamux.Session

	// Established is when the connection was established.
	Established time.Time
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
	delete(s.nodeConns, ctx.NodeID)
}

// serverWithNodeConn is used to determine which remote server has the most
// recent connection to the given node. The local server is not queried.
// ErrNoNodeConn is returned if all local peers could be queried but did not
// have a connection to the node. Otherwise if a connection could not be found
// and there were RPC errors, an error is returned.
func (s *Server) serverWithNodeConn(nodeID string) (*serverParts, error) {
	s.peerLock.RLock()
	defer s.peerLock.RUnlock()

	// We skip ourselves.
	selfAddr := s.LocalMember().Addr.String()

	// Build the request
	req := &structs.NodeSpecificRequest{
		NodeID: nodeID,
		QueryOptions: structs.QueryOptions{
			Region: s.config.Region,
		},
	}

	// connections is used to store the servers that have connections to the
	// requested node.
	var mostRecentServer *serverParts
	var mostRecent time.Time

	var rpcErr multierror.Error
	for addr, server := range s.localPeers {
		if string(addr) == selfAddr {
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

		return nil, ErrNoNodeConn
	}

	return mostRecentServer, nil
}
