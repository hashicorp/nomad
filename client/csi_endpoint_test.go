// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"errors"
	"fmt"
	"testing"

	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/dynamicplugins"
	"github.com/hashicorp/nomad/client/structs"
	nstructs "github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/csi"
	"github.com/hashicorp/nomad/plugins/csi/fake"
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
	ci.Parallel(t)

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
			ExpectedErr: errors.New("CSI.ControllerAttachVolume: CSI client error (retryable): plugin some-garbage for type csi-controller not found"),
		},
		{
			Name: "validates volumeid is not empty",
			Request: &structs.ClientCSIControllerAttachVolumeRequest{
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: fakePlugin.Name,
				},
			},
			ExpectedErr: errors.New("CSI.ControllerAttachVolume: VolumeID is required"),
		},
		{
			Name: "validates nodeid is not empty",
			Request: &structs.ClientCSIControllerAttachVolumeRequest{
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: fakePlugin.Name,
				},
				VolumeID: "1234-4321-1234-4321",
			},
			ExpectedErr: errors.New("CSI.ControllerAttachVolume: ClientCSINodeID is required"),
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
			ExpectedErr: errors.New("CSI.ControllerAttachVolume: unknown volume access mode: foo"),
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
			ExpectedErr: errors.New("CSI.ControllerAttachVolume: unknown volume attachment mode: bar"),
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
			ExpectedErr: errors.New("CSI.ControllerAttachVolume: hello"),
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
	ci.Parallel(t)

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
			ExpectedErr: errors.New("CSI.ControllerValidateVolume: VolumeID is required"),
		},
		{
			Name: "returns plugin not found errors",
			Request: &structs.ClientCSIControllerValidateVolumeRequest{
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: "some-garbage",
				},
				VolumeID: "foo",
			},
			ExpectedErr: errors.New("CSI.ControllerValidateVolume: CSI client error (retryable): plugin some-garbage for type csi-controller not found"),
		},
		{
			Name: "validates attachmentmode",
			Request: &structs.ClientCSIControllerValidateVolumeRequest{
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: fakePlugin.Name,
				},
				VolumeID: "1234-4321-1234-4321",
				VolumeCapabilities: []*nstructs.CSIVolumeCapability{{
					AttachmentMode: nstructs.CSIVolumeAttachmentMode("bar"),
					AccessMode:     nstructs.CSIVolumeAccessModeMultiNodeReader,
				}},
			},
			ExpectedErr: errors.New("CSI.ControllerValidateVolume: unknown volume attachment mode: bar"),
		},
		{
			Name: "validates AccessMode",
			Request: &structs.ClientCSIControllerValidateVolumeRequest{
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: fakePlugin.Name,
				},
				VolumeID: "1234-4321-1234-4321",
				VolumeCapabilities: []*nstructs.CSIVolumeCapability{{
					AttachmentMode: nstructs.CSIVolumeAttachmentModeFilesystem,
					AccessMode:     nstructs.CSIVolumeAccessMode("foo"),
				}},
			},
			ExpectedErr: errors.New("CSI.ControllerValidateVolume: unknown volume access mode: foo"),
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
			ExpectedErr: errors.New("CSI.ControllerValidateVolume: hello"),
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
	ci.Parallel(t)

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
			ExpectedErr: errors.New("CSI.ControllerDetachVolume: CSI client error (retryable): plugin some-garbage for type csi-controller not found"),
		},
		{
			Name: "validates volumeid is not empty",
			Request: &structs.ClientCSIControllerDetachVolumeRequest{
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: fakePlugin.Name,
				},
			},
			ExpectedErr: errors.New("CSI.ControllerDetachVolume: VolumeID is required"),
		},
		{
			Name: "validates nodeid is not empty",
			Request: &structs.ClientCSIControllerDetachVolumeRequest{
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: fakePlugin.Name,
				},
				VolumeID: "1234-4321-1234-4321",
			},
			ExpectedErr: errors.New("CSI.ControllerDetachVolume: ClientCSINodeID is required"),
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
			ExpectedErr: errors.New("CSI.ControllerDetachVolume: hello"),
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

