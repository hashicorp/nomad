// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/acl"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/state/paginator"
	"github.com/hashicorp/nomad/nomad/structs"
)

// HostVolume is the server RPC endpoint for host volumes
type HostVolume struct {
	srv    *Server
	ctx    *RPCContext
	logger hclog.Logger
}

func NewHostVolumeEndpoint(srv *Server, ctx *RPCContext) *HostVolume {
	return &HostVolume{srv: srv, ctx: ctx, logger: srv.logger.Named("host_volume")}
}

func (v *HostVolume) Get(args *structs.HostVolumeGetRequest, reply *structs.HostVolumeGetResponse) error {
	authErr := v.srv.Authenticate(v.ctx, args)
	if done, err := v.srv.forward("HostVolume.Get", args, args, reply); done {
		return err
	}
	v.srv.MeasureRPCRate("host_volume", structs.RateMetricRead, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "host_volume", "get"}, time.Now())

	allowVolume := acl.NamespaceValidator(acl.NamespaceCapabilityHostVolumeRead)
	aclObj, err := v.srv.ResolveACL(args)
	if err != nil {
		return err
	}
	if !allowVolume(aclObj, args.RequestNamespace()) {
		return structs.ErrPermissionDenied
	}

	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, store *state.StateStore) error {

			vol, err := store.HostVolumeByID(ws, args.Namespace, args.ID, true)
			if err != nil {
				return err
			}

			reply.Volume = vol
			if vol != nil {
				reply.Index = vol.ModifyIndex
			} else {
				index, err := store.Index(state.TableHostVolumes)
				if err != nil {
					return err
				}

				// Ensure we never set the index to zero, otherwise a blocking
				// query cannot be used.  We floor the index at one, since
				// realistically the first write must have a higher index.
				if index == 0 {
					index = 1
				}
				reply.Index = index
			}
			return nil
		}}
	return v.srv.blockingRPC(&opts)
}

func (v *HostVolume) List(args *structs.HostVolumeListRequest, reply *structs.HostVolumeListResponse) error {
	authErr := v.srv.Authenticate(v.ctx, args)
	if done, err := v.srv.forward("HostVolume.List", args, args, reply); done {
		return err
	}
	v.srv.MeasureRPCRate("host_volume", structs.RateMetricList, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "host_volume", "list"}, time.Now())

	aclObj, err := v.srv.ResolveACL(args)
	if err != nil {
		return err
	}

	ns := args.RequestNamespace()

	sort := state.SortOption(args.Reverse)
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, store *state.StateStore) error {

			var iter memdb.ResultIterator
			var err error

			switch {
			case args.NodeID != "":
				iter, err = store.HostVolumesByNodeID(ws, args.NodeID, sort)
			case args.NodePool != "":
				iter, err = store.HostVolumesByNodePool(ws, args.NodePool, sort)
			default:
				iter, err = store.HostVolumes(ws, sort)
			}
			if err != nil {
				return err
			}

			// Generate the tokenizer to use for pagination using namespace and
			// ID to ensure complete uniqueness.
			tokenizer := paginator.NewStructsTokenizer(iter,
				paginator.StructsTokenizerOptions{
					WithNamespace: true,
					WithID:        true,
				},
			)

			filters := []paginator.Filter{
				paginator.GenericFilter{
					Allow: func(raw any) (bool, error) {
						vol := raw.(*structs.HostVolume)
						// empty prefix doesn't filter
						if !strings.HasPrefix(vol.Name, args.Prefix) &&
							!strings.HasPrefix(vol.ID, args.Prefix) {
							return false, nil
						}
						if args.NodeID != "" && vol.NodeID != args.NodeID {
							return false, nil
						}
						if args.NodePool != "" && vol.NodePool != args.NodePool {
							return false, nil
						}

						if ns != structs.AllNamespacesSentinel &&
							vol.Namespace != ns {
							return false, nil
						}

						allowVolume := acl.NamespaceValidator(acl.NamespaceCapabilityHostVolumeRead)
						return allowVolume(aclObj, ns), nil
					},
				},
			}

			// Set up our output after we have checked the error.
			var vols []*structs.HostVolumeStub

			// Build the paginator. This includes the function that is
			// responsible for appending a variable to the variables
			// stubs slice.
			paginatorImpl, err := paginator.NewPaginator(iter, tokenizer, filters, args.QueryOptions,
				func(raw any) error {
					vol := raw.(*structs.HostVolume)
					vols = append(vols, vol.Stub())
					return nil
				})
			if err != nil {
				return structs.NewErrRPCCodedf(
					http.StatusBadRequest, "failed to create result paginator: %v", err)
			}

			// Calling page populates our output variable stub array as well as
			// returns the next token.
			nextToken, err := paginatorImpl.Page()
			if err != nil {
				return structs.NewErrRPCCodedf(
					http.StatusBadRequest, "failed to read result page: %v", err)
			}

			reply.Volumes = vols
			reply.NextToken = nextToken

			// Use the index table to populate the query meta as we have no way
			// of tracking the max index on deletes.
			return v.srv.setReplyQueryMeta(store, state.TableHostVolumes, &reply.QueryMeta)
		},
	}

	return v.srv.blockingRPC(&opts)
}

