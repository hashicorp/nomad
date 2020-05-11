package csi

import (
	"context"
	"errors"
	"fmt"
	"testing"

	csipbv1 "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/hashicorp/nomad/nomad/structs"
	fake "github.com/hashicorp/nomad/plugins/csi/testing"
	"github.com/stretchr/testify/require"
)

func newTestClient() (*fake.IdentityClient, *fake.ControllerClient, *fake.NodeClient, CSIPlugin) {
	ic := fake.NewIdentityClient()
	cc := fake.NewControllerClient()
	nc := fake.NewNodeClient()
	client := &client{
		identityClient:   ic,
		controllerClient: cc,
		nodeClient:       nc,
	}

	return ic, cc, nc, client
}

func TestClient_RPC_PluginProbe(t *testing.T) {
	cases := []struct {
		Name             string
		ResponseErr      error
		ProbeResponse    *csipbv1.ProbeResponse
		ExpectedResponse bool
		ExpectedErr      error
	}{
		{
			Name:        "handles underlying grpc errors",
			ResponseErr: fmt.Errorf("some grpc error"),
			ExpectedErr: fmt.Errorf("some grpc error"),
		},
		{
			Name: "returns false for ready when the provider returns false",
			ProbeResponse: &csipbv1.ProbeResponse{
				Ready: &wrappers.BoolValue{Value: false},
			},
			ExpectedResponse: false,
		},
		{
			Name: "returns true for ready when the provider returns true",
			ProbeResponse: &csipbv1.ProbeResponse{
				Ready: &wrappers.BoolValue{Value: true},
			},
			ExpectedResponse: true,
		},
		{
			/* When a SP does not return a ready value, a CO MAY treat this as ready.
			   We do so because example plugins rely on this behaviour. We may
				 re-evaluate this decision in the future. */
			Name: "returns true for ready when the provider returns a nil wrapper",
			ProbeResponse: &csipbv1.ProbeResponse{
				Ready: nil,
			},
			ExpectedResponse: true,
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			ic, _, _, client := newTestClient()
			defer client.Close()

			ic.NextErr = c.ResponseErr
			ic.NextPluginProbe = c.ProbeResponse

			resp, err := client.PluginProbe(context.TODO())
			if c.ExpectedErr != nil {
				require.Error(t, c.ExpectedErr, err)
			}

			require.Equal(t, c.ExpectedResponse, resp)
		})
	}

}

func TestClient_RPC_PluginInfo(t *testing.T) {
	cases := []struct {
		Name                    string
		ResponseErr             error
		InfoResponse            *csipbv1.GetPluginInfoResponse
		ExpectedResponseName    string
		ExpectedResponseVersion string
		ExpectedErr             error
	}{
		{
			Name:        "handles underlying grpc errors",
			ResponseErr: fmt.Errorf("some grpc error"),
			ExpectedErr: fmt.Errorf("some grpc error"),
		},
		{
			Name: "returns an error if we receive an empty `name`",
			InfoResponse: &csipbv1.GetPluginInfoResponse{
				Name:          "",
				VendorVersion: "",
			},
			ExpectedErr: fmt.Errorf("PluginGetInfo: plugin returned empty name field"),
		},
		{
			Name: "returns the name when successfully retrieved and not empty",
			InfoResponse: &csipbv1.GetPluginInfoResponse{
				Name:          "com.hashicorp.storage",
				VendorVersion: "1.0.1",
			},
			ExpectedResponseName:    "com.hashicorp.storage",
			ExpectedResponseVersion: "1.0.1",
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			ic, _, _, client := newTestClient()
			defer client.Close()

			ic.NextErr = c.ResponseErr
			ic.NextPluginInfo = c.InfoResponse

			name, version, err := client.PluginGetInfo(context.TODO())
			if c.ExpectedErr != nil {
				require.Error(t, c.ExpectedErr, err)
			}

			require.Equal(t, c.ExpectedResponseName, name)
			require.Equal(t, c.ExpectedResponseVersion, version)
		})
	}

}

