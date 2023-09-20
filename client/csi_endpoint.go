// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"context"
	"errors"
	"fmt"
	"time"

	metrics "github.com/armon/go-metrics"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"

	"github.com/hashicorp/nomad/client/dynamicplugins"
	"github.com/hashicorp/nomad/client/pluginmanager/csimanager"
	"github.com/hashicorp/nomad/client/structs"
	nstructs "github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/csi"
)

// CSI endpoint is used for interacting with CSI plugins on a client.
// TODO: Submit metrics with labels to allow debugging per plugin perf problems.
type CSI struct {
	c *Client
}

const (
	// CSIPluginRequestTimeout is the timeout that should be used when making reqs
	// against CSI Plugins. It is copied from Kubernetes as an initial seed value.
	// https://github.com/kubernetes/kubernetes/blob/e680ad7156f263a6d8129cc0117fda58602e50ad/pkg/volume/csi/csi_plugin.go#L52
	CSIPluginRequestTimeout = 2 * time.Minute
)

var (
	ErrPluginTypeError = errors.New("CSI Plugin loaded incorrectly")
)

// ControllerValidateVolume is used during volume registration to validate
// that a volume exists and that the capabilities it was registered with are
// supported by the CSI Plugin and external volume configuration.
func (c *CSI) ControllerValidateVolume(req *structs.ClientCSIControllerValidateVolumeRequest, resp *structs.ClientCSIControllerValidateVolumeResponse) error {
	defer metrics.MeasureSince([]string{"client", "csi_controller", "validate_volume"}, time.Now())

	if req.VolumeID == "" {
		return errors.New("CSI.ControllerValidateVolume: VolumeID is required")
	}

	if req.PluginID == "" {
		return errors.New("CSI.ControllerValidateVolume: PluginID is required")
	}

	plugin, err := c.findControllerPlugin(req.PluginID)
	if err != nil {
		// the server's view of the plugin health is stale, so let it know it
		// should retry with another controller instance
		return fmt.Errorf("CSI.ControllerValidateVolume: %w: %v",
			nstructs.ErrCSIClientRPCRetryable, err)
	}
	defer plugin.Close()

	csiReq, err := req.ToCSIRequest()
	if err != nil {
		return fmt.Errorf("CSI.ControllerValidateVolume: %v", err)
	}

	ctx, cancelFn := c.requestContext()
	defer cancelFn()

	// CSI ValidateVolumeCapabilities errors for timeout, codes.Unavailable and
	// codes.ResourceExhausted are retried; all other errors are fatal.
	err = plugin.ControllerValidateCapabilities(ctx, csiReq,
		grpc_retry.WithPerRetryTimeout(CSIPluginRequestTimeout),
		grpc_retry.WithMax(3),
		grpc_retry.WithBackoff(grpc_retry.BackoffExponential(100*time.Millisecond)))
	if err != nil {
		return fmt.Errorf("CSI.ControllerValidateVolume: %v", err)
	}
	return nil
}

// ControllerAttachVolume is used to attach a volume from a CSI Cluster to
// the storage node provided in the request.
//
// The controller attachment flow currently works as follows:
// 1. Validate the volume request
// 2. Call ControllerPublishVolume on the CSI Plugin to trigger a remote attachment
//
// In the future this may be expanded to request dynamic secrets for attachment.
func (c *CSI) ControllerAttachVolume(req *structs.ClientCSIControllerAttachVolumeRequest, resp *structs.ClientCSIControllerAttachVolumeResponse) error {
	defer metrics.MeasureSince([]string{"client", "csi_controller", "publish_volume"}, time.Now())
	plugin, err := c.findControllerPlugin(req.PluginID)
	if err != nil {
		// the server's view of the plugin health is stale, so let it know it
		// should retry with another controller instance
		return fmt.Errorf("CSI.ControllerAttachVolume: %w: %v",
			nstructs.ErrCSIClientRPCRetryable, err)
	}
	defer plugin.Close()

	// The following block of validation checks should not be reached on a
	// real Nomad cluster as all of this data should be validated when registering
	// volumes with the cluster. They serve as a defensive check before forwarding
	// requests to plugins, and to aid with development.

	if req.VolumeID == "" {
		return errors.New("CSI.ControllerAttachVolume: VolumeID is required")
	}

	if req.ClientCSINodeID == "" {
		return errors.New("CSI.ControllerAttachVolume: ClientCSINodeID is required")
	}

	csiReq, err := req.ToCSIRequest()
	if err != nil {
		return fmt.Errorf("CSI.ControllerAttachVolume: %v", err)
	}

	// Submit the request for a volume to the CSI Plugin.
	ctx, cancelFn := c.requestContext()
	defer cancelFn()
	// CSI ControllerPublishVolume errors for timeout, codes.Unavailable and
	// codes.ResourceExhausted are retried; all other errors are fatal.
	cresp, err := plugin.ControllerPublishVolume(ctx, csiReq,
		grpc_retry.WithPerRetryTimeout(CSIPluginRequestTimeout),
		grpc_retry.WithMax(3),
		grpc_retry.WithBackoff(grpc_retry.BackoffExponential(100*time.Millisecond)))
	if err != nil {
		return fmt.Errorf("CSI.ControllerAttachVolume: %v", err)
	}

	resp.PublishContext = cresp.PublishContext
	return nil
}

