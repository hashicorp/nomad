package allocrunner

import (
	"context"
	"fmt"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/pluginmanager/csimanager"
	"github.com/hashicorp/nomad/nomad/structs"
)

// RPCer is the interface needed by a csiHook to make RPC calls.
type RPCer interface {
	// RPC allows retrieving volume info.
	RPC(method string, args interface{}, reply interface{}) error
}

// csiHook will wait for remote csi volumes to be attached to the host before
// continuing.
//
// It is a noop for allocs that do not depend on CSI Volumes.
type csiHook struct {
	alloc      *structs.Allocation
	logger     hclog.Logger
	csimanager csimanager.Manager
	rpcClient  RPCer
	updater    allocResourceSetter
}

func (c *csiHook) Name() string {
	return "csi_hook"
}

func (c *csiHook) csiVolumesFromAlloc() (map[string]*structs.CSIVolume, error) {
	vols := make(map[string]*structs.VolumeRequest)
	tg := c.alloc.Job.LookupTaskGroup(c.alloc.TaskGroup)
	for alias, vol := range tg.Volumes {
		if vol.Type == structs.VolumeTypeCSI {
			vols[alias] = vol
		}
	}

	csiVols := make(map[string]*structs.CSIVolume, len(vols))
	for alias, request := range vols {
		req := &structs.CSIVolumeGetRequest{
			ID: request.Source,
		}
		req.Region = c.alloc.Job.Region

		var resp structs.CSIVolumeGetResponse
		err := c.rpcClient.RPC("CSIVolume.Get", req, &resp)

		if err != nil {
			return nil, err
		}

		if resp.Volume == nil {
			return nil, fmt.Errorf("Unexpected nil volume returned for ID: %v", request.Source)
		}

		csiVols[alias] = resp.Volume
	}

	return csiVols, nil
}

func (c *csiHook) Prerun() error {
	if !c.shouldRun() {
		return nil
	}
	ctx := context.TODO()
	volumes, err := c.csiVolumesFromAlloc()
	if err != nil {
		return err
	}

	mounts := make(map[string]*csimanager.MountInfo, len(volumes))

	for alias, volume := range volumes {
		mounter, err := c.csimanager.MounterForVolume(ctx, volume)
		if err != nil {
			return err
		}

		mountInfo, err := mounter.MountVolume(ctx, volume, c.alloc)
		if err != nil {
			return err
		}

		mounts[alias] = mountInfo
	}

	resources := c.updater.GetAllocResources()
	resources.CSIVolumeMountPoints = mounts
	c.updater.SetAllocResources(resources)

	return nil
}

func (c *csiHook) Postrun() error {
	// TODO: Unmount volumes
	return nil
}

func newCSIHook(logger hclog.Logger, csimanager csimanager.Manager, rpcClient RPCer, alloc *structs.Allocation, updater allocResourceSetter) *csiHook {
	return &csiHook{
		alloc:      alloc,
		logger:     logger.Named("csi_hook"),
		csimanager: csimanager,
		rpcClient:  rpcClient,
		updater:    updater,
	}
}

func (h *csiHook) shouldRun() bool {
	tg := h.alloc.Job.LookupTaskGroup(h.alloc.TaskGroup)
	for _, vol := range tg.Volumes {
		h.logger.Info("Found volume", "type", vol.Type)
		if vol.Type == structs.VolumeTypeCSI {
			return true
		}
	}

	h.logger.Error("Skipping CSI Hook")
	return false
}
