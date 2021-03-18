package nomad

import (
	"errors"
	"fmt"
	"time"

	metrics "github.com/armon/go-metrics"
	log "github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/acl"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// CSIVolume wraps the structs.CSIVolume with request data and server context
type CSIVolume struct {
	srv    *Server
	logger log.Logger
}

// QueryACLObj looks up the ACL token in the request and returns the acl.ACL object
// - fallback to node secret ids
func (srv *Server) QueryACLObj(args *structs.QueryOptions, allowNodeAccess bool) (*acl.ACL, error) {
	// Lookup the token
	aclObj, err := srv.ResolveToken(args.AuthToken)
	if err != nil {
		// If ResolveToken had an unexpected error return that
		if !structs.IsErrTokenNotFound(err) {
			return nil, err
		}

		// If we don't allow access to this endpoint from Nodes, then return token
		// not found.
		if !allowNodeAccess {
			return nil, structs.ErrTokenNotFound
		}

		ws := memdb.NewWatchSet()
		// Attempt to lookup AuthToken as a Node.SecretID since nodes may call
		// call this endpoint and don't have an ACL token.
		node, stateErr := srv.fsm.State().NodeBySecretID(ws, args.AuthToken)
		if stateErr != nil {
			// Return the original ResolveToken error with this err
			var merr multierror.Error
			merr.Errors = append(merr.Errors, err, stateErr)
			return nil, merr.ErrorOrNil()
		}

		// We did not find a Node for this ID, so return Token Not Found.
		if node == nil {
			return nil, structs.ErrTokenNotFound
		}
	}

	// Return either the users aclObj, or nil if ACLs are disabled.
	return aclObj, nil
}

// WriteACLObj calls QueryACLObj for a WriteRequest
func (srv *Server) WriteACLObj(args *structs.WriteRequest, allowNodeAccess bool) (*acl.ACL, error) {
	opts := &structs.QueryOptions{
		Region:    args.RequestRegion(),
		Namespace: args.RequestNamespace(),
		AuthToken: args.AuthToken,
	}
	return srv.QueryACLObj(opts, allowNodeAccess)
}

const (
	csiVolumeTable = "csi_volumes"
	csiPluginTable = "csi_plugins"
)

// replySetIndex sets the reply with the last index that modified the table
func (srv *Server) replySetIndex(table string, reply *structs.QueryMeta) error {
	s := srv.fsm.State()

	index, err := s.Index(table)
	if err != nil {
		return err
	}
	reply.Index = index

	// Set the query response
	srv.setQueryMeta(reply)
	return nil
}

// List replies with CSIVolumes, filtered by ACL access
func (v *CSIVolume) List(args *structs.CSIVolumeListRequest, reply *structs.CSIVolumeListResponse) error {
	if done, err := v.srv.forward("CSIVolume.List", args, args, reply); done {
		return err
	}

	allowVolume := acl.NamespaceValidator(acl.NamespaceCapabilityCSIListVolume,
		acl.NamespaceCapabilityCSIReadVolume,
		acl.NamespaceCapabilityCSIMountVolume,
		acl.NamespaceCapabilityListJobs)
	aclObj, err := v.srv.QueryACLObj(&args.QueryOptions, false)
	if err != nil {
		return err
	}

	if !allowVolume(aclObj, args.RequestNamespace()) {
		return structs.ErrPermissionDenied
	}

	metricsStart := time.Now()
	defer metrics.MeasureSince([]string{"nomad", "volume", "list"}, metricsStart)

	ns := args.RequestNamespace()
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			snap, err := state.Snapshot()
			if err != nil {
				return err
			}

			// Query all volumes
			var iter memdb.ResultIterator

			prefix := args.Prefix

			if args.NodeID != "" {
				iter, err = snap.CSIVolumesByNodeID(ws, prefix, args.NodeID)
			} else if args.PluginID != "" {
				iter, err = snap.CSIVolumesByPluginID(ws, ns, prefix, args.PluginID)
			} else {
				iter, err = snap.CSIVolumesByNamespace(ws, ns, prefix)
			}

			if err != nil {
				return err
			}

			// Collect results, filter by ACL access
			vs := []*structs.CSIVolListStub{}

			for {
				raw := iter.Next()
				if raw == nil {
					break
				}
				vol := raw.(*structs.CSIVolume)

				// Remove (possibly again) by PluginID to handle passing both
				// NodeID and PluginID
				if args.PluginID != "" && args.PluginID != vol.PluginID {
					continue
				}

				// Remove by Namespace, since CSIVolumesByNodeID hasn't used
				// the Namespace yet
				if vol.Namespace != ns {
					continue
				}

				vol, err := snap.CSIVolumeDenormalizePlugins(ws, vol.Copy())
				if err != nil {
					return err
				}

				vs = append(vs, vol.Stub())
			}
			reply.Volumes = vs
			return v.srv.replySetIndex(csiVolumeTable, &reply.QueryMeta)
		}}
	return v.srv.blockingRPC(&opts)
}