// ControllerDetachVolume is used to detach a volume from a CSI Cluster from
// the storage node provided in the request.
func (c *CSI) ControllerDetachVolume(req *structs.ClientCSIControllerDetachVolumeRequest, resp *structs.ClientCSIControllerDetachVolumeResponse) error {
	defer metrics.MeasureSince([]string{"client", "csi_controller", "unpublish_volume"}, time.Now())
	plugin, err := c.findControllerPlugin(req.PluginID)
	if err != nil {
		// the server's view of the plugin health is stale, so let it know it
		// should retry with another controller instance
		return fmt.Errorf("CSI.ControllerDetachVolume: %w: %v",
			nstructs.ErrCSIClientRPCRetryable, err)
	}
	defer plugin.Close()

	// The following block of validation checks should not be reached on a
	// real Nomad cluster as all of this data should be validated when registering
	// volumes with the cluster. They serve as a defensive check before forwarding
	// requests to plugins, and to aid with development.

	if req.VolumeID == "" {
		return errors.New("CSI.ControllerDetachVolume: VolumeID is required")
	}

	if req.ClientCSINodeID == "" {
		return errors.New("CSI.ControllerDetachVolume: ClientCSINodeID is required")
	}

	csiReq := req.ToCSIRequest()

	// Submit the request for a volume to the CSI Plugin.
	ctx, cancelFn := c.requestContext()
	defer cancelFn()
	// CSI ControllerUnpublishVolume errors for timeout, codes.Unavailable and
	// codes.ResourceExhausted are retried; all other errors are fatal.
	_, err = plugin.ControllerUnpublishVolume(ctx, csiReq,
		grpc_retry.WithPerRetryTimeout(CSIPluginRequestTimeout),
		grpc_retry.WithMax(3),
		grpc_retry.WithBackoff(grpc_retry.BackoffExponential(100*time.Millisecond)))
	if errors.Is(err, nstructs.ErrCSIClientRPCIgnorable) {
		// if the controller detach previously happened but the server failed to
		// checkpoint, we'll get an error from the plugin but can safely ignore it.
		c.c.logger.Debug("could not unpublish volume", "error", err)
		return nil
	}
	if err != nil {
		return fmt.Errorf("CSI.ControllerDetachVolume: %v", err)
	}
	return err
}

