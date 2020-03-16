package nomad

import (
	"errors"
	"fmt"
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
	defer metrics.MeasureSince([]string{"nomad", "client_csi_controller", "attach_volume"}, time.Now())

	// Verify the arguments.
	if args.ControllerNodeID == "" {
		return errors.New("missing ControllerNodeID")
	}

	// Make sure Node is valid and new enough to support RPC
	snap, err := a.srv.State().Snapshot()
	if err != nil {
		return err
	}

	_, err = getNodeForRpc(snap, args.ControllerNodeID)
	if err != nil {
		return err
	}

	// Get the connection to the client
	state, ok := a.srv.getNodeConn(args.ControllerNodeID)
	if !ok {
		return findNodeConnAndForward(a.srv, args.ControllerNodeID, "ClientCSIController.AttachVolume", args, reply)
	}

	// Make the RPC
	err = NodeRpc(state.Session, "CSIController.AttachVolume", args, reply)
	if err != nil {
		return fmt.Errorf("attach volume: %v", err)
	}
	return nil
}

func (a *ClientCSIController) ValidateVolume(args *cstructs.ClientCSIControllerValidateVolumeRequest, reply *cstructs.ClientCSIControllerValidateVolumeResponse) error {
	defer metrics.MeasureSince([]string{"nomad", "client_csi_controller", "validate_volume"}, time.Now())

	// Verify the arguments.
	if args.ControllerNodeID == "" {
		return errors.New("missing ControllerNodeID")
	}

	// Make sure Node is valid and new enough to support RPC
	snap, err := a.srv.State().Snapshot()
	if err != nil {
		return err
	}

	_, err = getNodeForRpc(snap, args.ControllerNodeID)
	if err != nil {
		return err
	}

	// Get the connection to the client
	state, ok := a.srv.getNodeConn(args.ControllerNodeID)
	if !ok {
		return findNodeConnAndForward(a.srv, args.ControllerNodeID, "ClientCSIController.ValidateVolume", args, reply)
	}

	// Make the RPC
	err = NodeRpc(state.Session, "CSIController.ValidateVolume", args, reply)
	if err != nil {
		return fmt.Errorf("validate volume: %v", err)
	}
	return nil
}

func (a *ClientCSIController) DetachVolume(args *cstructs.ClientCSIControllerDetachVolumeRequest, reply *cstructs.ClientCSIControllerDetachVolumeResponse) error {
	defer metrics.MeasureSince([]string{"nomad", "client_csi_controller", "detach_volume"}, time.Now())

	// Verify the arguments.
	if args.ControllerNodeID == "" {
		return errors.New("missing ControllerNodeID")
	}

	// Make sure Node is valid and new enough to support RPC
	snap, err := a.srv.State().Snapshot()
	if err != nil {
		return err
	}

	_, err = getNodeForRpc(snap, args.ControllerNodeID)
	if err != nil {
		return err
	}

	// Get the connection to the client
	state, ok := a.srv.getNodeConn(args.ControllerNodeID)
	if !ok {
		return findNodeConnAndForward(a.srv, args.ControllerNodeID, "ClientCSIController.DetachVolume", args, reply)
	}

	// Make the RPC
	err = NodeRpc(state.Session, "CSIController.DetachVolume", args, reply)
	if err != nil {
		return fmt.Errorf("detach volume: %v", err)
	}
	return nil

}
