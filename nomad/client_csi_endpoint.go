// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	metrics "github.com/armon/go-metrics"
	log "github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"

	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
)

// ClientCSI is used to forward RPC requests to the targed Nomad client's
// CSIController endpoint.
type ClientCSI struct {
	srv    *Server
	ctx    *RPCContext
	logger log.Logger
}

func NewClientCSIEndpoint(srv *Server, ctx *RPCContext) *ClientCSI {
	return &ClientCSI{srv: srv, ctx: ctx, logger: srv.logger.Named("client_csi")}
}

func (a *ClientCSI) ControllerAttachVolume(args *cstructs.ClientCSIControllerAttachVolumeRequest, reply *cstructs.ClientCSIControllerAttachVolumeResponse) error {
	defer metrics.MeasureSince([]string{"nomad", "client_csi_controller", "attach_volume"}, time.Now())

	err := a.sendCSIControllerRPC(args.PluginID,
		"CSI.ControllerAttachVolume",
		"ClientCSI.ControllerAttachVolume",
		structs.RateMetricWrite,
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
		structs.RateMetricWrite,
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
		structs.RateMetricWrite,
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
		structs.RateMetricWrite,
		args, reply)
	if err != nil {
		return fmt.Errorf("controller create volume: %v", err)
	}
	return nil
}

func (a *ClientCSI) ControllerExpandVolume(args *cstructs.ClientCSIControllerExpandVolumeRequest, reply *cstructs.ClientCSIControllerExpandVolumeResponse) error {
	defer metrics.MeasureSince([]string{"nomad", "client_csi_controller", "expand_volume"}, time.Now())

	err := a.sendCSIControllerRPC(args.PluginID,
		"CSI.ControllerExpandVolume",
		"ClientCSI.ControllerExpandVolume",
		structs.RateMetricWrite,
		args, reply)
	if err != nil {
		return fmt.Errorf("controller expand volume: %v", err)
	}
	return nil
}

func (a *ClientCSI) ControllerDeleteVolume(args *cstructs.ClientCSIControllerDeleteVolumeRequest, reply *cstructs.ClientCSIControllerDeleteVolumeResponse) error {
	defer metrics.MeasureSince([]string{"nomad", "client_csi_controller", "delete_volume"}, time.Now())

	err := a.sendCSIControllerRPC(args.PluginID,
		"CSI.ControllerDeleteVolume",
		"ClientCSI.ControllerDeleteVolume",
		structs.RateMetricWrite,
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
		structs.RateMetricList,
		args, reply)
	if err != nil {
		return fmt.Errorf("controller list volumes: %v", err)
	}
	return nil
}

func (a *ClientCSI) ControllerCreateSnapshot(args *cstructs.ClientCSIControllerCreateSnapshotRequest, reply *cstructs.ClientCSIControllerCreateSnapshotResponse) error {
	defer metrics.MeasureSince([]string{"nomad", "client_csi_controller", "create_snapshot"}, time.Now())

	err := a.sendCSIControllerRPC(args.PluginID,
		"CSI.ControllerCreateSnapshot",
		"ClientCSI.ControllerCreateSnapshot",
		structs.RateMetricWrite,
		args, reply)
	if err != nil {
		return fmt.Errorf("controller create snapshot: %v", err)
	}
	return nil
}

func (a *ClientCSI) ControllerDeleteSnapshot(args *cstructs.ClientCSIControllerDeleteSnapshotRequest, reply *cstructs.ClientCSIControllerDeleteSnapshotResponse) error {
	defer metrics.MeasureSince([]string{"nomad", "client_csi_controller", "delete_snapshot"}, time.Now())

	err := a.sendCSIControllerRPC(args.PluginID,
		"CSI.ControllerDeleteSnapshot",
		"ClientCSI.ControllerDeleteSnapshot",
		structs.RateMetricWrite,
		args, reply)
	if err != nil {
		return fmt.Errorf("controller delete snapshot: %v", err)
	}
	return nil
}