// Get fetches detailed information about a specific volume
func (v *CSIVolume) Get(args *structs.CSIVolumeGetRequest, reply *structs.CSIVolumeGetResponse) error {
	if done, err := v.srv.forward("CSIVolume.Get", args, args, reply); done {
		return err
	}

	allowCSIAccess := acl.NamespaceValidator(acl.NamespaceCapabilityCSIReadVolume,
		acl.NamespaceCapabilityCSIMountVolume,
		acl.NamespaceCapabilityReadJob)
	aclObj, err := v.srv.QueryACLObj(&args.QueryOptions, true)
	if err != nil {
		return err
	}

	ns := args.RequestNamespace()
	if !allowCSIAccess(aclObj, ns) {
		return structs.ErrPermissionDenied
	}

	metricsStart := time.Now()
	defer metrics.MeasureSince([]string{"nomad", "volume", "get"}, metricsStart)

	if args.ID == "" {
		return fmt.Errorf("missing volume ID")
	}

	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			snap, err := state.Snapshot()
			if err != nil {
				return err
			}

			vol, err := snap.CSIVolumeByID(ws, ns, args.ID)
			if err != nil {
				return err
			}
			if vol != nil {
				vol, err = snap.CSIVolumeDenormalize(ws, vol)
			}
			if err != nil {
				return err
			}

			reply.Volume = vol
			return v.srv.replySetIndex(csiVolumeTable, &reply.QueryMeta)
		}}
	return v.srv.blockingRPC(&opts)
}

func (v *CSIVolume) pluginValidateVolume(req *structs.CSIVolumeRegisterRequest, vol *structs.CSIVolume) (*structs.CSIPlugin, error) {
	state := v.srv.fsm.State()

	plugin, err := state.CSIPluginByID(nil, vol.PluginID)
	if err != nil {
		return nil, err
	}
	if plugin == nil {
		return nil, fmt.Errorf("no CSI plugin named: %s could be found", vol.PluginID)
	}

	vol.Provider = plugin.Provider
	vol.ProviderVersion = plugin.Version
	return plugin, nil
}

func (v *CSIVolume) controllerValidateVolume(req *structs.CSIVolumeRegisterRequest, vol *structs.CSIVolume, plugin *structs.CSIPlugin) error {

	if !plugin.ControllerRequired {
		// The plugin does not require a controller, so for now we won't do any
		// further validation of the volume.
		return nil
	}

	method := "ClientCSI.ControllerValidateVolume"
	cReq := &cstructs.ClientCSIControllerValidateVolumeRequest{
		VolumeID:       vol.RemoteID(),
		AttachmentMode: vol.AttachmentMode,
		AccessMode:     vol.AccessMode,
		Secrets:        vol.Secrets,
		Parameters:     vol.Parameters,
		Context:        vol.Context,
	}
	cReq.PluginID = plugin.ID
	cResp := &cstructs.ClientCSIControllerValidateVolumeResponse{}

	return v.srv.RPC(method, cReq, cResp)
}