func TestCSIController_CreateVolume(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		Name             string
		ClientSetupFunc  func(*fake.Client)
		Request          *structs.ClientCSIControllerCreateVolumeRequest
		ExpectedErr      error
		ExpectedResponse *structs.ClientCSIControllerCreateVolumeResponse
	}{
		{
			Name: "returns plugin not found errors",
			Request: &structs.ClientCSIControllerCreateVolumeRequest{
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: "some-garbage",
				},
			},
			ExpectedErr: errors.New("CSI.ControllerCreateVolume: CSI client error (retryable): plugin some-garbage for type csi-controller not found"),
		},
		{
			Name: "validates AccessMode",
			Request: &structs.ClientCSIControllerCreateVolumeRequest{
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: fakePlugin.Name,
				},
				Name: "1234-4321-1234-4321",
				VolumeCapabilities: []*nstructs.CSIVolumeCapability{
					{
						AttachmentMode: nstructs.CSIVolumeAttachmentModeFilesystem,
						AccessMode:     nstructs.CSIVolumeAccessMode("foo"),
					},
				},
			},
			ExpectedErr: errors.New("CSI.ControllerCreateVolume: unknown volume access mode: foo"),
		},
		{
			Name: "validates attachmentmode is not empty",
			Request: &structs.ClientCSIControllerCreateVolumeRequest{
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: fakePlugin.Name,
				},
				Name: "1234-4321-1234-4321",
				VolumeCapabilities: []*nstructs.CSIVolumeCapability{
					{
						AccessMode:     nstructs.CSIVolumeAccessModeMultiNodeReader,
						AttachmentMode: nstructs.CSIVolumeAttachmentMode("bar"),
					},
				},
			},
			ExpectedErr: errors.New("CSI.ControllerCreateVolume: unknown volume attachment mode: bar"),
		},
		{
			Name: "returns transitive errors",
			ClientSetupFunc: func(fc *fake.Client) {
				fc.NextControllerCreateVolumeErr = errors.New("internal plugin error")
			},
			Request: &structs.ClientCSIControllerCreateVolumeRequest{
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: fakePlugin.Name,
				},
				Name: "1234-4321-1234-4321",
				VolumeCapabilities: []*nstructs.CSIVolumeCapability{
					{
						AccessMode:     nstructs.CSIVolumeAccessModeSingleNodeWriter,
						AttachmentMode: nstructs.CSIVolumeAttachmentModeFilesystem,
					},
				},
			},
			ExpectedErr: errors.New("CSI.ControllerCreateVolume: internal plugin error"),
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
			client.dynamicRegistry.StubDispenserForType(
				dynamicplugins.PluginTypeCSIController, dispenserFunc)

			err := client.dynamicRegistry.RegisterPlugin(fakePlugin)
			require.Nil(err)

			var resp structs.ClientCSIControllerCreateVolumeResponse
			err = client.ClientRPC("CSI.ControllerCreateVolume", tc.Request, &resp)
			require.Equal(tc.ExpectedErr, err)
			if tc.ExpectedResponse != nil {
				require.Equal(tc.ExpectedResponse, &resp)
			}
		})
	}
}

