package nomad

import (
	"fmt"
	"math/rand"
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
	// Get a Nomad client node for the controller
	nodeID, err := a.nodeForController(args.PluginID, args.ControllerNodeID)
	if err != nil {
		return err
	}
	args.ControllerNodeID = nodeID

	// Get the connection to the client
	state, ok := a.srv.getNodeConn(args.ControllerNodeID)
	if !ok {
		return findNodeConnAndForward(a.srv, args.ControllerNodeID, "ClientCSI.ControllerAttachVolume", args, reply)
	}

	// Make the RPC
	err = NodeRpc(state.Session, "CSI.ControllerAttachVolume", args, reply)
	if err != nil {
		return fmt.Errorf("controller attach volume: %v", err)
	}
	return nil
}

func (a *ClientCSI) ControllerValidateVolume(args *cstructs.ClientCSIControllerValidateVolumeRequest, reply *cstructs.ClientCSIControllerValidateVolumeResponse) error {
	defer metrics.MeasureSince([]string{"nomad", "client_csi_controller", "validate_volume"}, time.Now())

	// Get a Nomad client node for the controller
	nodeID, err := a.nodeForController(args.PluginID, args.ControllerNodeID)
	if err != nil {
		return err
	}
	args.ControllerNodeID = nodeID

	// Get the connection to the client
	state, ok := a.srv.getNodeConn(args.ControllerNodeID)
	if !ok {
		return findNodeConnAndForward(a.srv, args.ControllerNodeID, "ClientCSI.ControllerValidateVolume", args, reply)
	}

	// Make the RPC
	err = NodeRpc(state.Session, "CSI.ControllerValidateVolume", args, reply)
	if err != nil {
		return fmt.Errorf("validate volume: %v", err)
	}
	return nil
}

func (a *ClientCSI) ControllerDetachVolume(args *cstructs.ClientCSIControllerDetachVolumeRequest, reply *cstructs.ClientCSIControllerDetachVolumeResponse) error {
	defer metrics.MeasureSince([]string{"nomad", "client_csi_controller", "detach_volume"}, time.Now())

	// Get a Nomad client node for the controller
	nodeID, err := a.nodeForController(args.PluginID, args.ControllerNodeID)
	if err != nil {
		return err
	}
	args.ControllerNodeID = nodeID

	// Get the connection to the client
	state, ok := a.srv.getNodeConn(args.ControllerNodeID)
	if !ok {
		return findNodeConnAndForward(a.srv, args.ControllerNodeID, "ClientCSI.ControllerDetachVolume", args, reply)
	}

	// Make the RPC
	err = NodeRpc(state.Session, "CSI.ControllerDetachVolume", args, reply)
	if err != nil {
		return fmt.Errorf("controller detach volume: %v", err)
	}
	return nil

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

// nodeForController validates that the Nomad client node ID for
// a plugin exists and is new enough to support client RPC. If no node
// ID is passed, select a random node ID for the controller to load-balance
// long blocking RPCs across client nodes.
func (a *ClientCSI) nodeForController(pluginID, nodeID string) (string, error) {

	snap, err := a.srv.State().Snapshot()
	if err != nil {
		return "", err
	}

	if nodeID != "" {
		_, err = getNodeForRpc(snap, nodeID)
		if err == nil {
			return nodeID, nil
		}
	}

	if pluginID == "" {
		return "", fmt.Errorf("missing plugin ID")
	}
	ws := memdb.NewWatchSet()

	// note: plugin IDs are not scoped to region/DC but volumes are.
	// so any node we get for a controller is already in the same
	// region/DC for the volume.
	plugin, err := snap.CSIPluginByID(ws, pluginID)
	if err != nil {
		return "", fmt.Errorf("error getting plugin: %s, %v", pluginID, err)
	}
	if plugin == nil {
		return "", fmt.Errorf("plugin missing: %s %v", pluginID, err)
	}
	count := len(plugin.Controllers)
	if count == 0 {
		return "", fmt.Errorf("no controllers available for plugin %q", plugin.ID)
	}

	// iterating maps is "random" but unspecified and isn't particularly
	// random with small maps, so not well-suited for load balancing.
	// so we shuffle the keys and iterate over them.
	clientIDs := make([]string, count)
	for clientID := range plugin.Controllers {
		clientIDs = append(clientIDs, clientID)
	}
	rand.Shuffle(count, func(i, j int) {
		clientIDs[i], clientIDs[j] = clientIDs[j], clientIDs[i]
	})

	for _, clientID := range clientIDs {
		controller := plugin.Controllers[clientID]
		if !controller.IsController() {
			// we don't have separate types for CSIInfo depending on
			// whether it's a controller or node. this error shouldn't
			// make it to production but is to aid developers during
			// development
			err = fmt.Errorf("plugin is not a controller")
			continue
		}
		_, err = getNodeForRpc(snap, clientID)
		if err != nil {
			continue
		}
		return clientID, nil
	}

	return "", err
}