// Register registers a new volume
func (v *CSIVolume) Register(args *structs.CSIVolumeRegisterRequest, reply *structs.CSIVolumeRegisterResponse) error {
	if done, err := v.srv.forward("CSIVolume.Register", args, args, reply); done {
		return err
	}

	allowVolume := acl.NamespaceValidator(acl.NamespaceCapabilityCSIWriteVolume)
	aclObj, err := v.srv.WriteACLObj(&args.WriteRequest, false)
	if err != nil {
		return err
	}

	metricsStart := time.Now()
	defer metrics.MeasureSince([]string{"nomad", "volume", "register"}, metricsStart)

	if !allowVolume(aclObj, args.RequestNamespace()) || !aclObj.AllowPluginRead() {
		return structs.ErrPermissionDenied
	}

	if args.Volumes == nil || len(args.Volumes) == 0 {
		return fmt.Errorf("missing volume definition")
	}

	// This is the only namespace we ACL checked, force all the volumes to use it.
	// We also validate that the plugin exists for each plugin, and validate the
	// capabilities when the plugin has a controller.
	for _, vol := range args.Volumes {
		vol.Namespace = args.RequestNamespace()
		if err = vol.Validate(); err != nil {
			return err
		}

		plugin, err := v.pluginValidateVolume(args, vol)
		if err != nil {
			return err
		}
		if err := v.controllerValidateVolume(args, vol, plugin); err != nil {
			return err
		}
	}

	resp, index, err := v.srv.raftApply(structs.CSIVolumeRegisterRequestType, args)
	if err != nil {
		v.logger.Error("csi raft apply failed", "error", err, "method", "register")
		return err
	}
	if respErr, ok := resp.(error); ok {
		return respErr
	}

	reply.Index = index
	v.srv.setQueryMeta(&reply.QueryMeta)
	return nil
}

// Deregister removes a set of volumes
func (v *CSIVolume) Deregister(args *structs.CSIVolumeDeregisterRequest, reply *structs.CSIVolumeDeregisterResponse) error {
	if done, err := v.srv.forward("CSIVolume.Deregister", args, args, reply); done {
		return err
	}

	allowVolume := acl.NamespaceValidator(acl.NamespaceCapabilityCSIWriteVolume)
	aclObj, err := v.srv.WriteACLObj(&args.WriteRequest, false)
	if err != nil {
		return err
	}

	metricsStart := time.Now()
	defer metrics.MeasureSince([]string{"nomad", "volume", "deregister"}, metricsStart)

	ns := args.RequestNamespace()
	if !allowVolume(aclObj, ns) {
		return structs.ErrPermissionDenied
	}

	if len(args.VolumeIDs) == 0 {
		return fmt.Errorf("missing volume IDs")
	}

	resp, index, err := v.srv.raftApply(structs.CSIVolumeDeregisterRequestType, args)
	if err != nil {
		v.logger.Error("csi raft apply failed", "error", err, "method", "deregister")
		return err
	}
	if respErr, ok := resp.(error); ok {
		return respErr
	}

	reply.Index = index
	v.srv.setQueryMeta(&reply.QueryMeta)
	return nil
}