func TestCSIController_ExpandVolume(t *testing.T) {
	cases := []struct {
		Name       string
		ModRequest func(request *structs.ClientCSIControllerExpandVolumeRequest)
		NextResp   *csi.ControllerExpandVolumeResponse
		NextErr    error
		ExpectErr  string
	}{
		{
			Name: "success",
			NextResp: &csi.ControllerExpandVolumeResponse{
				CapacityBytes:         99,
				NodeExpansionRequired: true,
			},
		},
		{
			Name: "plugin not found",
			ModRequest: func(r *structs.ClientCSIControllerExpandVolumeRequest) {
				r.CSIControllerQuery.PluginID = "nonexistent"
			},
			ExpectErr: "CSI.ControllerExpandVolume could not find plugin: CSI client error (retryable): plugin nonexistent for type csi-controller not found",
		},
		{
			Name:      "ignorable error",
			NextResp:  &csi.ControllerExpandVolumeResponse{},
			NextErr:   fmt.Errorf("you can ignore me (%w)", nstructs.ErrCSIClientRPCIgnorable),
			ExpectErr: "", // explicitly empty here for clarity.
		},
		{
			Name:      "controller error",
			NextErr:   errors.New("sad plugin"),
			ExpectErr: "CSI.ControllerExpandVolume: sad plugin",
		},
		{
			Name:      "nil response from plugin",
			NextResp:  nil, // again explicit for clarity.
			ExpectErr: "CSI.ControllerExpandVolume: plugin did not return error or response",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			client, cleanup := TestClient(t, nil)
			t.Cleanup(func() { test.NoError(t, cleanup()) })

			fakeClient := &fake.Client{
				NextControllerExpandVolumeResponse: tc.NextResp,
				NextControllerExpandVolumeErr:      tc.NextErr,
			}

			dispenserFunc := func(*dynamicplugins.PluginInfo) (interface{}, error) {
				return fakeClient, nil
			}
			client.dynamicRegistry.StubDispenserForType(
				dynamicplugins.PluginTypeCSIController, dispenserFunc)
			err := client.dynamicRegistry.RegisterPlugin(fakePlugin)
			must.NoError(t, err)

			req := &structs.ClientCSIControllerExpandVolumeRequest{
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: fakePlugin.Name,
				},

				ExternalVolumeID: "some-volume-id",
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 99,
				},
				Secrets: map[string]string{"super": "secret"},
			}
			if tc.ModRequest != nil {
				tc.ModRequest(req)
			}

			var resp structs.ClientCSIControllerExpandVolumeResponse
			err = client.ClientRPC("CSI.ControllerExpandVolume", req, &resp)

			if tc.ExpectErr != "" {
				must.EqError(t, err, tc.ExpectErr)
				return
			}
			must.NoError(t, err)
			must.Eq(t, tc.NextResp.CapacityBytes, resp.CapacityBytes)
			must.Eq(t, tc.NextResp.NodeExpansionRequired, resp.NodeExpansionRequired)

		})
	}

}

func TestCSIController_DeleteVolume(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		Name             string
		ClientSetupFunc  func(*fake.Client)
		Request          *structs.ClientCSIControllerDeleteVolumeRequest
		ExpectedErr      error
		ExpectedResponse *structs.ClientCSIControllerDeleteVolumeResponse
	}{
		{
			Name: "returns plugin not found errors",
			Request: &structs.ClientCSIControllerDeleteVolumeRequest{
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: "some-garbage",
				},
			},
			ExpectedErr: errors.New("CSI.ControllerDeleteVolume: CSI client error (retryable): plugin some-garbage for type csi-controller not found"),
		},
		{
			Name: "returns transitive errors",
			ClientSetupFunc: func(fc *fake.Client) {
				fc.NextControllerDeleteVolumeErr = errors.New("internal plugin error")
			},
			Request: &structs.ClientCSIControllerDeleteVolumeRequest{
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: fakePlugin.Name,
				},
				ExternalVolumeID: "1234-4321-1234-4321",
			},
			ExpectedErr: errors.New("CSI.ControllerDeleteVolume: internal plugin error"),
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
			client.dynamicRegistry.StubDispenserForType(
				dynamicplugins.PluginTypeCSIController, dispenserFunc)

			err := client.dynamicRegistry.RegisterPlugin(fakePlugin)
			require.Nil(err)

			var resp structs.ClientCSIControllerDeleteVolumeResponse
			err = client.ClientRPC("CSI.ControllerDeleteVolume", tc.Request, &resp)
			require.Equal(tc.ExpectedErr, err)
			if tc.ExpectedResponse != nil {
				require.Equal(tc.ExpectedResponse, &resp)
			}
		})
	}
}

