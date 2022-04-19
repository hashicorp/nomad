package allocrunner

import (
	"context"
	"fmt"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/pluginmanager/csimanager"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
)

// csiHook will wait for remote csi volumes to be attached to the host before
// continuing.
//
// It is a noop for allocs that do not depend on CSI Volumes.
type csiHook struct {
	alloc                *structs.Allocation
	logger               hclog.Logger
	csimanager           csimanager.Manager
	rpcClient            RPCer
	taskCapabilityGetter taskCapabilityGetter
	updater              hookResourceSetter
	nodeSecret           string

	volumeRequests     map[string]*volumeAndRequest
	maxBackoffInterval time.Duration
	maxBackoffDuration time.Duration
}

// implemented by allocrunner
type taskCapabilityGetter interface {
	GetTaskDriverCapabilities(string) (*drivers.Capabilities, error)
}

func newCSIHook(alloc *structs.Allocation, logger hclog.Logger, csi csimanager.Manager, rpcClient RPCer, taskCapabilityGetter taskCapabilityGetter, updater hookResourceSetter, nodeSecret string) *csiHook {
	return &csiHook{
		alloc:                alloc,
		logger:               logger.Named("csi_hook"),
		csimanager:           csi,
		rpcClient:            rpcClient,
		taskCapabilityGetter: taskCapabilityGetter,
		updater:              updater,
		nodeSecret:           nodeSecret,
		volumeRequests:       map[string]*volumeAndRequest{},
		maxBackoffInterval:   time.Minute,
		maxBackoffDuration:   time.Hour * 24,
	}
}

func (c *csiHook) Name() string {
	return "csi_hook"
}

