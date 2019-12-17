package client

import (
	"errors"
	"testing"

	"github.com/hashicorp/nomad/client/dynamicplugins"
	"github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/plugins/csi"
	"github.com/hashicorp/nomad/plugins/csi/fake"
	"github.com/stretchr/testify/require"
)

var fakePlugin = &dynamicplugins.PluginInfo{
	Name:           "test-plugin",
	Type:           "csi-controller",
	ConnectionInfo: &dynamicplugins.PluginConnectionInfo{},
}

func TestClientCSI_CSIControllerAttachVolume(t *testing.T) {
	t.Parallel()

	cases := []struct {
		Name             string
		ClientSetupFunc  func(*fake.Client)
		Request          *structs.ClientCSIControllerAttachVolumeRequest
		ExpectedErr      error
		ExpectedResponse *structs.ClientCSIControllerAttachVolumeResponse
	}{
		{
			Name: "returns plugin not found errors",
			Request: &structs.ClientCSIControllerAttachVolumeRequest{
				PluginName: "some-garbage",
			},
			ExpectedErr: errors.New("plugin some-garbage for type csi-controller not found"),
		},
		{
			Name: "validates volumeid is not empty",
			Request: &structs.ClientCSIControllerAttachVolumeRequest{
				PluginName: fakePlugin.Name,
			},
			ExpectedErr: errors.New("VolumeID is required"),
		},
		{
			Name: "validates nodeid is not empty",
			Request: &structs.ClientCSIControllerAttachVolumeRequest{
				PluginName: fakePlugin.Name,
				VolumeID:   "1234-4321-1234-4321",
			},
			ExpectedErr: errors.New("NodeID is required"),
		},
		{
			Name: "validates AccessMode",
			Request: &structs.ClientCSIControllerAttachVolumeRequest{
				PluginName: fakePlugin.Name,
				VolumeID:   "1234-4321-1234-4321",
				NodeID:     "abcde",
				AccessMode: structs.CSIVolumeAccessMode("foo"),
			},
			ExpectedErr: errors.New("Unknown access mode: foo"),
		},
		{
			Name: "validates attachmentmode is not empty",
			Request: &structs.ClientCSIControllerAttachVolumeRequest{
				PluginName:     fakePlugin.Name,
				VolumeID:       "1234-4321-1234-4321",
				NodeID:         "abcde",
				AccessMode:     structs.CSIVolumeAccessModeMultiNodeReader,
				AttachmentMode: structs.CSIVolumeAttachmentMode("bar"),
			},
			ExpectedErr: errors.New("Unknown attachment mode: bar"),
		},
		{
			Name: "returns transitive errors",
			ClientSetupFunc: func(fc *fake.Client) {
				fc.NextControllerPublishVolumeErr = errors.New("hello")
			},
			Request: &structs.ClientCSIControllerAttachVolumeRequest{
				PluginName:     fakePlugin.Name,
				VolumeID:       "1234-4321-1234-4321",
				NodeID:         "abcde",
				AccessMode:     structs.CSIVolumeAccessModeSingleNodeWriter,
				AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
			},
			ExpectedErr: errors.New("hello"),
		},
		{
			Name: "handles nil PublishContext",
			ClientSetupFunc: func(fc *fake.Client) {
				fc.NextControllerPublishVolumeResponse = &csi.ControllerPublishVolumeResponse{}
			},
			Request: &structs.ClientCSIControllerAttachVolumeRequest{
				PluginName: fakePlugin.Name,
				VolumeID:   "1234-4321-1234-4321",
				NodeID:     "abcde",
			},
			ExpectedResponse: &structs.ClientCSIControllerAttachVolumeResponse{},
		},
		{
			Name: "handles non-nil PublishContext",
			ClientSetupFunc: func(fc *fake.Client) {
				fc.NextControllerPublishVolumeResponse = &csi.ControllerPublishVolumeResponse{
					PublishContext: map[string]string{"foo": "bar"},
				}
			},
			Request: &structs.ClientCSIControllerAttachVolumeRequest{
				PluginName: fakePlugin.Name,
				VolumeID:   "1234-4321-1234-4321",
				NodeID:     "abcde",
			},
			ExpectedResponse: &structs.ClientCSIControllerAttachVolumeResponse{
				PublishContext: map[string]string{"foo": "bar"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			require := require.New(t)
			client, cleanup := TestClient(t, nil)
			defer cleanup()

			fakeClient := &fake.Client{}
			if tc.ClientSetupFunc != nil {
				tc.ClientSetupFunc(fakeClient)
			}

			dispenserFunc := func(*dynamicplugins.PluginInfo) (interface{}, error) {
				return fakeClient, nil
			}
			client.dynamicRegistry.StubDispenserForType(dynamicplugins.PluginTypeCSIController, dispenserFunc)

			err := client.dynamicRegistry.RegisterPlugin(fakePlugin)
			require.Nil(err)

			var resp structs.ClientCSIControllerAttachVolumeResponse
			err = client.ClientRPC("ClientCSI.CSIControllerAttachVolume", tc.Request, &resp)
			require.Equal(tc.ExpectedErr, err)
			if tc.ExpectedResponse != nil {
				require.Equal(tc.ExpectedResponse, &resp)
			}
		})
	}
}
