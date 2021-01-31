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
		return errors.New("VolumeID is required")
	}

	if req.PluginID == "" {
		return errors.New("PluginID is required")
	}

	plugin, err := c.findControllerPlugin(req.PluginID)
	if err != nil {
		// the server's view of the plugin health is stale, so let it know it
		// should retry with another controller instance
		return fmt.Errorf("%w: %v", nstructs.ErrCSIClientRPCRetryable, err)
	}
	defer plugin.Close()

	csiReq, err := req.ToCSIRequest()
	if err != nil {
		return err
	}

	ctx, cancelFn := c.requestContext()
	defer cancelFn()

	// CSI ValidateVolumeCapabilities errors for timeout, codes.Unavailable and
	// codes.ResourceExhausted are retried; all other errors are fatal.
	return plugin.ControllerValidateCapabilities(ctx, csiReq,
		grpc_retry.WithPerRetryTimeout(CSIPluginRequestTimeout),
		grpc_retry.WithMax(3),
		grpc_retry.WithBackoff(grpc_retry.BackoffExponential(100*time.Millisecond)))
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
		return fmt.Errorf("%w: %v", nstructs.ErrCSIClientRPCRetryable, err)
	}
	defer plugin.Close()

	// The following block of validation checks should not be reached on a
	// real Nomad cluster as all of this data should be validated when registering
	// volumes with the cluster. They serve as a defensive check before forwarding
	// requests to plugins, and to aid with development.

	if req.VolumeID == "" {
		return errors.New("VolumeID is required")
	}

	if req.ClientCSINodeID == "" {
		return errors.New("ClientCSINodeID is required")
	}

	csiReq, err := req.ToCSIRequest()
	if err != nil {
		return err
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
		return err
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
		return fmt.Errorf("%w: %v", nstructs.ErrCSIClientRPCRetryable, err)
	}
	defer plugin.Close()

	// The following block of validation checks should not be reached on a
	// real Nomad cluster as all of this data should be validated when registering
	// volumes with the cluster. They serve as a defensive check before forwarding
	// requests to plugins, and to aid with development.

	if req.VolumeID == "" {
		return errors.New("VolumeID is required")
	}

	if req.ClientCSINodeID == "" {
		return errors.New("ClientCSINodeID is required")
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
	if err != nil {
		if errors.Is(err, nstructs.ErrCSIClientRPCIgnorable) {
			// if the controller detach previously happened but the server failed to
			// checkpoint, we'll get an error from the plugin but can safely ignore it.
			c.c.logger.Debug("could not unpublish volume: %v", err)
			return nil
		}
		return err
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
		return errors.New("PluginID is required")
	}
	if req.VolumeID == "" {
		return errors.New("VolumeID is required")
	}
	if req.AllocID == "" {
		return errors.New("AllocID is required")
	}

	ctx, cancelFn := c.requestContext()
	defer cancelFn()

	mounter, err := c.c.csimanager.MounterForPlugin(ctx, req.PluginID)
	if err != nil {
		return err
	}

	usageOpts := &csimanager.UsageOptions{
		ReadOnly:       req.ReadOnly,
		AttachmentMode: string(req.AttachmentMode),
		AccessMode:     string(req.AccessMode),
	}

	err = mounter.UnmountVolume(ctx, req.VolumeID, req.ExternalID, req.AllocID, usageOpts)
	if err != nil && !errors.Is(err, nstructs.ErrCSIClientRPCIgnorable) {
		// if the unmounting previously happened but the server failed to
		// checkpoint, we'll get an error from Unmount but can safely
		// ignore it.
		return err
	}
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
