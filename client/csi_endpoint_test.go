package client

import (
	"errors"
	"testing"

	"github.com/hashicorp/nomad/client/dynamicplugins"
	"github.com/hashicorp/nomad/client/structs"
	nstructs "github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/csi"
	"github.com/hashicorp/nomad/plugins/csi/fake"
	"github.com/stretchr/testify/require"
)

var fakePlugin = &dynamicplugins.PluginInfo{
	Name:           "test-plugin",
	Type:           "csi-controller",
	ConnectionInfo: &dynamicplugins.PluginConnectionInfo{},
}

var fakeNodePlugin = &dynamicplugins.PluginInfo{
	Name:           "test-plugin",
	Type:           "csi-node",
	ConnectionInfo: &dynamicplugins.PluginConnectionInfo{},
}

func TestCSIController_AttachVolume(t *testing.T) {
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
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: "some-garbage",
				},
			},
			ExpectedErr: errors.New("CSI client error (retryable): plugin some-garbage for type csi-controller not found"),
		},
		{
			Name: "validates volumeid is not empty",
			Request: &structs.ClientCSIControllerAttachVolumeRequest{
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: fakePlugin.Name,
				},
			},
			ExpectedErr: errors.New("VolumeID is required"),
		},
		{
			Name: "validates nodeid is not empty",
			Request: &structs.ClientCSIControllerAttachVolumeRequest{
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: fakePlugin.Name,
				},
				VolumeID: "1234-4321-1234-4321",
			},
			ExpectedErr: errors.New("ClientCSINodeID is required"),
		},
		{
			Name: "validates AccessMode",
			Request: &structs.ClientCSIControllerAttachVolumeRequest{
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: fakePlugin.Name,
				},
				VolumeID:        "1234-4321-1234-4321",
				ClientCSINodeID: "abcde",
				AttachmentMode:  nstructs.CSIVolumeAttachmentModeFilesystem,
				AccessMode:      nstructs.CSIVolumeAccessMode("foo"),
			},
			ExpectedErr: errors.New("Unknown volume access mode: foo"),
		},
		{
			Name: "validates attachmentmode is not empty",
			Request: &structs.ClientCSIControllerAttachVolumeRequest{
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: fakePlugin.Name,
				},
				VolumeID:        "1234-4321-1234-4321",
				ClientCSINodeID: "abcde",
				AccessMode:      nstructs.CSIVolumeAccessModeMultiNodeReader,
				AttachmentMode:  nstructs.CSIVolumeAttachmentMode("bar"),
			},
			ExpectedErr: errors.New("Unknown volume attachment mode: bar"),
		},
		{
			Name: "returns transitive errors",
			ClientSetupFunc: func(fc *fake.Client) {
				fc.NextControllerPublishVolumeErr = errors.New("hello")
			},
			Request: &structs.ClientCSIControllerAttachVolumeRequest{
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: fakePlugin.Name,
				},
				VolumeID:        "1234-4321-1234-4321",
				ClientCSINodeID: "abcde",
				AccessMode:      nstructs.CSIVolumeAccessModeSingleNodeWriter,
				AttachmentMode:  nstructs.CSIVolumeAttachmentModeFilesystem,
			},
			ExpectedErr: errors.New("hello"),
		},
		{
			Name: "handles nil PublishContext",
			ClientSetupFunc: func(fc *fake.Client) {
				fc.NextControllerPublishVolumeResponse = &csi.ControllerPublishVolumeResponse{}
			},
			Request: &structs.ClientCSIControllerAttachVolumeRequest{
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: fakePlugin.Name,
				},
				VolumeID:        "1234-4321-1234-4321",
				ClientCSINodeID: "abcde",
				AccessMode:      nstructs.CSIVolumeAccessModeSingleNodeWriter,
				AttachmentMode:  nstructs.CSIVolumeAttachmentModeFilesystem,
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
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: fakePlugin.Name,
				},
				VolumeID:        "1234-4321-1234-4321",
				ClientCSINodeID: "abcde",
				AccessMode:      nstructs.CSIVolumeAccessModeSingleNodeWriter,
				AttachmentMode:  nstructs.CSIVolumeAttachmentModeFilesystem,
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
			err = client.ClientRPC("CSI.ControllerAttachVolume", tc.Request, &resp)
			require.Equal(tc.ExpectedErr, err)
			if tc.ExpectedResponse != nil {
				require.Equal(tc.ExpectedResponse, &resp)
			}
		})
	}
}

