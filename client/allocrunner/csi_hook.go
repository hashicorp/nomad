// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/allocrunner/state"
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
	rpcClient       config.RPCer
	allocRunnerShim allocRunnerShim
	hookResources   *cstructs.AllocHookResources

	nodeSecret         string
	minBackoffInterval time.Duration
	maxBackoffInterval time.Duration
	maxBackoffDuration time.Duration

	volumeResultsLock sync.Mutex
	volumeResults     map[string]*volumePublishResult // alias -> volumePublishResult

	shutdownCtx      context.Context
	shutdownCancelFn context.CancelFunc
}

// implemented by allocrunner
type allocRunnerShim interface {
	GetTaskDriverCapabilities(string) (*drivers.Capabilities, error)
	SetCSIVolumes(vols map[string]*state.CSIVolumeStub) error
	GetCSIVolumes() (map[string]*state.CSIVolumeStub, error)
}

func newCSIHook(alloc *structs.Allocation, logger hclog.Logger, csi csimanager.Manager, rpcClient config.RPCer, arShim allocRunnerShim, hookResources *cstructs.AllocHookResources, nodeSecret string) *csiHook {

	shutdownCtx, shutdownCancelFn := context.WithCancel(context.Background())

	return &csiHook{
		alloc:              alloc,
		logger:             logger.Named("csi_hook"),
		csimanager:         csi,
		rpcClient:          rpcClient,
		allocRunnerShim:    arShim,
		hookResources:      hookResources,
		nodeSecret:         nodeSecret,
		volumeResults:      map[string]*volumePublishResult{},
		minBackoffInterval: time.Second,
		maxBackoffInterval: time.Minute,
		maxBackoffDuration: time.Hour * 24,
		shutdownCtx:        shutdownCtx,
		shutdownCancelFn:   shutdownCancelFn,
	}
}

func (c *csiHook) Name() string {
	return "csi_hook"
}

func (c *csiHook) Prerun() error {
	if !c.shouldRun() {
		return nil
	}

	tg := c.alloc.Job.LookupTaskGroup(c.alloc.TaskGroup)
	if err := c.validateTasksSupportCSI(tg); err != nil {
		return err
	}

	// Because operations on CSI volumes are expensive and can error, we do each
	// step for all volumes before proceeding to the next step so we have to
	// unwind less work. In practice, most allocations with volumes will only
	// have one or a few at most. We lock the results so that if an update/stop
	// comes in while we're running we can assert we'll safely tear down
	// everything that's been done so far.

	c.volumeResultsLock.Lock()
	defer c.volumeResultsLock.Unlock()

	// Initially, populate the result map with all of the requests
	for alias, volumeRequest := range tg.Volumes {
		if volumeRequest.Type == structs.VolumeTypeCSI {
			c.volumeResults[alias] = &volumePublishResult{
				request: volumeRequest,
				stub: &state.CSIVolumeStub{
					VolumeID: volumeRequest.VolumeID(c.alloc.Name)},
			}
		}
	}

	err := c.restoreMounts(c.volumeResults)
	if err != nil {
		return fmt.Errorf("restoring mounts: %w", err)
	}

	err = c.claimVolumes(c.volumeResults)
	if err != nil {
		return fmt.Errorf("claiming volumes: %w", err)
	}

	err = c.mountVolumes(c.volumeResults)
	if err != nil {
		return fmt.Errorf("mounting volumes: %w", err)
	}

	// make the mounts available to the taskrunner's volume_hook
	mounts := helper.ConvertMap(c.volumeResults,
		func(result *volumePublishResult) *csimanager.MountInfo {
			return result.stub.MountInfo
		})
	c.hookResources.SetCSIMounts(mounts)

	// persist the published mount info so we can restore on client restarts
	stubs := helper.ConvertMap(c.volumeResults,
		func(result *volumePublishResult) *state.CSIVolumeStub {
			return result.stub
		})
	c.allocRunnerShim.SetCSIVolumes(stubs)

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

	c.volumeResultsLock.Lock()
	defer c.volumeResultsLock.Unlock()

	var wg sync.WaitGroup
	errs := make(chan error, len(c.volumeResults))

	for _, result := range c.volumeResults {
		wg.Add(1)
		// CSI RPCs can potentially take a long time. Split the work
		// into goroutines so that operators could potentially reuse
		// one of a set of volumes
		go func(result *volumePublishResult) {
			defer wg.Done()
			err := c.unmountImpl(result)
			if err != nil {
				// we can recover an unmount failure if the operator
				// brings the plugin back up, so retry every few minutes
				// but eventually give up. Don't block shutdown so that
				// we don't block shutting down the client in -dev mode
				go func(result *volumePublishResult) {
					err := c.unmountWithRetry(result)
					if err != nil {
						c.logger.Error("volume could not be unmounted")
					}
					err = c.unpublish(result)
					if err != nil {
						c.logger.Error("volume could not be unpublished")
					}
				}(result)
			}

			// we can't recover from this RPC error client-side; the
			// volume claim GC job will have to clean up for us once
			// the allocation is marked terminal
			errs <- c.unpublish(result)
		}(result)
	}

	wg.Wait()
	close(errs) // so we don't block waiting if there were no errors

	var mErr *multierror.Error
	for err := range errs {
		mErr = multierror.Append(mErr, err)
	}

	return mErr.ErrorOrNil()
}

type volumePublishResult struct {
	request        *structs.VolumeRequest // the request from the jobspec
	volume         *structs.CSIVolume     // the volume we get back from the server
	publishContext map[string]string      // populated after claim if provided by plugin
	stub           *state.CSIVolumeStub   // populated from volume, plugin, or stub
}