// Claim submits a change to a volume claim
func (v *CSIVolume) Claim(args *structs.CSIVolumeClaimRequest, reply *structs.CSIVolumeClaimResponse) error {
	if done, err := v.srv.forward("CSIVolume.Claim", args, args, reply); done {
		return err
	}

	allowVolume := acl.NamespaceValidator(acl.NamespaceCapabilityCSIMountVolume)
	aclObj, err := v.srv.WriteACLObj(&args.WriteRequest, true)
	if err != nil {
		return err
	}

	metricsStart := time.Now()
	defer metrics.MeasureSince([]string{"nomad", "volume", "claim"}, metricsStart)

	if !allowVolume(aclObj, args.RequestNamespace()) || !aclObj.AllowPluginRead() {
		return structs.ErrPermissionDenied
	}

	if args.VolumeID == "" {
		return fmt.Errorf("missing volume ID")
	}

	isNewClaim := args.Claim != structs.CSIVolumeClaimGC &&
		args.State == structs.CSIVolumeClaimStateTaken

	// COMPAT(1.0): the NodeID field was added after 0.11.0 and so we
	// need to ensure it's been populated during upgrades from 0.11.0
	// to later patch versions. Remove this block in 1.0
	if isNewClaim && args.NodeID == "" {
		state := v.srv.fsm.State()
		ws := memdb.NewWatchSet()
		alloc, err := state.AllocByID(ws, args.AllocationID)
		if err != nil {
			return err
		}
		if alloc == nil {
			return fmt.Errorf("%s: %s",
				structs.ErrUnknownAllocationPrefix, args.AllocationID)
		}
		args.NodeID = alloc.NodeID
	}

	if isNewClaim {
		// if this is a new claim, add a Volume and PublishContext from the
		// controller (if any) to the reply
		err = v.controllerPublishVolume(args, reply)
		if err != nil {
			return fmt.Errorf("controller publish: %v", err)
		}
	}

	resp, index, err := v.srv.raftApply(structs.CSIVolumeClaimRequestType, args)
	if err != nil {
		v.logger.Error("csi raft apply failed", "error", err, "method", "claim")
		return err
	}
	if respErr, ok := resp.(error); ok {
		return respErr
	}

	reply.Index = index
	v.srv.setQueryMeta(&reply.QueryMeta)
	return nil
}

// controllerPublishVolume sends publish request to the CSI controller
// plugin associated with a volume, if any.
func (v *CSIVolume) controllerPublishVolume(req *structs.CSIVolumeClaimRequest, resp *structs.CSIVolumeClaimResponse) error {
	plug, vol, err := v.volAndPluginLookup(req.RequestNamespace(), req.VolumeID)
	if err != nil {
		return err
	}

	// Set the Response volume from the lookup
	resp.Volume = vol

	// Validate the existence of the allocation, regardless of whether we need it
	// now.
	state := v.srv.fsm.State()
	ws := memdb.NewWatchSet()
	alloc, err := state.AllocByID(ws, req.AllocationID)
	if err != nil {
		return err
	}
	if alloc == nil {
		return fmt.Errorf("%s: %s", structs.ErrUnknownAllocationPrefix, req.AllocationID)
	}

	// if no plugin was returned then controller validation is not required.
	// Here we can return nil.
	if plug == nil {
		return nil
	}

	// get Nomad's ID for the client node (not the storage provider's ID)
	targetNode, err := state.NodeByID(ws, alloc.NodeID)
	if err != nil {
		return err
	}
	if targetNode == nil {
		return fmt.Errorf("%s: %s", structs.ErrUnknownNodePrefix, alloc.NodeID)
	}

	// get the the storage provider's ID for the client node (not
	// Nomad's ID for the node)
	targetCSIInfo, ok := targetNode.CSINodePlugins[plug.ID]
	if !ok {
		return fmt.Errorf("failed to find storage provider info for client %q, node plugin %q is not running or has not fingerprinted on this client", targetNode.ID, plug.ID)
	}
	externalNodeID := targetCSIInfo.NodeInfo.ID
	req.ExternalNodeID = externalNodeID // update with the target info

	method := "ClientCSI.ControllerAttachVolume"
	cReq := &cstructs.ClientCSIControllerAttachVolumeRequest{
		VolumeID:        vol.RemoteID(),
		ClientCSINodeID: externalNodeID,
		AttachmentMode:  vol.AttachmentMode,
		AccessMode:      vol.AccessMode,
		ReadOnly:        req.Claim == structs.CSIVolumeClaimRead,
		Secrets:         vol.Secrets,
		VolumeContext:   vol.Context,
	}
	cReq.PluginID = plug.ID
	cResp := &cstructs.ClientCSIControllerAttachVolumeResponse{}

	err = v.srv.RPC(method, cReq, cResp)
	if err != nil {
		return fmt.Errorf("attach volume: %v", err)
	}
	resp.PublishContext = cResp.PublishContext
	return nil
}