func (c *CSI) ControllerCreateVolume(req *structs.ClientCSIControllerCreateVolumeRequest, resp *structs.ClientCSIControllerCreateVolumeResponse) error {
	defer metrics.MeasureSince([]string{"client", "csi_controller", "create_volume"}, time.Now())

	plugin, err := c.findControllerPlugin(req.PluginID)
	if err != nil {
		// the server's view of the plugin health is stale, so let it know it
		// should retry with another controller instance
		return fmt.Errorf("CSI.ControllerCreateVolume: %w: %v",
			nstructs.ErrCSIClientRPCRetryable, err)
	}
	defer plugin.Close()

	csiReq, err := req.ToCSIRequest()
	if err != nil {
		return fmt.Errorf("CSI.ControllerCreateVolume: %v", err)
	}

	ctx, cancelFn := c.requestContext()
	defer cancelFn()

	// CSI ControllerCreateVolume errors for timeout, codes.Unavailable and
	// codes.ResourceExhausted are retried; all other errors are fatal.
	cresp, err := plugin.ControllerCreateVolume(ctx, csiReq,
		grpc_retry.WithPerRetryTimeout(CSIPluginRequestTimeout),
		grpc_retry.WithMax(3),
		grpc_retry.WithBackoff(grpc_retry.BackoffExponential(100*time.Millisecond)))
	if err != nil {
		return fmt.Errorf("CSI.ControllerCreateVolume: %v", err)
	}

	if cresp == nil || cresp.Volume == nil {
		c.c.logger.Warn("plugin did not return error or volume; this is a bug in the plugin and should be reported to the plugin author")
		return fmt.Errorf("CSI.ControllerCreateVolume: plugin did not return error or volume")
	}
	resp.ExternalVolumeID = cresp.Volume.ExternalVolumeID
	resp.CapacityBytes = cresp.Volume.CapacityBytes
	resp.VolumeContext = cresp.Volume.VolumeContext

	// Note: we safely throw away cresp.Volume.ContentSource here
	// because it's just round-tripping the value set by the user in
	// the server RPC call

	resp.Topologies = make([]*nstructs.CSITopology, len(cresp.Volume.AccessibleTopology))
	for _, topo := range cresp.Volume.AccessibleTopology {
		resp.Topologies = append(resp.Topologies,
			&nstructs.CSITopology{Segments: topo.Segments})
	}

	return nil
}

func (c *CSI) ControllerExpandVolume(req *structs.ClientCSIControllerExpandVolumeRequest, resp *structs.ClientCSIControllerExpandVolumeResponse) error {
	defer metrics.MeasureSince([]string{"client", "csi_controller", "expand_volume"}, time.Now())

	plugin, err := c.findControllerPlugin(req.PluginID)
	if err != nil {
		// the server's view of the plugin health is stale, so let it know it
		// should retry with another controller instance
		return fmt.Errorf("CSI.ControllerExpandVolume could not find plugin: %w: %v",
			nstructs.ErrCSIClientRPCRetryable, err)
	}
	defer plugin.Close()

	csiReq := req.ToCSIRequest()

	ctx, cancelFn := c.requestContext()
	defer cancelFn()

	// CSI ControllerExpandVolume errors for timeout, codes.Unavailable and
	// codes.ResourceExhausted are retried; all other errors are fatal.
	cresp, err := plugin.ControllerExpandVolume(ctx, csiReq,
		grpc_retry.WithPerRetryTimeout(CSIPluginRequestTimeout),
		grpc_retry.WithMax(3),
		grpc_retry.WithBackoff(grpc_retry.BackoffExponential(100*time.Millisecond)))
	if errors.Is(err, nstructs.ErrCSIClientRPCIgnorable) {
		// if the volume was deleted out-of-band, we'll get an error from
		// the plugin but can safely ignore it
		c.c.logger.Debug("could not expand volume", "error", err)
		return nil
	}
	if err != nil {
		return fmt.Errorf("CSI.ControllerExpandVolume: %v", err)
	}
	if cresp == nil {
		c.c.logger.Warn("plugin did not return error or response; this is a bug in the plugin and should be reported to the plugin author")
		return fmt.Errorf("CSI.ControllerExpandVolume: plugin did not return error or response")
	}
	resp.CapacityBytes = cresp.CapacityBytes
	resp.NodeExpansionRequired = cresp.NodeExpansionRequired
	return nil
}

func (c *CSI) ControllerDeleteVolume(req *structs.ClientCSIControllerDeleteVolumeRequest, resp *structs.ClientCSIControllerDeleteVolumeResponse) error {
	defer metrics.MeasureSince([]string{"client", "csi_controller", "delete_volume"}, time.Now())

	plugin, err := c.findControllerPlugin(req.PluginID)
	if err != nil {
		// the server's view of the plugin health is stale, so let it know it
		// should retry with another controller instance
		return fmt.Errorf("CSI.ControllerDeleteVolume: %w: %v",
			nstructs.ErrCSIClientRPCRetryable, err)
	}
	defer plugin.Close()

	csiReq := req.ToCSIRequest()

	ctx, cancelFn := c.requestContext()
	defer cancelFn()

	// CSI ControllerDeleteVolume errors for timeout, codes.Unavailable and
	// codes.ResourceExhausted are retried; all other errors are fatal.
	err = plugin.ControllerDeleteVolume(ctx, csiReq,
		grpc_retry.WithPerRetryTimeout(CSIPluginRequestTimeout),
		grpc_retry.WithMax(3),
		grpc_retry.WithBackoff(grpc_retry.BackoffExponential(100*time.Millisecond)))
	if errors.Is(err, nstructs.ErrCSIClientRPCIgnorable) {
		// if the volume was deleted out-of-band, we'll get an error from
		// the plugin but can safely ignore it
		c.c.logger.Debug("could not delete volume", "error", err)
		return nil
	}
	if err != nil {
		return fmt.Errorf("CSI.ControllerDeleteVolume: %v", err)
	}
	return err
}

