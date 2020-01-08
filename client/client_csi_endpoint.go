package client

import (
	"context"
	"errors"
	"fmt"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/nomad/client/dynamicplugins"
	"github.com/hashicorp/nomad/client/structs"
	s "github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/csi"
)

// ClientCSI endpoint is used for interacting with CSI plugins on a client.
// TODO: Submit metrics with labels to allow debugging per plugin perf problems.
type ClientCSI struct {
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

// CSIControllerAttachVolume is used to attach a volume from a CSI Cluster to
// the storage node provided in the request.
//
// The controller attachment flow currently works as follows:
// 1. Validate the volume request
// 2. Call ControllerPublishVolume on the CSI Plugin to trigger a remote attachment
//
// In the future this may be expanded to request dynamic secrets for attachement.
func (c *ClientCSI) CSIControllerAttachVolume(req *structs.ClientCSIControllerAttachVolumeRequest, resp *structs.ClientCSIControllerAttachVolumeResponse) error {
	defer metrics.MeasureSince([]string{"client", "csi_controller", "publish_volume"}, time.Now())
	plugin, err := c.findControllerPlugin(req.PluginName)
	if err != nil {
		return err
	}
	defer plugin.Close()

	// The following block of validation checks should not be reached on a
	// real Nomad cluster as all of this data should be validated when registering
	// volumes with the cluster. They serve as a defensive check before forwarding
	// requests to plugins, and to aid with development.

	if req.VolumeID == "" {
		return errors.New("VolumeID is required")
	}

	if req.NodeID == "" {
		return errors.New("NodeID is required")
	}

	if !s.ValidCSIVolumeAccessMode(req.AccessMode) {
		return fmt.Errorf("Unknown access mode: %v", req.AccessMode)
	}

	if !s.ValidCSIVolumeAttachmentMode(req.AttachmentMode) {
		return fmt.Errorf("Unknown attachment mode: %v", req.AttachmentMode)
	}

	// Submit the request for a volume to the CSI Plugin.
	ctx, cancelFn := c.requestContext()
	defer cancelFn()
	cresp, err := plugin.ControllerPublishVolume(ctx, req.ToCSIRequest())
	if err != nil {
		return err
	}

	resp.PublishContext = cresp.PublishContext
	return nil
}

func (c *ClientCSI) findControllerPlugin(name string) (csi.CSIPlugin, error) {
	return c.findPlugin(dynamicplugins.PluginTypeCSIController, name)
}

// TODO: Cache Plugin Clients?
func (c *ClientCSI) findPlugin(ptype, name string) (csi.CSIPlugin, error) {
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

func (c *ClientCSI) requestContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), CSIPluginRequestTimeout)
}