func (v *CSIVolume) volAndPluginLookup(namespace, volID string) (*structs.CSIPlugin, *structs.CSIVolume, error) {
	state := v.srv.fsm.State()
	vol, err := state.CSIVolumeByID(nil, namespace, volID)
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
	plug, err := state.CSIPluginByID(nil, vol.PluginID)
	if err != nil {
		return nil, nil, err
	}
	if plug == nil {
		return nil, nil, fmt.Errorf("plugin not found: %s", vol.PluginID)
	}
	return plug, vol, nil
}

// allowCSIMount is called on Job register to check mount permission
func allowCSIMount(aclObj *acl.ACL, namespace string) bool {
	return aclObj.AllowPluginRead() &&
		aclObj.AllowNsOp(namespace, acl.NamespaceCapabilityCSIMountVolume)
}

// Unpublish synchronously sends the NodeUnpublish, NodeUnstage, and
// ControllerUnpublish RPCs to the client. It handles errors according to the
// current claim state.
func (v *CSIVolume) Unpublish(args *structs.CSIVolumeUnpublishRequest, reply *structs.CSIVolumeUnpublishResponse) error {
	if done, err := v.srv.forward("CSIVolume.Unpublish", args, args, reply); done {
		return err
	}

	metricsStart := time.Now()
	defer metrics.MeasureSince([]string{"nomad", "volume", "unpublish"}, metricsStart)

	allowVolume := acl.NamespaceValidator(acl.NamespaceCapabilityCSIMountVolume)
	aclObj, err := v.srv.WriteACLObj(&args.WriteRequest, true)
	if err != nil {
		return err
	}
	if !allowVolume(aclObj, args.RequestNamespace()) || !aclObj.AllowPluginRead() {
		return structs.ErrPermissionDenied
	}

	if args.VolumeID == "" {
		return fmt.Errorf("missing volume ID")
	}
	if args.Claim == nil {
		return fmt.Errorf("missing volume claim")
	}

	ws := memdb.NewWatchSet()
	state := v.srv.fsm.State()
	vol, err := state.CSIVolumeByID(ws, args.Namespace, args.VolumeID)
	if err != nil {
		return err
	}
	if vol == nil {
		return fmt.Errorf("no such volume")
	}

	claim := args.Claim

	// previous checkpoints may have set the past claim state already.
	// in practice we should never see CSIVolumeClaimStateControllerDetached
	// but having an option for the state makes it easy to add a checkpoint
	// in a backwards compatible way if we need one later
	switch claim.State {
	case structs.CSIVolumeClaimStateNodeDetached:
		goto NODE_DETACHED
	case structs.CSIVolumeClaimStateControllerDetached:
		goto RELEASE_CLAIM
	case structs.CSIVolumeClaimStateReadyToFree:
		goto RELEASE_CLAIM
	}
	err = v.nodeUnpublishVolume(vol, claim)
	if err != nil {
		return err
	}

NODE_DETACHED:
	err = v.controllerUnpublishVolume(vol, claim)
	if err != nil {
		return err
	}

RELEASE_CLAIM:
	// advance a CSIVolumeClaimStateControllerDetached claim
	claim.State = structs.CSIVolumeClaimStateReadyToFree
	err = v.checkpointClaim(vol, claim)
	if err != nil {
		return err
	}

	reply.Index = vol.ModifyIndex
	v.srv.setQueryMeta(&reply.QueryMeta)
	return nil
}