func TestCSIController_ValidateVolume(t *testing.T) {
	t.Parallel()

	cases := []struct {
		Name             string
		ClientSetupFunc  func(*fake.Client)
		Request          *structs.ClientCSIControllerValidateVolumeRequest
		ExpectedErr      error
		ExpectedResponse *structs.ClientCSIControllerValidateVolumeResponse
	}{
		{
			Name: "validates volumeid is not empty",
			Request: &structs.ClientCSIControllerValidateVolumeRequest{
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: fakePlugin.Name,
				},
			},
			ExpectedErr: errors.New("VolumeID is required"),
		},
		{
			Name: "returns plugin not found errors",
			Request: &structs.ClientCSIControllerValidateVolumeRequest{
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: "some-garbage",
				},
				VolumeID: "foo",
			},
			ExpectedErr: errors.New("CSI client error (retryable): plugin some-garbage for type csi-controller not found"),
		},
		{
			Name: "validates attachmentmode",
			Request: &structs.ClientCSIControllerValidateVolumeRequest{
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: fakePlugin.Name,
				},
				VolumeID:       "1234-4321-1234-4321",
				AttachmentMode: nstructs.CSIVolumeAttachmentMode("bar"),
				AccessMode:     nstructs.CSIVolumeAccessModeMultiNodeReader,
			},
			ExpectedErr: errors.New("Unknown volume attachment mode: bar"),
		},
		{
			Name: "validates AccessMode",
			Request: &structs.ClientCSIControllerValidateVolumeRequest{
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: fakePlugin.Name,
				},
				VolumeID:       "1234-4321-1234-4321",
				AttachmentMode: nstructs.CSIVolumeAttachmentModeFilesystem,
				AccessMode:     nstructs.CSIVolumeAccessMode("foo"),
			},
			ExpectedErr: errors.New("Unknown volume access mode: foo"),
		},
		{
			Name: "returns transitive errors",
			ClientSetupFunc: func(fc *fake.Client) {
				fc.NextControllerValidateVolumeErr = errors.New("hello")
			},
			Request: &structs.ClientCSIControllerValidateVolumeRequest{
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: fakePlugin.Name,
				},
				VolumeID:       "1234-4321-1234-4321",
				AccessMode:     nstructs.CSIVolumeAccessModeSingleNodeWriter,
				AttachmentMode: nstructs.CSIVolumeAttachmentModeFilesystem,
			},
			ExpectedErr: errors.New("hello"),
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

			var resp structs.ClientCSIControllerValidateVolumeResponse
			err = client.ClientRPC("CSI.ControllerValidateVolume", tc.Request, &resp)
			require.Equal(tc.ExpectedErr, err)
			if tc.ExpectedResponse != nil {
				require.Equal(tc.ExpectedResponse, &resp)
			}
		})
	}
}

