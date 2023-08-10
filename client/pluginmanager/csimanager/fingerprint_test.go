// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package csimanager

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/csi"
	"github.com/stretchr/testify/require"
)

func TestBuildBasicFingerprint_Node(t *testing.T) {
	tt := []struct {
		Name string

		Capabilities          *csi.PluginCapabilitySet
		CapabilitiesErr       error
		CapabilitiesCallCount int64

		NodeInfo          *csi.NodeGetInfoResponse
		NodeInfoErr       error
		NodeInfoCallCount int64

		ExpectedCSIInfo *structs.CSIInfo
		ExpectedErr     error
	}{
		{
			Name: "Minimal successful response",

			Capabilities:          &csi.PluginCapabilitySet{},
			CapabilitiesCallCount: 1,

			NodeInfo: &csi.NodeGetInfoResponse{
				NodeID:             "foobar",
				MaxVolumes:         5,
				AccessibleTopology: nil,
			},
			NodeInfoCallCount: 1,

			ExpectedCSIInfo: &structs.CSIInfo{
				PluginID:          "test-plugin",
				Healthy:           false,
				HealthDescription: "initial fingerprint not completed",
				NodeInfo: &structs.CSINodeInfo{
					ID:         "foobar",
					MaxVolumes: 5,
				},
			},
		},
		{
			Name: "Successful response with capabilities and topologies",

			Capabilities:          csi.NewTestPluginCapabilitySet(true, false),
			CapabilitiesCallCount: 1,

			NodeInfo: &csi.NodeGetInfoResponse{
				NodeID:     "foobar",
				MaxVolumes: 5,
				AccessibleTopology: &csi.Topology{
					Segments: map[string]string{
						"com.hashicorp.nomad/node-id": "foobar",
					},
				},
			},
			NodeInfoCallCount: 1,

			ExpectedCSIInfo: &structs.CSIInfo{
				PluginID:          "test-plugin",
				Healthy:           false,
				HealthDescription: "initial fingerprint not completed",

				RequiresTopologies: true,

				NodeInfo: &structs.CSINodeInfo{
					ID:         "foobar",
					MaxVolumes: 5,
					AccessibleTopology: &structs.CSITopology{
						Segments: map[string]string{
							"com.hashicorp.nomad/node-id": "foobar",
						},
					},
				},
			},
		},
		{
			Name: "PluginGetCapabilities Failed",

			CapabilitiesErr:       errors.New("request failed"),
			CapabilitiesCallCount: 1,

			NodeInfoCallCount: 0,

			ExpectedCSIInfo: &structs.CSIInfo{
				PluginID:          "test-plugin",
				Healthy:           false,
				HealthDescription: "initial fingerprint not completed",
				NodeInfo:          &structs.CSINodeInfo{},
			},
			ExpectedErr: errors.New("request failed"),
		},
		{
			Name: "NodeGetInfo Failed",

			Capabilities:          &csi.PluginCapabilitySet{},
			CapabilitiesCallCount: 1,

			NodeInfoErr:       errors.New("request failed"),
			NodeInfoCallCount: 1,

			ExpectedCSIInfo: &structs.CSIInfo{
				PluginID:          "test-plugin",
				Healthy:           false,
				HealthDescription: "initial fingerprint not completed",
				NodeInfo:          &structs.CSINodeInfo{},
			},
			ExpectedErr: errors.New("request failed"),
		},
	}

	for _, test := range tt {
		t.Run(test.Name, func(t *testing.T) {
			client, im := setupTestNodeInstanceManager(t)

			client.NextPluginGetCapabilitiesResponse = test.Capabilities
			client.NextPluginGetCapabilitiesErr = test.CapabilitiesErr

			client.NextNodeGetInfoResponse = test.NodeInfo
			client.NextNodeGetInfoErr = test.NodeInfoErr

			info, err := im.fp.buildBasicFingerprint(context.Background())

			require.Equal(t, test.ExpectedCSIInfo, info)
			require.Equal(t, test.ExpectedErr, err)

			require.Equal(t, test.CapabilitiesCallCount, client.PluginGetCapabilitiesCallCount)
			require.Equal(t, test.NodeInfoCallCount, client.NodeGetInfoCallCount)
		})
	}
}

