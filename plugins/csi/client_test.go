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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			ic, _, _, client := newTestClient()
			defer client.Close()

			ic.NextErr = tc.ResponseErr
			ic.NextPluginProbe = tc.ProbeResponse

			resp, err := client.PluginProbe(context.TODO())
			if tc.ExpectedErr != nil {
				require.EqualError(t, err, tc.ExpectedErr.Error())
			}

			require.Equal(t, tc.ExpectedResponse, resp)
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

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			ic, _, _, client := newTestClient()
			defer client.Close()

			ic.NextErr = tc.ResponseErr
			ic.NextPluginInfo = tc.InfoResponse

			name, version, err := client.PluginGetInfo(context.TODO())
			if tc.ExpectedErr != nil {
				require.EqualError(t, err, tc.ExpectedErr.Error())
			}

			require.Equal(t, tc.ExpectedResponseName, name)
			require.Equal(t, tc.ExpectedResponseVersion, version)
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

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			ic, _, _, client := newTestClient()
			defer client.Close()

			ic.NextErr = tc.ResponseErr
			ic.NextPluginCapabilities = tc.Response

			resp, err := client.PluginGetCapabilities(context.TODO())
			if tc.ExpectedErr != nil {
				require.EqualError(t, err, tc.ExpectedErr.Error())
			}

			require.Equal(t, tc.ExpectedResponse, resp)
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
				require.EqualError(t, err, tc.ExpectedErr.Error())
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
				require.EqualError(t, err, tc.ExpectedErr.Error())
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
			Request:     &ControllerPublishVolumeRequest{ExternalID: "vol", NodeID: "node"},
			ResponseErr: status.Errorf(codes.Internal, "some grpc error"),
			ExpectedErr: fmt.Errorf("controller plugin returned an internal error, check the plugin allocation logs for more information: rpc error: code = Internal desc = some grpc error"),
		},
		{
			Name:        "handles missing NodeID",
			Request:     &ControllerPublishVolumeRequest{ExternalID: "vol"},
			Response:    &csipbv1.ControllerPublishVolumeResponse{},
			ExpectedErr: fmt.Errorf("missing NodeID"),
		},

		{
			Name: "handles PublishContext == nil",
			Request: &ControllerPublishVolumeRequest{
				ExternalID: "vol", NodeID: "node"},
			Response:         &csipbv1.ControllerPublishVolumeResponse{},
			ExpectedResponse: &ControllerPublishVolumeResponse{},
		},
		{
			Name:    "handles PublishContext != nil",
			Request: &ControllerPublishVolumeRequest{ExternalID: "vol", NodeID: "node"},
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

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			_, cc, _, client := newTestClient()
			defer client.Close()

			cc.NextErr = tc.ResponseErr
			cc.NextPublishVolumeResponse = tc.Response

			resp, err := client.ControllerPublishVolume(context.TODO(), tc.Request)
			if tc.ExpectedErr != nil {
				require.EqualError(t, err, tc.ExpectedErr.Error())
			}

			require.Equal(t, tc.ExpectedResponse, resp)
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
			Name:        "handles underlying grpc errors",
			Request:     &ControllerUnpublishVolumeRequest{ExternalID: "vol", NodeID: "node"},
			ResponseErr: status.Errorf(codes.Internal, "some grpc error"),
			ExpectedErr: fmt.Errorf("controller plugin returned an internal error, check the plugin allocation logs for more information: rpc error: code = Internal desc = some grpc error"),
		},
		{
			Name:             "handles missing NodeID",
			Request:          &ControllerUnpublishVolumeRequest{ExternalID: "vol"},
			ExpectedErr:      fmt.Errorf("missing NodeID"),
			ExpectedResponse: nil,
		},
		{
			Name:             "handles successful response",
			Request:          &ControllerUnpublishVolumeRequest{ExternalID: "vol", NodeID: "node"},
			ExpectedResponse: &ControllerUnpublishVolumeResponse{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			_, cc, _, client := newTestClient()
			defer client.Close()

			cc.NextErr = tc.ResponseErr
			cc.NextUnpublishVolumeResponse = tc.Response

			resp, err := client.ControllerUnpublishVolume(context.TODO(), tc.Request)
			if tc.ExpectedErr != nil {
				require.EqualError(t, err, tc.ExpectedErr.Error())
			}

			require.Equal(t, tc.ExpectedResponse, resp)
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
			ResponseErr: status.Errorf(codes.Internal, "some grpc error"),
			ExpectedErr: fmt.Errorf("controller plugin returned an internal error, check the plugin allocation logs for more information: rpc error: code = Internal desc = some grpc error"),
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
			// this is a multierror
			ExpectedErr: fmt.Errorf("volume capability validation failed: 1 error occurred:\n\t* requested AccessMode MULTI_NODE_MULTI_WRITER, got SINGLE_NODE_WRITER\n\n"),
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
			// this is a multierror
			ExpectedErr: fmt.Errorf("volume capability validation failed: 1 error occurred:\n\t* requested mount flags did not match available capabilities\n\n"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
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
			req := &ControllerValidateVolumeRequest{
				ExternalID:   "volumeID",
				Secrets:      structs.CSISecrets{},
				Capabilities: requestedCaps,
				Parameters:   map[string]string{},
				Context:      map[string]string{},
			}

			cc.NextValidateVolumeCapabilitiesResponse = tc.Response
			cc.NextErr = tc.ResponseErr

			err := client.ControllerValidateCapabilities(context.TODO(), req)
			if tc.ExpectedErr != nil {
				require.EqualError(t, err, tc.ExpectedErr.Error())
			} else {
				require.NoError(t, err, tc.Name)
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
			ResponseErr: status.Errorf(codes.AlreadyExists, "some grpc error"),
			ExpectedErr: fmt.Errorf("volume \"foo\" is already staged to \"/path\" but with incompatible capabilities for this request: rpc error: code = AlreadyExists desc = some grpc error"),
		},
		{
			Name:        "handles success",
			ResponseErr: nil,
			ExpectedErr: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			_, _, nc, client := newTestClient()
			defer client.Close()

			nc.NextErr = tc.ResponseErr
			nc.NextStageVolumeResponse = tc.Response

			err := client.NodeStageVolume(context.TODO(), &NodeStageVolumeRequest{
				ExternalID:        "foo",
				StagingTargetPath: "/path",
				VolumeCapability:  &VolumeCapability{},
			})
			if tc.ExpectedErr != nil {
				require.EqualError(t, err, tc.ExpectedErr.Error())
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
			ResponseErr: status.Errorf(codes.Internal, "some grpc error"),
			ExpectedErr: fmt.Errorf("node plugin returned an internal error, check the plugin allocation logs for more information: rpc error: code = Internal desc = some grpc error"),
		},
		{
			Name:        "handles success",
			ResponseErr: nil,
			ExpectedErr: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			_, _, nc, client := newTestClient()
			defer client.Close()

			nc.NextErr = tc.ResponseErr
			nc.NextUnstageVolumeResponse = tc.Response

			err := client.NodeUnstageVolume(context.TODO(), "foo", "/foo")
			if tc.ExpectedErr != nil {
				require.EqualError(t, err, tc.ExpectedErr.Error())
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
				ExternalID:       "foo",
				TargetPath:       "/dev/null",
				VolumeCapability: &VolumeCapability{},
			},
			ResponseErr: status.Errorf(codes.Internal, "some grpc error"),
			ExpectedErr: fmt.Errorf("node plugin returned an internal error, check the plugin allocation logs for more information: rpc error: code = Internal desc = some grpc error"),
		},
		{
			Name: "handles success",
			Request: &NodePublishVolumeRequest{
				ExternalID:       "foo",
				TargetPath:       "/dev/null",
				VolumeCapability: &VolumeCapability{},
			},
			ResponseErr: nil,
			ExpectedErr: nil,
		},
		{
			Name: "Performs validation of the publish volume request",
			Request: &NodePublishVolumeRequest{
				ExternalID: "",
			},
			ResponseErr: nil,
			ExpectedErr: errors.New("validation error: missing volume ID"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			_, _, nc, client := newTestClient()
			defer client.Close()

			nc.NextErr = tc.ResponseErr
			nc.NextPublishVolumeResponse = tc.Response

			err := client.NodePublishVolume(context.TODO(), tc.Request)
			if tc.ExpectedErr != nil {
				require.EqualError(t, err, tc.ExpectedErr.Error())
			} else {
				require.Nil(t, err)
			}
		})
	}
}
func TestClient_RPC_NodeUnpublishVolume(t *testing.T) {
	cases := []struct {
		Name        string
		ExternalID  string
		TargetPath  string
		ResponseErr error
		Response    *csipbv1.NodeUnpublishVolumeResponse
		ExpectedErr error
	}{
		{
			Name:        "handles underlying grpc errors",
			ExternalID:  "foo",
			TargetPath:  "/dev/null",
			ResponseErr: status.Errorf(codes.Internal, "some grpc error"),
			ExpectedErr: fmt.Errorf("node plugin returned an internal error, check the plugin allocation logs for more information: rpc error: code = Internal desc = some grpc error"),
		},
		{
			Name:        "handles success",
			ExternalID:  "foo",
			TargetPath:  "/dev/null",
			ResponseErr: nil,
			ExpectedErr: nil,
		},
		{
			Name:        "Performs validation of the request args - ExternalID",
			ResponseErr: nil,
			ExpectedErr: errors.New("missing volumeID"),
		},
		{
			Name:        "Performs validation of the request args - TargetPath",
			ExternalID:  "foo",
			ResponseErr: nil,
			ExpectedErr: errors.New("missing targetPath"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			_, _, nc, client := newTestClient()
			defer client.Close()

			nc.NextErr = tc.ResponseErr
			nc.NextUnpublishVolumeResponse = tc.Response

			err := client.NodeUnpublishVolume(context.TODO(), tc.ExternalID, tc.TargetPath)
			if tc.ExpectedErr != nil {
				require.EqualError(t, err, tc.ExpectedErr.Error())
			} else {
				require.Nil(t, err)
			}
		})
	}
}
