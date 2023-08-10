// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package csimanager

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/dynamicplugins"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/csi"
	"golang.org/x/exp/maps"
)

type pluginFingerprinter struct {
	logger hclog.Logger
	client csi.CSIPlugin
	info   *dynamicplugins.PluginInfo

	// basicInfo holds a cache of data that should not change within a CSI plugin.
	// This allows us to minimize the number of requests we make to plugins on each
	// run of the fingerprinter, and reduces the chances of performing overly
	// expensive actions repeatedly, and improves stability of data through
	// transient failures.
	basicInfo *structs.CSIInfo

	fingerprintNode       bool
	fingerprintController bool

	hadFirstSuccessfulFingerprint bool
	// hadFirstSuccessfulFingerprintCh is closed the first time a fingerprint
	// is completed successfully.
	hadFirstSuccessfulFingerprintCh chan struct{}

	// requiresStaging is set on a first successful fingerprint. It allows the
	// csimanager to efficiently query this as it shouldn't change after a plugin
	// is started. Removing this bool will require storing a cache of recent successful
	// results that can be used by subscribers of the `hadFirstSuccessfulFingerprintCh`.
	requiresStaging bool
}

func (p *pluginFingerprinter) fingerprint(ctx context.Context) *structs.CSIInfo {
	if p.basicInfo == nil {
		info, err := p.buildBasicFingerprint(ctx)
		if err != nil {
			// If we receive a fingerprinting error, update the stats with as much
			// info as possible and wait for the next fingerprint interval.
			info.HealthDescription = fmt.Sprintf("failed initial fingerprint with err: %v", err)
			info.Healthy = false

			return info
		}

		// If fingerprinting succeeded, we don't need to repopulate the basic
		// info again.
		p.basicInfo = info
	}

	info := p.basicInfo.Copy()
	var fp *structs.CSIInfo
	var err error

	if p.fingerprintNode {
		fp, err = p.buildNodeFingerprint(ctx, info)
	} else if p.fingerprintController {
		fp, err = p.buildControllerFingerprint(ctx, info)
	}

	if err != nil {
		info.Healthy = false
		info.HealthDescription = fmt.Sprintf("failed fingerprinting with error: %v", err)
	} else {
		info = fp
		if !p.hadFirstSuccessfulFingerprint {
			p.hadFirstSuccessfulFingerprint = true
			if p.fingerprintNode {
				p.requiresStaging = info.NodeInfo.RequiresNodeStageVolume
			}
			close(p.hadFirstSuccessfulFingerprintCh)
		}
	}

	return info
}

func (p *pluginFingerprinter) buildBasicFingerprint(ctx context.Context) (*structs.CSIInfo, error) {
	info := &structs.CSIInfo{
		PluginID:          p.info.Name,
		AllocID:           p.info.AllocID,
		Provider:          p.info.Options["Provider"],
		ProviderVersion:   p.info.Version,
		Healthy:           false,
		HealthDescription: "initial fingerprint not completed",
	}

	if p.fingerprintNode {
		info.NodeInfo = &structs.CSINodeInfo{}
	}
	if p.fingerprintController {
		info.ControllerInfo = &structs.CSIControllerInfo{}
	}

	capabilities, err := p.client.PluginGetCapabilities(ctx)
	if err != nil {
		return info, err
	}

	info.RequiresControllerPlugin = capabilities.HasControllerService()
	info.RequiresTopologies = capabilities.HasToplogies()

	if p.fingerprintNode {
		nodeInfo, err := p.client.NodeGetInfo(ctx)
		if err != nil {
			return info, err
		}

		info.NodeInfo.ID = nodeInfo.NodeID
		info.NodeInfo.MaxVolumes = nodeInfo.MaxVolumes
		info.NodeInfo.AccessibleTopology = structCSITopologyFromCSITopology(nodeInfo.AccessibleTopology)
	}

	return info, nil
}

func applyCapabilitySetToControllerInfo(cs *csi.ControllerCapabilitySet, info *structs.CSIControllerInfo) {
	info.SupportsCreateDelete = cs.HasCreateDeleteVolume
	info.SupportsAttachDetach = cs.HasPublishUnpublishVolume
	info.SupportsListVolumes = cs.HasListVolumes
	info.SupportsGetCapacity = cs.HasGetCapacity
	info.SupportsCreateDeleteSnapshot = cs.HasCreateDeleteSnapshot
	info.SupportsListSnapshots = cs.HasListSnapshots
	info.SupportsClone = cs.HasCloneVolume
	info.SupportsReadOnlyAttach = cs.HasPublishReadonly
	info.SupportsExpand = cs.HasExpandVolume
	info.SupportsListVolumesAttachedNodes = cs.HasListVolumesPublishedNodes
	info.SupportsCondition = cs.HasVolumeCondition
	info.SupportsGet = cs.HasGetVolume
}

func (p *pluginFingerprinter) buildControllerFingerprint(ctx context.Context, base *structs.CSIInfo) (*structs.CSIInfo, error) {
	fp := base.Copy()

	healthy, err := p.client.PluginProbe(ctx)
	if err != nil {
		return nil, err
	}
	fp.SetHealthy(healthy)

	caps, err := p.client.ControllerGetCapabilities(ctx)
	if err != nil {
		return fp, err
	}
	applyCapabilitySetToControllerInfo(caps, fp.ControllerInfo)

	return fp, nil
}

func (p *pluginFingerprinter) buildNodeFingerprint(ctx context.Context, base *structs.CSIInfo) (*structs.CSIInfo, error) {
	fp := base.Copy()

	healthy, err := p.client.PluginProbe(ctx)
	if err != nil {
		return nil, err
	}
	fp.SetHealthy(healthy)

	caps, err := p.client.NodeGetCapabilities(ctx)
	if err != nil {
		return fp, err
	}
	fp.NodeInfo.RequiresNodeStageVolume = caps.HasStageUnstageVolume
	fp.NodeInfo.SupportsStats = caps.HasGetVolumeStats
	fp.NodeInfo.SupportsExpand = caps.HasExpandVolume
	fp.NodeInfo.SupportsCondition = caps.HasVolumeCondition

	return fp, nil
}

func structCSITopologyFromCSITopology(a *csi.Topology) *structs.CSITopology {
	if a == nil {
		return nil
	}

	return &structs.CSITopology{
		Segments: maps.Clone(a.Segments),
	}
}