func TestCSIController_ListVolumes(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		Name             string
		ClientSetupFunc  func(*fake.Client)
		Request          *structs.ClientCSIControllerListVolumesRequest
		ExpectedErr      error
		ExpectedResponse *structs.ClientCSIControllerListVolumesResponse
	}{
		{
			Name: "returns plugin not found errors",
			Request: &structs.ClientCSIControllerListVolumesRequest{
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: "some-garbage",
				},
			},
			ExpectedErr: errors.New("CSI.ControllerListVolumes: CSI client error (retryable): plugin some-garbage for type csi-controller not found"),
		},
		{
			Name: "returns transitive errors",
			ClientSetupFunc: func(fc *fake.Client) {
				fc.NextControllerListVolumesErr = errors.New("internal plugin error")
			},
			Request: &structs.ClientCSIControllerListVolumesRequest{
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: fakePlugin.Name,
				},
			},
			ExpectedErr: errors.New("CSI.ControllerListVolumes: internal plugin error"),
		},
		{
			Name: "returns volumes",
			ClientSetupFunc: func(fc *fake.Client) {
				fc.NextControllerListVolumesResponse = &csi.ControllerListVolumesResponse{
					Entries: []*csi.ListVolumesResponse_Entry{
						{
							Volume: &csi.Volume{
								CapacityBytes:    1000000,
								ExternalVolumeID: "vol-1",
								VolumeContext:    map[string]string{"foo": "bar"},
								ContentSource: &csi.VolumeContentSource{
									SnapshotID: "snap-1",
								},
							},
							Status: &csi.ListVolumesResponse_VolumeStatus{
								PublishedNodeIds: []string{"i-1234", "i-5678"},
								VolumeCondition: &csi.VolumeCondition{
									Message: "ok",
								},
							},
						},
					},
					NextToken: "2",
				}
			},
			Request: &structs.ClientCSIControllerListVolumesRequest{
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: fakePlugin.Name,
				},
				StartingToken: "1",
				MaxEntries:    100,
			},
			ExpectedResponse: &structs.ClientCSIControllerListVolumesResponse{
				Entries: []*nstructs.CSIVolumeExternalStub{
					{
						ExternalID:               "vol-1",
						CapacityBytes:            1000000,
						VolumeContext:            map[string]string{"foo": "bar"},
						SnapshotID:               "snap-1",
						PublishedExternalNodeIDs: []string{"i-1234", "i-5678"},
						Status:                   "ok",
					},
				},
				NextToken: "2",
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
			client.dynamicRegistry.StubDispenserForType(
				dynamicplugins.PluginTypeCSIController, dispenserFunc)

			err := client.dynamicRegistry.RegisterPlugin(fakePlugin)
			require.Nil(err)

			var resp structs.ClientCSIControllerListVolumesResponse
			err = client.ClientRPC("CSI.ControllerListVolumes", tc.Request, &resp)
			require.Equal(tc.ExpectedErr, err)
			if tc.ExpectedResponse != nil {
				require.Equal(tc.ExpectedResponse, &resp)
			}
		})
	}
}
func TestCSIController_CreateSnapshot(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		Name             string
		ClientSetupFunc  func(*fake.Client)
		Request          *structs.ClientCSIControllerCreateSnapshotRequest
		ExpectedErr      error
		ExpectedResponse *structs.ClientCSIControllerCreateSnapshotResponse
	}{
		{
			Name: "returns plugin not found errors",
			Request: &structs.ClientCSIControllerCreateSnapshotRequest{
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: "some-garbage",
				},
			},
			ExpectedErr: errors.New("CSI.ControllerCreateSnapshot: CSI client error (retryable): plugin some-garbage for type csi-controller not found"),
		},
		{
			Name: "returns transitive errors",
			ClientSetupFunc: func(fc *fake.Client) {
				fc.NextControllerCreateSnapshotErr = errors.New("internal plugin error")
			},
			Request: &structs.ClientCSIControllerCreateSnapshotRequest{
				ExternalSourceVolumeID: "vol-1",
				Name:                   "1234-4321-1234-4321",
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: fakePlugin.Name,
				},
			},
			ExpectedErr: errors.New("CSI.ControllerCreateSnapshot: internal plugin error"),
		},
		{
			Name: "returns snapshot on success",
			ClientSetupFunc: func(fc *fake.Client) {
				fc.NextControllerCreateSnapshotResponse = &csi.ControllerCreateSnapshotResponse{
					Snapshot: &csi.Snapshot{
						ID:             "snap-12345",
						SourceVolumeID: "vol-1",
						SizeBytes:      10000000,
						IsReady:        true,
					},
				}
			},
			Request: &structs.ClientCSIControllerCreateSnapshotRequest{
				ExternalSourceVolumeID: "vol-1",
				Name:                   "1234-4321-1234-4321",
				Secrets:                nstructs.CSISecrets{"password": "xyzzy"},
				Parameters:             map[string]string{"foo": "bar"},
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: fakePlugin.Name,
				},
			},
			ExpectedResponse: &structs.ClientCSIControllerCreateSnapshotResponse{
				ID:                     "snap-12345",
				ExternalSourceVolumeID: "vol-1",
				SizeBytes:              10000000,
				IsReady:                true,
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
			client.dynamicRegistry.StubDispenserForType(
				dynamicplugins.PluginTypeCSIController, dispenserFunc)

			err := client.dynamicRegistry.RegisterPlugin(fakePlugin)
			require.Nil(err)

			var resp structs.ClientCSIControllerCreateSnapshotResponse
			err = client.ClientRPC("CSI.ControllerCreateSnapshot", tc.Request, &resp)
			require.Equal(tc.ExpectedErr, err)
			if tc.ExpectedResponse != nil {
				require.Equal(tc.ExpectedResponse, &resp)
			}
		})
	}
}