func TestClient_RPC_PluginGetCapabilities(t *testing.T) {
	cases := []struct {
		Name             string
		ResponseErr      error
		Response         *csipbv1.GetPluginCapabilitiesResponse
		ExpectedResponse *PluginCapabilitySet
		ExpectedErr      error
	}{
		{
			Name:        "handles underlying grpc errors",
			ResponseErr: fmt.Errorf("some grpc error"),
			ExpectedErr: fmt.Errorf("some grpc error"),
		},
		{
			Name: "HasControllerService is true when it's part of the response",
			Response: &csipbv1.GetPluginCapabilitiesResponse{
				Capabilities: []*csipbv1.PluginCapability{
					{
						Type: &csipbv1.PluginCapability_Service_{
							Service: &csipbv1.PluginCapability_Service{
								Type: csipbv1.PluginCapability_Service_CONTROLLER_SERVICE,
							},
						},
					},
				},
			},
			ExpectedResponse: &PluginCapabilitySet{hasControllerService: true},
		},
		{
			Name: "HasTopologies is true when it's part of the response",
			Response: &csipbv1.GetPluginCapabilitiesResponse{
				Capabilities: []*csipbv1.PluginCapability{
					{
						Type: &csipbv1.PluginCapability_Service_{
							Service: &csipbv1.PluginCapability_Service{
								Type: csipbv1.PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS,
							},
						},
					},
				},
			},
			ExpectedResponse: &PluginCapabilitySet{hasTopologies: true},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			ic, _, _, client := newTestClient()
			defer client.Close()

			ic.NextErr = c.ResponseErr
			ic.NextPluginCapabilities = c.Response

			resp, err := client.PluginGetCapabilities(context.TODO())
			if c.ExpectedErr != nil {
				require.Error(t, c.ExpectedErr, err)
			}

			require.Equal(t, c.ExpectedResponse, resp)
		})
	}
}

func TestClient_RPC_ControllerGetCapabilities(t *testing.T) {
	cases := []struct {
		Name             string
		ResponseErr      error
		Response         *csipbv1.ControllerGetCapabilitiesResponse
		ExpectedResponse *ControllerCapabilitySet
		ExpectedErr      error
	}{
		{
			Name:        "handles underlying grpc errors",
			ResponseErr: fmt.Errorf("some grpc error"),
			ExpectedErr: fmt.Errorf("some grpc error"),
		},
		{
			Name: "ignores unknown capabilities",
			Response: &csipbv1.ControllerGetCapabilitiesResponse{
				Capabilities: []*csipbv1.ControllerServiceCapability{
					{
						Type: &csipbv1.ControllerServiceCapability_Rpc{
							Rpc: &csipbv1.ControllerServiceCapability_RPC{
								Type: csipbv1.ControllerServiceCapability_RPC_GET_CAPACITY,
							},
						},
					},
				},
			},
			ExpectedResponse: &ControllerCapabilitySet{},
		},
		{
			Name: "detects list volumes capabilities",
			Response: &csipbv1.ControllerGetCapabilitiesResponse{
				Capabilities: []*csipbv1.ControllerServiceCapability{
					{
						Type: &csipbv1.ControllerServiceCapability_Rpc{
							Rpc: &csipbv1.ControllerServiceCapability_RPC{
								Type: csipbv1.ControllerServiceCapability_RPC_LIST_VOLUMES,
							},
						},
					},
					{
						Type: &csipbv1.ControllerServiceCapability_Rpc{
							Rpc: &csipbv1.ControllerServiceCapability_RPC{
								Type: csipbv1.ControllerServiceCapability_RPC_LIST_VOLUMES_PUBLISHED_NODES,
							},
						},
					},
				},
			},
			ExpectedResponse: &ControllerCapabilitySet{
				HasListVolumes:               true,
				HasListVolumesPublishedNodes: true,
			},
		},
		{
			Name: "detects publish capabilities",
			Response: &csipbv1.ControllerGetCapabilitiesResponse{
				Capabilities: []*csipbv1.ControllerServiceCapability{
					{
						Type: &csipbv1.ControllerServiceCapability_Rpc{
							Rpc: &csipbv1.ControllerServiceCapability_RPC{
								Type: csipbv1.ControllerServiceCapability_RPC_PUBLISH_READONLY,
							},
						},
					},
					{
						Type: &csipbv1.ControllerServiceCapability_Rpc{
							Rpc: &csipbv1.ControllerServiceCapability_RPC{
								Type: csipbv1.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
							},
						},
					},
				},
			},
			ExpectedResponse: &ControllerCapabilitySet{
				HasPublishUnpublishVolume: true,
				HasPublishReadonly:        true,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			_, cc, _, client := newTestClient()
			defer client.Close()

			cc.NextErr = tc.ResponseErr
			cc.NextCapabilitiesResponse = tc.Response

			resp, err := client.ControllerGetCapabilities(context.TODO())
			if tc.ExpectedErr != nil {
				require.Error(t, tc.ExpectedErr, err)
			}

			require.Equal(t, tc.ExpectedResponse, resp)
		})
	}
}

