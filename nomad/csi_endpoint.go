package nomad

import (
	"fmt"
	"math/rand"
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

	allowCSIAccess := acl.NamespaceValidator(acl.NamespaceCapabilityCSIAccess)
	aclObj, err := v.srv.QueryACLObj(&args.QueryOptions, false)
	if err != nil {
		return err
	}

	metricsStart := time.Now()
	defer metrics.MeasureSince([]string{"nomad", "volume", "list"}, metricsStart)

	ns := args.RequestNamespace()
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Query all volumes
			var err error
			var iter memdb.ResultIterator

			if args.PluginID != "" {
				iter, err = state.CSIVolumesByPluginID(ws, args.PluginID)
			} else {
				iter, err = state.CSIVolumes(ws)
			}

			if err != nil {
				return err
			}

			// Collect results, filter by ACL access
			var vs []*structs.CSIVolListStub
			cache := map[string]bool{}

			for {
				raw := iter.Next()
				if raw == nil {
					break
				}

				vol := raw.(*structs.CSIVolume)
				vol, err := state.CSIVolumeDenormalizePlugins(ws, vol)
				if err != nil {
					return err
				}

				// Filter on the request namespace to avoid ACL checks by volume
				if ns != "" && vol.Namespace != args.RequestNamespace() {
					continue
				}

				// Cache ACL checks QUESTION: are they expensive?
				allowed, ok := cache[vol.Namespace]
				if !ok {
					allowed = allowCSIAccess(aclObj, vol.Namespace)
					cache[vol.Namespace] = allowed
				}

				if allowed {
					vs = append(vs, vol.Stub())
				}
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

	allowCSIAccess := acl.NamespaceValidator(acl.NamespaceCapabilityCSIAccess)
	aclObj, err := v.srv.QueryACLObj(&args.QueryOptions, true)
	if err != nil {
		return err
	}

	if !allowCSIAccess(aclObj, args.RequestNamespace()) {
		return structs.ErrPermissionDenied
	}

	metricsStart := time.Now()
	defer metrics.MeasureSince([]string{"nomad", "volume", "get"}, metricsStart)

	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			vol, err := state.CSIVolumeByID(ws, args.ID)
			if err != nil {
				return err
			}
			if vol != nil {
				vol, err = state.CSIVolumeDenormalize(ws, vol)
			}
			if err != nil {
				return err
			}

			reply.Volume = vol
			return v.srv.replySetIndex(csiVolumeTable, &reply.QueryMeta)
		}}
	return v.srv.blockingRPC(&opts)
}