func (v *CSIVolume) nodeUnpublishVolume(vol *structs.CSIVolume, claim *structs.CSIVolumeClaim) error {
	if claim.AllocationID != "" {
		err := v.nodeUnpublishVolumeImpl(vol, claim)
		if err != nil {
			return err
		}
		claim.State = structs.CSIVolumeClaimStateNodeDetached
		return v.checkpointClaim(vol, claim)
	}

	// The RPC sent from the 'nomad node detach' command won't have an
	// allocation ID set so we try to unpublish every terminal or invalid
	// alloc on the node
	allocIDs := []string{}
	state := v.srv.fsm.State()
	vol, err := state.CSIVolumeDenormalize(memdb.NewWatchSet(), vol)
	if err != nil {
		return err
	}
	for allocID, alloc := range vol.ReadAllocs {
		if alloc == nil {
			rclaim, ok := vol.ReadClaims[allocID]
			if ok && rclaim.NodeID == claim.NodeID {
				allocIDs = append(allocIDs, allocID)
			}
		} else if alloc.NodeID == claim.NodeID && alloc.TerminalStatus() {
			allocIDs = append(allocIDs, allocID)
		}
	}
	for allocID, alloc := range vol.WriteAllocs {
		if alloc == nil {
			wclaim, ok := vol.WriteClaims[allocID]
			if ok && wclaim.NodeID == claim.NodeID {
				allocIDs = append(allocIDs, allocID)
			}
		} else if alloc.NodeID == claim.NodeID && alloc.TerminalStatus() {
			allocIDs = append(allocIDs, allocID)
		}
	}
	var merr multierror.Error
	for _, allocID := range allocIDs {
		claim.AllocationID = allocID
		err := v.nodeUnpublishVolumeImpl(vol, claim)
		if err != nil {
			merr.Errors = append(merr.Errors, err)
		}
	}
	err = merr.ErrorOrNil()
	if err != nil {
		return err
	}

	claim.State = structs.CSIVolumeClaimStateNodeDetached
	return v.checkpointClaim(vol, claim)
}

func (v *CSIVolume) nodeUnpublishVolumeImpl(vol *structs.CSIVolume, claim *structs.CSIVolumeClaim) error {
	req := &cstructs.ClientCSINodeDetachVolumeRequest{
		PluginID:       vol.PluginID,
		VolumeID:       vol.ID,
		ExternalID:     vol.RemoteID(),
		AllocID:        claim.AllocationID,
		NodeID:         claim.NodeID,
		AttachmentMode: vol.AttachmentMode,
		AccessMode:     vol.AccessMode,
		ReadOnly:       claim.Mode == structs.CSIVolumeClaimRead,
	}
	err := v.srv.RPC("ClientCSI.NodeDetachVolume",
		req, &cstructs.ClientCSINodeDetachVolumeResponse{})
	if err != nil {
		// we should only get this error if the Nomad node disconnects and
		// is garbage-collected, so at this point we don't have any reason
		// to operate as though the volume is attached to it.
		if !errors.Is(err, structs.ErrUnknownNode) {
			return fmt.Errorf("could not detach from node: %w", err)
		}
	}
	return nil
}

