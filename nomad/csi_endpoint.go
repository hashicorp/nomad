package nomad

import (
	"fmt"
	"time"

	metrics "github.com/armon/go-metrics"
	log "github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/acl"
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
func (srv *Server) QueryACLObj(args *structs.QueryOptions) (*acl.ACL, error) {
	if args.AuthToken == "" {
		return nil, fmt.Errorf("authorization required")
	}

	// Lookup the token
	aclObj, err := srv.ResolveToken(args.AuthToken)
	if err != nil {
		// If ResolveToken had an unexpected error return that
		return nil, err
	}

	if aclObj == nil {
		ws := memdb.NewWatchSet()
		node, stateErr := srv.fsm.State().NodeBySecretID(ws, args.AuthToken)
		if stateErr != nil {
			// Return the original ResolveToken error with this err
			var merr multierror.Error
			merr.Errors = append(merr.Errors, err, stateErr)
			return nil, merr.ErrorOrNil()
		}

		if node == nil {
			return nil, structs.ErrTokenNotFound
		}
	}

	return aclObj, nil
}

// WriteACLObj calls QueryACLObj for a WriteRequest
func (srv *Server) WriteACLObj(args *structs.WriteRequest) (*acl.ACL, error) {
	opts := &structs.QueryOptions{
		Region:    args.RequestRegion(),
		Namespace: args.RequestNamespace(),
		AuthToken: args.AuthToken,
	}
	return srv.QueryACLObj(opts)
}

// replyCSIVolumeIndex sets the reply with the last index that modified the table csi_volumes
func (srv *Server) replySetCSIVolumeIndex(state *state.StateStore, reply *structs.QueryMeta) error {
	// Use the last index that affected the table
	index, err := state.Index("csi_volumes")
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

	aclObj, err := v.srv.QueryACLObj(&args.QueryOptions)
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

			if args.Driver != "" {
				iter, err = state.CSIVolumesByDriver(ws, args.Driver)
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

				// Filter on the request namespace to avoid ACL checks by volume
				if ns != "" && vol.Namespace != args.RequestNamespace() {
					continue
				}

				// Cache ACL checks QUESTION: are they expensive?
				allowed, ok := cache[vol.Namespace]
				if !ok {
					allowed = aclObj.AllowNsOp(vol.Namespace, acl.NamespaceCapabilityCSIAccess)
					cache[vol.Namespace] = allowed
				}

				if allowed {
					vs = append(vs, vol.Stub())
				}
			}
			reply.Volumes = vs
			return v.srv.replySetCSIVolumeIndex(state, &reply.QueryMeta)
		}}
	return v.srv.blockingRPC(&opts)
}

// Get fetches detailed information about a specific volume
func (v *CSIVolume) Get(args *structs.CSIVolumeGetRequest, reply *structs.CSIVolumeGetResponse) error {
	if done, err := v.srv.forward("CSIVolume.Get", args, args, reply); done {
		return err
	}

	aclObj, err := v.srv.QueryACLObj(&args.QueryOptions)
	if err != nil {
		return err
	}

	if !aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilityCSIAccess) {
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

			if vol == nil {
				return structs.ErrMissingCSIVolumeID
			}

			reply.Volume = vol
			return v.srv.replySetCSIVolumeIndex(state, &reply.QueryMeta)
		}}
	return v.srv.blockingRPC(&opts)
}

// Register registers a new volume
func (v *CSIVolume) Register(args *structs.CSIVolumeRegisterRequest, reply *structs.CSIVolumeRegisterResponse) error {
	if done, err := v.srv.forward("CSIVolume.Register", args, args, reply); done {
		return err
	}

	aclObj, err := v.srv.WriteACLObj(&args.WriteRequest)
	if err != nil {
		return err
	}

	metricsStart := time.Now()
	defer metrics.MeasureSince([]string{"nomad", "volume", "register"}, metricsStart)

	if !aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilityCSICreateVolume) {
		return structs.ErrPermissionDenied
	}

	// This is the only namespace we ACL checked, force all the volumes to use it
	for _, v := range args.Volumes {
		v.Namespace = args.RequestNamespace()
		if err = v.Validate(); err != nil {
			return err
		}
	}

	state := v.srv.State()
	index, err := state.LatestIndex()
	if err != nil {
		return err
	}

	err = state.CSIVolumeRegister(index, args.Volumes)
	if err != nil {
		return err
	}

	return v.srv.replySetCSIVolumeIndex(state, &reply.QueryMeta)
}

// Deregister removes a set of volumes
func (v *CSIVolume) Deregister(args *structs.CSIVolumeDeregisterRequest, reply *structs.CSIVolumeDeregisterResponse) error {
	if done, err := v.srv.forward("CSIVolume.Deregister", args, args, reply); done {
		return err
	}

	aclObj, err := v.srv.WriteACLObj(&args.WriteRequest)
	if err != nil {
		return err
	}

	metricsStart := time.Now()
	defer metrics.MeasureSince([]string{"nomad", "volume", "deregister"}, metricsStart)

	ns := args.RequestNamespace()
	if !aclObj.AllowNsOp(ns, acl.NamespaceCapabilityCSICreateVolume) {
		return structs.ErrPermissionDenied
	}

	state := v.srv.State()
	index, err := state.LatestIndex()
	if err != nil {
		return err
	}

	err = state.CSIVolumeDeregister(index, args.VolumeIDs)
	if err != nil {
		return err
	}

	return v.srv.replySetCSIVolumeIndex(state, &reply.QueryMeta)
}