func (c *CSI) ControllerListVolumes(req *structs.ClientCSIControllerListVolumesRequest, resp *structs.ClientCSIControllerListVolumesResponse) error {
	defer metrics.MeasureSince([]string{"client", "csi_controller", "list_volumes"}, time.Now())

	plugin, err := c.findControllerPlugin(req.PluginID)
	if err != nil {
		// the server's view of the plugin health is stale, so let it know it
		// should retry with another controller instance
		return fmt.Errorf("CSI.ControllerListVolumes: %w: %v",
			nstructs.ErrCSIClientRPCRetryable, err)
	}
	defer plugin.Close()

	csiReq := req.ToCSIRequest()

	ctx, cancelFn := c.requestContext()
	defer cancelFn()

	// CSI ControllerListVolumes errors for timeout, codes.Unavailable and
	// codes.ResourceExhausted are retried; all other errors are fatal.
	cresp, err := plugin.ControllerListVolumes(ctx, csiReq,
		grpc_retry.WithPerRetryTimeout(CSIPluginRequestTimeout),
		grpc_retry.WithMax(3),
		grpc_retry.WithBackoff(grpc_retry.BackoffExponential(100*time.Millisecond)))
	if err != nil {
		return fmt.Errorf("CSI.ControllerListVolumes: %v", err)
	}

	resp.NextToken = cresp.NextToken
	resp.Entries = []*nstructs.CSIVolumeExternalStub{}

	for _, entry := range cresp.Entries {
		if entry.Volume == nil {
			return fmt.Errorf("CSI.ControllerListVolumes: plugin returned an invalid entry")
		}
		vol := &nstructs.CSIVolumeExternalStub{
			ExternalID:    entry.Volume.ExternalVolumeID,
			CapacityBytes: entry.Volume.CapacityBytes,
			VolumeContext: entry.Volume.VolumeContext,
			CloneID:       entry.Volume.ContentSource.CloneID,
			SnapshotID:    entry.Volume.ContentSource.SnapshotID,
		}
		if entry.Status != nil {
			vol.PublishedExternalNodeIDs = entry.Status.PublishedNodeIds
			vol.IsAbnormal = entry.Status.VolumeCondition.Abnormal
			if entry.Status.VolumeCondition != nil {
				vol.Status = entry.Status.VolumeCondition.Message
			}
		}
		resp.Entries = append(resp.Entries, vol)
		if req.MaxEntries != 0 && int32(len(resp.Entries)) == req.MaxEntries {
			break
		}
	}

	return nil
}

func (c *CSI) ControllerCreateSnapshot(req *structs.ClientCSIControllerCreateSnapshotRequest, resp *structs.ClientCSIControllerCreateSnapshotResponse) error {
	defer metrics.MeasureSince([]string{"client", "csi_controller", "create_snapshot"}, time.Now())

	plugin, err := c.findControllerPlugin(req.PluginID)
	if err != nil {
		// the server's view of the plugin health is stale, so let it know it
		// should retry with another controller instance
		return fmt.Errorf("CSI.ControllerCreateSnapshot: %w: %v",
			nstructs.ErrCSIClientRPCRetryable, err)
	}
	defer plugin.Close()

	csiReq, err := req.ToCSIRequest()
	if err != nil {
		return fmt.Errorf("CSI.ControllerCreateSnapshot: %v", err)
	}

	ctx, cancelFn := c.requestContext()
	defer cancelFn()

	// CSI ControllerCreateSnapshot errors for timeout, codes.Unavailable and
	// codes.ResourceExhausted are retried; all other errors are fatal.
	cresp, err := plugin.ControllerCreateSnapshot(ctx, csiReq,
		grpc_retry.WithPerRetryTimeout(CSIPluginRequestTimeout),
		grpc_retry.WithMax(3),
		grpc_retry.WithBackoff(grpc_retry.BackoffExponential(100*time.Millisecond)))
	if err != nil {
		return fmt.Errorf("CSI.ControllerCreateSnapshot: %v", err)
	}

	if cresp == nil || cresp.Snapshot == nil {
		c.c.logger.Warn("plugin did not return error or snapshot; this is a bug in the plugin and should be reported to the plugin author")
		return fmt.Errorf("CSI.ControllerCreateSnapshot: plugin did not return error or snapshot")
	}
	resp.ID = cresp.Snapshot.ID
	resp.ExternalSourceVolumeID = cresp.Snapshot.SourceVolumeID
	resp.SizeBytes = cresp.Snapshot.SizeBytes
	resp.CreateTime = cresp.Snapshot.CreateTime
	resp.IsReady = cresp.Snapshot.IsReady

	return nil
}