func TestCSIController_DeleteSnapshot(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		Name             string
		ClientSetupFunc  func(*fake.Client)
		Request          *structs.ClientCSIControllerDeleteSnapshotRequest
		ExpectedErr      error
		ExpectedResponse *structs.ClientCSIControllerDeleteSnapshotResponse
	}{
		{
			Name: "returns plugin not found errors",
			Request: &structs.ClientCSIControllerDeleteSnapshotRequest{
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: "some-garbage",
				},
			},
			ExpectedErr: errors.New("CSI.ControllerDeleteSnapshot: CSI client error (retryable): plugin some-garbage for type csi-controller not found"),
		},
		{
			Name: "returns transitive errors",
			ClientSetupFunc: func(fc *fake.Client) {
				fc.NextControllerDeleteSnapshotErr = errors.New("internal plugin error")
			},
			Request: &structs.ClientCSIControllerDeleteSnapshotRequest{
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: fakePlugin.Name,
				},
				ID: "1234-4321-1234-4321",
			},
			ExpectedErr: errors.New("CSI.ControllerDeleteSnapshot: internal plugin error"),
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
			client.dynamicRegistry.StubDispenserForType(
				dynamicplugins.PluginTypeCSIController, dispenserFunc)

			err := client.dynamicRegistry.RegisterPlugin(fakePlugin)
			require.Nil(err)

			var resp structs.ClientCSIControllerDeleteSnapshotResponse
			err = client.ClientRPC("CSI.ControllerDeleteSnapshot", tc.Request, &resp)
			require.Equal(tc.ExpectedErr, err)
			if tc.ExpectedResponse != nil {
				require.Equal(tc.ExpectedResponse, &resp)
			}
		})
	}
}