func (c *csiHook) Prerun() error {
	if !c.shouldRun() {
		return nil
	}

	// We use this context only to attach hclog to the gRPC context. The
	// lifetime is the lifetime of the gRPC stream, not specific RPC timeouts,
	// but we manage the stream lifetime via Close in the pluginmanager.
	ctx := context.Background()

	volumes, err := c.claimVolumesFromAlloc()
	if err != nil {
		return fmt.Errorf("claim volumes: %v", err)
	}
	c.volumeRequests = volumes

	mounts := make(map[string]*csimanager.MountInfo, len(volumes))
	for alias, pair := range volumes {
		mounter, err := c.csimanager.MounterForPlugin(ctx, pair.volume.PluginID)
		if err != nil {
			return err
		}

		usageOpts := &csimanager.UsageOptions{
			ReadOnly:       pair.request.ReadOnly,
			AttachmentMode: pair.request.AttachmentMode,
			AccessMode:     pair.request.AccessMode,
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

// Postrun sends an RPC to the server to unpublish the volume. This may
// forward client RPCs to the node plugins or to the controller plugins,
// depending on whether other allocations on this node have claims on this
// volume.
func (c *csiHook) Postrun() error {
	if !c.shouldRun() {
		return nil
	}

	var wg sync.WaitGroup
	errs := make(chan error, len(c.volumeRequests))

	for _, pair := range c.volumeRequests {
		wg.Add(1)

		// CSI RPCs can potentially fail for a very long time if a
		// node plugin has failed. split the work into goroutines so
		// that operators could potentially reuse one of a set of
		// volumes even if this hook is stuck waiting on the others
		go func(pair *volumeAndRequest) {
			defer wg.Done()

			// we can recover an unmount failure if the operator
			// brings the plugin back up, so retry every few minutes
			// but eventually give up
			err := c.unmountWithRetry(pair)
			if err != nil {
				errs <- err
				return
			}

			// we can't recover from this RPC error client-side; the
			// volume claim GC job will have to clean up for us once
			// the allocation is marked terminal
			errs <- c.unpublish(pair)
		}(pair)
	}

	wg.Wait()
	close(errs) // so we don't block waiting if there were no errors

	var mErr *multierror.Error
	for err := range errs {
		mErr = multierror.Append(mErr, err)
	}

	return mErr.ErrorOrNil()
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

			for _, task := range tg.Tasks {
				caps, err := c.taskCapabilityGetter.GetTaskDriverCapabilities(task.Name)
				if err != nil {
					return nil, fmt.Errorf("could not validate task driver capabilities: %v", err)
				}

				if caps.MountConfigs == drivers.MountConfigSupportNone {
					return nil, fmt.Errorf(
						"task driver %q for %q does not support CSI", task.Driver, task.Name)
				}
			}

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

		source := pair.request.Source
		if pair.request.PerAlloc {
			source = source + structs.AllocSuffix(c.alloc.Name)
		}

		req := &structs.CSIVolumeClaimRequest{
			VolumeID:       source,
			AllocationID:   c.alloc.ID,
			NodeID:         c.alloc.NodeID,
			Claim:          claimType,
			AccessMode:     pair.request.AccessMode,
			AttachmentMode: pair.request.AttachmentMode,
			WriteRequest: structs.WriteRequest{
				Region:    c.alloc.Job.Region,
				Namespace: c.alloc.Job.Namespace,
				AuthToken: c.nodeSecret,
			},
		}

		var resp structs.CSIVolumeClaimResponse
		if err := c.rpcClient.RPC("CSIVolume.Claim", req, &resp); err != nil {
			return nil, fmt.Errorf("could not claim volume %s: %w", req.VolumeID, err)
		}

		if resp.Volume == nil {
			return nil, fmt.Errorf("Unexpected nil volume returned for ID: %v", pair.request.Source)
		}

		result[alias].request = pair.request
		result[alias].volume = resp.Volume
		result[alias].publishContext = resp.PublishContext
	}

	return result, nil
}

func (c *csiHook) shouldRun() bool {
	tg := c.alloc.Job.LookupTaskGroup(c.alloc.TaskGroup)
	for _, vol := range tg.Volumes {
		if vol.Type == structs.VolumeTypeCSI {
			return true
		}
	}

	return false
}

func (c *csiHook) unpublish(pair *volumeAndRequest) error {

	mode := structs.CSIVolumeClaimRead
	if !pair.request.ReadOnly {
		mode = structs.CSIVolumeClaimWrite
	}

	source := pair.request.Source
	if pair.request.PerAlloc {
		// NOTE: PerAlloc can't be set if we have canaries
		source = source + structs.AllocSuffix(c.alloc.Name)
	}

	req := &structs.CSIVolumeUnpublishRequest{
		VolumeID: source,
		Claim: &structs.CSIVolumeClaim{
			AllocationID: c.alloc.ID,
			NodeID:       c.alloc.NodeID,
			Mode:         mode,
			State:        structs.CSIVolumeClaimStateUnpublishing,
		},
		WriteRequest: structs.WriteRequest{
			Region:    c.alloc.Job.Region,
			Namespace: c.alloc.Job.Namespace,
			AuthToken: c.nodeSecret,
		},
	}

	return c.rpcClient.RPC("CSIVolume.Unpublish",
		req, &structs.CSIVolumeUnpublishResponse{})

}

// unmountWithRetry tries to unmount/unstage the volume, retrying with
// exponential backoff capped to a maximum interval
func (c *csiHook) unmountWithRetry(pair *volumeAndRequest) error {

	// note: allocrunner hooks don't have access to the client's
	// shutdown context, just the allocrunner's shutdown; if we make
	// it available in the future we should thread it through here so
	// that retry can exit gracefully instead of dropping the
	// in-flight goroutine
	ctx, cancel := context.WithTimeout(context.TODO(), c.maxBackoffDuration)
	defer cancel()
	var err error
	backoff := time.Second
	t, stop := helper.NewSafeTimer(0)
	defer stop()
	for {
		select {
		case <-ctx.Done():
			return err
		case <-t.C:
		}

		err = c.unmountImpl(pair)
		if err == nil {
			break
		}

		if backoff < c.maxBackoffInterval {
			backoff = backoff * 2
			if backoff > c.maxBackoffInterval {
				backoff = c.maxBackoffInterval
			}
		}
		t.Reset(backoff)
	}
	return nil
}

// unmountImpl implements the call to the CSI plugin manager to
// unmount the volume. Each retry will write an "Unmount volume"
// NodeEvent
func (c *csiHook) unmountImpl(pair *volumeAndRequest) error {

	mounter, err := c.csimanager.MounterForPlugin(context.TODO(), pair.volume.PluginID)
	if err != nil {
		return err
	}

	usageOpts := &csimanager.UsageOptions{
		ReadOnly:       pair.request.ReadOnly,
		AttachmentMode: pair.request.AttachmentMode,
		AccessMode:     pair.request.AccessMode,
		MountOptions:   pair.request.MountOptions,
	}

	return mounter.UnmountVolume(context.TODO(),
		pair.volume.ID, pair.volume.RemoteID(), c.alloc.ID, usageOpts)
}