func (c *CSI) ControllerDeleteSnapshot(req *structs.ClientCSIControllerDeleteSnapshotRequest, resp *structs.ClientCSIControllerDeleteSnapshotResponse) error {
	defer metrics.MeasureSince([]string{"client", "csi_controller", "delete_snapshot"}, time.Now())

	plugin, err := c.findControllerPlugin(req.PluginID)
	if err != nil {
		// the server's view of the plugin health is stale, so let it know it
		// should retry with another controller instance
		return fmt.Errorf("CSI.ControllerDeleteSnapshot: %w: %v",
			nstructs.ErrCSIClientRPCRetryable, err)
	}
	defer plugin.Close()

	csiReq := req.ToCSIRequest()

	ctx, cancelFn := c.requestContext()
	defer cancelFn()

	// CSI ControllerDeleteSnapshot errors for timeout, codes.Unavailable and
	// codes.ResourceExhausted are retried; all other errors are fatal.
	err = plugin.ControllerDeleteSnapshot(ctx, csiReq,
		grpc_retry.WithPerRetryTimeout(CSIPluginRequestTimeout),
		grpc_retry.WithMax(3),
		grpc_retry.WithBackoff(grpc_retry.BackoffExponential(100*time.Millisecond)))
	if errors.Is(err, nstructs.ErrCSIClientRPCIgnorable) {
		// if the snapshot was deleted out-of-band, we'll get an error from
		// the plugin but can safely ignore it
		c.c.logger.Debug("could not delete snapshot", "error", err)
		return nil
	}
	if err != nil {
		return fmt.Errorf("CSI.ControllerDeleteSnapshot: %v", err)
	}
	return err
}

func (c *CSI) ControllerListSnapshots(req *structs.ClientCSIControllerListSnapshotsRequest, resp *structs.ClientCSIControllerListSnapshotsResponse) error {
	defer metrics.MeasureSince([]string{"client", "csi_controller", "list_snapshots"}, time.Now())

	plugin, err := c.findControllerPlugin(req.PluginID)
	if err != nil {
		// the server's view of the plugin health is stale, so let it know it
		// should retry with another controller instance
		return fmt.Errorf("CSI.ControllerListSnapshots: %w: %v",
			nstructs.ErrCSIClientRPCRetryable, err)
	}
	defer plugin.Close()

	csiReq := req.ToCSIRequest()

	ctx, cancelFn := c.requestContext()
	defer cancelFn()

	// CSI ControllerListSnapshots errors for timeout, codes.Unavailable and
	// codes.ResourceExhausted are retried; all other errors are fatal.
	cresp, err := plugin.ControllerListSnapshots(ctx, csiReq,
		grpc_retry.WithPerRetryTimeout(CSIPluginRequestTimeout),
		grpc_retry.WithMax(3),
		grpc_retry.WithBackoff(grpc_retry.BackoffExponential(100*time.Millisecond)))
	if err != nil {
		return fmt.Errorf("CSI.ControllerListSnapshots: %v", err)
	}

	resp.NextToken = cresp.NextToken
	resp.Entries = []*nstructs.CSISnapshot{}

	for _, entry := range cresp.Entries {
		if entry.Snapshot == nil {
			return fmt.Errorf("CSI.ControllerListSnapshot: plugin returned an invalid entry")
		}
		snap := &nstructs.CSISnapshot{
			ID:                     entry.Snapshot.ID,
			ExternalSourceVolumeID: entry.Snapshot.SourceVolumeID,
			SizeBytes:              entry.Snapshot.SizeBytes,
			CreateTime:             entry.Snapshot.CreateTime,
			IsReady:                entry.Snapshot.IsReady,
			PluginID:               req.PluginID,
		}
		resp.Entries = append(resp.Entries, snap)
		if req.MaxEntries != 0 && int32(len(resp.Entries)) == req.MaxEntries {
			break
		}
	}

	return nil
}