func TestCSIController_ListSnapshots(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		Name             string
		ClientSetupFunc  func(*fake.Client)
		Request          *structs.ClientCSIControllerListSnapshotsRequest
		ExpectedErr      error
		ExpectedResponse *structs.ClientCSIControllerListSnapshotsResponse
	}{
		{
			Name: "returns plugin not found errors",
			Request: &structs.ClientCSIControllerListSnapshotsRequest{
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: "some-garbage",
				},
			},
			ExpectedErr: errors.New("CSI.ControllerListSnapshots: CSI client error (retryable): plugin some-garbage for type csi-controller not found"),
		},
		{
			Name: "returns transitive errors",
			ClientSetupFunc: func(fc *fake.Client) {
				fc.NextControllerListSnapshotsErr = errors.New("internal plugin error")
			},
			Request: &structs.ClientCSIControllerListSnapshotsRequest{
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: fakePlugin.Name,
				},
			},
			ExpectedErr: errors.New("CSI.ControllerListSnapshots: internal plugin error"),
		},
		{
			Name: "returns volumes",
			ClientSetupFunc: func(fc *fake.Client) {
				fc.NextControllerListSnapshotsResponse = &csi.ControllerListSnapshotsResponse{
					Entries: []*csi.ListSnapshotsResponse_Entry{
						{
							Snapshot: &csi.Snapshot{
								ID:             "snap-1",
								SourceVolumeID: "vol-1",
								SizeBytes:      1000000,
								IsReady:        true,
							},
						},
					},
					NextToken: "2",
				}
			},
			Request: &structs.ClientCSIControllerListSnapshotsRequest{
				CSIControllerQuery: structs.CSIControllerQuery{
					PluginID: fakePlugin.Name,
				},
				Secrets: map[string]string{
					"secret-key-1": "secret-val-1",
					"secret-key-2": "secret-val-2",
				},
				StartingToken: "1",
				MaxEntries:    100,
			},
			ExpectedResponse: &structs.ClientCSIControllerListSnapshotsResponse{
				Entries: []*nstructs.CSISnapshot{
					{
						ID:                     "snap-1",
						ExternalSourceVolumeID: "vol-1",
						SizeBytes:              1000000,
						IsReady:                true,
						PluginID:               fakePlugin.Name,
					},
				},
				NextToken: "2",
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
			client.dynamicRegistry.StubDispenserForType(
				dynamicplugins.PluginTypeCSIController, dispenserFunc)

			err := client.dynamicRegistry.RegisterPlugin(fakePlugin)
			require.Nil(err)

			var resp structs.ClientCSIControllerListSnapshotsResponse
			err = client.ClientRPC("CSI.ControllerListSnapshots", tc.Request, &resp)
			require.Equal(tc.ExpectedErr, err)
			if tc.ExpectedResponse != nil {
				require.Equal(tc.ExpectedResponse, &resp)
			}
		})
	}
}

func TestCSINode_DetachVolume(t *testing.T) {
	ci.Parallel(t)

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
			ExpectedErr: errors.New("CSI.NodeDetachVolume: plugin some-garbage for type csi-node not found"),
		},
		{
			Name: "validates volumeid is not empty",
			Request: &structs.ClientCSINodeDetachVolumeRequest{
				PluginID: fakeNodePlugin.Name,
			},
			ExpectedErr: errors.New("CSI.NodeDetachVolume: VolumeID is required"),
		},
		{
			Name: "validates nodeid is not empty",
			Request: &structs.ClientCSINodeDetachVolumeRequest{
				PluginID: fakeNodePlugin.Name,
				VolumeID: "1234-4321-1234-4321",
			},
			ExpectedErr: errors.New("CSI.NodeDetachVolume: AllocID is required"),
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
			ExpectedErr: errors.New("CSI.NodeDetachVolume: plugin test-plugin for type csi-node not found"),
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