func TestClient_RPC_NodeGetCapabilities(t *testing.T) {
	cases := []struct {
		Name             string
		ResponseErr      error
		Response         *csipbv1.NodeGetCapabilitiesResponse
		ExpectedResponse *NodeCapabilitySet
		ExpectedErr      error
	}{
		{
			Name:        "handles underlying grpc errors",
			ResponseErr: fmt.Errorf("some grpc error"),
			ExpectedErr: fmt.Errorf("some grpc error"),
		},
		{
			Name: "ignores unknown capabilities",
			Response: &csipbv1.NodeGetCapabilitiesResponse{
				Capabilities: []*csipbv1.NodeServiceCapability{
					{
						Type: &csipbv1.NodeServiceCapability_Rpc{
							Rpc: &csipbv1.NodeServiceCapability_RPC{
								Type: csipbv1.NodeServiceCapability_RPC_EXPAND_VOLUME,
							},
						},
					},
				},
			},
			ExpectedResponse: &NodeCapabilitySet{},
		},
		{
			Name: "detects stage volumes capability",
			Response: &csipbv1.NodeGetCapabilitiesResponse{
				Capabilities: []*csipbv1.NodeServiceCapability{
					{
						Type: &csipbv1.NodeServiceCapability_Rpc{
							Rpc: &csipbv1.NodeServiceCapability_RPC{
								Type: csipbv1.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
							},
						},
					},
				},
			},
			ExpectedResponse: &NodeCapabilitySet{
				HasStageUnstageVolume: true,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			_, _, nc, client := newTestClient()
			defer client.Close()

			nc.NextErr = tc.ResponseErr
			nc.NextCapabilitiesResponse = tc.Response

			resp, err := client.NodeGetCapabilities(context.TODO())
			if tc.ExpectedErr != nil {
				require.Error(t, tc.ExpectedErr, err)
			}

			require.Equal(t, tc.ExpectedResponse, resp)
		})
	}
}

