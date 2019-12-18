package csi

import (
	"context"

	csipbv1 "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/hashicorp/nomad/plugins/base"
)

// CSIPlugin implements a lightweight abstraction layer around a CSI Plugin.
// It validates that responses from storage providers (SP's), correctly conform
// to the specification before returning response data or erroring.
type CSIPlugin interface {
	base.BasePlugin

	// PluginProbe is used to verify that the plugin is in a healthy state
	PluginProbe(ctx context.Context) (bool, error)

	// PluginGetInfo is used to return semantic data about the plugin.
	// Response:
	//  - string: name, the name of the plugin in domain notation format.
	PluginGetInfo(ctx context.Context) (string, error)

	// PluginGetCapabilities is used to return the available capabilities from the
	// identity service. This currently only looks for the CONTROLLER_SERVICE and
	// Accessible Topology Support
	PluginGetCapabilities(ctx context.Context) (*PluginCapabilitySet, error)

	// GetControllerCapabilities is used to get controller-specific capabilities
	// for a plugin.
	ControllerGetCapabilities(ctx context.Context) (*ControllerCapabilitySet, error)

	// ControllerPublishVolume is used to attach a remote volume to a cluster node.
	ControllerPublishVolume(ctx context.Context, req *ControllerPublishVolumeRequest) (*ControllerPublishVolumeResponse, error)

	// NodeGetCapabilities is used to return the available capabilities from the
	// Node Service.
	NodeGetCapabilities(ctx context.Context) (*NodeCapabilitySet, error)

	// NodeGetInfo is used to return semantic data about the current node in
	// respect to the SP.
	NodeGetInfo(ctx context.Context) (*NodeGetInfoResponse, error)

	// Shutdown the client and ensure any connections are cleaned up.
	Close() error
}

type PluginCapabilitySet struct {
	hasControllerService bool
	hasTopologies        bool
}

func (p *PluginCapabilitySet) HasControllerService() bool {
	return p.hasControllerService
}

// HasTopologies indicates whether the volumes for this plugin are equally
// accessible by all nodes in the cluster.
// If true, we MUST use the topology information when scheduling workloads.
func (p *PluginCapabilitySet) HasToplogies() bool {
	return p.hasTopologies
}

func (p *PluginCapabilitySet) IsEqual(o *PluginCapabilitySet) bool {
	return p.hasControllerService == o.hasControllerService && p.hasTopologies == o.hasTopologies
}

func NewTestPluginCapabilitySet(topologies, controller bool) *PluginCapabilitySet {
	return &PluginCapabilitySet{
		hasTopologies:        topologies,
		hasControllerService: controller,
	}
}

func NewPluginCapabilitySet(capabilities *csipbv1.GetPluginCapabilitiesResponse) *PluginCapabilitySet {
	cs := &PluginCapabilitySet{}

	pluginCapabilities := capabilities.GetCapabilities()

	for _, pcap := range pluginCapabilities {
		if svcCap := pcap.GetService(); svcCap != nil {
			switch svcCap.Type {
			case csipbv1.PluginCapability_Service_UNKNOWN:
				continue
			case csipbv1.PluginCapability_Service_CONTROLLER_SERVICE:
				cs.hasControllerService = true
			case csipbv1.PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS:
				cs.hasTopologies = true
			default:
				continue
			}
		}
	}

	return cs
}

type ControllerCapabilitySet struct {
	HasPublishUnpublishVolume    bool
	HasPublishReadonly           bool
	HasListVolumes               bool
	HasListVolumesPublishedNodes bool
}

func NewControllerCapabilitySet(resp *csipbv1.ControllerGetCapabilitiesResponse) *ControllerCapabilitySet {
	cs := &ControllerCapabilitySet{}

	pluginCapabilities := resp.GetCapabilities()
	for _, pcap := range pluginCapabilities {
		if c := pcap.GetRpc(); c != nil {
			switch c.Type {
			case csipbv1.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME:
				cs.HasPublishUnpublishVolume = true
			case csipbv1.ControllerServiceCapability_RPC_PUBLISH_READONLY:
				cs.HasPublishReadonly = true
			case csipbv1.ControllerServiceCapability_RPC_LIST_VOLUMES:
				cs.HasListVolumes = true
			case csipbv1.ControllerServiceCapability_RPC_LIST_VOLUMES_PUBLISHED_NODES:
				cs.HasListVolumesPublishedNodes = true
			default:
				continue
			}
		}
	}

	return cs
}

type ControllerPublishVolumeRequest struct {
	VolumeID string
	NodeID   string
	ReadOnly bool

	//TODO: Add Capabilities
}

type ControllerPublishVolumeResponse struct {
	PublishContext map[string]string
}

type NodeCapabilitySet struct {
	HasStageUnstageVolume bool
}

func NewNodeCapabilitySet(resp *csipbv1.NodeGetCapabilitiesResponse) *NodeCapabilitySet {
	cs := &NodeCapabilitySet{}
	pluginCapabilities := resp.GetCapabilities()
	for _, pcap := range pluginCapabilities {
		if c := pcap.GetRpc(); c != nil {
			switch c.Type {
			case csipbv1.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME:
				cs.HasStageUnstageVolume = true
			default:
				continue
			}
		}
	}

	return cs
}
