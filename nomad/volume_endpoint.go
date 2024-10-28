package nomad

import (
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/acl"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
)

// HostVolume wraps the structs.Volume with request data and server context
type HostVolume struct {
	srv    *Server
	ctx    *RPCContext
	logger hclog.Logger
}

func NewHostVolumeEndpoint(srv *Server, ctx *RPCContext) *HostVolume {
	return &HostVolume{srv: srv, ctx: ctx, logger: srv.logger.Named("host_volume")}
}

func (v *HostVolume) Create(args *structs.HostVolumeCreateRequest, reply *structs.HostVolumeCreateResponse) error {

	authErr := v.srv.Authenticate(v.ctx, args)
	if done, err := v.srv.forward("Volume.Create", args, args, reply); done {
		return err
	}
	v.srv.MeasureRPCRate("volume", structs.RateMetricWrite, args)
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

	type validated struct {
		vol     *structs.CSIVolume
		current *structs.CSIVolume
	}

	validatedVols := []*validated{}

	// TODO: should this really take multiple volumes like CSI does? we don't
	// use it in CSI and it makes error handling
	for _, vol := range args.Volumes {
		// This is the only namespace we ACL checked, force all the volumes to
		// use it.
		if vol.Namespace == "" {
			vol.Namespace = args.RequestNamespace()
		}
		if err = vol.Validate(); err != nil {
			return err
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
			&validated{vol, current})
	}

	// Attempt to create all the validated volumes and write only successfully
	// created volumes to raft. And we'll report errors for any failed volumes
	//
	// NOTE: creating the volume in the external storage provider can't be
	// made atomic with the registration, and creating the volume provides
	// values we want to write on the Volume in raft anyways.

	var mErr multierror.Error
	var index uint64

	args.Volumes = []*structs.CSIVolume{}

	for _, valid := range validatedVols {
		if valid.current != nil {
			// TODO: valid.current.Copy() and reconcile mutable fields of volumes
		} else {
			err = v.createVolume(valid.vol)
			if err != nil {
				mErr.Errors = append(mErr.Errors, err)
			} else {
				args.Volumes = append(args.Volumes, valid.vol)
			}
		}
	}

	// If we created or updated volumes, apply them to raft.
	if len(args.Volumes) > 0 {
		_, index, err = v.srv.raftApply(structs.CSIVolumeRegisterRequestType, args)
		if err != nil {
			v.logger.Error("raft apply failed", "error", err, "method", "register")
			mErr.Errors = append(mErr.Errors, err)
		}
	}

	err = mErr.ErrorOrNil()
	if err != nil {
		return err
	}

	reply.Volumes = args.Volumes
	reply.Index = index
	return nil
}

func (v *HostVolume) createVolume(vol *structs.CSIVolume) error {

	method := "ClientHostVolume.Create"
	cReq := &cstructs.ClientHostVolumeCreateRequest{
		Name:                vol.Name,
		NodeID:              vol.NodeID,
		Plugin:              vol.PluginID,
		VolumeCapabilities:  vol.RequestedCapabilities,
		MountOptions:        vol.MountOptions,
		CapacityMin:         vol.RequestedCapacityMin,
		CapacityMax:         vol.RequestedCapacityMax,
		RequestedTopologies: vol.RequestedTopologies,
	}
	cResp := &cstructs.ClientHostVolumeCreateResponse{}
	err := v.srv.RPC(method, cReq, cResp)
	if err != nil {
		return err
	}

	vol.ID = cResp.ID
	vol.HostPath = cResp.Path
	vol.Capacity = cResp.CapacityBytes
	vol.Context = cResp.VolumeContext
	vol.Topologies = cResp.Topologies

	return nil
}