func TestBuildControllerFingerprint(t *testing.T) {
	tt := []struct {
		Name string

		Capabilities          *csi.ControllerCapabilitySet
		CapabilitiesErr       error
		CapabilitiesCallCount int64

		ProbeResponse  bool
		ProbeErr       error
		ProbeCallCount int64

		ExpectedControllerInfo *structs.CSIControllerInfo
		ExpectedErr            error
	}{
		{
			Name: "Minimal successful response",

			Capabilities:          &csi.ControllerCapabilitySet{},
			CapabilitiesCallCount: 1,

			ProbeResponse:  true,
			ProbeCallCount: 1,

			ExpectedControllerInfo: &structs.CSIControllerInfo{},
		},
		{
			Name: "Successful response with capabilities",

			Capabilities: &csi.ControllerCapabilitySet{
				HasListVolumes: true,
			},
			CapabilitiesCallCount: 1,

			ProbeResponse:  true,
			ProbeCallCount: 1,

			ExpectedControllerInfo: &structs.CSIControllerInfo{
				SupportsListVolumes: true,
			},
		},
		{
			Name: "ControllerGetCapabilities Failed",

			CapabilitiesErr:       errors.New("request failed"),
			CapabilitiesCallCount: 1,

			ProbeResponse:  true,
			ProbeCallCount: 1,

			ExpectedControllerInfo: &structs.CSIControllerInfo{},
			ExpectedErr:            errors.New("request failed"),
		},
	}

	for _, test := range tt {
		t.Run(test.Name, func(t *testing.T) {
			client, im := setupTestNodeInstanceManager(t)

			client.NextControllerGetCapabilitiesResponse = test.Capabilities
			client.NextControllerGetCapabilitiesErr = test.CapabilitiesErr

			client.NextPluginProbeResponse = test.ProbeResponse
			client.NextPluginProbeErr = test.ProbeErr

			info, err := im.fp.buildControllerFingerprint(context.Background(), &structs.CSIInfo{ControllerInfo: &structs.CSIControllerInfo{}})

			require.Equal(t, test.ExpectedControllerInfo, info.ControllerInfo)
			require.Equal(t, test.ExpectedErr, err)

			require.Equal(t, test.CapabilitiesCallCount, client.ControllerGetCapabilitiesCallCount)
			require.Equal(t, test.ProbeCallCount, client.PluginProbeCallCount)
		})
	}
}

func TestBuildNodeFingerprint(t *testing.T) {
	tt := []struct {
		Name string

		Capabilities          *csi.NodeCapabilitySet
		CapabilitiesErr       error
		CapabilitiesCallCount int64

		ExpectedCSINodeInfo *structs.CSINodeInfo
		ExpectedErr         error
	}{
		{
			Name: "Minimal successful response",

			Capabilities:          &csi.NodeCapabilitySet{},
			CapabilitiesCallCount: 1,

			ExpectedCSINodeInfo: &structs.CSINodeInfo{
				RequiresNodeStageVolume: false,
			},
		},
		{
			Name: "Successful response with capabilities and topologies",

			Capabilities: &csi.NodeCapabilitySet{
				HasStageUnstageVolume: true,
			},
			CapabilitiesCallCount: 1,

			ExpectedCSINodeInfo: &structs.CSINodeInfo{
				RequiresNodeStageVolume: true,
			},
		},
		{
			Name: "NodeGetCapabilities Failed",

			CapabilitiesErr:       errors.New("request failed"),
			CapabilitiesCallCount: 1,

			ExpectedCSINodeInfo: &structs.CSINodeInfo{},
			ExpectedErr:         errors.New("request failed"),
		},
	}

	for _, test := range tt {
		t.Run(test.Name, func(t *testing.T) {
			client, im := setupTestNodeInstanceManager(t)

			client.NextNodeGetCapabilitiesResponse = test.Capabilities
			client.NextNodeGetCapabilitiesErr = test.CapabilitiesErr

			info, err := im.fp.buildNodeFingerprint(context.Background(), &structs.CSIInfo{NodeInfo: &structs.CSINodeInfo{}})

			require.Equal(t, test.ExpectedCSINodeInfo, info.NodeInfo)
			require.Equal(t, test.ExpectedErr, err)

			require.Equal(t, test.CapabilitiesCallCount, client.NodeGetCapabilitiesCallCount)
		})
	}
}
