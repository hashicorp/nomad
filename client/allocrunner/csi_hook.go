// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package allocrunner

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/dynamicplugins"
	"github.com/hashicorp/nomad/client/pluginmanager/csimanager"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
)

// csiHook will wait for remote csi volumes to be attached to the host before
// continuing.
//
// It is a noop for allocs that do not depend on CSI Volumes.
type csiHook struct {
	alloc      *structs.Allocation
	logger     hclog.Logger
	csimanager csimanager.Manager

	// interfaces implemented by the allocRunner
	rpcClient            config.RPCer
	taskCapabilityGetter taskCapabilityGetter
	hookResources        *cstructs.AllocHookResources

	nodeSecret         string
	volumeRequests     map[string]*volumeAndRequest
	minBackoffInterval time.Duration
	maxBackoffInterval time.Duration
	maxBackoffDuration time.Duration

	shutdownCtx      context.Context
	shutdownCancelFn context.CancelFunc
}

// implemented by allocrunner
type taskCapabilityGetter interface {
	GetTaskDriverCapabilities(string) (*drivers.Capabilities, error)
}

func newCSIHook(alloc *structs.Allocation, logger hclog.Logger, csi csimanager.Manager, rpcClient config.RPCer, taskCapabilityGetter taskCapabilityGetter, hookResources *cstructs.AllocHookResources, nodeSecret string) *csiHook {

	shutdownCtx, shutdownCancelFn := context.WithCancel(context.Background())

	return &csiHook{
		alloc:                alloc,
		logger:               logger.Named("csi_hook"),
		csimanager:           csi,
		rpcClient:            rpcClient,
		taskCapabilityGetter: taskCapabilityGetter,
		hookResources:        hookResources,
		nodeSecret:           nodeSecret,
		volumeRequests:       map[string]*volumeAndRequest{},
		minBackoffInterval:   time.Second,
		maxBackoffInterval:   time.Minute,
		maxBackoffDuration:   time.Hour * 24,
		shutdownCtx:          shutdownCtx,
		shutdownCancelFn:     shutdownCancelFn,
	}
}

func (c *csiHook) Name() string {
	return "csi_hook"
}