func (v *HostVolume) Create(args *structs.HostVolumeCreateRequest, reply *structs.HostVolumeCreateResponse) error {

	authErr := v.srv.Authenticate(v.ctx, args)
	if done, err := v.srv.forward("HostVolume.Create", args, args, reply); done {
		return err
	}
	v.srv.MeasureRPCRate("host_volume", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "host_volume", "create"}, time.Now())

	allowVolume := acl.NamespaceValidator(acl.NamespaceCapabilityHostVolumeCreate)
	aclObj, err := v.srv.ResolveACL(args)
	if err != nil {
		return err
	}

	if len(args.Volumes) == 0 {
		return fmt.Errorf("missing volume definition")
	}

	for _, vol := range args.Volumes {
		if vol.Namespace == "" {
			vol.Namespace = args.RequestNamespace()
		}
		if !allowVolume(aclObj, vol.Namespace) {
			return structs.ErrPermissionDenied
		}
	}

	// ensure we only try to create valid volumes or make valid updates to
	// volumes
	validVols, err := v.validateVolumeUpdates(args.Volumes)
	if err != nil {
		return err
	}

	// Attempt to create all the validated volumes and write only successfully
	// created volumes to raft. And we'll report errors for any failed volumes
	//
	// NOTE: creating the volume on the client via the plugin can't be made
	// atomic with the registration, and creating the volume provides values we
	// want to write on the Volume in raft anyways.

	// This can't reuse the validVols slice because we only want to write
	// volumes we've successfully created or updated on the client to get
	// updated in Raft.
	raftArgs := &structs.HostVolumeRegisterRequest{
		Volumes:      []*structs.HostVolume{},
		WriteRequest: args.WriteRequest,
	}

	var mErr *multierror.Error
	for _, vol := range validVols {
		err = v.createVolume(vol) // mutates the vol
		if err != nil {
			mErr = multierror.Append(mErr, err)
		} else {
			raftArgs.Volumes = append(raftArgs.Volumes, vol)
		}
	}

	// if we created or updated any volumes, apply them to raft.
	var index uint64
	if len(raftArgs.Volumes) > 0 {
		_, index, err = v.srv.raftApply(structs.HostVolumeRegisterRequestType, raftArgs)
		if err != nil {
			v.logger.Error("raft apply failed", "error", err, "method", "register")
			mErr = multierror.Append(mErr, err)
		}
	}

	reply.Volumes = raftArgs.Volumes
	reply.Index = index
	return helper.FlattenMultierror(mErr)
}

func (v *HostVolume) Register(args *structs.HostVolumeRegisterRequest, reply *structs.HostVolumeRegisterResponse) error {

	authErr := v.srv.Authenticate(v.ctx, args)
	if done, err := v.srv.forward("HostVolume.Register", args, args, reply); done {
		return err
	}
	v.srv.MeasureRPCRate("host_volume", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "host_volume", "register"}, time.Now())

	allowVolume := acl.NamespaceValidator(acl.NamespaceCapabilityHostVolumeRegister)
	aclObj, err := v.srv.ResolveACL(args)
	if err != nil {
		return err
	}

	if len(args.Volumes) == 0 {
		return fmt.Errorf("missing volume definition")
	}

	for _, vol := range args.Volumes {
		if vol.Namespace == "" {
			vol.Namespace = args.RequestNamespace()
		}
		if !allowVolume(aclObj, vol.Namespace) {
			return structs.ErrPermissionDenied
		}
	}

	// ensure we only try to create valid volumes or make valid updates to
	// volumes
	validVols, err := v.validateVolumeUpdates(args.Volumes)
	if err != nil {
		return err
	}

	raftArgs := &structs.HostVolumeRegisterRequest{
		Volumes:      validVols,
		WriteRequest: args.WriteRequest,
	}

	var mErr *multierror.Error
	var index uint64
	if len(raftArgs.Volumes) > 0 {
		_, index, err = v.srv.raftApply(structs.HostVolumeRegisterRequestType, raftArgs)
		if err != nil {
			v.logger.Error("raft apply failed", "error", err, "method", "register")
			mErr = multierror.Append(mErr, err)
		}
	}

	reply.Volumes = raftArgs.Volumes
	reply.Index = index
	return helper.FlattenMultierror(mErr)
}

