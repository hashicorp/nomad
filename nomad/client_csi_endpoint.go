package nomad

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	metrics "github.com/armon/go-metrics"
	log "github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"
	cstructs "github.com/hashicorp/nomad/client/structs"
)

// ClientCSI is used to forward RPC requests to the targed Nomad client's
// CSIController endpoint.
type ClientCSI struct {
	srv    *Server
	logger log.Logger
}

func (a *ClientCSI) ControllerAttachVolume(args *cstructs.ClientCSIControllerAttachVolumeRequest, reply *cstructs.ClientCSIControllerAttachVolumeResponse) error {
	defer metrics.MeasureSince([]string{"nomad", "client_csi_controller", "attach_volume"}, time.Now())

	err := a.sendCSIControllerRPC(args.PluginID,
		"CSI.ControllerAttachVolume",
		"ClientCSI.ControllerAttachVolume",
		args, reply)
	if err != nil {
		return fmt.Errorf("controller attach volume: %v", err)
	}
	return nil
}

func (a *ClientCSI) ControllerValidateVolume(args *cstructs.ClientCSIControllerValidateVolumeRequest, reply *cstructs.ClientCSIControllerValidateVolumeResponse) error {
	defer metrics.MeasureSince([]string{"nomad", "client_csi_controller", "validate_volume"}, time.Now())

	err := a.sendCSIControllerRPC(args.PluginID,
		"CSI.ControllerValidateVolume",
		"ClientCSI.ControllerValidateVolume",
		args, reply)
	if err != nil {
		return fmt.Errorf("controller validate volume: %v", err)
	}
	return nil
}

func (a *ClientCSI) ControllerDetachVolume(args *cstructs.ClientCSIControllerDetachVolumeRequest, reply *cstructs.ClientCSIControllerDetachVolumeResponse) error {
	defer metrics.MeasureSince([]string{"nomad", "client_csi_controller", "detach_volume"}, time.Now())

	err := a.sendCSIControllerRPC(args.PluginID,
		"CSI.ControllerDetachVolume",
		"ClientCSI.ControllerDetachVolume",
		args, reply)
	if err != nil {
		return fmt.Errorf("controller detach volume: %v", err)
	}
	return nil
}

func (a *ClientCSI) ControllerCreateVolume(args *cstructs.ClientCSIControllerCreateVolumeRequest, reply *cstructs.ClientCSIControllerCreateVolumeResponse) error {
	defer metrics.MeasureSince([]string{"nomad", "client_csi_controller", "create_volume"}, time.Now())

	err := a.sendCSIControllerRPC(args.PluginID,
		"CSI.ControllerCreateVolume",
		"ClientCSI.ControllerCreateVolume",
		args, reply)
	if err != nil {
		return fmt.Errorf("controller create volume: %v", err)
	}
	return nil
}

func (a *ClientCSI) ControllerDeleteVolume(args *cstructs.ClientCSIControllerDeleteVolumeRequest, reply *cstructs.ClientCSIControllerDeleteVolumeResponse) error {
	defer metrics.MeasureSince([]string{"nomad", "client_csi_controller", "delete_volume"}, time.Now())

	err := a.sendCSIControllerRPC(args.PluginID,
		"CSI.ControllerDeleteVolume",
		"ClientCSI.ControllerDeleteVolume",
		args, reply)
	if err != nil {
		return fmt.Errorf("controller delete volume: %v", err)
	}
	return nil
}

func (a *ClientCSI) ControllerListVolumes(args *cstructs.ClientCSIControllerListVolumesRequest, reply *cstructs.ClientCSIControllerListVolumesResponse) error {
	defer metrics.MeasureSince([]string{"nomad", "client_csi_controller", "list_volumes"}, time.Now())

	err := a.sendCSIControllerRPC(args.PluginID,
		"CSI.ControllerListVolumes",
		"ClientCSI.ControllerListVolumes",
		args, reply)
	if err != nil {
		return fmt.Errorf("controller list volumes: %v", err)
	}
	return nil
}

func (a *ClientCSI) sendCSIControllerRPC(pluginID, method, fwdMethod string, args cstructs.CSIControllerRequest, reply interface{}) error {

	clientIDs, err := a.clientIDsForController(pluginID)
	if err != nil {
		return err
	}

	for _, clientID := range clientIDs {
		args.SetControllerNodeID(clientID)

		state, ok := a.srv.getNodeConn(clientID)
		if !ok {
			return findNodeConnAndForward(a.srv,
				clientID, fwdMethod, args, reply)
		}

		err = NodeRpc(state.Session, method, args, reply)
		if err == nil {
			return nil
		}
		if a.isRetryable(err) {
			a.logger.Debug("failed to reach controller on client",
				"nodeID", clientID, "err", err)
			continue
		}
		return err
	}
	return err
}

// we can retry the same RPC on a different controller in the cases where the
// client has stopped and been GC'd, or where the controller has stopped but
// we don't have the fingerprint update yet
func (a *ClientCSI) isRetryable(err error) bool {
	// TODO: msgpack-rpc mangles the error so we lose the wrapping,
	// but if that can be fixed upstream we should use that here instead
	return strings.Contains(err.Error(), "CSI client error (retryable)") ||
		strings.Contains(err.Error(), "Unknown node")
}

func (a *ClientCSI) NodeDetachVolume(args *cstructs.ClientCSINodeDetachVolumeRequest, reply *cstructs.ClientCSINodeDetachVolumeResponse) error {
	defer metrics.MeasureSince([]string{"nomad", "client_csi_node", "detach_volume"}, time.Now())

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
		return findNodeConnAndForward(a.srv, args.NodeID, "ClientCSI.NodeDetachVolume", args, reply)
	}

	// Make the RPC
	err = NodeRpc(state.Session, "CSI.NodeDetachVolume", args, reply)
	if err != nil {
		return fmt.Errorf("node detach volume: %v", err)
	}
	return nil

}

// clientIDsForController returns a shuffled list of client IDs where the
// controller plugin is expected to be running.
func (a *ClientCSI) clientIDsForController(pluginID string) ([]string, error) {

	snap, err := a.srv.State().Snapshot()
	if err != nil {
		return nil, err
	}

	if pluginID == "" {
		return nil, fmt.Errorf("missing plugin ID")
	}

	ws := memdb.NewWatchSet()

	// note: plugin IDs are not scoped to region/DC but volumes are.
	// so any node we get for a controller is already in the same
	// region/DC for the volume.
	plugin, err := snap.CSIPluginByID(ws, pluginID)
	if err != nil {
		return nil, fmt.Errorf("error getting plugin: %s, %v", pluginID, err)
	}
	if plugin == nil {
		return nil, fmt.Errorf("plugin missing: %s", pluginID)
	}

	// iterating maps is "random" but unspecified and isn't particularly
	// random with small maps, so not well-suited for load balancing.
	// so we shuffle the keys and iterate over them.
	clientIDs := []string{}

	for clientID, controller := range plugin.Controllers {
		if !controller.IsController() {
			// we don't have separate types for CSIInfo depending on
			// whether it's a controller or node. this error shouldn't
			// make it to production but is to aid developers during
			// development
			continue
		}
		node, err := getNodeForRpc(snap, clientID)
		if err == nil && node != nil && node.Ready() {
			clientIDs = append(clientIDs, clientID)
		}
	}
	if len(clientIDs) == 0 {
		return nil, fmt.Errorf("failed to find clients running controller plugin %q", pluginID)
	}

	rand.Shuffle(len(clientIDs), func(i, j int) {
		clientIDs[i], clientIDs[j] = clientIDs[j], clientIDs[i]
	})

	return clientIDs, nil
}
