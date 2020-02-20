package nomad

import (
	"errors"
	"time"

	metrics "github.com/armon/go-metrics"
	log "github.com/hashicorp/go-hclog"
	cstructs "github.com/hashicorp/nomad/client/structs"
)

// ClientCSIController is used to forward RPC requests to the targed Nomad client's
// CSIController endpoint.
type ClientCSIController struct {
	srv    *Server
	logger log.Logger
}

func (a *ClientCSIController) AttachVolume(args *cstructs.ClientCSIControllerAttachVolumeRequest, reply *cstructs.ClientCSIControllerAttachVolumeResponse) error {
	// We only allow stale reads since the only potentially stale information is
	// the Node registration and the cost is fairly high for adding another hop
	// in the forwarding chain.
	args.QueryOptions.AllowStale = true

	// Potentially forward to a different region.
	if done, err := a.srv.forward("ClientCSIController.AttachVolume", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "client_csi_controller", "attach_volume"}, time.Now())

	// Verify the arguments.
	if args.NodeID == "" {
		return errors.New("missing NodeID")
	}

	// Make sure Node is valid and new enough to support RPC
	snap, err := a.srv.State().Snapshot()
	if err != nil {
		return err
	}

	_, err = getNodeForRpc(snap, args.NodeID)
	if err != nil {
		return err
	}

	// Get the connection to the client
	state, ok := a.srv.getNodeConn(args.NodeID)
	if !ok {
		return findNodeConnAndForward(a.srv, args.NodeID, "ClientCSIController.AttachVolume", args, reply)
	}

	// Make the RPC
	return NodeRpc(state.Session, "CSIController.AttachVolume", args, reply)
}