// validateTasksSupportCSI verifies that at least one task in the group uses a
// task driver that supports CSI. This prevents us from publishing CSI volumes
// only to find out once we get to the taskrunner/volume_hook that no task can
// mount them.
func (c *csiHook) validateTasksSupportCSI(tg *structs.TaskGroup) error {

	for _, task := range tg.Tasks {
		caps, err := c.allocRunnerShim.GetTaskDriverCapabilities(task.Name)
		if err != nil {
			return fmt.Errorf("could not validate task driver capabilities: %v", err)
		}

		if caps.MountConfigs == drivers.MountConfigSupportNone {
			continue
		}

		return nil
	}

	return fmt.Errorf("no task supports CSI")
}

// restoreMounts tries to restore the mount info from the local client state and
// then verifies it with the plugin. If the volume is already mounted, we don't
// want to re-run the claim and mount workflow again. This lets us tolerate
// restarting clients even on disconnected nodes.
func (c *csiHook) restoreMounts(results map[string]*volumePublishResult) error {
	stubs, err := c.allocRunnerShim.GetCSIVolumes()
	if err != nil {
		return err
	}
	if stubs == nil {
		return nil // no previous volumes
	}
	for _, result := range results {
		stub := stubs[result.request.Name]
		if stub == nil {
			continue
		}

		result.stub = stub

		if result.stub.MountInfo != nil && result.stub.PluginID != "" {

			// make sure the plugin is ready or becomes so quickly.
			plugin := result.stub.PluginID
			pType := dynamicplugins.PluginTypeCSINode
			if err := c.csimanager.WaitForPlugin(c.shutdownCtx, pType, plugin); err != nil {
				return err
			}
			c.logger.Debug("found CSI plugin", "type", pType, "name", plugin)

			mounter, err := c.csimanager.MounterForPlugin(c.shutdownCtx, plugin)
			if err != nil {
				return err
			}

			isMounted, err := mounter.HasMount(c.shutdownCtx, result.stub.MountInfo)
			if err != nil {
				return err
			}
			if !isMounted {
				// the mount is gone, so clear this from our result state so it
				// we can try to remount it with the plugin ID we have
				result.stub.MountInfo = nil
			}
		}
	}

	return nil
}

// claimVolumes sends a claim to the server for each volume to mark it in use
// and kick off the controller publish workflow (optionally)
func (c *csiHook) claimVolumes(results map[string]*volumePublishResult) error {

	for _, result := range results {
		if result.stub.MountInfo != nil {
			continue // already mounted
		}

		request := result.request

		claimType := structs.CSIVolumeClaimWrite
		if request.ReadOnly {
			claimType = structs.CSIVolumeClaimRead
		}

		req := &structs.CSIVolumeClaimRequest{
			VolumeID:       result.stub.VolumeID,
			AllocationID:   c.alloc.ID,
			NodeID:         c.alloc.NodeID,
			ExternalNodeID: result.stub.ExternalNodeID,
			Claim:          claimType,
			AccessMode:     request.AccessMode,
			AttachmentMode: request.AttachmentMode,
			WriteRequest: structs.WriteRequest{
				Region:    c.alloc.Job.Region,
				Namespace: c.alloc.Job.Namespace,
				AuthToken: c.nodeSecret,
			},
		}

		resp, err := c.claimWithRetry(req)
		if err != nil {
			return fmt.Errorf("could not claim volume %s: %w", req.VolumeID, err)
		}
		if resp.Volume == nil {
			return fmt.Errorf("Unexpected nil volume returned for ID: %v", request.Source)
		}

		result.volume = resp.Volume

		// populate data we'll write later to disk
		result.stub.VolumeID = resp.Volume.ID
		result.stub.VolumeExternalID = resp.Volume.RemoteID()
		result.stub.PluginID = resp.Volume.PluginID
		result.publishContext = resp.PublishContext
	}

	return nil
}

func (c *csiHook) mountVolumes(results map[string]*volumePublishResult) error {

	for _, result := range results {
		if result.stub.MountInfo != nil {
			continue // already mounted
		}
		if result.volume == nil {
			return fmt.Errorf("volume not available from claim for mounting volume request %q",
				result.request.Name) // should be unreachable
		}

		// make sure the plugin is ready or becomes so quickly.
		plugin := result.volume.PluginID
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
			ReadOnly:       result.request.ReadOnly,
			AttachmentMode: result.request.AttachmentMode,
			AccessMode:     result.request.AccessMode,
			MountOptions:   result.request.MountOptions,
		}

		mountInfo, err := mounter.MountVolume(
			c.shutdownCtx, result.volume, c.alloc, usageOpts, result.publishContext)
		if err != nil {
			return err
		}
		result.stub.MountInfo = mountInfo
	}

	return nil
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

func (c *csiHook) unpublish(result *volumePublishResult) error {

	mode := structs.CSIVolumeClaimRead
	if !result.request.ReadOnly {
		mode = structs.CSIVolumeClaimWrite
	}

	source := result.request.Source
	if result.request.PerAlloc {
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
func (c *csiHook) unmountWithRetry(result *volumePublishResult) error {

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

		err = c.unmountImpl(result)
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
func (c *csiHook) unmountImpl(result *volumePublishResult) error {

	mounter, err := c.csimanager.MounterForPlugin(c.shutdownCtx, result.stub.PluginID)
	if err != nil {
		return err
	}

	usageOpts := &csimanager.UsageOptions{
		ReadOnly:       result.request.ReadOnly,
		AttachmentMode: result.request.AttachmentMode,
		AccessMode:     result.request.AccessMode,
		MountOptions:   result.request.MountOptions,
	}

	return mounter.UnmountVolume(c.shutdownCtx,
		result.stub.VolumeID, result.stub.VolumeExternalID, c.alloc.ID, usageOpts)
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