// NodeDetachVolume is used to detach a volume from a CSI Cluster from
// the storage node provided in the request.
func (c *CSI) NodeDetachVolume(req *structs.ClientCSINodeDetachVolumeRequest, resp *structs.ClientCSINodeDetachVolumeResponse) error {
	defer metrics.MeasureSince([]string{"client", "csi_node", "detach_volume"}, time.Now())

	// The following block of validation checks should not be reached on a
	// real Nomad cluster. They serve as a defensive check before forwarding
	// requests to plugins, and to aid with development.
	if req.PluginID == "" {
		return errors.New("CSI.NodeDetachVolume: PluginID is required")
	}
	if req.VolumeID == "" {
		return errors.New("CSI.NodeDetachVolume: VolumeID is required")
	}
	if req.AllocID == "" {
		return errors.New("CSI.NodeDetachVolume: AllocID is required")
	}

	ctx, cancelFn := c.requestContext()
	defer cancelFn()

	manager, err := c.c.csimanager.ManagerForPlugin(ctx, req.PluginID)
	if err != nil {
		return fmt.Errorf("CSI.NodeDetachVolume: %v", err)
	}

	usageOpts := &csimanager.UsageOptions{
		ReadOnly:       req.ReadOnly,
		AttachmentMode: req.AttachmentMode,
		AccessMode:     req.AccessMode,
	}

	err = manager.UnmountVolume(ctx, req.VolumeID, req.ExternalID, req.AllocID, usageOpts)
	if err != nil && !errors.Is(err, nstructs.ErrCSIClientRPCIgnorable) {
		// if the unmounting previously happened but the server failed to
		// checkpoint, we'll get an error from Unmount but can safely
		// ignore it.
		return fmt.Errorf("CSI.NodeDetachVolume: %v", err)
	}
	return nil
}

// NodeExpandVolume instructs the node plugin to complete a volume expansion
// for a particular claim held by an allocation.
func (c *CSI) NodeExpandVolume(req *structs.ClientCSINodeExpandVolumeRequest, resp *structs.ClientCSINodeExpandVolumeResponse) error {
	defer metrics.MeasureSince([]string{"client", "csi_node", "expand_volume"}, time.Now())

	if err := req.Validate(); err != nil {
		return err
	}
	usageOpts := &csimanager.UsageOptions{
		// Claim will not be nil here, per req.Validate() above.
		ReadOnly:       req.Claim.Mode == nstructs.CSIVolumeClaimRead,
		AttachmentMode: req.Claim.AttachmentMode,
		AccessMode:     req.Claim.AccessMode,
	}

	ctx, cancel := c.requestContext() // note: this has a 2-minute timeout
	defer cancel()

	err := c.c.csimanager.WaitForPlugin(ctx, dynamicplugins.PluginTypeCSINode, req.PluginID)
	if err != nil {
		return err
	}

	manager, err := c.c.csimanager.ManagerForPlugin(ctx, req.PluginID)
	if err != nil {
		return err
	}

	newCapacity, err := manager.ExpandVolume(ctx,
		req.VolumeID, req.ExternalID, req.Claim.AllocationID, usageOpts, req.Capacity)

	if err != nil && !errors.Is(err, nstructs.ErrCSIClientRPCIgnorable) {
		return err
	}
	resp.CapacityBytes = newCapacity

	return nil
}

func (c *CSI) findControllerPlugin(name string) (csi.CSIPlugin, error) {
	return c.findPlugin(dynamicplugins.PluginTypeCSIController, name)
}

func (c *CSI) findPlugin(ptype, name string) (csi.CSIPlugin, error) {
	pIface, err := c.c.dynamicRegistry.DispensePlugin(ptype, name)
	if err != nil {
		return nil, err
	}

	plugin, ok := pIface.(csi.CSIPlugin)
	if !ok {
		return nil, ErrPluginTypeError
	}

	return plugin, nil
}

func (c *CSI) requestContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), CSIPluginRequestTimeout)
}