func TestClient_RPC_ControllerPublishVolume(t *testing.T) {
	cases := []struct {
		Name             string
		Request          *ControllerPublishVolumeRequest
		ResponseErr      error
		Response         *csipbv1.ControllerPublishVolumeResponse
		ExpectedResponse *ControllerPublishVolumeResponse
		ExpectedErr      error
	}{
		{
			Name:        "handles underlying grpc errors",
			Request:     &ControllerPublishVolumeRequest{},
			ResponseErr: fmt.Errorf("some grpc error"),
			ExpectedErr: fmt.Errorf("some grpc error"),
		},
		{
			Name:        "Handles missing NodeID",
			Request:     &ControllerPublishVolumeRequest{},
			Response:    &csipbv1.ControllerPublishVolumeResponse{},
			ExpectedErr: fmt.Errorf("missing NodeID"),
		},

		{
			Name:             "Handles PublishContext == nil",
			Request:          &ControllerPublishVolumeRequest{VolumeID: "vol", NodeID: "node"},
			Response:         &csipbv1.ControllerPublishVolumeResponse{},
			ExpectedResponse: &ControllerPublishVolumeResponse{},
		},
		{
			Name:    "Handles PublishContext != nil",
			Request: &ControllerPublishVolumeRequest{VolumeID: "vol", NodeID: "node"},
			Response: &csipbv1.ControllerPublishVolumeResponse{
				PublishContext: map[string]string{
					"com.hashicorp/nomad-node-id": "foobar",
					"com.plugin/device":           "/dev/sdc1",
				},
			},
			ExpectedResponse: &ControllerPublishVolumeResponse{
				PublishContext: map[string]string{
					"com.hashicorp/nomad-node-id": "foobar",
					"com.plugin/device":           "/dev/sdc1",
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			_, cc, _, client := newTestClient()
			defer client.Close()

			cc.NextErr = c.ResponseErr
			cc.NextPublishVolumeResponse = c.Response

			resp, err := client.ControllerPublishVolume(context.TODO(), c.Request)
			if c.ExpectedErr != nil {
				require.Error(t, c.ExpectedErr, err)
			}

			require.Equal(t, c.ExpectedResponse, resp)
		})
	}
}

func TestClient_RPC_ControllerUnpublishVolume(t *testing.T) {
	cases := []struct {
		Name             string
		Request          *ControllerUnpublishVolumeRequest
		ResponseErr      error
		Response         *csipbv1.ControllerUnpublishVolumeResponse
		ExpectedResponse *ControllerUnpublishVolumeResponse
		ExpectedErr      error
	}{
		{
			Name:        "Handles underlying grpc errors",
			Request:     &ControllerUnpublishVolumeRequest{},
			ResponseErr: fmt.Errorf("some grpc error"),
			ExpectedErr: fmt.Errorf("some grpc error"),
		},
		{
			Name:             "Handles missing NodeID",
			Request:          &ControllerUnpublishVolumeRequest{},
			ExpectedErr:      fmt.Errorf("missing NodeID"),
			ExpectedResponse: nil,
		},
		{
			Name:             "Handles successful response",
			Request:          &ControllerUnpublishVolumeRequest{VolumeID: "vol", NodeID: "node"},
			ExpectedErr:      fmt.Errorf("missing NodeID"),
			ExpectedResponse: &ControllerUnpublishVolumeResponse{},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			_, cc, _, client := newTestClient()
			defer client.Close()

			cc.NextErr = c.ResponseErr
			cc.NextUnpublishVolumeResponse = c.Response

			resp, err := client.ControllerUnpublishVolume(context.TODO(), c.Request)
			if c.ExpectedErr != nil {
				require.Error(t, c.ExpectedErr, err)
			}

			require.Equal(t, c.ExpectedResponse, resp)
		})
	}
}