func (a *ClientCSI) ControllerListSnapshots(args *cstructs.ClientCSIControllerListSnapshotsRequest, reply *cstructs.ClientCSIControllerListSnapshotsResponse) error {
	defer metrics.MeasureSince([]string{"nomad", "client_csi_controller", "list_snapshots"}, time.Now())

	err := a.sendCSIControllerRPC(args.PluginID,
		"CSI.ControllerListSnapshots",
		"ClientCSI.ControllerListSnapshots",
		structs.RateMetricList,
		args, reply)
	if err != nil {
		return fmt.Errorf("controller list snapshots: %v", err)
	}
	return nil
}

func (a *ClientCSI) sendCSIControllerRPC(pluginID, method, fwdMethod, op string, args cstructs.CSIControllerRequest, reply interface{}) error {

	// client requests aren't RequestWithIdentity, so we use a placeholder here
	// to populate the identity data for metrics
	identityReq := &structs.GenericRequest{}
	authErr := a.srv.Authenticate(a.ctx, identityReq)
	a.srv.MeasureRPCRate("client_csi", op, identityReq)

	// only servers can send these client RPCs
	err := validateTLSCertificateLevel(a.srv, a.ctx, tlsCertificateLevelServer)
	if authErr != nil || err != nil {
		return structs.ErrPermissionDenied
	}

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
				"nodeID", clientID, "error", err)
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

	// client requests aren't RequestWithIdentity, so we use a placeholder here
	// to populate the identity data for metrics
	identityReq := &structs.GenericRequest{}
	authErr := a.srv.Authenticate(a.ctx, identityReq)
	a.srv.MeasureRPCRate("client_csi", structs.RateMetricWrite, identityReq)

	// only servers can send these client RPCs
	err := validateTLSCertificateLevel(a.srv, a.ctx, tlsCertificateLevelServer)
	if authErr != nil || err != nil {
		return structs.ErrPermissionDenied
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

	// note: plugin IDs are not scoped to region but volumes are. so any Nomad
	// client we get for a controller is already in the same region for the
	// volume.
	plugin, err := snap.CSIPluginByID(ws, pluginID)
	if err != nil {
		return nil, fmt.Errorf("error getting plugin: %s, %v", pluginID, err)
	}
	if plugin == nil {
		return nil, fmt.Errorf("plugin missing: %s", pluginID)
	}

	clientIDs := []string{}

	if len(plugin.Controllers) == 0 {
		return nil, fmt.Errorf("failed to find instances of controller plugin %q", pluginID)
	}

	var merr error
	for clientID, controller := range plugin.Controllers {
		if !controller.IsController() {
			// we don't have separate types for CSIInfo depending on whether
			// it's a controller or node. this error should never make it to
			// production
			merr = errors.Join(merr, fmt.Errorf(
				"plugin instance %q is not a controller but was registered as one - this is always a bug", controller.AllocID))
			continue
		}

		if !controller.Healthy {
			merr = errors.Join(merr, fmt.Errorf(
				"plugin instance %q is not healthy", controller.AllocID))
			continue
		}

		node, err := getNodeForRpc(snap, clientID)
		if err != nil || node == nil {
			merr = errors.Join(merr, fmt.Errorf(
				"cannot find node %q for plugin instance %q", clientID, controller.AllocID))
			continue
		}

		if node.Status != structs.NodeStatusReady {
			merr = errors.Join(merr, fmt.Errorf(
				"node %q for plugin instance %q is not ready", clientID, controller.AllocID))
			continue
		}

		clientIDs = append(clientIDs, clientID)
	}

	if len(clientIDs) == 0 {
		return nil, fmt.Errorf("failed to find clients running controller plugin %q: %v",
			pluginID, merr)
	}

	// Many plugins don't handle concurrent requests as described in the spec,
	// and have undocumented expectations of using k8s-specific sidecars to
	// leader elect. Sort the client IDs so that we prefer sending requests to
	// the same controller to hack around this.
	clientIDs = sort.StringSlice(clientIDs)

	return clientIDs, nil
}
