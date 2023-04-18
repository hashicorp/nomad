package nomad

import (
	"errors"
	"time"

	metrics "github.com/armon/go-metrics"
	log "github.com/hashicorp/go-hclog"
	nstructs "github.com/hashicorp/nomad/nomad/structs"

	"github.com/hashicorp/nomad/client/structs"
)

// ClientStats is used to forward RPC requests to the targed Nomad client's
// ClientStats endpoint.
type ClientStats struct {
	srv    *Server
	logger log.Logger
}

func (s *ClientStats) Stats(args *nstructs.NodeSpecificRequest, reply *structs.ClientStatsResponse) error {
	// We only allow stale reads since the only potentially stale information is
	// the Node registration and the cost is fairly high for adding another hope
	// in the forwarding chain.
	args.QueryOptions.AllowStale = true

	// Potentially forward to a different region.
	if done, err := s.srv.forward("ClientStats.Stats", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "client_stats", "stats"}, time.Now())

	// Check node read permissions
	if aclObj, err := s.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNodeRead() {
		return nstructs.ErrPermissionDenied
	}

	// Verify the arguments.
	if args.NodeID == "" {
		return errors.New("missing NodeID")
	}

	// Check if the node even exists and is compatible with NodeRpc
	snap, err := s.srv.State().Snapshot()
	if err != nil {
		return err
	}

	// Make sure Node is new enough to support RPC
	_, err = getNodeForRpc(snap, args.NodeID)
	if err != nil {
		return err
	}

	// Get the connection to the client
	state, ok := s.srv.getNodeConn(args.NodeID)
	if !ok {

		// Determine the Server that has a connection to the node.
		srv, err := s.srv.serverWithNodeConn(args.NodeID, s.srv.Region())
		if err != nil {
			return err
		}

		if srv == nil {
			return nstructs.ErrNoNodeConn
		}

		return s.srv.forwardServer(srv, "ClientStats.Stats", args, reply)
	}

	// Make the RPC
	return NodeRpc(state.Session, "ClientStats.Stats", args, reply)
}