func (c *csiHook) Prerun() error {
	if !c.shouldRun() {
		return nil
	}

	volumes, err := c.claimVolumesFromAlloc()
	if err != nil {
		return fmt.Errorf("claim volumes: %v", err)
	}
	c.volumeRequests = volumes

	mounts := make(map[string]*csimanager.MountInfo, len(volumes))
	for alias, pair := range volumes {

		// make sure the plugin is ready or becomes so quickly.
		plugin := pair.volume.PluginID
		pType := dynamicplugins.PluginTypeCSINode
		if err := c.csimanager.WaitForPlugin(c.shutdownCtx, pType, plugin); err != nil {
			return err
		}
		c.logger.Debug("found CSI plugin", "type", pType, "name", plugin)

		mounter, err := c.csimanager.MounterForPlugin(c.shutdownCtx, plugin)
		if err != nil {
			return err
		}

		usageOpts := &csimanager.UsageOptions{
			ReadOnly:       pair.request.ReadOnly,
			AttachmentMode: pair.request.AttachmentMode,
			AccessMode:     pair.request.AccessMode,
			MountOptions:   pair.request.MountOptions,
		}

		mountInfo, err := mounter.MountVolume(
			c.shutdownCtx, pair.volume, c.alloc, usageOpts, pair.publishContext)
		if err != nil {
			return err
		}

		mounts[alias] = mountInfo
	}

	// make the mounts available to the taskrunner's volume_hook
	c.hookResources.SetCSIMounts(mounts)

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
		// CSI RPCs can potentially take a long time. Split the work
		// into goroutines so that operators could potentially reuse
		// one of a set of volumes
		go func(pair *volumeAndRequest) {
			defer wg.Done()
			err := c.unmountImpl(pair)
			if err != nil {
				// we can recover an unmount failure if the operator
				// brings the plugin back up, so retry every few minutes
				// but eventually give up. Don't block shutdown so that
				// we don't block shutting down the client in -dev mode
				go func(pair *volumeAndRequest) {
					err := c.unmountWithRetry(pair)
					if err != nil {
						c.logger.Error("volume could not be unmounted")
					}
					err = c.unpublish(pair)
					if err != nil {
						c.logger.Error("volume could not be unpublished")
					}
				}(pair)
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
	supportsVolumes := false

	for _, task := range tg.Tasks {
		caps, err := c.taskCapabilityGetter.GetTaskDriverCapabilities(task.Name)
		if err != nil {
			return nil, fmt.Errorf("could not validate task driver capabilities: %v", err)
		}

		if caps.MountConfigs == drivers.MountConfigSupportNone {
			continue
		}

		supportsVolumes = true
		break
	}

	if !supportsVolumes {
		return nil, fmt.Errorf("no task supports CSI")
	}

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

		resp, err := c.claimWithRetry(req)
		if err != nil {
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

// claimWithRetry tries to claim the volume on the server, retrying
// with exponential backoff capped to a maximum interval
func (c *csiHook) claimWithRetry(req *structs.CSIVolumeClaimRequest) (*structs.CSIVolumeClaimResponse, error) {

	ctx, cancel := context.WithTimeout(c.shutdownCtx, c.maxBackoffDuration)
	defer cancel()

	var resp structs.CSIVolumeClaimResponse
	var err error
	backoff := c.minBackoffInterval
	t, stop := helper.NewSafeTimer(0)
	defer stop()
	for {
		select {
		case <-ctx.Done():
			return nil, err
		case <-t.C:
		}

		err = c.rpcClient.RPC("CSIVolume.Claim", req, &resp)
		if err == nil {
			break
		}

		if !isRetryableClaimRPCError(err) {
			break
		}

		if backoff < c.maxBackoffInterval {
			backoff = backoff * 2
			if backoff > c.maxBackoffInterval {
				backoff = c.maxBackoffInterval
			}
		}
		c.logger.Debug(
			"volume could not be claimed because it is in use", "retry_in", backoff)
		t.Reset(backoff)
	}
	return &resp, err
}

// isRetryableClaimRPCError looks for errors where we need to retry
// with backoff because we expect them to be eventually resolved.
func isRetryableClaimRPCError(err error) bool {

	// note: because these errors are returned via RPC which breaks error
	// wrapping, we can't check with errors.Is and need to read the string
	errMsg := err.Error()
	if strings.Contains(errMsg, structs.ErrCSIVolumeMaxClaims.Error()) {
		return true
	}
	if strings.Contains(errMsg, structs.ErrCSIClientRPCRetryable.Error()) {
		return true
	}
	if strings.Contains(errMsg, "no servers") {
		return true
	}
	if strings.Contains(errMsg, structs.ErrNoLeader.Error()) {
		return true
	}
	return false
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

	ctx, cancel := context.WithTimeout(c.shutdownCtx, c.maxBackoffDuration)
	defer cancel()
	var err error
	backoff := c.minBackoffInterval
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
		c.logger.Debug("volume could not be unmounted", "retry_in", backoff)
		t.Reset(backoff)
	}
	return nil
}

// unmountImpl implements the call to the CSI plugin manager to
// unmount the volume. Each retry will write an "Unmount volume"
// NodeEvent
func (c *csiHook) unmountImpl(pair *volumeAndRequest) error {

	mounter, err := c.csimanager.MounterForPlugin(c.shutdownCtx, pair.volume.PluginID)
	if err != nil {
		return err
	}

	usageOpts := &csimanager.UsageOptions{
		ReadOnly:       pair.request.ReadOnly,
		AttachmentMode: pair.request.AttachmentMode,
		AccessMode:     pair.request.AccessMode,
		MountOptions:   pair.request.MountOptions,
	}

	return mounter.UnmountVolume(c.shutdownCtx,
		pair.volume.ID, pair.volume.RemoteID(), c.alloc.ID, usageOpts)
}

// Shutdown will get called when the client is gracefully
// stopping. Cancel our shutdown context so that we don't block client
// shutdown while in the CSI RPC retry loop.
func (c *csiHook) Shutdown() {
	c.logger.Trace("shutting down hook")
	c.shutdownCancelFn()
}

// Destroy will get called when an allocation gets GC'd on the client
// or when a -dev mode client is stopped. Cancel our shutdown context
// so that we don't block client shutdown while in the CSI RPC retry
// loop.
func (c *csiHook) Destroy() {
	c.logger.Trace("destroying hook")
	c.shutdownCancelFn()
}
