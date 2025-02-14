// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	metrics "github.com/hashicorp/go-metrics/compat"
	"github.com/hashicorp/nomad/acl"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/state/paginator"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/scheduler"
)

// HostVolume is the server RPC endpoint for host volumes
type HostVolume struct {
	srv    *Server
	ctx    *RPCContext
	logger hclog.Logger

	// volOps is used to serialize operations per volume ID
	volOps sync.Map
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

	if args.Volume == nil {
		return fmt.Errorf("missing volume definition")
	}

	vol := args.Volume
	if vol.Namespace == "" {
		vol.Namespace = args.RequestNamespace()
	}
	if !allowVolume(aclObj, vol.Namespace) {
		return structs.ErrPermissionDenied
	}

	// ensure we only try to create a valid volume or make valid updates to a
	// volume
	snap, err := v.srv.State().Snapshot()
	if err != nil {
		return err
	}
	existing, err := v.validateVolumeUpdate(vol, snap)
	if err != nil {
		return err
	}

	// set zero values as needed, possibly from existing
	now := time.Now()
	vol.CanonicalizeForCreate(existing, now)

	// make sure any nodes or pools actually exist
	err = v.validateVolumeForState(vol, snap)
	if err != nil {
		return fmt.Errorf("validating volume %q against state failed: %v", vol.Name, err)
	}

	_, err = v.placeHostVolume(snap, vol)
	if err != nil {
		return fmt.Errorf("could not place volume %q: %w", vol.Name, err)
	}

	warn, err := v.enforceEnterprisePolicy(
		snap, vol, args.GetIdentity().GetACLToken(), args.PolicyOverride)
	if warn != nil {
		reply.Warnings = warn.Error()
	}
	if err != nil {
		return err
	}

	// serialize client RPC and raft write per volume ID
	index, err := v.serializeCall(vol.ID, "create", func() (uint64, error) {
		// Attempt to create the volume on the client.
		//
		// NOTE: creating the volume on the client via the plugin can't be made
		// atomic with the registration, and creating the volume provides values
		// we want to write on the Volume in raft anyways.
		if err = v.createVolume(vol); err != nil {
			return 0, err
		}

		// Write a newly created or modified volume to raft. We create a new
		// request here because we've likely mutated the volume.
		_, idx, err := v.srv.raftApply(structs.HostVolumeRegisterRequestType,
			&structs.HostVolumeRegisterRequest{
				Volume:       vol,
				WriteRequest: args.WriteRequest,
			})
		if err != nil {
			v.logger.Error("raft apply failed", "error", err, "method", "register")
			return 0, err
		}
		return idx, nil
	})
	if err != nil {
		return err
	}

	reply.Volume = vol
	reply.Index = index
	return nil
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

	if args.Volume == nil {
		return fmt.Errorf("missing volume definition")
	}

	vol := args.Volume
	if vol.Namespace == "" {
		vol.Namespace = args.RequestNamespace()
	}
	if !allowVolume(aclObj, vol.Namespace) {
		return structs.ErrPermissionDenied
	}

	snap, err := v.srv.State().Snapshot()
	if err != nil {
		return err
	}

	if vol.NodeID == "" {
		return errors.New("cannot register volume: node ID is required")
	}
	if vol.HostPath == "" {
		return errors.New("cannot register volume: host path is required")
	}

	existing, err := v.validateVolumeUpdate(vol, snap)
	if err != nil {
		return err
	}

	// set zero values as needed, possibly from existing
	now := time.Now()
	vol.CanonicalizeForRegister(existing, now)

	// make sure any nodes or pools actually exist
	err = v.validateVolumeForState(vol, snap)
	if err != nil {
		return fmt.Errorf("validating volume %q against state failed: %v", vol.ID, err)
	}

	warn, err := v.enforceEnterprisePolicy(
		snap, vol, args.GetIdentity().GetACLToken(), args.PolicyOverride)
	if warn != nil {
		reply.Warnings = warn.Error()
	}
	if err != nil {
		return err
	}

	// serialize client RPC and raft write per volume ID
	index, err := v.serializeCall(vol.ID, "register", func() (uint64, error) {
		// Attempt to register the volume on the client.
		//
		// NOTE: registering the volume on the client via the plugin can't be made
		// atomic with the registration.
		if err = v.registerVolume(vol); err != nil {
			return 0, err
		}

		// Write a newly created or modified volume to raft. We create a new
		// request here because we've likely mutated the volume.
		_, idx, err := v.srv.raftApply(structs.HostVolumeRegisterRequestType,
			&structs.HostVolumeRegisterRequest{
				Volume:       vol,
				WriteRequest: args.WriteRequest,
			})
		if err != nil {
			v.logger.Error("raft apply failed", "error", err, "method", "register")
			return 0, err
		}
		return idx, nil
	})
	if err != nil {
		return err
	}

	reply.Volume = vol
	reply.Index = index
	return nil
}