func (v *CSIVolume) controllerUnpublishVolume(vol *structs.CSIVolume, claim *structs.CSIVolumeClaim) error {

	if !vol.ControllerRequired {
		claim.State = structs.CSIVolumeClaimStateReadyToFree
		return nil
	}

	// we only send a controller detach if a Nomad client no longer has
	// any claim to the volume, so we need to check the status of claimed
	// allocations
	state := v.srv.fsm.State()
	vol, err := state.CSIVolumeDenormalize(memdb.NewWatchSet(), vol)
	if err != nil {
		return err
	}
	for _, alloc := range vol.ReadAllocs {
		if alloc != nil && alloc.NodeID == claim.NodeID && !alloc.TerminalStatus() {
			claim.State = structs.CSIVolumeClaimStateReadyToFree
			return nil
		}
	}
	for _, alloc := range vol.WriteAllocs {
		if alloc != nil && alloc.NodeID == claim.NodeID && !alloc.TerminalStatus() {
			claim.State = structs.CSIVolumeClaimStateReadyToFree
			return nil
		}
	}

	// if the RPC is sent by a client node, it doesn't know the claim's
	// external node ID.
	if claim.ExternalNodeID == "" {
		externalNodeID, err := v.lookupExternalNodeID(vol, claim)
		if err != nil {
			return fmt.Errorf("missing external node ID: %v", err)
		}
		claim.ExternalNodeID = externalNodeID
	}

	req := &cstructs.ClientCSIControllerDetachVolumeRequest{
		VolumeID:        vol.RemoteID(),
		ClientCSINodeID: claim.ExternalNodeID,
		Secrets:         vol.Secrets,
	}
	req.PluginID = vol.PluginID
	err = v.srv.RPC("ClientCSI.ControllerDetachVolume", req,
		&cstructs.ClientCSIControllerDetachVolumeResponse{})
	if err != nil {
		return fmt.Errorf("could not detach from controller: %v", err)
	}
	claim.State = structs.CSIVolumeClaimStateReadyToFree
	return v.checkpointClaim(vol, claim)
}

// lookupExternalNodeID gets the CSI plugin's ID for a node.  we look it up in
// the volume's claims first because it's possible the client has been stopped
// and GC'd by this point, so looking there is the last resort.
func (v *CSIVolume) lookupExternalNodeID(vol *structs.CSIVolume, claim *structs.CSIVolumeClaim) (string, error) {
	for _, rClaim := range vol.ReadClaims {
		if rClaim.NodeID == claim.NodeID {
			return rClaim.ExternalNodeID, nil
		}
	}
	for _, wClaim := range vol.WriteClaims {
		if wClaim.NodeID == claim.NodeID {
			return wClaim.ExternalNodeID, nil
		}
	}
	for _, pClaim := range vol.PastClaims {
		if pClaim.NodeID == claim.NodeID {
			return pClaim.ExternalNodeID, nil
		}
	}

	// fallback to looking up the node plugin
	ws := memdb.NewWatchSet()
	state := v.srv.fsm.State()
	targetNode, err := state.NodeByID(ws, claim.NodeID)
	if err != nil {
		return "", err
	}
	if targetNode == nil {
		return "", fmt.Errorf("%s: %s", structs.ErrUnknownNodePrefix, claim.NodeID)
	}

	// get the the storage provider's ID for the client node (not
	// Nomad's ID for the node)
	targetCSIInfo, ok := targetNode.CSINodePlugins[vol.PluginID]
	if !ok || targetCSIInfo.NodeInfo == nil {
		return "", fmt.Errorf("failed to find storage provider info for client %q, node plugin %q is not running or has not fingerprinted on this client", targetNode.ID, vol.PluginID)
	}
	return targetCSIInfo.NodeInfo.ID, nil
}

func (v *CSIVolume) checkpointClaim(vol *structs.CSIVolume, claim *structs.CSIVolumeClaim) error {
	v.logger.Trace("checkpointing claim")
	req := structs.CSIVolumeClaimRequest{
		VolumeID:     vol.ID,
		AllocationID: claim.AllocationID,
		NodeID:       claim.NodeID,
		Claim:        claim.Mode,
		State:        claim.State,
		WriteRequest: structs.WriteRequest{
			Namespace: vol.Namespace,
		},
	}
	resp, index, err := v.srv.raftApply(structs.CSIVolumeClaimRequestType, req)
	if err != nil {
		v.logger.Error("csi raft apply failed", "error", err)
		return err
	}
	if respErr, ok := resp.(error); ok {
		return respErr
	}
	vol.ModifyIndex = index
	return nil
}

// CSIPlugin wraps the structs.CSIPlugin with request data and server context
type CSIPlugin struct {
	srv    *Server
	logger log.Logger
}