func (v *HostVolume) validateVolumeUpdates(requested []*structs.HostVolume) ([]*structs.HostVolume, error) {

	now := time.Now().UnixNano()
	var vols []*structs.HostVolume

	snap, err := v.srv.State().Snapshot()
	if err != nil {
		return nil, err
	}

	var mErr *multierror.Error
	for _, vol := range requested {
		vol.ModifyTime = now

		if vol.ID == "" {
			vol.ID = uuid.Generate()
			vol.CreateTime = now
		}

		// if the volume already exists, we'll ensure we're validating the
		// update
		current, err := snap.HostVolumeByID(nil, vol.Namespace, vol.ID, false)
		if err != nil {
			mErr = multierror.Append(mErr, err)
			continue
		}
		if err = vol.Validate(current); err != nil {
			mErr = multierror.Append(mErr, err)
			continue
		}

		vols = append(vols, vol.Copy())
	}

	return vols, mErr.ErrorOrNil()
}

func (v *HostVolume) createVolume(vol *structs.HostVolume) error {

	// TODO(1.10.0): proper node selection based on constraints and node
	// pool. Also, should we move this into the validator step?
	if vol.NodeID == "" {
		var iter memdb.ResultIterator
		var err error
		var raw any
		if vol.NodePool != "" {
			iter, err = v.srv.State().NodesByNodePool(nil, vol.NodePool)
		} else {
			iter, err = v.srv.State().Nodes(nil)
		}
		if err != nil {
			return err
		}
		raw = iter.Next()
		if raw == nil {
			return fmt.Errorf("no node meets constraints for volume")
		}

		node := raw.(*structs.Node)
		vol.NodeID = node.ID
	}

	method := "ClientHostVolume.Create"
	cReq := &cstructs.ClientHostVolumeCreateRequest{
		ID:                        vol.ID,
		Name:                      vol.Name,
		PluginID:                  vol.PluginID,
		NodeID:                    vol.NodeID,
		RequestedCapacityMinBytes: vol.RequestedCapacityMinBytes,
		RequestedCapacityMaxBytes: vol.RequestedCapacityMaxBytes,
		Parameters:                vol.Parameters,
	}
	cResp := &cstructs.ClientHostVolumeCreateResponse{}
	err := v.srv.RPC(method, cReq, cResp)
	if err != nil {
		return err
	}

	if vol.State == structs.HostVolumeStateUnknown {
		vol.State = structs.HostVolumeStatePending
	}

	vol.HostPath = cResp.HostPath
	vol.CapacityBytes = cResp.CapacityBytes

	return nil
}

func (v *HostVolume) Delete(args *structs.HostVolumeDeleteRequest, reply *structs.HostVolumeDeleteResponse) error {

	authErr := v.srv.Authenticate(v.ctx, args)
	if done, err := v.srv.forward("HostVolume.Delete", args, args, reply); done {
		return err
	}
	v.srv.MeasureRPCRate("host_volume", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "host_volume", "delete"}, time.Now())

	// Note that all deleted volumes need to be in the same namespace
	allowVolume := acl.NamespaceValidator(acl.NamespaceCapabilityHostVolumeDelete)
	aclObj, err := v.srv.ResolveACL(args)
	if err != nil {
		return err
	}
	if !allowVolume(aclObj, args.RequestNamespace()) {
		return structs.ErrPermissionDenied
	}

	if len(args.VolumeIDs) == 0 {
		return fmt.Errorf("missing volumes to delete")
	}

	var deletedVols []string
	var index uint64

	snap, err := v.srv.State().Snapshot()
	if err != nil {
		return err
	}

	var mErr *multierror.Error
	ns := args.RequestNamespace()

	for _, id := range args.VolumeIDs {
		vol, err := snap.HostVolumeByID(nil, ns, id, true)
		if err != nil {
			return fmt.Errorf("could not query host volume: %w", err)
		}
		if vol == nil {
			return fmt.Errorf("no such volume: %s", id)
		}
		if len(vol.Allocations) > 0 {
			allocIDs := helper.ConvertSlice(vol.Allocations,
				func(a *structs.AllocListStub) string { return a.ID })
			mErr = multierror.Append(mErr,
				fmt.Errorf("volume %s in use by allocations: %v", id, allocIDs))
			continue
		}

		err = v.deleteVolume(vol)
		if err != nil {
			mErr = multierror.Append(mErr, err)
		} else {
			deletedVols = append(deletedVols, id)
		}
	}

	if len(deletedVols) > 0 {
		args.VolumeIDs = deletedVols
		_, index, err = v.srv.raftApply(structs.HostVolumeDeleteRequestType, args)
		if err != nil {
			v.logger.Error("raft apply failed", "error", err, "method", "delete")
			mErr = multierror.Append(mErr, err)
		}
	}

	reply.VolumeIDs = deletedVols
	reply.Index = index
	return helper.FlattenMultierror(mErr)
}

func (v *HostVolume) deleteVolume(vol *structs.HostVolume) error {

	method := "ClientHostVolume.Delete"
	cReq := &cstructs.ClientHostVolumeDeleteRequest{
		ID:         vol.ID,
		NodeID:     vol.NodeID,
		HostPath:   vol.HostPath,
		Parameters: vol.Parameters,
	}
	cResp := &cstructs.ClientHostVolumeDeleteResponse{}
	err := v.srv.RPC(method, cReq, cResp)
	if err != nil {
		return err
	}

	return nil
}