func (v *HostVolume) validateVolumeUpdate(
	vol *structs.HostVolume, snap *state.StateSnapshot) (*structs.HostVolume, error) {

	// validate the volume spec
	err := vol.Validate()
	if err != nil {
		return nil, fmt.Errorf("volume validation failed: %v", err)
	}

	// validate any update we're making
	var existing *structs.HostVolume
	if vol.ID != "" {
		existing, err = snap.HostVolumeByID(nil, vol.Namespace, vol.ID, true)
		if err != nil {
			return nil, err // should never hit, bail out
		}
		if existing == nil {
			return nil, fmt.Errorf("cannot update volume %q: volume does not exist", vol.ID)
		}
		err = vol.ValidateUpdate(existing)
		if err != nil {
			return existing, fmt.Errorf("validating volume %q update failed: %v", vol.ID, err)
		}
	}
	return existing, nil
}

// validateVolumeForState ensures that any references to node IDs or node pools are valid
func (v *HostVolume) validateVolumeForState(vol *structs.HostVolume, snap *state.StateSnapshot) error {
	var poolFromExistingNode string
	if vol.NodeID != "" {
		node, err := snap.NodeByID(nil, vol.NodeID)
		if err != nil {
			return err // should never hit, bail out
		}
		if node == nil {
			return fmt.Errorf("node %q does not exist", vol.NodeID)
		}
		poolFromExistingNode = node.NodePool
	}

	if vol.NodePool != "" {
		pool, err := snap.NodePoolByName(nil, vol.NodePool)
		if err != nil {
			return err // should never hit, bail out
		}
		if pool == nil {
			return fmt.Errorf("node pool %q does not exist", vol.NodePool)
		}
		if poolFromExistingNode != "" && poolFromExistingNode != pool.Name {
			return fmt.Errorf("node ID %q is not in pool %q", vol.NodeID, vol.NodePool)
		}
	}

	return nil
}