func TestClient_RPC_ControllerValidateVolume(t *testing.T) {

	cases := []struct {
		Name        string
		ResponseErr error
		Response    *csipbv1.ValidateVolumeCapabilitiesResponse
		ExpectedErr error
	}{
		{
			Name:        "handles underlying grpc errors",
			ResponseErr: fmt.Errorf("some grpc error"),
			ExpectedErr: fmt.Errorf("some grpc error"),
		},
		{
			Name:        "handles empty success",
			Response:    &csipbv1.ValidateVolumeCapabilitiesResponse{},
			ResponseErr: nil,
			ExpectedErr: nil,
		},
		{
			Name: "handles validate success",
			Response: &csipbv1.ValidateVolumeCapabilitiesResponse{
				Confirmed: &csipbv1.ValidateVolumeCapabilitiesResponse_Confirmed{
					VolumeContext: map[string]string{},
					VolumeCapabilities: []*csipbv1.VolumeCapability{
						{
							AccessType: &csipbv1.VolumeCapability_Mount{
								Mount: &csipbv1.VolumeCapability_MountVolume{
									FsType:     "ext4",
									MountFlags: []string{"errors=remount-ro", "noatime"},
								},
							},
							AccessMode: &csipbv1.VolumeCapability_AccessMode{
								Mode: csipbv1.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
							},
						},
					},
				},
			},
			ResponseErr: nil,
			ExpectedErr: nil,
		},
		{
			Name: "handles validation failure block mismatch",
			Response: &csipbv1.ValidateVolumeCapabilitiesResponse{
				Confirmed: &csipbv1.ValidateVolumeCapabilitiesResponse_Confirmed{
					VolumeContext: map[string]string{},
					VolumeCapabilities: []*csipbv1.VolumeCapability{
						{
							AccessType: &csipbv1.VolumeCapability_Block{
								Block: &csipbv1.VolumeCapability_BlockVolume{},
							},
							AccessMode: &csipbv1.VolumeCapability_AccessMode{
								Mode: csipbv1.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
							},
						},
					},
				},
			},
			ResponseErr: nil,
			ExpectedErr: fmt.Errorf("volume capability validation failed"),
		},
		{
			Name: "handles validation failure mount flags",
			Response: &csipbv1.ValidateVolumeCapabilitiesResponse{
				Confirmed: &csipbv1.ValidateVolumeCapabilitiesResponse_Confirmed{
					VolumeContext: map[string]string{},
					VolumeCapabilities: []*csipbv1.VolumeCapability{
						{
							AccessType: &csipbv1.VolumeCapability_Mount{
								Mount: &csipbv1.VolumeCapability_MountVolume{
									FsType:     "ext4",
									MountFlags: []string{},
								},
							},
							AccessMode: &csipbv1.VolumeCapability_AccessMode{
								Mode: csipbv1.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
							},
						},
					},
				},
			},
			ResponseErr: nil,
			ExpectedErr: fmt.Errorf("volume capability validation failed"),
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			_, cc, _, client := newTestClient()
			defer client.Close()

			requestedCaps := &VolumeCapability{
				AccessType: VolumeAccessTypeMount,
				AccessMode: VolumeAccessModeMultiNodeMultiWriter,
				MountVolume: &structs.CSIMountOptions{ // should be ignored
					FSType:     "ext4",
					MountFlags: []string{"noatime", "errors=remount-ro"},
				},
			}
			cc.NextValidateVolumeCapabilitiesResponse = c.Response
			cc.NextErr = c.ResponseErr

			err := client.ControllerValidateCapabilities(
				context.TODO(), "volumeID", requestedCaps, structs.CSISecrets{})
			if c.ExpectedErr != nil {
				require.Error(t, c.ExpectedErr, err, c.Name)
			} else {
				require.NoError(t, err, c.Name)
			}
		})
	}

}

func TestClient_RPC_NodeStageVolume(t *testing.T) {
	cases := []struct {
		Name        string
		ResponseErr error
		Response    *csipbv1.NodeStageVolumeResponse
		ExpectedErr error
	}{
		{
			Name:        "handles underlying grpc errors",
			ResponseErr: fmt.Errorf("some grpc error"),
			ExpectedErr: fmt.Errorf("some grpc error"),
		},
		{
			Name:        "handles success",
			ResponseErr: nil,
			ExpectedErr: nil,
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			_, _, nc, client := newTestClient()
			defer client.Close()

			nc.NextErr = c.ResponseErr
			nc.NextStageVolumeResponse = c.Response

			err := client.NodeStageVolume(context.TODO(), "foo", nil, "/foo",
				&VolumeCapability{}, structs.CSISecrets{})
			if c.ExpectedErr != nil {
				require.Error(t, c.ExpectedErr, err)
			} else {
				require.Nil(t, err)
			}
		})
	}
}

