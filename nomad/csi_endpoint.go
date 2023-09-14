// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/armon/go-metrics"
	"github.com/dustin/go-humanize"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/nomad/acl"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/state/paginator"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/csi"
)

// CSIVolume wraps the structs.CSIVolume with request data and server context
type CSIVolume struct {
	srv    *Server
	ctx    *RPCContext
	logger hclog.Logger
}

func NewCSIVolumeEndpoint(srv *Server, ctx *RPCContext) *CSIVolume {
	return &CSIVolume{srv: srv, ctx: ctx, logger: srv.logger.Named("csi_volume")}
}

const (
	csiVolumeTable = "csi_volumes"
	csiPluginTable = "csi_plugins"
)

// replySetIndex sets the reply with the last index that modified the table
func (s *Server) replySetIndex(table string, reply *structs.QueryMeta) error {
	fmsState := s.fsm.State()

	index, err := fmsState.Index(table)
	if err != nil {
		return err
	}
	reply.Index = index

	// Set the query response
	s.setQueryMeta(reply)
	return nil
}

// List replies with CSIVolumes, filtered by ACL access
func (v *CSIVolume) List(args *structs.CSIVolumeListRequest, reply *structs.CSIVolumeListResponse) error {

	authErr := v.srv.Authenticate(v.ctx, args)
	if done, err := v.srv.forward("CSIVolume.List", args, args, reply); done {
		return err
	}
	v.srv.MeasureRPCRate("csi_volume", structs.RateMetricList, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	allowVolume := acl.NamespaceValidator(acl.NamespaceCapabilityCSIListVolume,
		acl.NamespaceCapabilityCSIReadVolume,
		acl.NamespaceCapabilityCSIMountVolume,
		acl.NamespaceCapabilityListJobs)
	aclObj, err := v.srv.ResolveACL(args)
	if err != nil {
		return err
	}

	if !allowVolume(aclObj, args.RequestNamespace()) {
		return structs.ErrPermissionDenied
	}

	defer metrics.MeasureSince([]string{"nomad", "volume", "list"}, time.Now())

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
			} else if prefix != "" {
				iter, err = snap.CSIVolumesByIDPrefix(ws, ns, prefix)
			} else if ns != structs.AllNamespacesSentinel {
				iter, err = snap.CSIVolumesByNamespace(ws, ns, prefix)
			} else {
				iter, err = snap.CSIVolumes(ws)
			}
			if err != nil {
				return err
			}

			tokenizer := paginator.NewStructsTokenizer(
				iter,
				paginator.StructsTokenizerOptions{
					WithNamespace: true,
					WithID:        true,
				},
			)
			volFilter := paginator.GenericFilter{
				Allow: func(raw interface{}) (bool, error) {
					vol := raw.(*structs.CSIVolume)

					// Remove (possibly again) by PluginID to handle passing both
					// NodeID and PluginID
					if args.PluginID != "" && args.PluginID != vol.PluginID {
						return false, nil
					}

					// Remove by Namespace, since CSIVolumesByNodeID hasn't used
					// the Namespace yet
					if ns != structs.AllNamespacesSentinel && vol.Namespace != ns {
						return false, nil
					}

					return true, nil
				},
			}
			filters := []paginator.Filter{volFilter}

			// Collect results, filter by ACL access
			vs := []*structs.CSIVolListStub{}

			paginator, err := paginator.NewPaginator(iter, tokenizer, filters, args.QueryOptions,
				func(raw interface{}) error {
					vol := raw.(*structs.CSIVolume)

					vol, err := snap.CSIVolumeDenormalizePlugins(ws, vol.Copy())
					if err != nil {
						return err
					}

					vs = append(vs, vol.Stub())
					return nil
				})
			if err != nil {
				return structs.NewErrRPCCodedf(
					http.StatusBadRequest, "failed to create result paginator: %v", err)
			}

			nextToken, err := paginator.Page()
			if err != nil {
				return structs.NewErrRPCCodedf(
					http.StatusBadRequest, "failed to read result page: %v", err)
			}

			reply.QueryMeta.NextToken = nextToken
			reply.Volumes = vs
			return v.srv.replySetIndex(csiVolumeTable, &reply.QueryMeta)
		}}
	return v.srv.blockingRPC(&opts)
}