func (v *HostVolume) createVolume(vol *structs.HostVolume) error {

	method := "ClientHostVolume.Create"
	cReq := &cstructs.ClientHostVolumeCreateRequest{
		ID:                        vol.ID,
		Name:                      vol.Name,
		PluginID:                  vol.PluginID,
		Namespace:                 vol.Namespace,
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

func (v *HostVolume) registerVolume(vol *structs.HostVolume) error {

	method := "ClientHostVolume.Register"
	cReq := &cstructs.ClientHostVolumeRegisterRequest{
		ID:            vol.ID,
		Name:          vol.Name,
		NodeID:        vol.NodeID,
		HostPath:      vol.HostPath,
		CapacityBytes: vol.CapacityBytes,
		Parameters:    vol.Parameters,
	}
	cResp := &cstructs.ClientHostVolumeRegisterResponse{}
	err := v.srv.RPC(method, cReq, cResp)
	if err != nil {
		return err
	}

	if vol.State == structs.HostVolumeStateUnknown {
		vol.State = structs.HostVolumeStatePending
	}

	return nil
}

// placeHostVolume adds a node to volumes that don't already have one. The node
// will match the node pool and constraints, which doesn't already have a volume
// by that name. It returns the node (for testing) and an error indicating
// placement failed.
func (v *HostVolume) placeHostVolume(snap *state.StateSnapshot, vol *structs.HostVolume) (*structs.Node, error) {
	if vol.NodeID != "" {
		node, err := snap.NodeByID(nil, vol.NodeID)
		if err != nil {
			return nil, err
		}
		if node == nil {
			return nil, fmt.Errorf("no such node %s", vol.NodeID)
		}
		vol.NodePool = node.NodePool
		return node, nil
	}

	poolFilterFn, err := v.enterpriseNodePoolFilter(snap, vol)
	if err != nil {
		return nil, err
	}

	var iter memdb.ResultIterator
	if vol.NodePool != "" {
		if !poolFilterFn(vol.NodePool) {
			return nil, fmt.Errorf("namespace %q does not allow volumes to use node pool %q",
				vol.Namespace, vol.NodePool)
		}
		iter, err = snap.NodesByNodePool(nil, vol.NodePool)
	} else {
		iter, err = snap.Nodes(nil)
	}
	if err != nil {
		return nil, err
	}

	var checker *scheduler.ConstraintChecker
	ctx := &placementContext{
		regexpCache:  make(map[string]*regexp.Regexp),
		versionCache: make(map[string]scheduler.VerConstraints),
		semverCache:  make(map[string]scheduler.VerConstraints),
	}
	constraints := []*structs.Constraint{{
		LTarget: fmt.Sprintf("${attr.plugins.host_volume.%s.version}", vol.PluginID),
		Operand: "is_set",
	}}
	constraints = append(constraints, vol.Constraints...)
	checker = scheduler.NewConstraintChecker(ctx, constraints)

	var (
		filteredByExisting    int
		filteredByGovernance  int
		filteredByFeasibility int
	)

	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		candidate := raw.(*structs.Node)

		// note: this is a race if multiple users create volumes of the same
		// name concurrently, but we can't solve it on the server because we
		// haven't yet written to state. The client will reject requests to
		// create/register a volume with the same name with a different ID.
		if _, hasVol := candidate.HostVolumes[vol.Name]; hasVol {
			filteredByExisting++
			continue
		}

		if !poolFilterFn(candidate.NodePool) {
			filteredByGovernance++
			continue
		}

		if checker != nil {
			if ok := checker.Feasible(candidate); !ok {
				filteredByFeasibility++
				continue
			}
		}

		vol.NodeID = candidate.ID
		vol.NodePool = candidate.NodePool
		return candidate, nil

	}

	return nil, fmt.Errorf(
		"no node meets constraints: %d nodes had existing volume, %d nodes filtered by node pool governance, %d nodes were infeasible",
		filteredByExisting, filteredByGovernance, filteredByFeasibility)
}

// placementContext implements the scheduler.ConstraintContext interface, a
// minimal subset of the scheduler.Context interface that we need to create a
// feasibility checker for constraints
type placementContext struct {
	regexpCache  map[string]*regexp.Regexp
	versionCache map[string]scheduler.VerConstraints
	semverCache  map[string]scheduler.VerConstraints
}

func (ctx *placementContext) Metrics() *structs.AllocMetric          { return &structs.AllocMetric{} }
func (ctx *placementContext) RegexpCache() map[string]*regexp.Regexp { return ctx.regexpCache }

func (ctx *placementContext) VersionConstraintCache() map[string]scheduler.VerConstraints {
	return ctx.versionCache
}

func (ctx *placementContext) SemverConstraintCache() map[string]scheduler.VerConstraints {
	return ctx.semverCache
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

	if args.VolumeID == "" {
		return fmt.Errorf("missing volume ID to delete")
	}

	snap, err := v.srv.State().Snapshot()
	if err != nil {
		return err
	}

	ns := args.RequestNamespace()
	id := args.VolumeID

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
		return fmt.Errorf("volume %s in use by allocations: %v", id, allocIDs)
	}

	// serialize client RPC and raft write per volume ID
	index, err := v.serializeCall(vol.ID, "delete", func() (uint64, error) {
		if err := v.deleteVolume(vol); err != nil {
			return 0, err
		}
		_, idx, err := v.srv.raftApply(structs.HostVolumeDeleteRequestType, args)
		if err != nil {
			v.logger.Error("raft apply failed", "error", err, "method", "delete")
			return 0, err
		}
		return idx, nil
	})
	if err != nil {
		return err
	}

	reply.Index = index
	return nil
}

func (v *HostVolume) deleteVolume(vol *structs.HostVolume) error {

	method := "ClientHostVolume.Delete"
	cReq := &cstructs.ClientHostVolumeDeleteRequest{
		ID:         vol.ID,
		Name:       vol.Name,
		PluginID:   vol.PluginID,
		Namespace:  vol.Namespace,
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

// serializeCall serializes fn() per volume, so DHV plugins can assume that
// Nomad will not run concurrent operations for the same volume, and for us
// to avoid interleaving client RPCs with raft writes.
// Concurrent calls should all run eventually (or timeout, or server shutdown),
// but there is no guarantee that they will run in the order received.
// The passed fn is expected to return a raft index and error.
func (v *HostVolume) serializeCall(volumeID, op string, fn func() (uint64, error)) (uint64, error) {
	timeout := 2 * time.Minute // 2x the client RPC timeout
	for {
		ctx, done := context.WithTimeout(v.srv.shutdownCtx, timeout)

		loaded, occupied := v.volOps.LoadOrStore(volumeID, ctx)

		if !occupied {
			v.logger.Trace("HostVolume RPC running ", "operation", op)
			// run the fn!
			index, err := fn()

			// done() must come after Delete, so that other unblocked requests
			// will Store a fresh context when they continue.
			v.volOps.Delete(volumeID)
			done()

			return index, err
		}

		// another one is running; wait for it to finish.
		v.logger.Trace("HostVolume RPC waiting", "operation", op)

		// cancel the tentative context; we'll use the one we pulled from
		// volOps (set by another RPC call) instead.
		done()

		otherCtx := loaded.(context.Context)
		select {
		case <-otherCtx.Done():
			continue
		case <-v.srv.shutdownCh:
			return 0, structs.ErrNoLeader
		}
	}
}
