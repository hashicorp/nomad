package allocrunner

import (
	"context"
	"fmt"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/pluginmanager/csimanager"
	"github.com/hashicorp/nomad/nomad/structs"
)

// csiHook will wait for remote csi volumes to be attached to the host before
// continuing.
//
// It is a noop for allocs that do not depend on CSI Volumes.
type csiHook struct {
	alloc      *structs.Allocation
	logger     hclog.Logger
	csimanager csimanager.Manager
	rpcClient  RPCer
	updater    hookResourceSetter
}

func (c *csiHook) Name() string {
	return "csi_hook"
}

func (c *csiHook) Prerun() error {
	if !c.shouldRun() {
		return nil
	}

	// TODO(tgross): the contexts for the CSI RPC calls made during
	// mounting can have very long timeouts. Until these are better
	// tuned, there's not a good value to put here for a WithCancel
	// without risking conflicts with the grpc retries/timeouts in the
	// pluginmanager package.
	ctx := context.TODO()

	volumes, err := c.claimVolumesFromAlloc()
	if err != nil {
		return fmt.Errorf("claim volumes: %v", err)
	}

	mounts := make(map[string]*csimanager.MountInfo, len(volumes))
	for alias, pair := range volumes {
		mounter, err := c.csimanager.MounterForPlugin(ctx, pair.volume.PluginID)
		if err != nil {
			return err
		}

		usageOpts := &csimanager.UsageOptions{
			ReadOnly:       pair.request.ReadOnly,
			AttachmentMode: string(pair.volume.AttachmentMode),
			AccessMode:     string(pair.volume.AccessMode),
			MountOptions:   pair.request.MountOptions,
		}

		mountInfo, err := mounter.MountVolume(ctx, pair.volume, c.alloc, usageOpts, pair.publishContext)
		if err != nil {
			return err
		}

		mounts[alias] = mountInfo
	}

	res := c.updater.GetAllocHookResources()
	res.CSIMounts = mounts
	c.updater.SetAllocHookResources(res)

	return nil
}

type volumeAndRequest struct {
	volume  *structs.CSIVolume
	request *structs.VolumeRequest

	// When volumeAndRequest was returned from a volume claim, this field will be
	// populated for plugins that require it.
	publishContext map[string]string
}

// claimVolumesFromAlloc is used by the pre-run hook to fetch all of the volume
// metadata and claim it for use by this alloc/node at the same time.
func (c *csiHook) claimVolumesFromAlloc() (map[string]*volumeAndRequest, error) {
	result := make(map[string]*volumeAndRequest)
	tg := c.alloc.Job.LookupTaskGroup(c.alloc.TaskGroup)

	// Initially, populate the result map with all of the requests
	for alias, volumeRequest := range tg.Volumes {
		if volumeRequest.Type == structs.VolumeTypeCSI {
			result[alias] = &volumeAndRequest{request: volumeRequest}
		}
	}

	// Iterate over the result map and upsert the volume field as each volume gets
	// claimed by the server.
	for alias, pair := range result {
		claimType := structs.CSIVolumeClaimWrite
		if pair.request.ReadOnly {
			claimType = structs.CSIVolumeClaimRead
		}

		req := &structs.CSIVolumeClaimRequest{
			VolumeID:     pair.request.Source,
			AllocationID: c.alloc.ID,
			Claim:        claimType,
		}
		req.Region = c.alloc.Job.Region

		var resp structs.CSIVolumeClaimResponse
		if err := c.rpcClient.RPC("CSIVolume.Claim", req, &resp); err != nil {
			return nil, err
		}

		if resp.Volume == nil {
			return nil, fmt.Errorf("Unexpected nil volume returned for ID: %v", pair.request.Source)
		}

		result[alias].volume = resp.Volume
		result[alias].publishContext = resp.PublishContext
	}

	return result, nil
}

func newCSIHook(logger hclog.Logger, alloc *structs.Allocation, rpcClient RPCer, csi csimanager.Manager, updater hookResourceSetter) *csiHook {
	return &csiHook{
		alloc:      alloc,
		logger:     logger.Named("csi_hook"),
		rpcClient:  rpcClient,
		csimanager: csi,
		updater:    updater,
	}
}

func (h *csiHook) shouldRun() bool {
	tg := h.alloc.Job.LookupTaskGroup(h.alloc.TaskGroup)
	for _, vol := range tg.Volumes {
		if vol.Type == structs.VolumeTypeCSI {
			return true
		}
	}

	return false
}
