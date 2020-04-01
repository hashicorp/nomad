package nomad

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	metrics "github.com/armon/go-metrics"
	log "github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// ClientCSI is used to forward RPC requests to the targed Nomad client's
// CSIController endpoint.
type ClientCSI struct {
	srv    *Server
	logger log.Logger
}

func (a *ClientCSI) ControllerAttachVolume(args *cstructs.ClientCSIControllerAttachVolumeRequest, reply *cstructs.ClientCSIControllerAttachVolumeResponse) error {
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

func (srv *Server) volAndPluginLookup(namespace, volID string) (*structs.CSIPlugin, *structs.CSIVolume, error) {
	state := srv.fsm.State()
	ws := memdb.NewWatchSet()

	vol, err := state.CSIVolumeByID(ws, namespace, volID)
	if err != nil {
		return nil, nil, err
	}
	if vol == nil {
		return nil, nil, fmt.Errorf("volume not found: %s", volID)
	}
	if !vol.ControllerRequired {
		return nil, vol, nil
	}

	// note: we do this same lookup in CSIVolumeByID but then throw
	// away the pointer to the plugin rather than attaching it to
	// the volume so we have to do it again here.
	plug, err := state.CSIPluginByID(ws, vol.PluginID)
	if err != nil {
		return nil, nil, err
	}
	if plug == nil {
		return nil, nil, fmt.Errorf("plugin not found: %s", vol.PluginID)
	}
	return plug, vol, nil
}

// nodeForControllerPlugin returns the node ID for a random controller
// to load-balance long-blocking RPCs across client nodes.
func nodeForControllerPlugin(state *state.StateStore, plugin *structs.CSIPlugin) (string, error) {
	count := len(plugin.Controllers)
	if count == 0 {
		return "", fmt.Errorf("no controllers available for plugin %q", plugin.ID)
	}
	snap, err := state.Snapshot()
	if err != nil {
		return "", err
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
