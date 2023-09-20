// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package csi

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	csipbv1 "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/structs"
	fake "github.com/hashicorp/nomad/plugins/csi/testing"
)

func newTestClient(t *testing.T) (*fake.IdentityClient, *fake.ControllerClient, *fake.NodeClient, CSIPlugin) {
	ic := fake.NewIdentityClient()
	cc := fake.NewControllerClient()
	nc := fake.NewNodeClient()

	// we've set this as non-blocking so it won't connect to the
	// socket unless a RPC is invoked
	conn, err := grpc.DialContext(context.Background(),
		filepath.Join(t.TempDir(), "csi.sock"), grpc.WithInsecure())
	if err != nil {
		t.Errorf("failed: %v", err)
	}

	client := &client{
		conn:             conn,
		identityClient:   ic,
		controllerClient: cc,
		nodeClient:       nc,
	}
	t.Cleanup(func() {
		_ = client.Close()
	})

	return ic, cc, nc, client
}

func TestClient_RPC_PluginProbe(t *testing.T) {
	ci.Parallel(t)

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
			ic, _, _, client := newTestClient(t)
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
	ci.Parallel(t)

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
			ic, _, _, client := newTestClient(t)
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
	ci.Parallel(t)

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
			ic, _, _, client := newTestClient(t)
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
	ci.Parallel(t)

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
								Type: csipbv1.ControllerServiceCapability_RPC_UNKNOWN,
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
			_, cc, _, client := newTestClient(t)
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
	ci.Parallel(t)

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
			Name: "detects multiple capabilities",
			Response: &csipbv1.NodeGetCapabilitiesResponse{
				Capabilities: []*csipbv1.NodeServiceCapability{
					{
						Type: &csipbv1.NodeServiceCapability_Rpc{
							Rpc: &csipbv1.NodeServiceCapability_RPC{
								Type: csipbv1.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
							},
						},
					},
					{
						Type: &csipbv1.NodeServiceCapability_Rpc{
							Rpc: &csipbv1.NodeServiceCapability_RPC{
								Type: csipbv1.NodeServiceCapability_RPC_EXPAND_VOLUME,
							},
						},
					},
				},
			},
			ExpectedResponse: &NodeCapabilitySet{
				HasStageUnstageVolume: true,
				HasExpandVolume:       true,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			_, _, nc, client := newTestClient(t)
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
	ci.Parallel(t)

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
			_, cc, _, client := newTestClient(t)
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
	ci.Parallel(t)

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
			_, cc, _, client := newTestClient(t)
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
	ci.Parallel(t)

	cases := []struct {
		Name        string
		AccessType  VolumeAccessType
		AccessMode  VolumeAccessMode
		ResponseErr error
		Response    *csipbv1.ValidateVolumeCapabilitiesResponse
		ExpectedErr error
	}{
		{
			Name:        "handles underlying grpc errors",
			AccessType:  VolumeAccessTypeMount,
			AccessMode:  VolumeAccessModeMultiNodeMultiWriter,
			ResponseErr: status.Errorf(codes.Internal, "some grpc error"),
			ExpectedErr: fmt.Errorf("controller plugin returned an internal error, check the plugin allocation logs for more information: rpc error: code = Internal desc = some grpc error"),
		},
		{
			Name:        "handles success empty capabilities",
			AccessType:  VolumeAccessTypeMount,
			AccessMode:  VolumeAccessModeMultiNodeMultiWriter,
			Response:    &csipbv1.ValidateVolumeCapabilitiesResponse{},
			ResponseErr: nil,
			ExpectedErr: nil,
		},
		{
			Name:       "handles success exact match MountVolume",
			AccessType: VolumeAccessTypeMount,
			AccessMode: VolumeAccessModeMultiNodeMultiWriter,
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
			Name:       "handles success exact match BlockVolume",
			AccessType: VolumeAccessTypeBlock,
			AccessMode: VolumeAccessModeMultiNodeMultiWriter,
			Response: &csipbv1.ValidateVolumeCapabilitiesResponse{
				Confirmed: &csipbv1.ValidateVolumeCapabilitiesResponse_Confirmed{
					VolumeCapabilities: []*csipbv1.VolumeCapability{
						{
							AccessType: &csipbv1.VolumeCapability_Block{
								Block: &csipbv1.VolumeCapability_BlockVolume{},
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
			Name:       "handles failure AccessMode mismatch",
			AccessMode: VolumeAccessModeMultiNodeMultiWriter,
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
			ExpectedErr: fmt.Errorf("volume capability validation failed: 1 error occurred:\n\t* requested access mode MULTI_NODE_MULTI_WRITER, got SINGLE_NODE_WRITER\n\n"),
		},

		{
			Name:       "handles failure MountFlags mismatch",
			AccessType: VolumeAccessTypeMount,
			AccessMode: VolumeAccessModeMultiNodeMultiWriter,
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

		{
			Name:       "handles failure MountFlags with Block",
			AccessType: VolumeAccessTypeBlock,
			AccessMode: VolumeAccessModeMultiNodeMultiWriter,
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
			ExpectedErr: fmt.Errorf("volume capability validation failed: 1 error occurred:\n\t* 'file-system' access type was not requested but was validated by the controller\n\n"),
		},

		{
			Name:       "handles success incomplete no AccessType",
			AccessType: VolumeAccessTypeMount,
			AccessMode: VolumeAccessModeMultiNodeMultiWriter,
			Response: &csipbv1.ValidateVolumeCapabilitiesResponse{
				Confirmed: &csipbv1.ValidateVolumeCapabilitiesResponse_Confirmed{
					VolumeCapabilities: []*csipbv1.VolumeCapability{
						{
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
			Name:       "handles success incomplete no AccessMode",
			AccessType: VolumeAccessTypeBlock,
			AccessMode: VolumeAccessModeMultiNodeMultiWriter,
			Response: &csipbv1.ValidateVolumeCapabilitiesResponse{
				Confirmed: &csipbv1.ValidateVolumeCapabilitiesResponse_Confirmed{
					VolumeCapabilities: []*csipbv1.VolumeCapability{
						{
							AccessType: &csipbv1.VolumeCapability_Block{
								Block: &csipbv1.VolumeCapability_BlockVolume{},
							},
						},
					},
				},
			},
			ResponseErr: nil,
			ExpectedErr: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			_, cc, _, client := newTestClient(t)
			defer client.Close()

			requestedCaps := []*VolumeCapability{{
				AccessType: tc.AccessType,
				AccessMode: tc.AccessMode,
				MountVolume: &structs.CSIMountOptions{ // should be ignored
					FSType:     "ext4",
					MountFlags: []string{"noatime", "errors=remount-ro"},
				},
			}}
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

func TestClient_RPC_ControllerCreateVolume(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		Name          string
		CapacityRange *CapacityRange
		ContentSource *VolumeContentSource
		ResponseErr   error
		Response      *csipbv1.CreateVolumeResponse
		ExpectedErr   error
	}{
		{
			Name:        "handles underlying grpc errors",
			ResponseErr: status.Errorf(codes.Internal, "some grpc error"),
			ExpectedErr: fmt.Errorf("controller plugin returned an internal error, check the plugin allocation logs for more information: rpc error: code = Internal desc = some grpc error"),
		},

		{
			Name: "handles error invalid capacity range",
			CapacityRange: &CapacityRange{
				RequiredBytes: 1000,
				LimitBytes:    500,
			},
			ExpectedErr: errors.New("LimitBytes cannot be less than RequiredBytes"),
		},

		{
			Name: "handles error invalid content source",
			ContentSource: &VolumeContentSource{
				SnapshotID: "snap-12345",
				CloneID:    "vol-12345",
			},
			ExpectedErr: errors.New(
				"one of SnapshotID or CloneID must be set if ContentSource is set"),
		},

		{
			Name:     "handles success missing source and range",
			Response: &csipbv1.CreateVolumeResponse{},
		},

		{
			Name: "handles success with capacity range, source, and topology",
			CapacityRange: &CapacityRange{
				RequiredBytes: 500,
				LimitBytes:    1000,
			},
			ContentSource: &VolumeContentSource{
				SnapshotID: "snap-12345",
			},
			Response: &csipbv1.CreateVolumeResponse{
				Volume: &csipbv1.Volume{
					CapacityBytes: 1000,
					ContentSource: &csipbv1.VolumeContentSource{
						Type: &csipbv1.VolumeContentSource_Snapshot{
							Snapshot: &csipbv1.VolumeContentSource_SnapshotSource{
								SnapshotId: "snap-12345",
							},
						},
					},
					AccessibleTopology: []*csipbv1.Topology{
						{Segments: map[string]string{"rack": "R1"}},
					},
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			_, cc, _, client := newTestClient(t)
			defer client.Close()

			req := &ControllerCreateVolumeRequest{
				Name:          "vol-123456",
				CapacityRange: tc.CapacityRange,
				VolumeCapabilities: []*VolumeCapability{
					{
						AccessType: VolumeAccessTypeMount,
						AccessMode: VolumeAccessModeMultiNodeMultiWriter,
					},
				},
				Parameters:    map[string]string{},
				Secrets:       structs.CSISecrets{},
				ContentSource: tc.ContentSource,
				AccessibilityRequirements: &TopologyRequirement{
					Requisite: []*Topology{
						{
							Segments: map[string]string{"rack": "R1"},
						},
						{
							Segments: map[string]string{"rack": "R2"},
						},
					},
				},
			}

			cc.NextCreateVolumeResponse = tc.Response
			cc.NextErr = tc.ResponseErr

			resp, err := client.ControllerCreateVolume(context.TODO(), req)
			if tc.ExpectedErr != nil {
				require.EqualError(t, err, tc.ExpectedErr.Error())
				return
			}
			require.NoError(t, err, tc.Name)
			if tc.Response == nil {
				require.Nil(t, resp)
				return
			}
			if tc.CapacityRange != nil {
				require.Greater(t, resp.Volume.CapacityBytes, int64(0))
			}
			if tc.ContentSource != nil {
				require.Equal(t, tc.ContentSource.CloneID, resp.Volume.ContentSource.CloneID)
				require.Equal(t, tc.ContentSource.SnapshotID, resp.Volume.ContentSource.SnapshotID)
			}
			if tc.Response != nil && tc.Response.Volume != nil {
				require.Len(t, resp.Volume.AccessibleTopology, 1)
				require.Equal(t,
					req.AccessibilityRequirements.Requisite[0].Segments,
					resp.Volume.AccessibleTopology[0].Segments,
				)
			}

		})
	}
}

func TestClient_RPC_ControllerDeleteVolume(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		Name        string
		Request     *ControllerDeleteVolumeRequest
		ResponseErr error
		ExpectedErr error
	}{
		{
			Name:        "handles underlying grpc errors",
			Request:     &ControllerDeleteVolumeRequest{ExternalVolumeID: "vol-12345"},
			ResponseErr: status.Errorf(codes.Internal, "some grpc error"),
			ExpectedErr: fmt.Errorf("controller plugin returned an internal error, check the plugin allocation logs for more information: rpc error: code = Internal desc = some grpc error"),
		},

		{
			Name:        "handles error missing volume ID",
			Request:     &ControllerDeleteVolumeRequest{},
			ExpectedErr: errors.New("missing ExternalVolumeID"),
		},

		{
			Name:    "handles success",
			Request: &ControllerDeleteVolumeRequest{ExternalVolumeID: "vol-12345"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			_, cc, _, client := newTestClient(t)
			defer client.Close()

			cc.NextErr = tc.ResponseErr
			err := client.ControllerDeleteVolume(context.TODO(), tc.Request)
			if tc.ExpectedErr != nil {
				require.EqualError(t, err, tc.ExpectedErr.Error())
				return
			}
			require.NoError(t, err, tc.Name)
		})
	}
}

func TestClient_RPC_ControllerListVolume(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		Name        string
		Request     *ControllerListVolumesRequest
		ResponseErr error
		ExpectedErr error
	}{
		{
			Name:        "handles underlying grpc errors",
			Request:     &ControllerListVolumesRequest{},
			ResponseErr: status.Errorf(codes.Internal, "some grpc error"),
			ExpectedErr: fmt.Errorf("controller plugin returned an internal error, check the plugin allocation logs for more information: rpc error: code = Internal desc = some grpc error"),
		},

		{
			Name:        "handles error invalid max entries",
			Request:     &ControllerListVolumesRequest{MaxEntries: -1},
			ExpectedErr: errors.New("MaxEntries cannot be negative"),
		},

		{
			Name:    "handles success",
			Request: &ControllerListVolumesRequest{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			_, cc, _, client := newTestClient(t)
			defer client.Close()

			cc.NextErr = tc.ResponseErr
			if tc.ResponseErr != nil {
				// note: there's nothing interesting to assert here other than
				// that we don't throw a NPE during transformation from
				// protobuf to our struct
				cc.NextListVolumesResponse = &csipbv1.ListVolumesResponse{
					Entries: []*csipbv1.ListVolumesResponse_Entry{
						{
							Volume: &csipbv1.Volume{
								CapacityBytes: 1000000,
								VolumeId:      "vol-0",
								VolumeContext: map[string]string{"foo": "bar"},

								ContentSource: &csipbv1.VolumeContentSource{},
								AccessibleTopology: []*csipbv1.Topology{
									{
										Segments: map[string]string{"rack": "A"},
									},
								},
							},
						},

						{
							Volume: &csipbv1.Volume{
								VolumeId: "vol-1",
								AccessibleTopology: []*csipbv1.Topology{
									{
										Segments: map[string]string{"rack": "A"},
									},
								},
							},
						},

						{
							Volume: &csipbv1.Volume{
								VolumeId: "vol-3",
								ContentSource: &csipbv1.VolumeContentSource{
									Type: &csipbv1.VolumeContentSource_Snapshot{
										Snapshot: &csipbv1.VolumeContentSource_SnapshotSource{
											SnapshotId: "snap-12345",
										},
									},
								},
							},
						},
					},
					NextToken: "abcdef",
				}
			}

			resp, err := client.ControllerListVolumes(context.TODO(), tc.Request)
			if tc.ExpectedErr != nil {
				require.EqualError(t, err, tc.ExpectedErr.Error())
				return
			}
			require.NoError(t, err, tc.Name)
			require.NotNil(t, resp)

		})
	}
}

func TestClient_RPC_ControllerCreateSnapshot(t *testing.T) {
	ci.Parallel(t)

	now := time.Now()

	cases := []struct {
		Name        string
		Request     *ControllerCreateSnapshotRequest
		Response    *csipbv1.CreateSnapshotResponse
		ResponseErr error
		ExpectedErr error
	}{
		{
			Name: "handles underlying grpc errors",
			Request: &ControllerCreateSnapshotRequest{
				VolumeID: "vol-12345",
				Name:     "snap-12345",
			},
			ResponseErr: status.Errorf(codes.Internal, "some grpc error"),
			ExpectedErr: fmt.Errorf("controller plugin returned an internal error, check the plugin allocation logs for more information: rpc error: code = Internal desc = some grpc error"),
		},

		{
			Name:        "handles error missing volume ID",
			Request:     &ControllerCreateSnapshotRequest{},
			ExpectedErr: errors.New("missing VolumeID"),
		},

		{
			Name: "handles success",
			Request: &ControllerCreateSnapshotRequest{
				VolumeID: "vol-12345",
				Name:     "snap-12345",
			},
			Response: &csipbv1.CreateSnapshotResponse{
				Snapshot: &csipbv1.Snapshot{
					SizeBytes:      100000,
					SnapshotId:     "snap-12345",
					SourceVolumeId: "vol-12345",
					CreationTime:   timestamppb.New(now),
					ReadyToUse:     true,
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			_, cc, _, client := newTestClient(t)
			defer client.Close()

			cc.NextErr = tc.ResponseErr
			cc.NextCreateSnapshotResponse = tc.Response
			// note: there's nothing interesting to assert about the response
			// here other than that we don't throw a NPE during transformation
			// from protobuf to our struct
			resp, err := client.ControllerCreateSnapshot(context.TODO(), tc.Request)
			if tc.ExpectedErr != nil {
				require.EqualError(t, err, tc.ExpectedErr.Error())
			} else {
				require.NoError(t, err, tc.Name)
				require.NotZero(t, resp.Snapshot.CreateTime)
				require.Equal(t, now.Second(), time.Unix(resp.Snapshot.CreateTime, 0).Second())
			}
		})
	}
}

func TestClient_RPC_ControllerDeleteSnapshot(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		Name        string
		Request     *ControllerDeleteSnapshotRequest
		ResponseErr error
		ExpectedErr error
	}{
		{
			Name:        "handles underlying grpc errors",
			Request:     &ControllerDeleteSnapshotRequest{SnapshotID: "vol-12345"},
			ResponseErr: status.Errorf(codes.Internal, "some grpc error"),
			ExpectedErr: fmt.Errorf("controller plugin returned an internal error, check the plugin allocation logs for more information: rpc error: code = Internal desc = some grpc error"),
		},

		{
			Name:        "handles error missing volume ID",
			Request:     &ControllerDeleteSnapshotRequest{},
			ExpectedErr: errors.New("missing SnapshotID"),
		},

		{
			Name:    "handles success",
			Request: &ControllerDeleteSnapshotRequest{SnapshotID: "vol-12345"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			_, cc, _, client := newTestClient(t)
			defer client.Close()

			cc.NextErr = tc.ResponseErr
			err := client.ControllerDeleteSnapshot(context.TODO(), tc.Request)
			if tc.ExpectedErr != nil {
				require.EqualError(t, err, tc.ExpectedErr.Error())
				return
			}
			require.NoError(t, err, tc.Name)
		})
	}
}

func TestClient_RPC_ControllerListSnapshots(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		Name        string
		Request     *ControllerListSnapshotsRequest
		ResponseErr error
		ExpectedErr error
	}{
		{
			Name:        "handles underlying grpc errors",
			Request:     &ControllerListSnapshotsRequest{},
			ResponseErr: status.Errorf(codes.Internal, "some grpc error"),
			ExpectedErr: fmt.Errorf("controller plugin returned an internal error, check the plugin allocation logs for more information: rpc error: code = Internal desc = some grpc error"),
		},

		{
			Name:        "handles error invalid max entries",
			Request:     &ControllerListSnapshotsRequest{MaxEntries: -1},
			ExpectedErr: errors.New("MaxEntries cannot be negative"),
		},

		{
			Name:    "handles success",
			Request: &ControllerListSnapshotsRequest{},
		},
	}

	now := time.Now()

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			_, cc, _, client := newTestClient(t)
			defer client.Close()

			cc.NextErr = tc.ResponseErr
			if tc.ResponseErr == nil {
				cc.NextListSnapshotsResponse = &csipbv1.ListSnapshotsResponse{
					Entries: []*csipbv1.ListSnapshotsResponse_Entry{
						{
							Snapshot: &csipbv1.Snapshot{
								SizeBytes:      1000000,
								SnapshotId:     "snap-12345",
								SourceVolumeId: "vol-12345",
								ReadyToUse:     true,
								CreationTime:   timestamppb.New(now),
							},
						},
					},
					NextToken: "abcdef",
				}
			}

			resp, err := client.ControllerListSnapshots(context.TODO(), tc.Request)
			if tc.ExpectedErr != nil {
				require.EqualError(t, err, tc.ExpectedErr.Error())
				return
			}
			require.NoError(t, err, tc.Name)
			require.NotNil(t, resp)
			require.Len(t, resp.Entries, 1)
			require.NotZero(t, resp.Entries[0].Snapshot.CreateTime)
			require.Equal(t, now.Second(),
				time.Unix(resp.Entries[0].Snapshot.CreateTime, 0).Second())
		})
	}
}

func TestClient_RPC_ControllerExpandVolume(t *testing.T) {

	cases := []struct {
		Name        string
		Request     *ControllerExpandVolumeRequest
		ExpectCall  *csipbv1.ControllerExpandVolumeRequest
		ResponseErr error
		ExpectedErr error
	}{
		{
			Name: "success",
			Request: &ControllerExpandVolumeRequest{
				ExternalVolumeID: "vol-1",
				RequiredBytes:    1,
				LimitBytes:       2,
				Capability: &VolumeCapability{
					AccessMode: VolumeAccessModeMultiNodeSingleWriter,
				},
				Secrets: map[string]string{"super": "secret"},
			},
			ExpectCall: &csipbv1.ControllerExpandVolumeRequest{
				VolumeId: "vol-1",
				CapacityRange: &csipbv1.CapacityRange{
					RequiredBytes: 1,
					LimitBytes:    2,
				},
				VolumeCapability: &csipbv1.VolumeCapability{
					AccessMode: &csipbv1.VolumeCapability_AccessMode{
						Mode: csipbv1.VolumeCapability_AccessMode_Mode(csipbv1.VolumeCapability_AccessMode_MULTI_NODE_SINGLE_WRITER),
					},
					AccessType: &csipbv1.VolumeCapability_Block{Block: &csipbv1.VolumeCapability_BlockVolume{}},
				},
				Secrets: map[string]string{"super": "secret"},
			},
		},

		{
			Name: "validate only min set",
			Request: &ControllerExpandVolumeRequest{
				ExternalVolumeID: "vol-1",
				RequiredBytes:    4,
			},
			ExpectCall: &csipbv1.ControllerExpandVolumeRequest{
				VolumeId: "vol-1",
				CapacityRange: &csipbv1.CapacityRange{
					RequiredBytes: 4,
				},
			},
		},
		{
			Name:        "validate missing volume ID",
			Request:     &ControllerExpandVolumeRequest{},
			ExpectedErr: errors.New("missing ExternalVolumeID"),
		},
		{
			Name: "validate missing max/min size",
			Request: &ControllerExpandVolumeRequest{
				ExternalVolumeID: "vol-1",
			},
			ExpectedErr: errors.New("one of LimitBytes or RequiredBytes must be set"),
		},
		{
			Name: "validate min greater than max",
			Request: &ControllerExpandVolumeRequest{
				ExternalVolumeID: "vol-1",
				RequiredBytes:    4,
				LimitBytes:       2,
			},
			ExpectedErr: errors.New("LimitBytes cannot be less than RequiredBytes"),
		},

		{
			Name: "grpc error InvalidArgument",
			Request: &ControllerExpandVolumeRequest{
				ExternalVolumeID: "vol-1", LimitBytes: 1000},
			ResponseErr: status.Errorf(codes.InvalidArgument, "sad args"),
			ExpectedErr: errors.New("requested capabilities not compatible with volume \"vol-1\": rpc error: code = InvalidArgument desc = sad args"),
		},

		{
			Name: "grpc error NotFound",
			Request: &ControllerExpandVolumeRequest{
				ExternalVolumeID: "vol-1", LimitBytes: 1000},
			ResponseErr: status.Errorf(codes.NotFound, "does not exist"),
			ExpectedErr: errors.New("volume \"vol-1\" could not be found: rpc error: code = NotFound desc = does not exist"),
		},
		{
			Name: "grpc error FailedPrecondition",
			Request: &ControllerExpandVolumeRequest{
				ExternalVolumeID: "vol-1", LimitBytes: 1000},
			ResponseErr: status.Errorf(codes.FailedPrecondition, "unsupported"),
			ExpectedErr: errors.New("volume \"vol-1\" cannot be expanded online: rpc error: code = FailedPrecondition desc = unsupported"),
		},
		{
			Name: "grpc error OutOfRange",
			Request: &ControllerExpandVolumeRequest{
				ExternalVolumeID: "vol-1", LimitBytes: 1000},
			ResponseErr: status.Errorf(codes.OutOfRange, "too small"),
			ExpectedErr: errors.New("unsupported capacity_range for volume \"vol-1\": rpc error: code = OutOfRange desc = too small"),
		},
		{
			Name: "grpc error Internal",
			Request: &ControllerExpandVolumeRequest{
				ExternalVolumeID: "vol-1", LimitBytes: 1000},
			ResponseErr: status.Errorf(codes.Internal, "some grpc error"),
			ExpectedErr: errors.New("controller plugin returned an internal error, check the plugin allocation logs for more information: rpc error: code = Internal desc = some grpc error"),
		},
		{
			Name: "grpc error default case",
			Request: &ControllerExpandVolumeRequest{
				ExternalVolumeID: "vol-1", LimitBytes: 1000},
			ResponseErr: status.Errorf(codes.DataLoss, "misc unspecified error"),
			ExpectedErr: errors.New("controller plugin returned an error: rpc error: code = DataLoss desc = misc unspecified error"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			_, cc, _, client := newTestClient(t)

			cc.NextErr = tc.ResponseErr
			// the fake client should take ~no time, but set a timeout just in case
			ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*50)
			defer cancel()
			resp, err := client.ControllerExpandVolume(ctx, tc.Request)
			if tc.ExpectedErr != nil {
				must.EqError(t, err, tc.ExpectedErr.Error())
				return
			}
			must.NoError(t, err)
			must.NotNil(t, resp)
			must.Eq(t, tc.ExpectCall, cc.LastExpandVolumeRequest)

		})
	}

	t.Run("connection error", func(t *testing.T) {
		c := &client{} // induce c.ensureConnected() error
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*50)
		defer cancel()
		resp, err := c.ControllerExpandVolume(ctx, &ControllerExpandVolumeRequest{
			ExternalVolumeID: "valid-id",
			RequiredBytes:    1,
		})
		must.Nil(t, resp)
		must.EqError(t, err, "address is empty")
	})
}

func TestClient_RPC_NodeStageVolume(t *testing.T) {
	ci.Parallel(t)

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
			_, _, nc, client := newTestClient(t)
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
	ci.Parallel(t)

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
			_, _, nc, client := newTestClient(t)
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
	ci.Parallel(t)

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
			_, _, nc, client := newTestClient(t)
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
	ci.Parallel(t)

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
			_, _, nc, client := newTestClient(t)
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

func TestClient_RPC_NodeExpandVolume(t *testing.T) {
	// minimum valid request
	minRequest := &NodeExpandVolumeRequest{
		ExternalVolumeID: "test-vol",
		TargetPath:       "/test-path",
	}

	cases := []struct {
		Name        string
		Request     *NodeExpandVolumeRequest
		ExpectCall  *csipbv1.NodeExpandVolumeRequest
		ResponseErr error
		ExpectedErr error
	}{
		{
			Name:    "success min",
			Request: minRequest,
			ExpectCall: &csipbv1.NodeExpandVolumeRequest{
				VolumeId:   "test-vol",
				VolumePath: "/test-path",
			},
		},
		{
			Name: "success full",
			Request: &NodeExpandVolumeRequest{
				ExternalVolumeID: "test-vol",
				TargetPath:       "/test-path",
				StagingPath:      "/test-staging-path",
				CapacityRange: &CapacityRange{
					RequiredBytes: 5,
					LimitBytes:    10,
				},
				Capability: &VolumeCapability{
					AccessType: VolumeAccessTypeMount,
					AccessMode: VolumeAccessModeMultiNodeSingleWriter,
					MountVolume: &structs.CSIMountOptions{
						FSType:     "test-fstype",
						MountFlags: []string{"test-flags"},
					},
				},
			},
			ExpectCall: &csipbv1.NodeExpandVolumeRequest{
				VolumeId:          "test-vol",
				VolumePath:        "/test-path",
				StagingTargetPath: "/test-staging-path",
				CapacityRange: &csipbv1.CapacityRange{
					RequiredBytes: 5,
					LimitBytes:    10,
				},
				VolumeCapability: &csipbv1.VolumeCapability{
					AccessType: &csipbv1.VolumeCapability_Mount{
						Mount: &csipbv1.VolumeCapability_MountVolume{
							FsType:           "test-fstype",
							MountFlags:       []string{"test-flags"},
							VolumeMountGroup: "",
						}},
					AccessMode: &csipbv1.VolumeCapability_AccessMode{
						Mode: csipbv1.VolumeCapability_AccessMode_MULTI_NODE_SINGLE_WRITER},
				},
			},
		},

		{
			Name: "validate missing volume id",
			Request: &NodeExpandVolumeRequest{
				TargetPath: "/test-path",
			},
			ExpectedErr: errors.New("ExternalVolumeID is required"),
		},
		{
			Name: "validate missing target path",
			Request: &NodeExpandVolumeRequest{
				ExternalVolumeID: "test-volume",
			},
			ExpectedErr: errors.New("TargetPath is required"),
		},
		{
			Name: "validate min greater than max",
			Request: &NodeExpandVolumeRequest{
				ExternalVolumeID: "test-vol",
				TargetPath:       "/test-path",
				CapacityRange: &CapacityRange{
					RequiredBytes: 4,
					LimitBytes:    2,
				},
			},
			ExpectedErr: errors.New("LimitBytes cannot be less than RequiredBytes"),
		},

		{
			Name:        "grpc error default case",
			Request:     minRequest,
			ResponseErr: status.Errorf(codes.DataLoss, "misc unspecified error"),
			ExpectedErr: errors.New("node plugin returned an error: rpc error: code = DataLoss desc = misc unspecified error"),
		},
		{
			Name:        "grpc error invalid argument",
			Request:     minRequest,
			ResponseErr: status.Errorf(codes.InvalidArgument, "sad args"),
			ExpectedErr: errors.New("requested capabilities not compatible with volume \"test-vol\": rpc error: code = InvalidArgument desc = sad args"),
		},
		{
			Name:        "grpc error NotFound",
			Request:     minRequest,
			ResponseErr: status.Errorf(codes.NotFound, "does not exist"),
			ExpectedErr: errors.New("CSI client error (ignorable): volume \"test-vol\" could not be found: rpc error: code = NotFound desc = does not exist"),
		},
		{
			Name:        "grpc error FailedPrecondition",
			Request:     minRequest,
			ResponseErr: status.Errorf(codes.FailedPrecondition, "unsupported"),
			ExpectedErr: errors.New("volume \"test-vol\" cannot be expanded while in use: rpc error: code = FailedPrecondition desc = unsupported"),
		},
		{
			Name:        "grpc error OutOfRange",
			Request:     minRequest,
			ResponseErr: status.Errorf(codes.OutOfRange, "too small"),
			ExpectedErr: errors.New("unsupported capacity_range for volume \"test-vol\": rpc error: code = OutOfRange desc = too small"),
		},
		{
			Name:        "grpc error Internal",
			Request:     minRequest,
			ResponseErr: status.Errorf(codes.Internal, "some grpc error"),
			ExpectedErr: errors.New("node plugin returned an internal error, check the plugin allocation logs for more information: rpc error: code = Internal desc = some grpc error"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			_, _, nc, client := newTestClient(t)

			nc.NextErr = tc.ResponseErr
			// the fake client should take ~no time, but set a timeout just in case
			ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*50)
			defer cancel()
			resp, err := client.NodeExpandVolume(ctx, tc.Request)
			if tc.ExpectedErr != nil {
				must.EqError(t, err, tc.ExpectedErr.Error())
				return
			}
			must.NoError(t, err)
			must.NotNil(t, resp)
			must.Eq(t, tc.ExpectCall, nc.LastExpandVolumeRequest)

		})
	}

	t.Run("connection error", func(t *testing.T) {
		c := &client{} // induce c.ensureConnected() error
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*50)
		defer cancel()
		resp, err := c.NodeExpandVolume(ctx, &NodeExpandVolumeRequest{
			ExternalVolumeID: "valid-id",
			TargetPath:       "/some-path",
		})
		must.Nil(t, resp)
		must.EqError(t, err, "address is empty")
	})
}