func (srv *Server) pluginValidateVolume(req *structs.CSIVolumeRegisterRequest, vol *structs.CSIVolume) (*structs.CSIPlugin, error) {
	state := srv.fsm.State()
	ws := memdb.NewWatchSet()

	plugin, err := state.CSIPluginByID(ws, vol.PluginID)
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

func (srv *Server) controllerValidateVolume(req *structs.CSIVolumeRegisterRequest, vol *structs.CSIVolume, plugin *structs.CSIPlugin) error {

	if !plugin.ControllerRequired {
		// The plugin does not require a controller, so for now we won't do any
		// further validation of the volume.
		return nil
	}

	// The plugin requires a controller. Now we do some validation of the Volume
	// to ensure that the registered capabilities are valid and that the volume
	// exists.

	// plugin IDs are not scoped to region/DC but volumes are.
	// so any node we get for a controller is already in the same region/DC
	// for the volume.
	nodeID, err := srv.nodeForControllerPlugin(plugin)
	if err != nil || nodeID == "" {
		return err
	}

	method := "ClientCSIController.ValidateVolume"
	cReq := &cstructs.ClientCSIControllerValidateVolumeRequest{
		VolumeID:       vol.ID,
		AttachmentMode: vol.AttachmentMode,
		AccessMode:     vol.AccessMode,
	}
	cReq.PluginID = plugin.ID
	cReq.ControllerNodeID = nodeID
	cResp := &cstructs.ClientCSIControllerValidateVolumeResponse{}

	return srv.RPC(method, cReq, cResp)
}

// Register registers a new volume
func (v *CSIVolume) Register(args *structs.CSIVolumeRegisterRequest, reply *structs.CSIVolumeRegisterResponse) error {
	if done, err := v.srv.forward("CSIVolume.Register", args, args, reply); done {
		return err
	}

	allowCSIVolumeManagement := acl.NamespaceValidator(acl.NamespaceCapabilityCSICreateVolume)
	aclObj, err := v.srv.WriteACLObj(&args.WriteRequest, false)
	if err != nil {
		return err
	}

	metricsStart := time.Now()
	defer metrics.MeasureSince([]string{"nomad", "volume", "register"}, metricsStart)

	if !allowCSIVolumeManagement(aclObj, args.RequestNamespace()) {
		return structs.ErrPermissionDenied
	}

	// This is the only namespace we ACL checked, force all the volumes to use it.
	// We also validate that the plugin exists for each plugin, and validate the
	// capabilities when the plugin has a controller.
	for _, vol := range args.Volumes {
		vol.Namespace = args.RequestNamespace()
		if err = vol.Validate(); err != nil {
			return err
		}
		plugin, err := v.srv.pluginValidateVolume(args, vol)
		if err != nil {
			return err
		}
		if err := v.srv.controllerValidateVolume(args, vol, plugin); err != nil {
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

	allowCSIVolumeManagement := acl.NamespaceValidator(acl.NamespaceCapabilityCSICreateVolume)
	aclObj, err := v.srv.WriteACLObj(&args.WriteRequest, false)
	if err != nil {
		return err
	}

	metricsStart := time.Now()
	defer metrics.MeasureSince([]string{"nomad", "volume", "deregister"}, metricsStart)

	ns := args.RequestNamespace()
	if !allowCSIVolumeManagement(aclObj, ns) {
		return structs.ErrPermissionDenied
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

// Claim claims a volume
func (v *CSIVolume) Claim(args *structs.CSIVolumeClaimRequest, reply *structs.CSIVolumeClaimResponse) error {
	if done, err := v.srv.forward("CSIVolume.Claim", args, args, reply); done {
		return err
	}

	allowCSIAccess := acl.NamespaceValidator(acl.NamespaceCapabilityCSIAccess)
	aclObj, err := v.srv.WriteACLObj(&args.WriteRequest, true)
	if err != nil {
		return err
	}

	metricsStart := time.Now()
	defer metrics.MeasureSince([]string{"nomad", "volume", "claim"}, metricsStart)

	if !allowCSIAccess(aclObj, args.RequestNamespace()) {
		return structs.ErrPermissionDenied
	}

	// adds a Volume and PublishContext from the controller (if any) to the reply
	err = v.srv.controllerPublishVolume(args, reply)
	if err != nil {
		return err
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

	allowCSIAccess := acl.NamespaceValidator(acl.NamespaceCapabilityCSIAccess)
	aclObj, err := v.srv.QueryACLObj(&args.QueryOptions, false)
	if err != nil {
		return err
	}

	if !allowCSIAccess(aclObj, args.RequestNamespace()) {
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
			var ps []*structs.CSIPluginListStub
			for {
				raw := iter.Next()
				if raw == nil {
					break
				}

				plug := raw.(*structs.CSIPlugin)

				// FIXME we should filter the ACL access for the plugin's
				// namespace, but plugins don't currently have namespaces
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

	allowCSIAccess := acl.NamespaceValidator(acl.NamespaceCapabilityCSIAccess)
	aclObj, err := v.srv.QueryACLObj(&args.QueryOptions, false)
	if err != nil {
		return err
	}

	if !allowCSIAccess(aclObj, args.RequestNamespace()) {
		return structs.ErrPermissionDenied
	}

	metricsStart := time.Now()
	defer metrics.MeasureSince([]string{"nomad", "plugin", "get"}, metricsStart)

	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			plug, err := state.CSIPluginByID(ws, args.ID)
			if err != nil {
				return err
			}

			if plug != nil {
				plug, err = state.CSIPluginDenormalize(ws, plug)
			}
			if err != nil {
				return err
			}

			// FIXME we should re-check the ACL access for the plugin's
			// namespace, but plugins don't currently have namespaces

			reply.Plugin = plug
			return v.srv.replySetIndex(csiPluginTable, &reply.QueryMeta)
		}}
	return v.srv.blockingRPC(&opts)
}

// controllerPublishVolume sends publish request to the CSI controller
// plugin associated with a volume, if any.
func (srv *Server) controllerPublishVolume(req *structs.CSIVolumeClaimRequest, resp *structs.CSIVolumeClaimResponse) error {
	plug, vol, err := srv.volAndPluginLookup(req.VolumeID)
	if err != nil {
		return err
	}

	// Set the Response volume from the lookup
	resp.Volume = vol

	// Validate the existence of the allocation, regardless of whether we need it
	// now.
	state := srv.fsm.State()
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

	// plugin IDs are not scoped to region/DC but volumes are.
	// so any node we get for a controller is already in the same region/DC
	// for the volume.
	nodeID, err := srv.nodeForControllerPlugin(plug)
	if err != nil || nodeID == "" {
		return err
	}

	targetNode, err := state.NodeByID(ws, alloc.NodeID)
	if err != nil {
		return err
	}
	if targetNode == nil {
		return fmt.Errorf("%s: %s", structs.ErrUnknownNodePrefix, alloc.NodeID)
	}
	targetCSIInfo, ok := targetNode.CSINodePlugins[plug.ID]
	if !ok {
		return fmt.Errorf("Failed to find NodeInfo for node: %s", targetNode.ID)
	}

	method := "ClientCSIController.AttachVolume"
	cReq := &cstructs.ClientCSIControllerAttachVolumeRequest{
		VolumeID:        req.VolumeID,
		ClientCSINodeID: targetCSIInfo.NodeInfo.ID,
		AttachmentMode:  vol.AttachmentMode,
		AccessMode:      vol.AccessMode,
		ReadOnly:        req.Claim == structs.CSIVolumeClaimRead,
		// TODO(tgross): we don't have a way of setting these yet.
		// ref https://github.com/hashicorp/nomad/issues/7007
		// MountOptions:   vol.MountOptions,
	}
	cReq.PluginID = plug.ID
	cReq.ControllerNodeID = nodeID
	cResp := &cstructs.ClientCSIControllerAttachVolumeResponse{}

	err = srv.RPC(method, cReq, cResp)
	if err != nil {
		return err
	}
	resp.PublishContext = cResp.PublishContext
	return nil
}

// controllerUnpublishVolume sends an unpublish request to the CSI
// controller plugin associated with a volume, if any.
// TODO: the only caller of this won't have an alloc pointer handy, should it be its own request arg type?
func (srv *Server) controllerUnpublishVolume(req *structs.CSIVolumeClaimRequest, targetNomadNodeID string) error {
	plug, vol, err := srv.volAndPluginLookup(req.VolumeID)
	if plug == nil || vol == nil || err != nil {
		return err // possibly nil if no controller required
	}

	ws := memdb.NewWatchSet()
	state := srv.State()

	targetNode, err := state.NodeByID(ws, targetNomadNodeID)
	if err != nil {
		return err
	}
	if targetNode == nil {
		return fmt.Errorf("%s: %s", structs.ErrUnknownNodePrefix, targetNomadNodeID)
	}
	targetCSIInfo, ok := targetNode.CSINodePlugins[plug.ID]
	if !ok {
		return fmt.Errorf("Failed to find NodeInfo for node: %s", targetNode.ID)
	}

	// plugin IDs are not scoped to region/DC but volumes are.
	// so any node we get for a controller is already in the same region/DC
	// for the volume.
	nodeID, err := srv.nodeForControllerPlugin(plug)
	if err != nil || nodeID == "" {
		return err
	}

	method := "ClientCSIController.DetachVolume"
	cReq := &cstructs.ClientCSIControllerDetachVolumeRequest{
		VolumeID:        req.VolumeID,
		ClientCSINodeID: targetCSIInfo.NodeInfo.ID,
	}
	cReq.PluginID = plug.ID
	cReq.ControllerNodeID = nodeID
	return srv.RPC(method, cReq, &cstructs.ClientCSIControllerDetachVolumeResponse{})
}

func (srv *Server) volAndPluginLookup(volID string) (*structs.CSIPlugin, *structs.CSIVolume, error) {
	state := srv.fsm.State()
	ws := memdb.NewWatchSet()

	vol, err := state.CSIVolumeByID(ws, volID)
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
func (srv *Server) nodeForControllerPlugin(plugin *structs.CSIPlugin) (string, error) {
	count := len(plugin.Controllers)
	if count == 0 {
		return "", fmt.Errorf("no controllers available for plugin %q", plugin.ID)
	}
	snap, err := srv.fsm.State().Snapshot()
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
