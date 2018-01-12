package nomad

import (
	"errors"
	"fmt"
	"strings"
	"time"

	metrics "github.com/armon/go-metrics"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/pool"
	nstructs "github.com/hashicorp/nomad/nomad/structs"
)

// TODO(alexdadgar): move to errors.go
const (
	errNoNodeConn = "No path to node"
)

var (
	ErrNoNodeConn = errors.New(errNoNodeConn)
)

func IsErrNoNodeConn(err error) bool {
	return err != nil && strings.Contains(err.Error(), errNoNodeConn)
}

// ClientStats is used to forward RPC requests to the targed Nomad client's
// ClientStats endpoint.
type ClientStats struct {
	srv *Server
}

func (s *ClientStats) Stats(args *structs.ClientStatsRequest, reply *structs.ClientStatsResponse) error {
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

	// Get the connection to the client
	session, ok := s.srv.getNodeConn(args.NodeID)
	if !ok {
		// Check if the node even exists
		snap, err := s.srv.State().Snapshot()
		if err != nil {
			return err
		}

		node, err := snap.NodeByID(nil, args.NodeID)
		if err != nil {
			return err
		}

		if node == nil {
			return fmt.Errorf("Unknown node %q", args.NodeID)
		}

		// TODO Handle forwarding to other servers
		return ErrNoNodeConn
	}

	// Open a new session
	stream, err := session.Open()
	if err != nil {
		return err
	}

	// Make the RPC
	err = msgpackrpc.CallWithCodec(pool.NewClientCodec(stream), "ClientStats.Stats", args, reply)
	if err != nil {
		return err
	}

	return nil
}