func TestCSIController_DetachVolume(t *testing.T) {
	t.Parallel()

	cases := []struct {
		Name             string
		ClientSetupFunc  func(*fake.Client)
		Request          *structs.ClientCSIControllerDetachVolumeRequest
		ExpectedErr      error
		ExpectedResponse *structs.ClientCSIControllerDetachVolumeResponse
	}{
		{
			Name: "returns plugin not found errors",
			Request: &structs.ClientCSIControllerDetachVolumeRequest{
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: "some-garbage",
				},
			},
			ExpectedErr: errors.New("CSI client error (retryable): plugin some-garbage for type csi-controller not found"),
		},
		{
			Name: "validates volumeid is not empty",
			Request: &structs.ClientCSIControllerDetachVolumeRequest{
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: fakePlugin.Name,
				},
			},
			ExpectedErr: errors.New("VolumeID is required"),
		},
		{
			Name: "validates nodeid is not empty",
			Request: &structs.ClientCSIControllerDetachVolumeRequest{
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: fakePlugin.Name,
				},
				VolumeID: "1234-4321-1234-4321",
			},
			ExpectedErr: errors.New("ClientCSINodeID is required"),
		},
		{
			Name: "returns transitive errors",
			ClientSetupFunc: func(fc *fake.Client) {
				fc.NextControllerUnpublishVolumeErr = errors.New("hello")
			},
			Request: &structs.ClientCSIControllerDetachVolumeRequest{
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: fakePlugin.Name,
				},
				VolumeID:        "1234-4321-1234-4321",
				ClientCSINodeID: "abcde",
			},
			ExpectedErr: errors.New("hello"),
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

			var resp structs.ClientCSIControllerDetachVolumeResponse
			err = client.ClientRPC("CSI.ControllerDetachVolume", tc.Request, &resp)
			require.Equal(tc.ExpectedErr, err)
			if tc.ExpectedResponse != nil {
				require.Equal(tc.ExpectedResponse, &resp)
			}
		})
	}
}

func TestCSINode_DetachVolume(t *testing.T) {
	t.Parallel()

	cases := []struct {
		Name             string
		ClientSetupFunc  func(*fake.Client)
		Request          *structs.ClientCSINodeDetachVolumeRequest
		ExpectedErr      error
		ExpectedResponse *structs.ClientCSINodeDetachVolumeResponse
	}{
		{
			Name: "returns plugin not found errors",
			Request: &structs.ClientCSINodeDetachVolumeRequest{
				PluginID:       "some-garbage",
				VolumeID:       "-",
				AllocID:        "-",
				NodeID:         "-",
				AttachmentMode: nstructs.CSIVolumeAttachmentModeFilesystem,
				AccessMode:     nstructs.CSIVolumeAccessModeMultiNodeReader,
				ReadOnly:       true,
			},
			ExpectedErr: errors.New("plugin some-garbage for type csi-node not found"),
		},
		{
			Name: "validates volumeid is not empty",
			Request: &structs.ClientCSINodeDetachVolumeRequest{
				PluginID: fakeNodePlugin.Name,
			},
			ExpectedErr: errors.New("VolumeID is required"),
		},
		{
			Name: "validates nodeid is not empty",
			Request: &structs.ClientCSINodeDetachVolumeRequest{
				PluginID: fakeNodePlugin.Name,
				VolumeID: "1234-4321-1234-4321",
			},
			ExpectedErr: errors.New("AllocID is required"),
		},
		{
			Name: "returns transitive errors",
			ClientSetupFunc: func(fc *fake.Client) {
				fc.NextNodeUnpublishVolumeErr = errors.New("wont-see-this")
			},
			Request: &structs.ClientCSINodeDetachVolumeRequest{
				PluginID: fakeNodePlugin.Name,
				VolumeID: "1234-4321-1234-4321",
				AllocID:  "4321-1234-4321-1234",
			},
			// we don't have a csimanager in this context
			ExpectedErr: errors.New("plugin test-plugin for type csi-node not found"),
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
			client.dynamicRegistry.StubDispenserForType(dynamicplugins.PluginTypeCSINode, dispenserFunc)
			err := client.dynamicRegistry.RegisterPlugin(fakeNodePlugin)
			require.Nil(err)

			var resp structs.ClientCSINodeDetachVolumeResponse
			err = client.ClientRPC("CSI.NodeDetachVolume", tc.Request, &resp)
			require.Equal(tc.ExpectedErr, err)
			if tc.ExpectedResponse != nil {
				require.Equal(tc.ExpectedResponse, &resp)
			}
		})
	}
}