func TestClient_RPC_NodeUnstageVolume(t *testing.T) {
	cases := []struct {
		Name        string
		ResponseErr error
		Response    *csipbv1.NodeUnstageVolumeResponse
		ExpectedErr error
	}{
		{
			Name:        "handles underlying grpc errors",
			ResponseErr: fmt.Errorf("some grpc error"),
			ExpectedErr: fmt.Errorf("some grpc error"),
		},
		{
			Name:        "handles success",
			ResponseErr: nil,
			ExpectedErr: nil,
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			_, _, nc, client := newTestClient()
			defer client.Close()

			nc.NextErr = c.ResponseErr
			nc.NextUnstageVolumeResponse = c.Response

			err := client.NodeUnstageVolume(context.TODO(), "foo", "/foo")
			if c.ExpectedErr != nil {
				require.Error(t, c.ExpectedErr, err)
			} else {
				require.Nil(t, err)
			}
		})
	}
}

func TestClient_RPC_NodePublishVolume(t *testing.T) {
	cases := []struct {
		Name        string
		Request     *NodePublishVolumeRequest
		ResponseErr error
		Response    *csipbv1.NodePublishVolumeResponse
		ExpectedErr error
	}{
		{
			Name: "handles underlying grpc errors",
			Request: &NodePublishVolumeRequest{
				VolumeID:         "foo",
				TargetPath:       "/dev/null",
				VolumeCapability: &VolumeCapability{},
			},
			ResponseErr: fmt.Errorf("some grpc error"),
			ExpectedErr: fmt.Errorf("some grpc error"),
		},
		{
			Name: "handles success",
			Request: &NodePublishVolumeRequest{
				VolumeID:         "foo",
				TargetPath:       "/dev/null",
				VolumeCapability: &VolumeCapability{},
			},
			ResponseErr: nil,
			ExpectedErr: nil,
		},
		{
			Name: "Performs validation of the publish volume request",
			Request: &NodePublishVolumeRequest{
				VolumeID: "",
			},
			ResponseErr: nil,
			ExpectedErr: errors.New("missing VolumeID"),
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			_, _, nc, client := newTestClient()
			defer client.Close()

			nc.NextErr = c.ResponseErr
			nc.NextPublishVolumeResponse = c.Response

			err := client.NodePublishVolume(context.TODO(), c.Request)
			if c.ExpectedErr != nil {
				require.Error(t, c.ExpectedErr, err)
			} else {
				require.Nil(t, err)
			}
		})
	}
}
func TestClient_RPC_NodeUnpublishVolume(t *testing.T) {
	cases := []struct {
		Name        string
		VolumeID    string
		TargetPath  string
		ResponseErr error
		Response    *csipbv1.NodeUnpublishVolumeResponse
		ExpectedErr error
	}{
		{
			Name:        "handles underlying grpc errors",
			VolumeID:    "foo",
			TargetPath:  "/dev/null",
			ResponseErr: fmt.Errorf("some grpc error"),
			ExpectedErr: fmt.Errorf("some grpc error"),
		},
		{
			Name:        "handles success",
			VolumeID:    "foo",
			TargetPath:  "/dev/null",
			ResponseErr: nil,
			ExpectedErr: nil,
		},
		{
			Name:        "Performs validation of the request args - VolumeID",
			ResponseErr: nil,
			ExpectedErr: errors.New("missing VolumeID"),
		},
		{
			Name:        "Performs validation of the request args - TargetPath",
			VolumeID:    "foo",
			ResponseErr: nil,
			ExpectedErr: errors.New("missing TargetPath"),
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			_, _, nc, client := newTestClient()
			defer client.Close()

			nc.NextErr = c.ResponseErr
			nc.NextUnpublishVolumeResponse = c.Response

			err := client.NodeUnpublishVolume(context.TODO(), c.VolumeID, c.TargetPath)
			if c.ExpectedErr != nil {
				require.Error(t, c.ExpectedErr, err)
			} else {
				require.Nil(t, err)
			}
		})
	}
}
