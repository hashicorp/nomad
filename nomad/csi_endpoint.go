package nomad

import (
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
			// Query all volumes
			var err error
			var iter memdb.ResultIterator

			if args.NodeID != "" {
				iter, err = state.CSIVolumesByNodeID(ws, args.NodeID)
			} else if args.PluginID != "" {
				iter, err = state.CSIVolumesByPluginID(ws, ns, args.PluginID)
			} else {
				iter, err = state.CSIVolumesByNamespace(ws, ns)
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
				vol, err := state.CSIVolumeDenormalizePlugins(ws, vol.Copy())
				if err != nil {
					return err
				}

				// Remove (possibly again) by PluginID to handle passing both NodeID and PluginID
				if args.PluginID != "" && args.PluginID != vol.PluginID {
					continue
				}

				// Remove by Namespace, since CSIVolumesByNodeID hasn't used the Namespace yet
				if vol.Namespace != ns {
					continue
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

	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			vol, err := state.CSIVolumeByID(ws, ns, args.ID)
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
	nodeID, err := nodeForControllerPlugin(srv.fsm.State(), plugin)
	if err != nil || nodeID == "" {
		return err
	}

	method := "ClientCSI.ControllerValidateVolume"
	cReq := &cstructs.ClientCSIControllerValidateVolumeRequest{
		VolumeID:       vol.RemoteID(),
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

	// if this is a new claim, add a Volume and PublishContext from the
	// controller (if any) to the reply
	if args.Claim != structs.CSIVolumeClaimRelease {
		err = v.srv.controllerPublishVolume(args, reply)
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

// allowCSIMount is called on Job register to check mount permission
func allowCSIMount(aclObj *acl.ACL, namespace string) bool {
	return aclObj.AllowPluginRead() &&
		aclObj.AllowNsOp(namespace, acl.NamespaceCapabilityCSIMountVolume)
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

	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			plug, err := state.CSIPluginByID(ws, args.ID)
			if err != nil {
				return err
			}

			if plug == nil {
				return nil
			}

			if withAllocs {
				plug, err = state.CSIPluginDenormalize(ws, plug.Copy())
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

// controllerPublishVolume sends publish request to the CSI controller
// plugin associated with a volume, if any.
func (srv *Server) controllerPublishVolume(req *structs.CSIVolumeClaimRequest, resp *structs.CSIVolumeClaimResponse) error {
	plug, vol, err := srv.volAndPluginLookup(req.RequestNamespace(), req.VolumeID)
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
	nodeID, err := nodeForControllerPlugin(state, plug)
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

	method := "ClientCSI.ControllerAttachVolume"
	cReq := &cstructs.ClientCSIControllerAttachVolumeRequest{
		VolumeID:        vol.RemoteID(),
		ClientCSINodeID: targetCSIInfo.NodeInfo.ID,
		AttachmentMode:  vol.AttachmentMode,
		AccessMode:      vol.AccessMode,
		ReadOnly:        req.Claim == structs.CSIVolumeClaimRead,
	}
	cReq.PluginID = plug.ID
	cReq.ControllerNodeID = nodeID
	cResp := &cstructs.ClientCSIControllerAttachVolumeResponse{}

	err = srv.RPC(method, cReq, cResp)
	if err != nil {
		return fmt.Errorf("attach volume: %v", err)
	}
	resp.PublishContext = cResp.PublishContext
	return nil
}