// List replies with CSIPlugins, filtered by ACL access
func (v *CSIPlugin) List(args *structs.CSIPluginListRequest, reply *structs.CSIPluginListResponse) error {
	if done, err := v.srv.forward("CSIPlugin.List", args, args, reply); done {
		return err
	}

	aclObj, err := v.srv.QueryACLObj(&args.QueryOptions, false)
	if err != nil {
		return err
	}

	if !aclObj.AllowPluginList() {
		return structs.ErrPermissionDenied
	}

	metricsStart := time.Now()
	defer metrics.MeasureSince([]string{"nomad", "plugin", "list"}, metricsStart)

	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Query all plugins
			iter, err := state.CSIPlugins(ws)
			if err != nil {
				return err
			}

			// Collect results
			ps := []*structs.CSIPluginListStub{}
			for {
				raw := iter.Next()
				if raw == nil {
					break
				}

				plug := raw.(*structs.CSIPlugin)
				ps = append(ps, plug.Stub())
			}

			reply.Plugins = ps
			return v.srv.replySetIndex(csiPluginTable, &reply.QueryMeta)
		}}
	return v.srv.blockingRPC(&opts)
}

// Get fetches detailed information about a specific plugin
func (v *CSIPlugin) Get(args *structs.CSIPluginGetRequest, reply *structs.CSIPluginGetResponse) error {
	if done, err := v.srv.forward("CSIPlugin.Get", args, args, reply); done {
		return err
	}

	aclObj, err := v.srv.QueryACLObj(&args.QueryOptions, false)
	if err != nil {
		return err
	}

	if !aclObj.AllowPluginRead() {
		return structs.ErrPermissionDenied
	}

	withAllocs := aclObj == nil ||
		aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilityReadJob)

	metricsStart := time.Now()
	defer metrics.MeasureSince([]string{"nomad", "plugin", "get"}, metricsStart)

	if args.ID == "" {
		return fmt.Errorf("missing plugin ID")
	}

	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			snap, err := state.Snapshot()
			if err != nil {
				return err
			}

			plug, err := snap.CSIPluginByID(ws, args.ID)
			if err != nil {
				return err
			}

			if plug == nil {
				return nil
			}

			if withAllocs {
				plug, err = snap.CSIPluginDenormalize(ws, plug.Copy())
				if err != nil {
					return err
				}

				// Filter the allocation stubs by our namespace. withAllocs
				// means we're allowed
				var as []*structs.AllocListStub
				for _, a := range plug.Allocations {
					if a.Namespace == args.RequestNamespace() {
						as = append(as, a)
					}
				}
				plug.Allocations = as
			}

			reply.Plugin = plug
			return v.srv.replySetIndex(csiPluginTable, &reply.QueryMeta)
		}}
	return v.srv.blockingRPC(&opts)
}

// Delete deletes a plugin if it is unused
func (v *CSIPlugin) Delete(args *structs.CSIPluginDeleteRequest, reply *structs.CSIPluginDeleteResponse) error {
	if done, err := v.srv.forward("CSIPlugin.Delete", args, args, reply); done {
		return err
	}

	// Check that it is a management token.
	if aclObj, err := v.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.IsManagement() {
		return structs.ErrPermissionDenied
	}

	metricsStart := time.Now()
	defer metrics.MeasureSince([]string{"nomad", "plugin", "delete"}, metricsStart)

	if args.ID == "" {
		return fmt.Errorf("missing plugin ID")
	}

	resp, index, err := v.srv.raftApply(structs.CSIPluginDeleteRequestType, args)
	if err != nil {
		v.logger.Error("csi raft apply failed", "error", err, "method", "delete")
		return err
	}

	if respErr, ok := resp.(error); ok {
		return respErr
	}

	reply.Index = index
	v.srv.setQueryMeta(&reply.QueryMeta)
	return nil
}