// Get fetches detailed information about a specific volume
func (v *CSIVolume) Get(args *structs.CSIVolumeGetRequest, reply *structs.CSIVolumeGetResponse) error {

	authErr := v.srv.Authenticate(v.ctx, args)
	if done, err := v.srv.forward("CSIVolume.Get", args, args, reply); done {
		return err
	}
	v.srv.MeasureRPCRate("csi_volume", structs.RateMetricRead, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	allowCSIAccess := acl.NamespaceValidator(acl.NamespaceCapabilityCSIReadVolume,
		acl.NamespaceCapabilityCSIMountVolume,
		acl.NamespaceCapabilityReadJob)
	aclObj, err := v.srv.ResolveClientOrACL(args)
	if err != nil {
		return err
	}

	ns := args.RequestNamespace()
	if !allowCSIAccess(aclObj, ns) {
		return structs.ErrPermissionDenied
	}

	defer metrics.MeasureSince([]string{"nomad", "volume", "get"}, time.Now())

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

func (v *CSIVolume) pluginValidateVolume(vol *structs.CSIVolume) (*structs.CSIPlugin, error) {
	state := v.srv.fsm.State()

	plugin, err := state.CSIPluginByID(nil, vol.PluginID)
	if err != nil {
		return nil, err
	}
	if plugin == nil {
		return nil, fmt.Errorf("no CSI plugin named: %s could be found", vol.PluginID)
	}

	if plugin.ControllerRequired && plugin.ControllersHealthy < 1 {
		return nil, fmt.Errorf("no healthy controllers for CSI plugin: %s", vol.PluginID)
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
		VolumeID:           vol.RemoteID(),
		VolumeCapabilities: vol.RequestedCapabilities,
		Secrets:            vol.Secrets,
		Parameters:         vol.Parameters,
		Context:            vol.Context,
	}
	cReq.PluginID = plugin.ID
	cResp := &cstructs.ClientCSIControllerValidateVolumeResponse{}

	return v.srv.RPC(method, cReq, cResp)
}

// Register registers a new volume or updates an existing volume.
//
// Note that most user-defined CSIVolume fields are immutable once
// the volume has been created, but exceptions include min and max
// requested capacity values.
//
// If the user needs to change fields because they've misconfigured
// the registration of the external volume, we expect that claims
// won't work either, and the user can deregister the volume and try
// again with the right settings. This lets us be as strict with
// validation here as the CreateVolume CSI RPC is expected to be.
func (v *CSIVolume) Register(args *structs.CSIVolumeRegisterRequest, reply *structs.CSIVolumeRegisterResponse) error {

	authErr := v.srv.Authenticate(v.ctx, args)
	if done, err := v.srv.forward("CSIVolume.Register", args, args, reply); done {
		return err
	}
	v.srv.MeasureRPCRate("csi_volume", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	allowVolume := acl.NamespaceValidator(acl.NamespaceCapabilityCSIWriteVolume)
	aclObj, err := v.srv.ResolveACL(args)
	if err != nil {
		return err
	}

	defer metrics.MeasureSince([]string{"nomad", "volume", "register"}, time.Now())

	if !allowVolume(aclObj, args.RequestNamespace()) || !aclObj.AllowPluginRead() {
		return structs.ErrPermissionDenied
	}

	if len(args.Volumes) == 0 {
		return fmt.Errorf("missing volume definition")
	}

	// This is the only namespace we ACL checked, force all the volumes to use it.
	// We also validate that the plugin exists for each plugin, and validate the
	// capabilities when the plugin has a controller.
	for _, vol := range args.Volumes {

		snap, err := v.srv.State().Snapshot()
		if err != nil {
			return err
		}
		if vol.Namespace == "" {
			vol.Namespace = args.RequestNamespace()
		}
		if err = vol.Validate(); err != nil {
			return err
		}

		ws := memdb.NewWatchSet()
		existingVol, err := snap.CSIVolumeByID(ws, vol.Namespace, vol.ID)
		if err != nil {
			return err
		}

		plugin, err := v.pluginValidateVolume(vol)
		if err != nil {
			return err
		}

		// CSIVolume has many user-defined fields which are immutable
		// once set, and many fields that are controlled by Nomad and
		// are not user-settable. We merge onto a copy of the existing
		// volume to allow a user to submit a volume spec for `volume
		// create` and reuse it for updates in `volume register`
		// without having to manually remove the fields unused by
		// register (and similar use cases with API consumers such as
		// Terraform).
		if existingVol != nil {
			existingVol = existingVol.Copy()

			// reconcile mutable fields
			if err = v.reconcileVolume(plugin, existingVol, vol); err != nil {
				return fmt.Errorf("unable to update volume: %s", err)
			}

			*vol = *existingVol

		} else if vol.Topologies == nil || len(vol.Topologies) == 0 {
			// The topologies for the volume have already been set
			// when it was created, so for newly register volumes
			// we accept the user's description of that topology
			if vol.RequestedTopologies != nil {
				vol.Topologies = vol.RequestedTopologies.Required
			}
		}

		if err := v.controllerValidateVolume(args, vol, plugin); err != nil {
			return err
		}
	}

	_, index, err := v.srv.raftApply(structs.CSIVolumeRegisterRequestType, args)
	if err != nil {
		v.logger.Error("csi raft apply failed", "error", err, "method", "register")
		return err
	}

	reply.Index = index
	v.srv.setQueryMeta(&reply.QueryMeta)
	return nil
}

// reconcileVolume updates a volume with many of the contents of another.
// It may or may not do extra work to actually expand a volume outside of Nomad,
// depending on whether requested capacity values have changed.
func (v *CSIVolume) reconcileVolume(plugin *structs.CSIPlugin, vol *structs.CSIVolume, update *structs.CSIVolume) error {
	// Merge does some validation, before we attempt any potential CSI RPCs,
	// and mutates `vol` with (most of) the values of `update`,
	// notably excluding capacity values, which are covered below.
	err := vol.Merge(update)
	if err != nil {
		return err
	}
	// expandVolume will mutate `vol` with new capacity-related values, if needed.
	return v.expandVolume(vol, plugin, &csi.CapacityRange{
		RequiredBytes: update.RequestedCapacityMin,
		LimitBytes:    update.RequestedCapacityMax,
	})
}

// Deregister removes a set of volumes
func (v *CSIVolume) Deregister(args *structs.CSIVolumeDeregisterRequest, reply *structs.CSIVolumeDeregisterResponse) error {

	authErr := v.srv.Authenticate(v.ctx, args)
	if done, err := v.srv.forward("CSIVolume.Deregister", args, args, reply); done {
		return err
	}
	v.srv.MeasureRPCRate("csi_volume", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	allowVolume := acl.NamespaceValidator(acl.NamespaceCapabilityCSIWriteVolume)
	aclObj, err := v.srv.ResolveACL(args)
	if err != nil {
		return err
	}

	defer metrics.MeasureSince([]string{"nomad", "volume", "deregister"}, time.Now())

	ns := args.RequestNamespace()
	if !allowVolume(aclObj, ns) {
		return structs.ErrPermissionDenied
	}

	if len(args.VolumeIDs) == 0 {
		return fmt.Errorf("missing volume IDs")
	}

	_, index, err := v.srv.raftApply(structs.CSIVolumeDeregisterRequestType, args)
	if err != nil {
		v.logger.Error("csi raft apply failed", "error", err, "method", "deregister")
		return err
	}

	reply.Index = index
	v.srv.setQueryMeta(&reply.QueryMeta)
	return nil
}

// Claim submits a change to a volume claim
func (v *CSIVolume) Claim(args *structs.CSIVolumeClaimRequest, reply *structs.CSIVolumeClaimResponse) error {

	authErr := v.srv.Authenticate(v.ctx, args)
	if done, err := v.srv.forward("CSIVolume.Claim", args, args, reply); done {
		return err
	}
	v.srv.MeasureRPCRate("csi_volume", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	allowVolume := acl.NamespaceValidator(acl.NamespaceCapabilityCSIMountVolume)
	aclObj, err := v.srv.ResolveClientOrACL(args)
	if err != nil {
		return err
	}

	defer metrics.MeasureSince([]string{"nomad", "volume", "claim"}, time.Now())

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

	_, index, err := v.srv.raftApply(structs.CSIVolumeClaimRequestType, args)
	if err != nil {
		v.logger.Error("csi raft apply failed", "error", err, "method", "claim")
		return err
	}

	if isNewClaim {
		// if this is a new claim, add a Volume and PublishContext from the
		// controller (if any) to the reply
		err = v.controllerPublishVolume(args, reply)
		if err != nil {
			return fmt.Errorf("controller publish: %v", err)
		}
	}

	reply.Index = index
	v.srv.setQueryMeta(&reply.QueryMeta)
	return nil
}

func csiVolumeMountOptions(c *structs.CSIMountOptions) *cstructs.CSIVolumeMountOptions {
	if c == nil {
		return nil
	}

	return &cstructs.CSIVolumeMountOptions{
		Filesystem: c.FSType,
		MountFlags: c.MountFlags,
	}
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

	// Some plugins support controllers for create/snapshot but not attach. So
	// if there's no plugin or the plugin doesn't attach volumes, then we can
	// skip the controller publish workflow and return nil.
	if plug == nil || !plug.HasControllerCapability(structs.CSIControllerSupportsAttachDetach) {
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

	// if the RPC is sent by a client node, it may not know the claim's
	// external node ID.
	if req.ExternalNodeID == "" {
		externalNodeID, err := v.lookupExternalNodeID(vol, req.ToClaim())
		if err != nil {
			return fmt.Errorf("missing external node ID: %v", err)
		}
		req.ExternalNodeID = externalNodeID
	}

	method := "ClientCSI.ControllerAttachVolume"
	cReq := &cstructs.ClientCSIControllerAttachVolumeRequest{
		VolumeID:        vol.RemoteID(),
		ClientCSINodeID: req.ExternalNodeID,
		AttachmentMode:  req.AttachmentMode,
		AccessMode:      req.AccessMode,
		MountOptions:    csiVolumeMountOptions(vol.MountOptions),
		ReadOnly:        req.Claim == structs.CSIVolumeClaimRead,
		Secrets:         vol.Secrets,
		VolumeContext:   vol.Context,
	}
	cReq.PluginID = plug.ID
	cResp := &cstructs.ClientCSIControllerAttachVolumeResponse{}

	err = v.serializedControllerRPC(plug.ID, func() error {
		return v.srv.RPC(method, cReq, cResp)
	})
	if err != nil {
		if strings.Contains(err.Error(), "FailedPrecondition") {
			return fmt.Errorf("%v: %v", structs.ErrCSIClientRPCRetryable, err)
		}
		return err
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

// serializedControllerRPC ensures we're only sending a single controller RPC to
// a given plugin if the RPC can cause conflicting state changes.
//
// The CSI specification says that we SHOULD send no more than one in-flight
// request per *volume* at a time, with an allowance for losing state
// (ex. leadership transitions) which the plugins SHOULD handle gracefully.
//
// In practice many CSI plugins rely on k8s-specific sidecars for serializing
// storage provider API calls globally (ex. concurrently attaching EBS volumes
// to an EC2 instance results in a race for device names). So we have to be much
// more conservative about concurrency in Nomad than the spec allows.
func (v *CSIVolume) serializedControllerRPC(pluginID string, fn func() error) error {

	for {
		v.srv.volumeControllerLock.Lock()
		future := v.srv.volumeControllerFutures[pluginID]
		if future == nil {
			future, futureDone := context.WithCancel(v.srv.shutdownCtx)
			v.srv.volumeControllerFutures[pluginID] = future
			v.srv.volumeControllerLock.Unlock()

			err := fn()

			// close the future while holding the lock and not in a defer so
			// that we can ensure we've cleared it from the map before allowing
			// anyone else to take the lock and write a new one
			v.srv.volumeControllerLock.Lock()
			futureDone()
			delete(v.srv.volumeControllerFutures, pluginID)
			v.srv.volumeControllerLock.Unlock()

			return err
		} else {
			v.srv.volumeControllerLock.Unlock()

			select {
			case <-future.Done():
				continue
			case <-v.srv.shutdownCh:
				// The csi_hook publish workflow on the client will retry if it
				// gets this error. On unpublish, we don't want to block client
				// shutdown so we give up on error. The new leader's
				// volumewatcher will iterate all the claims at startup to
				// detect this and mop up any claims in the NodeDetached state
				// (volume GC will run periodically as well)
				return structs.ErrNoLeader
			}
		}
	}
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

	authErr := v.srv.Authenticate(v.ctx, args)
	if done, err := v.srv.forward("CSIVolume.Unpublish", args, args, reply); done {
		return err
	}
	v.srv.MeasureRPCRate("csi_volume", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	defer metrics.MeasureSince([]string{"nomad", "volume", "unpublish"}, time.Now())

	allowVolume := acl.NamespaceValidator(acl.NamespaceCapabilityCSIMountVolume)
	aclObj, err := v.srv.ResolveClientOrACL(args)
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

	// we need to checkpoint when we first get the claim to ensure we've set the
	// initial "past claim" state, otherwise a client that unpublishes (skipping
	// the node unpublish b/c it's done that work) fail to get written if the
	// controller unpublish fails.
	vol = vol.Copy()
	err = v.checkpointClaim(vol, claim)
	if err != nil {
		return err
	}

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
	vol = vol.Copy()
	err = v.nodeUnpublishVolume(vol, claim)
	if err != nil {
		return err
	}

NODE_DETACHED:
	vol = vol.Copy()
	err = v.controllerUnpublishVolume(vol, claim)
	if err != nil {
		return err
	}

RELEASE_CLAIM:
	v.logger.Trace("releasing claim", "vol", vol.ID)
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

// nodeUnpublishVolume handles the sending RPCs to the Node plugin to unmount
// it. Typically this task is already completed on the client, but we need to
// have this here so that GC can re-send it in case of client-side
// problems. This function should only be called on a copy of the volume.
func (v *CSIVolume) nodeUnpublishVolume(vol *structs.CSIVolume, claim *structs.CSIVolumeClaim) error {
	v.logger.Trace("node unpublish", "vol", vol.ID)

	// We need a new snapshot after each checkpoint
	snap, err := v.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}

	// If the node has been GC'd or is down, we can't send it a node
	// unpublish. We need to assume the node has unpublished at its
	// end. If it hasn't, any controller unpublish will potentially
	// hang or error and need to be retried.
	if claim.NodeID != "" {
		node, err := snap.NodeByID(memdb.NewWatchSet(), claim.NodeID)
		if err != nil {
			return err
		}
		if node == nil || node.Status == structs.NodeStatusDown {
			v.logger.Debug("skipping node unpublish for down or GC'd node")
			claim.State = structs.CSIVolumeClaimStateNodeDetached
			return v.checkpointClaim(vol, claim)
		}
	}

	if claim.AllocationID != "" {
		err := v.nodeUnpublishVolumeImpl(vol, claim)
		if err != nil {
			return err
		}
		claim.State = structs.CSIVolumeClaimStateNodeDetached
		return v.checkpointClaim(vol, claim)
	}

	// The RPC sent from the 'nomad node detach' command or GC won't have an
	// allocation ID set so we try to unpublish every terminal or invalid
	// alloc on the node, all of which will be in PastClaims after denormalizing
	vol, err = snap.CSIVolumeDenormalize(memdb.NewWatchSet(), vol)
	if err != nil {
		return err
	}

	claimsToUnpublish := []*structs.CSIVolumeClaim{}
	for _, pastClaim := range vol.PastClaims {
		if claim.NodeID == pastClaim.NodeID {
			claimsToUnpublish = append(claimsToUnpublish, pastClaim)
		}
	}

	var merr multierror.Error
	for _, pastClaim := range claimsToUnpublish {
		err := v.nodeUnpublishVolumeImpl(vol, pastClaim)
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
	if claim.AccessMode == structs.CSIVolumeAccessModeUnknown {
		// claim has already been released client-side
		return nil
	}

	req := &cstructs.ClientCSINodeDetachVolumeRequest{
		PluginID:       vol.PluginID,
		VolumeID:       vol.ID,
		ExternalID:     vol.RemoteID(),
		AllocID:        claim.AllocationID,
		NodeID:         claim.NodeID,
		AttachmentMode: claim.AttachmentMode,
		AccessMode:     claim.AccessMode,
		ReadOnly:       claim.Mode == structs.CSIVolumeClaimRead,
	}
	err := v.srv.RPC("ClientCSI.NodeDetachVolume",
		req, &cstructs.ClientCSINodeDetachVolumeResponse{})
	if err != nil {
		// we should only get this error if the Nomad node disconnects and
		// is garbage-collected, so at this point we don't have any reason
		// to operate as though the volume is attached to it.
		// note: errors.Is cannot be used because the RPC call breaks
		// error wrapping.
		if !strings.Contains(err.Error(), structs.ErrUnknownNode.Error()) {
			return fmt.Errorf("could not detach from node: %w", err)
		}
	}
	return nil
}

// controllerUnpublishVolume handles the sending RPCs to the Controller plugin
// to unpublish the volume (detach it from its host). This function should only
// be called on a copy of the volume.
func (v *CSIVolume) controllerUnpublishVolume(vol *structs.CSIVolume, claim *structs.CSIVolumeClaim) error {
	v.logger.Trace("controller unpublish", "vol", vol.ID)

	if !vol.ControllerRequired {
		claim.State = structs.CSIVolumeClaimStateReadyToFree
		return nil
	}

	// We need a new snapshot after each checkpoint
	snap, err := v.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}

	ws := memdb.NewWatchSet()

	plugin, err := snap.CSIPluginByID(ws, vol.PluginID)
	if err != nil {
		return fmt.Errorf("could not query plugin: %v", err)
	} else if plugin == nil {
		return fmt.Errorf("no such plugin: %q", vol.PluginID)
	}

	if !plugin.HasControllerCapability(structs.CSIControllerSupportsAttachDetach) {
		claim.State = structs.CSIVolumeClaimStateReadyToFree
		return nil
	}

	vol, err = snap.CSIVolumeDenormalize(ws, vol)
	if err != nil {
		return err
	}

	// we only send a controller detach if a Nomad client no longer has any
	// claim to the volume, so we need to check the status of any other claimed
	// allocations
	shouldCancel := func(alloc *structs.Allocation) bool {
		if alloc != nil && alloc.ID != claim.AllocationID &&
			alloc.NodeID == claim.NodeID && !alloc.TerminalStatus() {
			claim.State = structs.CSIVolumeClaimStateReadyToFree
			v.logger.Debug(
				"controller unpublish canceled: another non-terminal alloc is on this node",
				"vol", vol.ID, "alloc", alloc.ID)
			return true
		}
		return false
	}

	for _, alloc := range vol.ReadAllocs {
		if shouldCancel(alloc) {
			return nil
		}
	}
	for _, alloc := range vol.WriteAllocs {
		if shouldCancel(alloc) {
			return nil
		}
	}

	// if the RPC is sent by a client node, it may not know the claim's
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

	err = v.serializedControllerRPC(vol.PluginID, func() error {
		return v.srv.RPC("ClientCSI.ControllerDetachVolume", req,
			&cstructs.ClientCSIControllerDetachVolumeResponse{})
	})
	if err != nil {
		return fmt.Errorf("could not detach from controller: %v", err)
	}

	v.logger.Trace("controller detach complete", "vol", vol.ID)
	claim.State = structs.CSIVolumeClaimStateReadyToFree
	return v.checkpointClaim(vol, claim)
}

// lookupExternalNodeID gets the CSI plugin's ID for a node.  we look it up in
// the volume's claims first because it's possible the client has been stopped
// and GC'd by this point, so looking there is the last resort.
func (v *CSIVolume) lookupExternalNodeID(vol *structs.CSIVolume, claim *structs.CSIVolumeClaim) (string, error) {
	for _, rClaim := range vol.ReadClaims {
		if rClaim.NodeID == claim.NodeID && rClaim.ExternalNodeID != "" {
			return rClaim.ExternalNodeID, nil
		}
	}
	for _, wClaim := range vol.WriteClaims {
		if wClaim.NodeID == claim.NodeID && wClaim.ExternalNodeID != "" {
			return wClaim.ExternalNodeID, nil
		}
	}
	for _, pClaim := range vol.PastClaims {
		if pClaim.NodeID == claim.NodeID && pClaim.ExternalNodeID != "" {
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
	_, index, err := v.srv.raftApply(structs.CSIVolumeClaimRequestType, req)
	if err != nil {
		v.logger.Error("csi raft apply failed", "error", err)
		return err
	}
	vol.ModifyIndex = index
	return nil
}

func (v *CSIVolume) Create(args *structs.CSIVolumeCreateRequest, reply *structs.CSIVolumeCreateResponse) error {

	authErr := v.srv.Authenticate(v.ctx, args)
	if done, err := v.srv.forward("CSIVolume.Create", args, args, reply); done {
		return err
	}
	v.srv.MeasureRPCRate("csi_volume", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "volume", "create"}, time.Now())

	allowVolume := acl.NamespaceValidator(acl.NamespaceCapabilityCSIWriteVolume)
	aclObj, err := v.srv.ResolveACL(args)
	if err != nil {
		return err
	}

	if !allowVolume(aclObj, args.RequestNamespace()) || !aclObj.AllowPluginRead() {
		return structs.ErrPermissionDenied
	}

	if len(args.Volumes) == 0 {
		return fmt.Errorf("missing volume definition")
	}

	regArgs := &structs.CSIVolumeRegisterRequest{WriteRequest: args.WriteRequest}

	type validated struct {
		vol    *structs.CSIVolume
		plugin *structs.CSIPlugin
		// if the volume already exists, we'll update it instead of creating.
		current *structs.CSIVolume
	}
	validatedVols := []validated{}

	// This is the only namespace we ACL checked, force all the volumes to use it.
	// We also validate that the plugin exists for each plugin, and validate the
	// capabilities when the plugin has a controller.
	for _, vol := range args.Volumes {
		if vol.Namespace == "" {
			vol.Namespace = args.RequestNamespace()
		}
		if err = vol.Validate(); err != nil {
			return err
		}
		plugin, err := v.pluginValidateVolume(vol)
		if err != nil {
			return err
		}
		if !plugin.ControllerRequired {
			return fmt.Errorf("plugin has no controller")
		}
		if !plugin.HasControllerCapability(structs.CSIControllerSupportsCreateDelete) {
			return fmt.Errorf("plugin does not support creating volumes")
		}

		// if the volume already exists, we'll update it instead
		snap, err := v.srv.State().Snapshot()
		if err != nil {
			return err
		}
		// current will be nil if it does not exist.
		current, err := snap.CSIVolumeByID(nil, vol.Namespace, vol.ID)
		if err != nil {
			return err
		}

		validatedVols = append(validatedVols,
			validated{vol, plugin, current})
	}

	// Attempt to create all the validated volumes and write only successfully
	// created volumes to raft. And we'll report errors for any failed volumes
	//
	// NOTE: creating the volume in the external storage provider can't be
	// made atomic with the registration, and creating the volume provides
	// values we want to write on the CSIVolume in raft anyways. For now
	// we'll block the RPC on the external storage provider so that we can
	// easily return meaningful errors to the user, but in the future we
	// should consider creating registering first and creating a "volume
	// eval" that can do the plugin RPCs async.

	var mErr multierror.Error
	var index uint64

	for _, valid := range validatedVols {
		if valid.current != nil {
			// reconcile mutable fields
			cp := valid.current.Copy()
			err = v.reconcileVolume(valid.plugin, cp, valid.vol)
			if err != nil {
				mErr.Errors = append(mErr.Errors, err)
			} else {
				// we merged valid.vol into cp, so update state with the copy
				regArgs.Volumes = append(regArgs.Volumes, cp)
			}

		} else {
			err = v.createVolume(valid.vol, valid.plugin)
			if err != nil {
				mErr.Errors = append(mErr.Errors, err)
			} else {
				regArgs.Volumes = append(regArgs.Volumes, valid.vol)
			}
		}
	}

	// If we created or updated volumes, apply them to raft.
	if len(regArgs.Volumes) > 0 {
		_, index, err = v.srv.raftApply(structs.CSIVolumeRegisterRequestType, regArgs)
		if err != nil {
			v.logger.Error("csi raft apply failed", "error", err, "method", "register")
			mErr.Errors = append(mErr.Errors, err)
		}
	}

	err = mErr.ErrorOrNil()
	if err != nil {
		return err
	}

	reply.Volumes = regArgs.Volumes
	reply.Index = index
	v.srv.setQueryMeta(&reply.QueryMeta)
	return nil
}

func (v *CSIVolume) createVolume(vol *structs.CSIVolume, plugin *structs.CSIPlugin) error {

	method := "ClientCSI.ControllerCreateVolume"
	cReq := &cstructs.ClientCSIControllerCreateVolumeRequest{
		Name:                vol.Name,
		VolumeCapabilities:  vol.RequestedCapabilities,
		MountOptions:        vol.MountOptions,
		Parameters:          vol.Parameters,
		Secrets:             vol.Secrets,
		CapacityMin:         vol.RequestedCapacityMin,
		CapacityMax:         vol.RequestedCapacityMax,
		SnapshotID:          vol.SnapshotID,
		CloneID:             vol.CloneID,
		RequestedTopologies: vol.RequestedTopologies,
	}
	cReq.PluginID = plugin.ID
	cResp := &cstructs.ClientCSIControllerCreateVolumeResponse{}
	err := v.srv.RPC(method, cReq, cResp)
	if err != nil {
		return err
	}

	vol.ExternalID = cResp.ExternalVolumeID
	vol.Capacity = cResp.CapacityBytes
	vol.Context = cResp.VolumeContext
	vol.Topologies = cResp.Topologies
	return nil
}

// expandVolume validates the requested capacity values and issues
// ControllerExpandVolume (and NodeExpandVolume, if needed) to the CSI plugin,
// via Nomad client RPC.
//
// Note that capacity can only be increased; reduction in size is not possible,
// and if the volume is already at the desired capacity, no action is taken.
// vol Capacity-related values are mutated if successful, so callers should
// pass in a copy, then commit changes to raft.
func (v *CSIVolume) expandVolume(vol *structs.CSIVolume, plugin *structs.CSIPlugin, capacity *csi.CapacityRange) error {
	if vol == nil || plugin == nil || capacity == nil {
		return errors.New("unexpected nil value")
	}

	newMax := capacity.LimitBytes
	newMin := capacity.RequiredBytes
	logger := v.logger.Named("expandVolume").With(
		"vol", vol.ID,
		"requested_min", humanize.Bytes(uint64(newMin)),
		"requested_max", humanize.Bytes(uint64(newMax)),
	)

	// If requested capacity values are unset, skip everything.
	if newMax == 0 && newMin == 0 {
		logger.Debug("min and max values are zero")
		return nil
	}

	// New values same as current, so nothing to do.
	if vol.RequestedCapacityMax == newMax &&
		vol.RequestedCapacityMin == newMin {
		logger.Debug("requested capacity unchanged")
		return nil
	}

	// If max is specified, it cannot be less than min or current capacity.
	if newMax > 0 {
		if newMax < newMin {
			return fmt.Errorf("max requested capacity (%s) less than or equal to min (%s)",
				humanize.Bytes(uint64(newMax)),
				humanize.Bytes(uint64(newMin)))
		}
		if newMax < vol.Capacity {
			return fmt.Errorf("max requested capacity (%s) less than or equal to current (%s)",
				humanize.Bytes(uint64(newMax)),
				humanize.Bytes(uint64(vol.Capacity)))
		}
	}

	// Values are validated, so go ahead and update vol to commit to state,
	// even if the external volume does not need expanding.
	vol.RequestedCapacityMin = newMin
	vol.RequestedCapacityMax = newMax

	// Only expand if new min is greater than current capacity.
	if newMin <= vol.Capacity {
		return nil
	}

	if !plugin.HasControllerCapability(structs.CSIControllerSupportsExpand) {
		return errors.New("expand is not implemented by this controller plugin")
	}

	capability, err := csi.VolumeCapabilityFromStructs(vol.AttachmentMode, vol.AccessMode, vol.MountOptions)
	if err != nil {
		logger.Debug("unable to get capability from volume", "error", err)
		// We'll optimistically send a nil capability, as an "unknown"
		// attachment mode (likely not attached) is acceptable per the spec.
	}

	method := "ClientCSI.ControllerExpandVolume"
	cReq := &cstructs.ClientCSIControllerExpandVolumeRequest{
		ExternalVolumeID: vol.ExternalID,
		Secrets:          vol.Secrets,
		CapacityRange:    capacity,
		VolumeCapability: capability,
	}
	cReq.PluginID = plugin.ID
	cResp := &cstructs.ClientCSIControllerExpandVolumeResponse{}

	logger.Info("starting volume expansion")
	// This is the real work. The client RPC sends a gRPC to the controller plugin,
	// then that controller may reach out to cloud APIs, etc.
	err = v.serializedControllerRPC(plugin.ID, func() error {
		return v.srv.RPC(method, cReq, cResp)
	})
	if err != nil {
		return fmt.Errorf("unable to expand volume: %w", err)
	}
	vol.Capacity = cResp.CapacityBytes
	logger.Info("controller done expanding volume")

	if cResp.NodeExpansionRequired {
		v.logger.Warn("TODO: also do node volume expansion if needed") // TODO
	}

	return nil
}

func (v *CSIVolume) Delete(args *structs.CSIVolumeDeleteRequest, reply *structs.CSIVolumeDeleteResponse) error {

	authErr := v.srv.Authenticate(v.ctx, args)
	if done, err := v.srv.forward("CSIVolume.Delete", args, args, reply); done {
		return err
	}
	v.srv.MeasureRPCRate("csi_volume", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "volume", "delete"}, time.Now())

	allowVolume := acl.NamespaceValidator(acl.NamespaceCapabilityCSIWriteVolume)
	aclObj, err := v.srv.ResolveACL(args)
	if err != nil {
		return err
	}

	if !allowVolume(aclObj, args.RequestNamespace()) || !aclObj.AllowPluginRead() {
		return structs.ErrPermissionDenied
	}

	if len(args.VolumeIDs) == 0 {
		return fmt.Errorf("missing volume IDs")
	}

	for _, volID := range args.VolumeIDs {

		plugin, vol, err := v.volAndPluginLookup(args.Namespace, volID)
		if err != nil {
			if err == fmt.Errorf("volume not found: %s", volID) {
				v.logger.Warn("volume %q to be deleted was already deregistered")
				continue
			} else {
				return err
			}
		}
		if plugin == nil {
			return fmt.Errorf("plugin %q for volume %q not found", vol.PluginID, volID)
		}

		// NOTE: deleting the volume in the external storage provider can't be
		// made atomic with deregistration. We can't delete a volume that's
		// not registered because we need to be able to lookup its plugin.
		err = v.deleteVolume(vol, plugin, args.Secrets)
		if err != nil {
			return err
		}
	}

	deregArgs := &structs.CSIVolumeDeregisterRequest{
		VolumeIDs:    args.VolumeIDs,
		WriteRequest: args.WriteRequest,
	}
	_, index, err := v.srv.raftApply(structs.CSIVolumeDeregisterRequestType, deregArgs)
	if err != nil {
		v.logger.Error("csi raft apply failed", "error", err, "method", "deregister")
		return err
	}

	reply.Index = index
	v.srv.setQueryMeta(&reply.QueryMeta)
	return nil
}

func (v *CSIVolume) deleteVolume(vol *structs.CSIVolume, plugin *structs.CSIPlugin, querySecrets structs.CSISecrets) error {
	// Combine volume and query secrets into one map.
	// Query secrets override any secrets stored with the volume.
	combinedSecrets := vol.Secrets
	for k, v := range querySecrets {
		combinedSecrets[k] = v
	}

	method := "ClientCSI.ControllerDeleteVolume"
	cReq := &cstructs.ClientCSIControllerDeleteVolumeRequest{
		ExternalVolumeID: vol.ExternalID,
		Secrets:          combinedSecrets,
	}
	cReq.PluginID = plugin.ID
	cResp := &cstructs.ClientCSIControllerDeleteVolumeResponse{}

	return v.serializedControllerRPC(plugin.ID, func() error {
		return v.srv.RPC(method, cReq, cResp)
	})
}

func (v *CSIVolume) ListExternal(args *structs.CSIVolumeExternalListRequest, reply *structs.CSIVolumeExternalListResponse) error {

	authErr := v.srv.Authenticate(v.ctx, args)
	if done, err := v.srv.forward("CSIVolume.ListExternal", args, args, reply); done {
		return err
	}
	v.srv.MeasureRPCRate("csi_volume", structs.RateMetricList, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "volume", "list_external"}, time.Now())

	allowVolume := acl.NamespaceValidator(acl.NamespaceCapabilityCSIListVolume,
		acl.NamespaceCapabilityCSIReadVolume,
		acl.NamespaceCapabilityCSIMountVolume,
		acl.NamespaceCapabilityListJobs)
	aclObj, err := v.srv.ResolveACL(args)
	if err != nil {
		return err
	}

	// NOTE: this is the plugin's namespace, not the volume(s) because they
	// might not even be registered
	if !allowVolume(aclObj, args.RequestNamespace()) {
		return structs.ErrPermissionDenied
	}
	snap, err := v.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}

	plugin, err := snap.CSIPluginByID(nil, args.PluginID)
	if err != nil {
		return err
	}
	if plugin == nil {
		return fmt.Errorf("no such plugin")
	}
	if !plugin.HasControllerCapability(structs.CSIControllerSupportsListVolumes) {
		return fmt.Errorf("unimplemented for this plugin")
	}

	method := "ClientCSI.ControllerListVolumes"
	cReq := &cstructs.ClientCSIControllerListVolumesRequest{
		MaxEntries:    args.PerPage,
		StartingToken: args.NextToken,
	}
	cReq.PluginID = plugin.ID
	cResp := &cstructs.ClientCSIControllerListVolumesResponse{}

	err = v.srv.RPC(method, cReq, cResp)
	if err != nil {
		return err
	}
	if args.PerPage > 0 && args.PerPage < int32(len(cResp.Entries)) {
		// this should be done in the plugin already, but enforce it
		reply.Volumes = cResp.Entries[:args.PerPage]
	} else {
		reply.Volumes = cResp.Entries
	}
	reply.NextToken = cResp.NextToken

	return nil
}

func (v *CSIVolume) CreateSnapshot(args *structs.CSISnapshotCreateRequest, reply *structs.CSISnapshotCreateResponse) error {

	authErr := v.srv.Authenticate(v.ctx, args)
	if done, err := v.srv.forward("CSIVolume.CreateSnapshot", args, args, reply); done {
		return err
	}
	v.srv.MeasureRPCRate("csi_volume", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "volume", "create_snapshot"}, time.Now())

	allowVolume := acl.NamespaceValidator(acl.NamespaceCapabilityCSIWriteVolume)
	aclObj, err := v.srv.ResolveACL(args)
	if err != nil {
		return err
	}
	if !allowVolume(aclObj, args.RequestNamespace()) || !aclObj.AllowPluginRead() {
		return structs.ErrPermissionDenied
	}

	state, err := v.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}

	method := "ClientCSI.ControllerCreateSnapshot"
	var mErr multierror.Error
	for _, snap := range args.Snapshots {
		if snap == nil {
			// we intentionally don't multierror here because we're in a weird state
			return fmt.Errorf("snapshot cannot be nil")
		}

		vol, err := state.CSIVolumeByID(nil, args.RequestNamespace(), snap.SourceVolumeID)
		if err != nil {
			multierror.Append(&mErr, fmt.Errorf("error querying volume %q: %v", snap.SourceVolumeID, err))
			continue
		}
		if vol == nil {
			multierror.Append(&mErr, fmt.Errorf("no such volume %q", snap.SourceVolumeID))
			continue
		}

		pluginID := snap.PluginID
		if pluginID == "" {
			pluginID = vol.PluginID
		}

		plugin, err := state.CSIPluginByID(nil, pluginID)
		if err != nil {
			multierror.Append(&mErr,
				fmt.Errorf("error querying plugin %q: %v", pluginID, err))
			continue
		}
		if plugin == nil {
			multierror.Append(&mErr, fmt.Errorf("no such plugin %q", pluginID))
			continue
		}
		if !plugin.HasControllerCapability(structs.CSIControllerSupportsCreateDeleteSnapshot) {
			multierror.Append(&mErr,
				fmt.Errorf("plugin %q does not support snapshot", pluginID))
			continue
		}

		secrets := vol.Secrets
		for k, v := range snap.Secrets {
			// merge request secrets onto volume secrets
			secrets[k] = v
		}

		cReq := &cstructs.ClientCSIControllerCreateSnapshotRequest{
			ExternalSourceVolumeID: vol.ExternalID,
			Name:                   snap.Name,
			Secrets:                secrets,
			Parameters:             snap.Parameters,
		}
		cReq.PluginID = pluginID
		cResp := &cstructs.ClientCSIControllerCreateSnapshotResponse{}
		err = v.serializedControllerRPC(pluginID, func() error {
			return v.srv.RPC(method, cReq, cResp)
		})
		if err != nil {
			multierror.Append(&mErr, fmt.Errorf("could not create snapshot: %v", err))
			continue
		}
		reply.Snapshots = append(reply.Snapshots, &structs.CSISnapshot{
			ID:                     cResp.ID,
			ExternalSourceVolumeID: cResp.ExternalSourceVolumeID,
			SizeBytes:              cResp.SizeBytes,
			CreateTime:             cResp.CreateTime,
			IsReady:                cResp.IsReady,
		})
	}

	return mErr.ErrorOrNil()
}

func (v *CSIVolume) DeleteSnapshot(args *structs.CSISnapshotDeleteRequest, reply *structs.CSISnapshotDeleteResponse) error {

	authErr := v.srv.Authenticate(v.ctx, args)
	if done, err := v.srv.forward("CSIVolume.DeleteSnapshot", args, args, reply); done {
		return err
	}
	v.srv.MeasureRPCRate("csi_volume", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "volume", "delete_snapshot"}, time.Now())

	allowVolume := acl.NamespaceValidator(acl.NamespaceCapabilityCSIWriteVolume)
	aclObj, err := v.srv.ResolveACL(args)
	if err != nil {
		return err
	}

	// NOTE: this is the plugin's namespace, not the snapshot(s) because we
	// don't track snapshots in the state store at all and their source
	// volume(s) because they might not even be registered
	if !allowVolume(aclObj, args.RequestNamespace()) || !aclObj.AllowPluginRead() {
		return structs.ErrPermissionDenied
	}

	stateSnap, err := v.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}

	var mErr multierror.Error
	for _, snap := range args.Snapshots {
		if snap == nil {
			// we intentionally don't multierror here because we're in a weird state
			return fmt.Errorf("snapshot cannot be nil")
		}

		plugin, err := stateSnap.CSIPluginByID(nil, snap.PluginID)
		if err != nil {
			multierror.Append(&mErr,
				fmt.Errorf("could not query plugin %q: %v", snap.PluginID, err))
			continue
		}
		if plugin == nil {
			multierror.Append(&mErr, fmt.Errorf("no such plugin"))
			continue
		}
		if !plugin.HasControllerCapability(structs.CSIControllerSupportsCreateDeleteSnapshot) {
			multierror.Append(&mErr, fmt.Errorf("plugin does not support snapshot"))
			continue
		}

		method := "ClientCSI.ControllerDeleteSnapshot"

		cReq := &cstructs.ClientCSIControllerDeleteSnapshotRequest{ID: snap.ID}
		cReq.PluginID = plugin.ID
		cResp := &cstructs.ClientCSIControllerDeleteSnapshotResponse{}
		err = v.serializedControllerRPC(plugin.ID, func() error {
			return v.srv.RPC(method, cReq, cResp)
		})
		if err != nil {
			multierror.Append(&mErr, fmt.Errorf("could not delete %q: %v", snap.ID, err))
		}
	}
	return mErr.ErrorOrNil()
}

func (v *CSIVolume) ListSnapshots(args *structs.CSISnapshotListRequest, reply *structs.CSISnapshotListResponse) error {

	authErr := v.srv.Authenticate(v.ctx, args)
	if done, err := v.srv.forward("CSIVolume.ListSnapshots", args, args, reply); done {
		return err
	}
	v.srv.MeasureRPCRate("csi_volume", structs.RateMetricList, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "volume", "list_snapshots"}, time.Now())

	allowVolume := acl.NamespaceValidator(acl.NamespaceCapabilityCSIListVolume,
		acl.NamespaceCapabilityCSIReadVolume,
		acl.NamespaceCapabilityCSIMountVolume,
		acl.NamespaceCapabilityListJobs)
	aclObj, err := v.srv.ResolveACL(args)
	if err != nil {
		return err
	}

	// NOTE: this is the plugin's namespace, not the volume(s) because they
	// might not even be registered
	if !allowVolume(aclObj, args.RequestNamespace()) {
		return structs.ErrPermissionDenied
	}
	snap, err := v.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}

	plugin, err := snap.CSIPluginByID(nil, args.PluginID)
	if err != nil {
		return err
	}
	if plugin == nil {
		return fmt.Errorf("no such plugin")
	}
	if !plugin.HasControllerCapability(structs.CSIControllerSupportsListSnapshots) {
		return fmt.Errorf("plugin does not support listing snapshots")
	}

	method := "ClientCSI.ControllerListSnapshots"
	cReq := &cstructs.ClientCSIControllerListSnapshotsRequest{
		MaxEntries:    args.PerPage,
		StartingToken: args.NextToken,
		Secrets:       args.Secrets,
	}
	cReq.PluginID = plugin.ID
	cResp := &cstructs.ClientCSIControllerListSnapshotsResponse{}

	err = v.srv.RPC(method, cReq, cResp)
	if err != nil {
		return err
	}
	if args.PerPage > 0 && args.PerPage < int32(len(cResp.Entries)) {
		// this should be done in the plugin already, but enforce it
		reply.Snapshots = cResp.Entries[:args.PerPage]
	} else {
		reply.Snapshots = cResp.Entries
	}
	reply.NextToken = cResp.NextToken

	return nil
}

// CSIPlugin wraps the structs.CSIPlugin with request data and server context
type CSIPlugin struct {
	srv    *Server
	ctx    *RPCContext
	logger hclog.Logger
}

func NewCSIPluginEndpoint(srv *Server, ctx *RPCContext) *CSIPlugin {
	return &CSIPlugin{srv: srv, ctx: ctx, logger: srv.logger.Named("csi_plugin")}
}

// List replies with CSIPlugins, filtered by ACL access
func (v *CSIPlugin) List(args *structs.CSIPluginListRequest, reply *structs.CSIPluginListResponse) error {

	authErr := v.srv.Authenticate(v.ctx, args)
	if done, err := v.srv.forward("CSIPlugin.List", args, args, reply); done {
		return err
	}
	v.srv.MeasureRPCRate("csi_plugin", structs.RateMetricList, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "plugin", "list"}, time.Now())

	aclObj, err := v.srv.ResolveACL(args)
	if err != nil {
		return err
	}
	if !aclObj.AllowPluginList() {
		return structs.ErrPermissionDenied
	}

	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {

			var iter memdb.ResultIterator
			var err error
			if args.Prefix != "" {
				iter, err = state.CSIPluginsByIDPrefix(ws, args.Prefix)
				if err != nil {
					return err
				}
			} else {
				// Query all plugins
				iter, err = state.CSIPlugins(ws)
				if err != nil {
					return err
				}
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

	authErr := v.srv.Authenticate(v.ctx, args)
	if done, err := v.srv.forward("CSIPlugin.Get", args, args, reply); done {
		return err
	}
	v.srv.MeasureRPCRate("csi_plugin", structs.RateMetricRead, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "plugin", "get"}, time.Now())

	aclObj, err := v.srv.ResolveACL(args)
	if err != nil {
		return err
	}
	if !aclObj.AllowPluginRead() {
		return structs.ErrPermissionDenied
	}

	withAllocs := aclObj == nil ||
		aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilityReadJob)

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

	authErr := v.srv.Authenticate(v.ctx, args)
	if done, err := v.srv.forward("CSIPlugin.Delete", args, args, reply); done {
		return err
	}
	v.srv.MeasureRPCRate("csi_plugin", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "plugin", "delete"}, time.Now())

	// Check that it is a management token.
	if aclObj, err := v.srv.ResolveACL(args); err != nil {
		return err
	} else if aclObj != nil && !aclObj.IsManagement() {
		return structs.ErrPermissionDenied
	}

	if args.ID == "" {
		return fmt.Errorf("missing plugin ID")
	}

	_, index, err := v.srv.raftApply(structs.CSIPluginDeleteRequestType, args)
	if err != nil {
		v.logger.Error("csi raft apply failed", "error", err, "method", "delete")
		return err
	}

	reply.Index = index
	v.srv.setQueryMeta(&reply.QueryMeta)
	return nil
}
