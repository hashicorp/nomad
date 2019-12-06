package csimanager

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/nomad/client/dynamicplugins"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/csi"
	"github.com/hashicorp/nomad/plugins/csi/fake"
	"github.com/stretchr/testify/require"
)

func setupTestNodeInstanceManager(t *testing.T) (*fake.Client, *instanceManager) {
	tp := &fake.Client{}

	logger := testlog.HCLogger(t)
	pinfo := &dynamicplugins.PluginInfo{
		Name: "test-plugin",
	}

	return tp, &instanceManager{
		logger:          logger,
		info:            pinfo,
		client:          tp,
		fingerprintNode: true,
	}
}

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

			info, err := im.buildBasicFingerprint(context.TODO())

			require.Equal(t, test.ExpectedCSIInfo, info)
			require.Equal(t, test.ExpectedErr, err)

			require.Equal(t, test.CapabilitiesCallCount, client.PluginGetCapabilitiesCallCount)
			require.Equal(t, test.NodeInfoCallCount, client.NodeGetInfoCallCount)
		})
	}
}
