// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/go-memdb"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client"
	cconfig "github.com/hashicorp/nomad/client/config"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/lib/lang"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/csi"
	"github.com/hashicorp/nomad/testutil"
)

func TestCSIVolumeEndpoint_Get(t *testing.T) {
	ci.Parallel(t)
	srv, shutdown := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)

	ns := structs.DefaultNamespace

	state := srv.fsm.State()

	codec := rpcClient(t, srv)

	id0 := uuid.Generate()

	// Create the volume
	vols := []*structs.CSIVolume{{
		ID:        id0,
		Namespace: ns,
		PluginID:  "minnie",
		Secrets:   structs.CSISecrets{"mysecret": "secretvalue"},
		RequestedCapabilities: []*structs.CSIVolumeCapability{{
			AccessMode:     structs.CSIVolumeAccessModeMultiNodeSingleWriter,
			AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
		}},
	}}
	err := state.UpsertCSIVolume(999, vols)
	require.NoError(t, err)

	// Create the register request
	req := &structs.CSIVolumeGetRequest{
		ID: id0,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: ns,
		},
	}

	var resp structs.CSIVolumeGetResponse
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Get", req, &resp)
	require.NoError(t, err)
	require.Equal(t, uint64(999), resp.Index)
	require.Equal(t, vols[0].ID, resp.Volume.ID)
}

func TestCSIVolumeEndpoint_Get_ACL(t *testing.T) {
	ci.Parallel(t)
	srv, _, shutdown := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)

	ns := structs.DefaultNamespace

	state := srv.fsm.State()
	policy := mock.NamespacePolicy(ns, "", []string{acl.NamespaceCapabilityCSIReadVolume})
	validToken := mock.CreatePolicyAndToken(t, state, 1001, "csi-access", policy)

	codec := rpcClient(t, srv)

	id0 := uuid.Generate()

	// Create the volume
	vols := []*structs.CSIVolume{{
		ID:        id0,
		Namespace: ns,
		PluginID:  "minnie",
		Secrets:   structs.CSISecrets{"mysecret": "secretvalue"},
		RequestedCapabilities: []*structs.CSIVolumeCapability{{
			AccessMode:     structs.CSIVolumeAccessModeMultiNodeSingleWriter,
			AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
		}},
	}}
	err := state.UpsertCSIVolume(999, vols)
	require.NoError(t, err)

	// Create the register request
	req := &structs.CSIVolumeGetRequest{
		ID: id0,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: ns,
			AuthToken: validToken.SecretID,
		},
	}

	var resp structs.CSIVolumeGetResponse
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Get", req, &resp)
	require.NoError(t, err)
	require.Equal(t, uint64(999), resp.Index)
	require.Equal(t, vols[0].ID, resp.Volume.ID)
}

func TestCSIVolume_pluginValidateVolume(t *testing.T) {
	// bare minimum server for this method
	store := state.TestStateStore(t)
	srv := &Server{
		fsm: &nomadFSM{state: store},
	}
	// has our method under test
	csiVolume := &CSIVolume{srv: srv}
	// volume for which we will request a valid plugin
	vol := &structs.CSIVolume{PluginID: "neat-plugin"}

	// plugin not found
	got, err := csiVolume.pluginValidateVolume(vol)
	must.Nil(t, got, must.Sprint("nonexistent plugin should be nil"))
	must.ErrorContains(t, err, "no CSI plugin named")

	// we'll upsert this plugin after optionally modifying it
	basePlug := &structs.CSIPlugin{
		ID: vol.PluginID,
		// these should be set on the volume after success
		Provider: "neat-provider",
		Version:  "v0",
		// explicit zero values, because these modify behavior we care about
		ControllerRequired: false,
		ControllersHealthy: 0,
	}

	cases := []struct {
		name         string
		updatePlugin func(*structs.CSIPlugin)
		expectErr    string
	}{
		{
			name: "controller not required",
		},
		{
			name: "controller unhealthy",
			updatePlugin: func(p *structs.CSIPlugin) {
				p.ControllerRequired = true
			},
			expectErr: "no healthy controllers",
		},
		{
			name: "controller healthy",
			updatePlugin: func(p *structs.CSIPlugin) {
				p.ControllerRequired = true
				p.ControllersHealthy = 1
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vol := vol.Copy()
			plug := basePlug.Copy()

			if tc.updatePlugin != nil {
				tc.updatePlugin(plug)
			}
			must.NoError(t, store.UpsertCSIPlugin(1000, plug))

			got, err := csiVolume.pluginValidateVolume(vol)

			if tc.expectErr == "" {
				must.NoError(t, err)
				must.NotNil(t, got, must.Sprint("plugin should not be nil"))
				must.Eq(t, vol.Provider, plug.Provider)
				must.Eq(t, vol.ProviderVersion, plug.Version)
			} else {
				must.Error(t, err, must.Sprint("expect error:", tc.expectErr))
				must.ErrorContains(t, err, tc.expectErr)
				must.Nil(t, got, must.Sprint("plugin should be nil"))
			}
		})
	}
}

func TestCSIVolumeEndpoint_Register(t *testing.T) {
	ci.Parallel(t)
	srv, shutdown := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)

	store := srv.fsm.State()
	codec := rpcClient(t, srv)

	id0 := uuid.Generate()

	// Create the register request
	ns := mock.Namespace()
	store.UpsertNamespaces(900, []*structs.Namespace{ns})

	// Create the node and plugin
	node := mock.Node()
	node.CSINodePlugins = map[string]*structs.CSIInfo{
		"minnie": {PluginID: "minnie",
			Healthy: true,
			// Registers as node plugin that does not require a controller to skip
			// the client RPC during registration.
			NodeInfo: &structs.CSINodeInfo{},
		},
	}
	require.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, 1000, node))

	// Create the volume
	vols := []*structs.CSIVolume{{
		ID:             id0,
		Namespace:      ns.Name,
		PluginID:       "minnie",
		AccessMode:     structs.CSIVolumeAccessModeSingleNodeReader, // legacy field ignored
		AttachmentMode: structs.CSIVolumeAttachmentModeBlockDevice,  // legacy field ignored
		MountOptions: &structs.CSIMountOptions{
			FSType: "ext4", MountFlags: []string{"sensitive"}},
		Secrets:    structs.CSISecrets{"mysecret": "secretvalue"},
		Parameters: map[string]string{"myparam": "paramvalue"},
		Context:    map[string]string{"mycontext": "contextvalue"},
		RequestedCapabilities: []*structs.CSIVolumeCapability{{
			AccessMode:     structs.CSIVolumeAccessModeMultiNodeReader,
			AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
		}},
	}}

	// Create the register request
	req1 := &structs.CSIVolumeRegisterRequest{
		Volumes: vols,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: "",
		},
	}
	resp1 := &structs.CSIVolumeRegisterResponse{}
	err := msgpackrpc.CallWithCodec(codec, "CSIVolume.Register", req1, resp1)
	require.NoError(t, err)
	require.NotEqual(t, uint64(0), resp1.Index)

	// Get the volume back out
	req2 := &structs.CSIVolumeGetRequest{
		ID: id0,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: ns.Name,
		},
	}
	resp2 := &structs.CSIVolumeGetResponse{}
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Get", req2, resp2)
	require.NoError(t, err)
	require.Equal(t, resp1.Index, resp2.Index)
	require.Equal(t, vols[0].ID, resp2.Volume.ID)
	require.Equal(t, "csi.CSISecrets(map[mysecret:[REDACTED]])",
		resp2.Volume.Secrets.String())
	require.Equal(t, "csi.CSIOptions(FSType: ext4, MountFlags: [REDACTED])",
		resp2.Volume.MountOptions.String())

	// Registration does not update
	req1.Volumes[0].PluginID = "adam"
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Register", req1, resp1)
	require.Error(t, err, "exists")

	// Deregistration works
	req3 := &structs.CSIVolumeDeregisterRequest{
		VolumeIDs: []string{id0},
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: ns.Name,
		},
	}
	resp3 := &structs.CSIVolumeDeregisterResponse{}
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Deregister", req3, resp3)
	require.NoError(t, err)

	// Volume is missing
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Get", req2, resp2)
	require.NoError(t, err)
	require.Nil(t, resp2.Volume)
}

// TestCSIVolumeEndpoint_Claim exercises the VolumeClaim RPC, verifying that claims
// are honored only if the volume exists, the mode is permitted, and the volume
// is schedulable according to its count of claims.
func TestCSIVolumeEndpoint_Claim(t *testing.T) {
	ci.Parallel(t)
	srv, shutdown := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)

	index := uint64(1000)

	state := srv.fsm.State()
	codec := rpcClient(t, srv)
	id0 := uuid.Generate()
	alloc := mock.BatchAlloc()

	// Create a client node and alloc
	node := mock.Node()
	alloc.NodeID = node.ID
	summary := mock.JobSummary(alloc.JobID)
	index++
	require.NoError(t, state.UpsertJobSummary(index, summary))
	index++
	require.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, index, []*structs.Allocation{alloc}))

	// Create an initial volume claim request; we expect it to fail
	// because there's no such volume yet.
	claimReq := &structs.CSIVolumeClaimRequest{
		VolumeID:       id0,
		AllocationID:   alloc.ID,
		Claim:          structs.CSIVolumeClaimWrite,
		AccessMode:     structs.CSIVolumeAccessModeMultiNodeSingleWriter,
		AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: structs.DefaultNamespace,
		},
	}
	claimResp := &structs.CSIVolumeClaimResponse{}
	err := msgpackrpc.CallWithCodec(codec, "CSIVolume.Claim", claimReq, claimResp)
	require.EqualError(t, err, fmt.Sprintf("volume not found: %s", id0),
		"expected 'volume not found' error because volume hasn't yet been created")

	// Create a plugin and volume

	node.CSINodePlugins = map[string]*structs.CSIInfo{
		"minnie": {
			PluginID: "minnie",
			Healthy:  true,
			NodeInfo: &structs.CSINodeInfo{},
		},
	}
	index++
	err = state.UpsertNode(structs.MsgTypeTestSetup, index, node)
	require.NoError(t, err)

	vols := []*structs.CSIVolume{{
		ID:        id0,
		Namespace: structs.DefaultNamespace,
		PluginID:  "minnie",
		RequestedTopologies: &structs.CSITopologyRequest{
			Required: []*structs.CSITopology{
				{Segments: map[string]string{"foo": "bar"}}},
		},
		Secrets: structs.CSISecrets{"mysecret": "secretvalue"},
		RequestedCapabilities: []*structs.CSIVolumeCapability{{
			AccessMode:     structs.CSIVolumeAccessModeMultiNodeSingleWriter,
			AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
		}},
	}}
	index++
	err = state.UpsertCSIVolume(index, vols)
	require.NoError(t, err)

	// Verify that the volume exists, and is healthy
	volGetReq := &structs.CSIVolumeGetRequest{
		ID: id0,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: structs.DefaultNamespace,
		},
	}
	volGetResp := &structs.CSIVolumeGetResponse{}
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Get", volGetReq, volGetResp)
	require.NoError(t, err)
	require.Equal(t, id0, volGetResp.Volume.ID)
	require.True(t, volGetResp.Volume.Schedulable)
	require.Len(t, volGetResp.Volume.ReadAllocs, 0)
	require.Len(t, volGetResp.Volume.WriteAllocs, 0)

	// Now our claim should succeed
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Claim", claimReq, claimResp)
	require.NoError(t, err)

	// Verify the claim was set
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Get", volGetReq, volGetResp)
	require.NoError(t, err)
	require.Equal(t, id0, volGetResp.Volume.ID)
	require.Len(t, volGetResp.Volume.ReadAllocs, 0)
	require.Len(t, volGetResp.Volume.WriteAllocs, 1)

	// Make another writer claim for a different job
	alloc2 := mock.Alloc()
	alloc2.JobID = uuid.Generate()
	summary = mock.JobSummary(alloc2.JobID)
	index++
	require.NoError(t, state.UpsertJobSummary(index, summary))
	index++
	require.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, index, []*structs.Allocation{alloc2}))
	claimReq.AllocationID = alloc2.ID
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Claim", claimReq, claimResp)
	require.EqualError(t, err, structs.ErrCSIVolumeMaxClaims.Error(),
		"expected 'volume max claims reached' because we only allow 1 writer")

	// Fix the mode and our claim will succeed
	claimReq.Claim = structs.CSIVolumeClaimRead
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Claim", claimReq, claimResp)
	require.NoError(t, err)

	// Verify the new claim was set
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Get", volGetReq, volGetResp)
	require.NoError(t, err)
	require.Equal(t, id0, volGetResp.Volume.ID)
	require.Len(t, volGetResp.Volume.ReadAllocs, 1)
	require.Len(t, volGetResp.Volume.WriteAllocs, 1)

	// Claim is idempotent
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Claim", claimReq, claimResp)
	require.NoError(t, err)
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Get", volGetReq, volGetResp)
	require.NoError(t, err)
	require.Equal(t, id0, volGetResp.Volume.ID)
	require.Len(t, volGetResp.Volume.ReadAllocs, 1)
	require.Len(t, volGetResp.Volume.WriteAllocs, 1)

	// Make a second reader claim
	alloc3 := mock.Alloc()
	alloc3.JobID = uuid.Generate()
	summary = mock.JobSummary(alloc3.JobID)
	index++
	require.NoError(t, state.UpsertJobSummary(index, summary))
	index++
	require.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, index, []*structs.Allocation{alloc3}))
	claimReq.AllocationID = alloc3.ID
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Claim", claimReq, claimResp)
	require.NoError(t, err)

	// Verify the new claim was set
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Get", volGetReq, volGetResp)
	require.NoError(t, err)
	require.Equal(t, id0, volGetResp.Volume.ID)
	require.Len(t, volGetResp.Volume.ReadAllocs, 2)
	require.Len(t, volGetResp.Volume.WriteAllocs, 1)
}

// TestCSIVolumeEndpoint_ClaimWithController exercises the VolumeClaim RPC
// when a controller is required.
func TestCSIVolumeEndpoint_ClaimWithController(t *testing.T) {
	ci.Parallel(t)
	srv, _, shutdown := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)

	ns := structs.DefaultNamespace
	state := srv.fsm.State()

	policy := mock.NamespacePolicy(ns, "", []string{acl.NamespaceCapabilityCSIMountVolume}) +
		mock.PluginPolicy("read")
	accessToken := mock.CreatePolicyAndToken(t, state, 1001, "claim", policy)

	codec := rpcClient(t, srv)
	id0 := uuid.Generate()

	// Create a client node, plugin, alloc, and volume
	node := mock.Node()
	node.Attributes["nomad.version"] = "0.11.0" // client RPCs not supported on early version
	node.CSIControllerPlugins = map[string]*structs.CSIInfo{
		"minnie": {
			PluginID: "minnie",
			Healthy:  true,
			ControllerInfo: &structs.CSIControllerInfo{
				SupportsAttachDetach: true,
			},
			RequiresControllerPlugin: true,
		},
	}
	node.CSINodePlugins = map[string]*structs.CSIInfo{
		"minnie": {
			PluginID: "minnie",
			Healthy:  true,
			NodeInfo: &structs.CSINodeInfo{},
		},
	}
	err := state.UpsertNode(structs.MsgTypeTestSetup, 1002, node)
	require.NoError(t, err)
	vols := []*structs.CSIVolume{{
		ID:                 id0,
		Namespace:          ns,
		PluginID:           "minnie",
		ControllerRequired: true,
		Secrets:            structs.CSISecrets{"mysecret": "secretvalue"},
		RequestedCapabilities: []*structs.CSIVolumeCapability{{
			AccessMode:     structs.CSIVolumeAccessModeMultiNodeSingleWriter,
			AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
		}},
	}}
	err = state.UpsertCSIVolume(1003, vols)
	require.NoError(t, err)

	alloc := mock.BatchAlloc()
	alloc.NodeID = node.ID
	summary := mock.JobSummary(alloc.JobID)
	require.NoError(t, state.UpsertJobSummary(1004, summary))
	require.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1005, []*structs.Allocation{alloc}))

	// Make the volume claim
	claimReq := &structs.CSIVolumeClaimRequest{
		VolumeID:     id0,
		AllocationID: alloc.ID,
		Claim:        structs.CSIVolumeClaimWrite,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: ns,
			AuthToken: accessToken.SecretID,
		},
	}
	claimResp := &structs.CSIVolumeClaimResponse{}
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Claim", claimReq, claimResp)
	// Because the node is not registered
	require.EqualError(t, err, "controller publish: controller attach volume: No path to node")

	// The node SecretID is authorized for all policies
	claimReq.AuthToken = node.SecretID
	claimReq.Namespace = ""
	claimResp = &structs.CSIVolumeClaimResponse{}
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Claim", claimReq, claimResp)
	require.EqualError(t, err, "controller publish: controller attach volume: No path to node")
}

func TestCSIVolumeEndpoint_Unpublish(t *testing.T) {
	ci.Parallel(t)
	srv, _, shutdown := TestACLServer(t, func(c *Config) { c.NumSchedulers = 0 })
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)

	var err error
	index := uint64(1000)
	ns := structs.DefaultNamespace
	state := srv.fsm.State()

	policy := mock.NamespacePolicy(ns, "", []string{acl.NamespaceCapabilityCSIMountVolume}) +
		mock.PluginPolicy("read")
	index++
	accessToken := mock.CreatePolicyAndToken(t, state, index, "claim", policy)

	codec := rpcClient(t, srv)

	// setup: create a client node with a controller and node plugin
	node := mock.Node()
	node.Attributes["nomad.version"] = "0.11.0"
	node.CSINodePlugins = map[string]*structs.CSIInfo{
		"minnie": {PluginID: "minnie",
			Healthy:  true,
			NodeInfo: &structs.CSINodeInfo{},
		},
	}
	node.CSIControllerPlugins = map[string]*structs.CSIInfo{
		"minnie": {PluginID: "minnie",
			Healthy:                  true,
			ControllerInfo:           &structs.CSIControllerInfo{SupportsAttachDetach: true},
			RequiresControllerPlugin: true,
		},
	}
	index++
	must.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, index, node))

	type tc struct {
		name           string
		startingState  structs.CSIVolumeClaimState
		endState       structs.CSIVolumeClaimState
		nodeID         string
		otherNodeID    string
		expectedErrMsg string
	}
	testCases := []tc{
		{
			name:          "success",
			startingState: structs.CSIVolumeClaimStateControllerDetached,
			nodeID:        node.ID,
			otherNodeID:   uuid.Generate(),
		},
		{
			name:          "non-terminal allocation on same node",
			startingState: structs.CSIVolumeClaimStateNodeDetached,
			nodeID:        node.ID,
			otherNodeID:   node.ID,
		},
		{
			name:           "unpublish previously detached node",
			startingState:  structs.CSIVolumeClaimStateNodeDetached,
			endState:       structs.CSIVolumeClaimStateNodeDetached,
			expectedErrMsg: "could not detach from controller: controller detach volume: No path to node",
			nodeID:         node.ID,
			otherNodeID:    uuid.Generate(),
		},
		{
			name:           "unpublish claim on garbage collected node",
			startingState:  structs.CSIVolumeClaimStateTaken,
			endState:       structs.CSIVolumeClaimStateNodeDetached,
			expectedErrMsg: "could not detach from controller: controller detach volume: No path to node",
			nodeID:         uuid.Generate(),
			otherNodeID:    uuid.Generate(),
		},
		{
			name:           "first unpublish",
			startingState:  structs.CSIVolumeClaimStateTaken,
			endState:       structs.CSIVolumeClaimStateNodeDetached,
			expectedErrMsg: "could not detach from controller: controller detach volume: No path to node",
			nodeID:         node.ID,
			otherNodeID:    uuid.Generate(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// setup: register a volume
			volID := uuid.Generate()
			vol := &structs.CSIVolume{
				ID:                 volID,
				Namespace:          ns,
				PluginID:           "minnie",
				Secrets:            structs.CSISecrets{"mysecret": "secretvalue"},
				ControllerRequired: true,
				RequestedCapabilities: []*structs.CSIVolumeCapability{{
					AccessMode:     structs.CSIVolumeAccessModeMultiNodeSingleWriter,
					AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
				}},
			}

			index++
			err = state.UpsertCSIVolume(index, []*structs.CSIVolume{vol})
			must.NoError(t, err)

			// setup: create an alloc that will claim our volume
			alloc := mock.BatchAlloc()
			alloc.NodeID = tc.nodeID
			alloc.ClientStatus = structs.AllocClientStatusRunning

			otherAlloc := mock.BatchAlloc()
			otherAlloc.NodeID = tc.otherNodeID
			otherAlloc.ClientStatus = structs.AllocClientStatusRunning

			index++
			must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, index,
				[]*structs.Allocation{alloc, otherAlloc}))

			// setup: claim the volume for our to-be-failed alloc
			claim := &structs.CSIVolumeClaim{
				AllocationID:   alloc.ID,
				NodeID:         tc.nodeID,
				ExternalNodeID: "i-example",
				Mode:           structs.CSIVolumeClaimRead,
			}

			index++
			claim.State = structs.CSIVolumeClaimStateTaken
			err = state.CSIVolumeClaim(index, ns, volID, claim)
			must.NoError(t, err)

			// setup: claim the volume for our other alloc
			otherClaim := &structs.CSIVolumeClaim{
				AllocationID:   otherAlloc.ID,
				NodeID:         tc.otherNodeID,
				ExternalNodeID: "i-example",
				Mode:           structs.CSIVolumeClaimRead,
			}

			index++
			otherClaim.State = structs.CSIVolumeClaimStateTaken
			err = state.CSIVolumeClaim(index, ns, volID, otherClaim)
			must.NoError(t, err)

			// test: unpublish and check the results
			claim.State = tc.startingState
			req := &structs.CSIVolumeUnpublishRequest{
				VolumeID: volID,
				Claim:    claim,
				WriteRequest: structs.WriteRequest{
					Region:    "global",
					Namespace: ns,
					AuthToken: accessToken.SecretID,
				},
			}

			alloc = alloc.Copy()
			alloc.ClientStatus = structs.AllocClientStatusFailed
			index++
			must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, index,
				[]*structs.Allocation{alloc}))

			err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Unpublish", req,
				&structs.CSIVolumeUnpublishResponse{})

			snap, snapErr := state.Snapshot()
			must.NoError(t, snapErr)

			vol, volErr := snap.CSIVolumeByID(nil, ns, volID)
			must.NoError(t, volErr)
			must.NotNil(t, vol)

			if tc.expectedErrMsg == "" {
				must.NoError(t, err)
				assert.Len(t, vol.ReadAllocs, 1)
			} else {
				must.Error(t, err)
				assert.Len(t, vol.ReadAllocs, 2)
				test.True(t, strings.Contains(err.Error(), tc.expectedErrMsg),
					test.Sprintf("error %v did not contain %q", err, tc.expectedErrMsg))
				claim = vol.PastClaims[alloc.ID]
				must.NotNil(t, claim)
				test.Eq(t, tc.endState, claim.State)
			}

		})
	}

}

func TestCSIVolumeEndpoint_List(t *testing.T) {
	ci.Parallel(t)
	srv, _, shutdown := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)

	state := srv.fsm.State()
	codec := rpcClient(t, srv)

	nsPolicy := mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityCSIReadVolume}) +
		mock.PluginPolicy("read")
	nsTok := mock.CreatePolicyAndToken(t, state, 1000, "csi-token-name", nsPolicy)

	// Empty list results
	req := &structs.CSIVolumeListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			AuthToken: nsTok.SecretID,
			Namespace: structs.DefaultNamespace,
		},
	}
	var resp structs.CSIVolumeListResponse
	err := msgpackrpc.CallWithCodec(codec, "CSIVolume.List", req, &resp)
	require.NoError(t, err)
	require.NotNil(t, resp.Volumes)
	require.Equal(t, 0, len(resp.Volumes))

	// Create the volume
	id0 := uuid.Generate()
	id1 := uuid.Generate()
	vols := []*structs.CSIVolume{{
		ID:        id0,
		Namespace: structs.DefaultNamespace,
		PluginID:  "minnie",
		Secrets:   structs.CSISecrets{"mysecret": "secretvalue"},
		RequestedCapabilities: []*structs.CSIVolumeCapability{{
			AccessMode:     structs.CSIVolumeAccessModeMultiNodeReader,
			AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
		}},
	}, {
		ID:        id1,
		Namespace: structs.DefaultNamespace,
		PluginID:  "adam",
		RequestedCapabilities: []*structs.CSIVolumeCapability{{
			AccessMode:     structs.CSIVolumeAccessModeMultiNodeSingleWriter,
			AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
		}},
	}}
	err = state.UpsertCSIVolume(1002, vols)
	require.NoError(t, err)

	// Query everything in the namespace
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.List", req, &resp)
	require.NoError(t, err)

	require.Equal(t, uint64(1002), resp.Index)
	require.Equal(t, 2, len(resp.Volumes))
	ids := map[string]bool{vols[0].ID: true, vols[1].ID: true}
	for _, v := range resp.Volumes {
		delete(ids, v.ID)
	}
	require.Equal(t, 0, len(ids))

	// Query by PluginID in ns
	req = &structs.CSIVolumeListRequest{
		PluginID: "adam",
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: structs.DefaultNamespace,
			AuthToken: nsTok.SecretID,
		},
	}
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.List", req, &resp)
	require.NoError(t, err)
	require.Equal(t, 1, len(resp.Volumes))
	require.Equal(t, vols[1].ID, resp.Volumes[0].ID)
}

func TestCSIVolumeEndpoint_ListAllNamespaces(t *testing.T) {
	ci.Parallel(t)
	srv, shutdown := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)

	state := srv.fsm.State()
	codec := rpcClient(t, srv)

	// Create namespaces.
	ns0 := structs.DefaultNamespace
	ns1 := "namespace-1"
	ns2 := "namespace-2"
	err := state.UpsertNamespaces(1000, []*structs.Namespace{{Name: ns1}, {Name: ns2}})
	require.NoError(t, err)

	// Create volumes in multiple namespaces.
	id0 := uuid.Generate()
	id1 := uuid.Generate()
	id2 := uuid.Generate()
	vols := []*structs.CSIVolume{{
		ID:        id0,
		Namespace: ns0,
		PluginID:  "minnie",
		Secrets:   structs.CSISecrets{"mysecret": "secretvalue"},
		RequestedCapabilities: []*structs.CSIVolumeCapability{{
			AccessMode:     structs.CSIVolumeAccessModeMultiNodeReader,
			AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
		}},
	}, {
		ID:        id1,
		Namespace: ns1,
		PluginID:  "adam",
		RequestedCapabilities: []*structs.CSIVolumeCapability{{
			AccessMode:     structs.CSIVolumeAccessModeMultiNodeSingleWriter,
			AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
		}},
	}, {
		ID:        id2,
		Namespace: ns2,
		PluginID:  "beth",
		RequestedCapabilities: []*structs.CSIVolumeCapability{{
			AccessMode:     structs.CSIVolumeAccessModeMultiNodeSingleWriter,
			AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
		}},
	},
	}
	err = state.UpsertCSIVolume(1001, vols)
	require.NoError(t, err)

	// Lookup volumes in all namespaces
	get := &structs.CSIVolumeListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: "*",
		},
	}
	var resp structs.CSIVolumeListResponse
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.List", get, &resp)
	require.NoError(t, err)
	require.Equal(t, uint64(1001), resp.Index)
	require.Len(t, resp.Volumes, len(vols))

	// Lookup volumes in all namespaces with prefix
	get = &structs.CSIVolumeListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Prefix:    id0[:4],
			Namespace: "*",
		},
	}
	var resp2 structs.CSIVolumeListResponse
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.List", get, &resp2)
	require.NoError(t, err)
	require.Equal(t, uint64(1001), resp.Index)
	require.Len(t, resp2.Volumes, 1)
	require.Equal(t, vols[0].ID, resp2.Volumes[0].ID)
	require.Equal(t, structs.DefaultNamespace, resp2.Volumes[0].Namespace)
}

func TestCSIVolumeEndpoint_List_PaginationFiltering(t *testing.T) {
	ci.Parallel(t)
	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	nonDefaultNS := "non-default"

	// create a set of volumes. these are in the order that the state store
	// will return them from the iterator (sorted by create index), for ease of
	// writing tests
	mocks := []struct {
		id        string
		namespace string
	}{
		{id: "vol-01"},                          // 0
		{id: "vol-02"},                          // 1
		{id: "vol-03", namespace: nonDefaultNS}, // 2
		{id: "vol-04"},                          // 3
		{id: "vol-05"},                          // 4
		{id: "vol-06"},                          // 5
		{id: "vol-07"},                          // 6
		{id: "vol-08"},                          // 7
		{},                                      // 9, missing volume
		{id: "vol-10"},                          // 10
	}

	state := s1.fsm.State()
	plugin := mock.CSIPlugin()

	// Create namespaces.
	err := state.UpsertNamespaces(999, []*structs.Namespace{{Name: nonDefaultNS}})
	require.NoError(t, err)

	for i, m := range mocks {
		if m.id == "" {
			continue
		}

		volume := mock.CSIVolume(plugin)
		volume.ID = m.id
		if m.namespace != "" { // defaults to "default"
			volume.Namespace = m.namespace
		}
		index := 1000 + uint64(i)
		require.NoError(t, state.UpsertCSIVolume(index, []*structs.CSIVolume{volume}))
	}

	cases := []struct {
		name              string
		namespace         string
		prefix            string
		filter            string
		nextToken         string
		pageSize          int32
		expectedNextToken string
		expectedIDs       []string
		expectedError     string
	}{
		{
			name:              "test01 size-2 page-1 default NS",
			pageSize:          2,
			expectedNextToken: "default.vol-04",
			expectedIDs: []string{
				"vol-01",
				"vol-02",
			},
		},
		{
			name:              "test02 size-2 page-1 default NS with prefix",
			prefix:            "vol",
			pageSize:          2,
			expectedNextToken: "default.vol-04",
			expectedIDs: []string{
				"vol-01",
				"vol-02",
			},
		},
		{
			name:              "test03 size-2 page-2 default NS",
			pageSize:          2,
			nextToken:         "default.vol-04",
			expectedNextToken: "default.vol-06",
			expectedIDs: []string{
				"vol-04",
				"vol-05",
			},
		},
		{
			name:              "test04 size-2 page-2 default NS with prefix",
			prefix:            "vol",
			pageSize:          2,
			nextToken:         "default.vol-04",
			expectedNextToken: "default.vol-06",
			expectedIDs: []string{
				"vol-04",
				"vol-05",
			},
		},
		{
			name:        "test05 no valid results with filters and prefix",
			prefix:      "cccc",
			pageSize:    2,
			nextToken:   "",
			expectedIDs: []string{},
		},
		{
			name:      "test06 go-bexpr filter",
			namespace: "*",
			filter:    `ID matches "^vol-0[123]"`,
			expectedIDs: []string{
				"vol-01",
				"vol-02",
				"vol-03",
			},
		},
		{
			name:              "test07 go-bexpr filter with pagination",
			namespace:         "*",
			filter:            `ID matches "^vol-0[123]"`,
			pageSize:          2,
			expectedNextToken: "non-default.vol-03",
			expectedIDs: []string{
				"vol-01",
				"vol-02",
			},
		},
		{
			name:      "test08 go-bexpr filter in namespace",
			namespace: "non-default",
			filter:    `Provider == "com.hashicorp:mock"`,
			expectedIDs: []string{
				"vol-03",
			},
		},
		{
			name:        "test09 go-bexpr wrong namespace",
			namespace:   "default",
			filter:      `Namespace == "non-default"`,
			expectedIDs: []string{},
		},
		{
			name:          "test10 go-bexpr invalid expression",
			filter:        `NotValid`,
			expectedError: "failed to read filter expression",
		},
		{
			name:          "test11 go-bexpr invalid field",
			filter:        `InvalidField == "value"`,
			expectedError: "error finding value in datum",
		},
		{
			name:      "test14 missing volume",
			pageSize:  1,
			nextToken: "default.vol-09",
			expectedIDs: []string{
				"vol-10",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := &structs.CSIVolumeListRequest{
				QueryOptions: structs.QueryOptions{
					Region:    "global",
					Namespace: tc.namespace,
					Prefix:    tc.prefix,
					Filter:    tc.filter,
					PerPage:   tc.pageSize,
					NextToken: tc.nextToken,
				},
			}
			var resp structs.CSIVolumeListResponse
			err := msgpackrpc.CallWithCodec(codec, "CSIVolume.List", req, &resp)
			if tc.expectedError == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedError)
				return
			}

			gotIDs := []string{}
			for _, deployment := range resp.Volumes {
				gotIDs = append(gotIDs, deployment.ID)
			}
			require.Equal(t, tc.expectedIDs, gotIDs, "unexpected page of volumes")
			require.Equal(t, tc.expectedNextToken, resp.QueryMeta.NextToken, "unexpected NextToken")
		})
	}
}

func TestCSIVolumeEndpoint_Create(t *testing.T) {
	ci.Parallel(t)
	var err error
	srv, shutdown := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()

	testutil.WaitForLeader(t, srv.RPC)

	fake := newMockClientCSI()
	fake.NextValidateError = nil
	fake.NextCreateError = nil
	fake.NextCreateResponse = &cstructs.ClientCSIControllerCreateVolumeResponse{
		ExternalVolumeID: "vol-12345",
		CapacityBytes:    42,
		VolumeContext:    map[string]string{"plugincontext": "bar"},
		Topologies: []*structs.CSITopology{
			{Segments: map[string]string{"rack": "R1"}},
		},
	}

	client, cleanup := client.TestClientWithRPCs(t,
		func(c *cconfig.Config) {
			c.Servers = []string{srv.config.RPCAddr.String()}
		},
		map[string]interface{}{"CSI": fake},
	)
	defer cleanup()

	node := client.UpdateConfig(func(c *cconfig.Config) {
		// client RPCs not supported on early versions
		c.Node.Attributes["nomad.version"] = "0.11.0"
	}).Node

	req0 := &structs.NodeRegisterRequest{
		Node:         node,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp0 structs.NodeUpdateResponse
	err = client.RPC("Node.Register", req0, &resp0)
	require.NoError(t, err)

	testutil.WaitForResult(func() (bool, error) {
		nodes := srv.connectedNodes()
		return len(nodes) == 1, nil
	}, func(err error) {
		t.Fatalf("should have a client")
	})

	ns := structs.DefaultNamespace

	state := srv.fsm.State()
	codec := rpcClient(t, srv)
	index := uint64(1000)

	node = client.UpdateConfig(func(c *cconfig.Config) {
		c.Node.CSIControllerPlugins = map[string]*structs.CSIInfo{
			"minnie": {
				PluginID: "minnie",
				Healthy:  true,
				ControllerInfo: &structs.CSIControllerInfo{
					SupportsAttachDetach: true,
					SupportsCreateDelete: true,
				},
				RequiresControllerPlugin: true,
			},
		}
		c.Node.CSINodePlugins = map[string]*structs.CSIInfo{
			"minnie": {
				PluginID: "minnie",
				Healthy:  true,
				NodeInfo: &structs.CSINodeInfo{},
			},
		}
	}).Node
	index++
	require.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, index, node))

	// Create the volume
	volID := uuid.Generate()
	vols := []*structs.CSIVolume{{
		ID:             volID,
		Name:           "vol",
		Namespace:      "", // overriden by WriteRequest
		PluginID:       "minnie",
		AccessMode:     structs.CSIVolumeAccessModeSingleNodeReader, // legacy field ignored
		AttachmentMode: structs.CSIVolumeAttachmentModeBlockDevice,  // legacy field ignored
		MountOptions: &structs.CSIMountOptions{
			FSType: "ext4", MountFlags: []string{"sensitive"}}, // ignored in create
		Secrets:    structs.CSISecrets{"mysecret": "secretvalue"},
		Parameters: map[string]string{"myparam": "paramvalue"},
		Context:    map[string]string{"mycontext": "contextvalue"}, // dropped by create
		RequestedCapabilities: []*structs.CSIVolumeCapability{
			{
				AccessMode:     structs.CSIVolumeAccessModeMultiNodeReader,
				AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
			},
		},
		Topologies: []*structs.CSITopology{
			{Segments: map[string]string{"rack": "R1"}},
			{Segments: map[string]string{"zone": "Z2"}},
		},
	}}

	// Create the create request
	req1 := &structs.CSIVolumeCreateRequest{
		Volumes: vols,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: ns,
		},
	}
	resp1 := &structs.CSIVolumeCreateResponse{}
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Create", req1, resp1)
	require.NoError(t, err)

	// Get the volume back out
	req2 := &structs.CSIVolumeGetRequest{
		ID: volID,
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}
	resp2 := &structs.CSIVolumeGetResponse{}
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Get", req2, resp2)
	require.NoError(t, err)
	require.Equal(t, resp1.Index, resp2.Index)

	vol := resp2.Volume
	require.NotNil(t, vol)
	require.Equal(t, volID, vol.ID)

	// these fields are set from the args
	require.Equal(t, "csi.CSISecrets(map[mysecret:[REDACTED]])",
		vol.Secrets.String())
	require.Equal(t, "csi.CSIOptions(FSType: ext4, MountFlags: [REDACTED])",
		vol.MountOptions.String())
	require.Equal(t, ns, vol.Namespace)
	require.Len(t, vol.RequestedCapabilities, 1)

	// these fields are set from the plugin and should have been written to raft
	require.Equal(t, "vol-12345", vol.ExternalID)
	require.Equal(t, int64(42), vol.Capacity)
	require.Equal(t, "bar", vol.Context["plugincontext"])
	require.Equal(t, "", vol.Context["mycontext"])
	require.Equal(t, map[string]string{"rack": "R1"}, vol.Topologies[0].Segments)
}

func TestCSIVolumeEndpoint_Delete(t *testing.T) {
	ci.Parallel(t)
	var err error
	srv, shutdown := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()

	testutil.WaitForLeader(t, srv.RPC)

	fake := newMockClientCSI()
	fake.NextDeleteError = fmt.Errorf("should not see this")

	client, cleanup := client.TestClientWithRPCs(t,
		func(c *cconfig.Config) {
			c.Servers = []string{srv.config.RPCAddr.String()}
		},
		map[string]interface{}{"CSI": fake},
	)
	defer cleanup()

	node := client.UpdateConfig(func(c *cconfig.Config) {
		// client RPCs not supported on early versions
		c.Node.Attributes["nomad.version"] = "0.11.0"
	}).Node

	req0 := &structs.NodeRegisterRequest{
		Node:         node,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp0 structs.NodeUpdateResponse
	err = client.RPC("Node.Register", req0, &resp0)
	must.NoError(t, err)

	testutil.WaitForResult(func() (bool, error) {
		nodes := srv.connectedNodes()
		return len(nodes) == 1, nil
	}, func(err error) {
		t.Fatalf("should have a client")
	})

	ns := structs.DefaultNamespace

	state := srv.fsm.State()
	codec := rpcClient(t, srv)
	index := uint64(1000)

	node = client.UpdateConfig(func(c *cconfig.Config) {
		c.Node.CSIControllerPlugins = map[string]*structs.CSIInfo{
			"minnie": {
				PluginID: "minnie",
				Healthy:  true,
				ControllerInfo: &structs.CSIControllerInfo{
					SupportsAttachDetach: true,
				},
				RequiresControllerPlugin: true,
			},
		}
		c.Node.CSINodePlugins = map[string]*structs.CSIInfo{
			"minnie": {
				PluginID: "minnie",
				Healthy:  true,
				NodeInfo: &structs.CSINodeInfo{},
			},
		}
	}).Node
	index++
	must.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, index, node))

	volID := uuid.Generate()
	noPluginVolID := uuid.Generate()
	vols := []*structs.CSIVolume{
		{
			ID:        volID,
			Namespace: structs.DefaultNamespace,
			PluginID:  "minnie",
			Secrets:   structs.CSISecrets{"mysecret": "secretvalue"},
		},
		{
			ID:        noPluginVolID,
			Namespace: structs.DefaultNamespace,
			PluginID:  "doesnt-exist",
			Secrets:   structs.CSISecrets{"mysecret": "secretvalue"},
		},
	}
	index++
	err = state.UpsertCSIVolume(index, vols)
	must.NoError(t, err)

	// Delete volumes

	// Create an invalid delete request, ensure it doesn't hit the plugin
	req1 := &structs.CSIVolumeDeleteRequest{
		VolumeIDs: []string{"bad", volID},
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: ns,
		},
		Secrets: structs.CSISecrets{
			"secret-key-1": "secret-val-1",
		},
	}
	resp1 := &structs.CSIVolumeCreateResponse{}
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Delete", req1, resp1)
	must.EqError(t, err, "volume not found: bad")

	// Make sure the valid volume wasn't deleted
	req2 := &structs.CSIVolumeGetRequest{
		ID: volID,
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}
	resp2 := &structs.CSIVolumeGetResponse{}
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Get", req2, resp2)
	must.NoError(t, err)
	must.NotNil(t, resp2.Volume)

	// Fix the delete request
	fake.NextDeleteError = nil
	req1.VolumeIDs = []string{volID}
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Delete", req1, resp1)
	must.NoError(t, err)

	// Make sure it was deregistered
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Get", req2, resp2)
	must.NoError(t, err)
	must.Nil(t, resp2.Volume)

	// Create a delete request for a volume without plugin.
	req3 := &structs.CSIVolumeDeleteRequest{
		VolumeIDs: []string{noPluginVolID},
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: ns,
		},
		Secrets: structs.CSISecrets{
			"secret-key-1": "secret-val-1",
		},
	}
	resp3 := &structs.CSIVolumeCreateResponse{}
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Delete", req3, resp3)
	must.EqError(t, err, fmt.Sprintf(`plugin "doesnt-exist" for volume "%s" not found`, noPluginVolID))
}

func TestCSIVolumeEndpoint_ListExternal(t *testing.T) {
	ci.Parallel(t)
	var err error
	srv, shutdown := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()

	testutil.WaitForLeader(t, srv.RPC)

	fake := newMockClientCSI()
	fake.NextDeleteError = fmt.Errorf("should not see this")
	fake.NextListExternalResponse = &cstructs.ClientCSIControllerListVolumesResponse{
		Entries: []*structs.CSIVolumeExternalStub{
			{
				ExternalID:               "vol-12345",
				CapacityBytes:            70000,
				PublishedExternalNodeIDs: []string{"i-12345"},
			},
			{
				ExternalID:    "vol-abcde",
				CapacityBytes: 50000,
				IsAbnormal:    true,
				Status:        "something went wrong",
			},
			{
				ExternalID: "vol-00000",
				Status:     "you should not see me",
			},
		},
		NextToken: "page2",
	}

	client, cleanup := client.TestClientWithRPCs(t,
		func(c *cconfig.Config) {
			c.Servers = []string{srv.config.RPCAddr.String()}
		},
		map[string]interface{}{"CSI": fake},
	)
	defer cleanup()

	node := client.UpdateConfig(func(c *cconfig.Config) {
		// client RPCs not supported on early versions
		c.Node.Attributes["nomad.version"] = "0.11.0"
	}).Node

	req0 := &structs.NodeRegisterRequest{
		Node:         node,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp0 structs.NodeUpdateResponse
	err = client.RPC("Node.Register", req0, &resp0)
	require.NoError(t, err)

	testutil.WaitForResult(func() (bool, error) {
		nodes := srv.connectedNodes()
		return len(nodes) == 1, nil
	}, func(err error) {
		t.Fatalf("should have a client")
	})

	state := srv.fsm.State()
	codec := rpcClient(t, srv)
	index := uint64(1000)

	node = client.UpdateConfig(func(c *cconfig.Config) {
		c.Node.CSIControllerPlugins = map[string]*structs.CSIInfo{
			"minnie": {
				PluginID: "minnie",
				Healthy:  true,
				ControllerInfo: &structs.CSIControllerInfo{
					SupportsAttachDetach: true,
					SupportsListVolumes:  true,
				},
				RequiresControllerPlugin: true,
			},
		}
		c.Node.CSINodePlugins = map[string]*structs.CSIInfo{
			"minnie": {
				PluginID: "minnie",
				Healthy:  true,
				NodeInfo: &structs.CSINodeInfo{},
			},
		}
	}).Node
	index++
	require.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, index, node))

	// List external volumes; note that none of these exist in the state store

	req := &structs.CSIVolumeExternalListRequest{
		PluginID: "minnie",
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: structs.DefaultNamespace,
			PerPage:   2,
			NextToken: "page1",
		},
	}
	resp := &structs.CSIVolumeExternalListResponse{}
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.ListExternal", req, resp)
	require.NoError(t, err)
	require.Len(t, resp.Volumes, 2)
	require.Equal(t, "vol-12345", resp.Volumes[0].ExternalID)
	require.Equal(t, "vol-abcde", resp.Volumes[1].ExternalID)
	require.True(t, resp.Volumes[1].IsAbnormal)
	require.Equal(t, "page2", resp.NextToken)
}

func TestCSIVolumeEndpoint_CreateSnapshot(t *testing.T) {
	ci.Parallel(t)
	var err error
	srv, shutdown := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()

	testutil.WaitForLeader(t, srv.RPC)

	now := time.Now().Unix()
	fake := newMockClientCSI()
	fake.NextCreateSnapshotError = nil
	fake.NextCreateSnapshotResponse = &cstructs.ClientCSIControllerCreateSnapshotResponse{
		ID:                     "snap-12345",
		ExternalSourceVolumeID: "vol-12345",
		SizeBytes:              42,
		CreateTime:             now,
		IsReady:                true,
	}

	client, cleanup := client.TestClientWithRPCs(t,
		func(c *cconfig.Config) {
			c.Servers = []string{srv.config.RPCAddr.String()}
		},
		map[string]interface{}{"CSI": fake},
	)
	defer cleanup()

	req0 := &structs.NodeRegisterRequest{
		Node:         client.Node(),
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp0 structs.NodeUpdateResponse
	err = client.RPC("Node.Register", req0, &resp0)
	require.NoError(t, err)

	testutil.WaitForResult(func() (bool, error) {
		nodes := srv.connectedNodes()
		return len(nodes) == 1, nil
	}, func(err error) {
		t.Fatalf("should have a client")
	})

	ns := structs.DefaultNamespace

	state := srv.fsm.State()
	codec := rpcClient(t, srv)
	index := uint64(1000)

	node := client.UpdateConfig(func(c *cconfig.Config) {
		c.Node.CSIControllerPlugins = map[string]*structs.CSIInfo{
			"minnie": {
				PluginID: "minnie",
				Healthy:  true,
				ControllerInfo: &structs.CSIControllerInfo{
					SupportsCreateDeleteSnapshot: true,
				},
				RequiresControllerPlugin: true,
			},
		}
	}).Node
	index++
	require.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, index, node))

	// Create the volume
	vols := []*structs.CSIVolume{{
		ID:             "test-volume0",
		Namespace:      ns,
		AccessMode:     structs.CSIVolumeAccessModeMultiNodeSingleWriter,
		AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
		PluginID:       "minnie",
		ExternalID:     "vol-12345",
	}}
	index++
	require.NoError(t, state.UpsertCSIVolume(index, vols))

	// Create the snapshot request
	req1 := &structs.CSISnapshotCreateRequest{
		Snapshots: []*structs.CSISnapshot{{
			Name:           "snap",
			SourceVolumeID: "test-volume0",
			Secrets:        structs.CSISecrets{"mysecret": "secretvalue"},
			Parameters:     map[string]string{"myparam": "paramvalue"},
		}},
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: ns,
		},
	}
	resp1 := &structs.CSISnapshotCreateResponse{}
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.CreateSnapshot", req1, resp1)
	require.NoError(t, err)

	snap := resp1.Snapshots[0]
	require.Equal(t, "vol-12345", snap.ExternalSourceVolumeID)       // set by the args
	require.Equal(t, "snap-12345", snap.ID)                          // set by the plugin
	require.Equal(t, "csi.CSISecrets(map[])", snap.Secrets.String()) // should not be set
	require.Len(t, snap.Parameters, 0)                               // should not be set
}

func TestCSIVolumeEndpoint_DeleteSnapshot(t *testing.T) {
	ci.Parallel(t)
	var err error
	srv, shutdown := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()

	testutil.WaitForLeader(t, srv.RPC)

	fake := newMockClientCSI()
	fake.NextDeleteSnapshotError = nil

	client, cleanup := client.TestClientWithRPCs(t,
		func(c *cconfig.Config) {
			c.Servers = []string{srv.config.RPCAddr.String()}
		},
		map[string]interface{}{"CSI": fake},
	)
	defer cleanup()

	req0 := &structs.NodeRegisterRequest{
		Node:         client.Node(),
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp0 structs.NodeUpdateResponse
	err = client.RPC("Node.Register", req0, &resp0)
	require.NoError(t, err)

	testutil.WaitForResult(func() (bool, error) {
		nodes := srv.connectedNodes()
		return len(nodes) == 1, nil
	}, func(err error) {
		t.Fatalf("should have a client")
	})

	ns := structs.DefaultNamespace

	state := srv.fsm.State()
	codec := rpcClient(t, srv)
	index := uint64(1000)

	node := client.UpdateConfig(func(c *cconfig.Config) {
		c.Node.CSIControllerPlugins = map[string]*structs.CSIInfo{
			"minnie": {
				PluginID: "minnie",
				Healthy:  true,
				ControllerInfo: &structs.CSIControllerInfo{
					SupportsCreateDeleteSnapshot: true,
				},
				RequiresControllerPlugin: true,
			},
		}
	}).Node
	index++
	require.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, index, node))

	// Delete the snapshot request
	req1 := &structs.CSISnapshotDeleteRequest{
		Snapshots: []*structs.CSISnapshot{
			{
				ID:       "snap-12345",
				PluginID: "minnie",
			},
			{
				ID:       "snap-34567",
				PluginID: "minnie",
			},
		},
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: ns,
		},
	}

	resp1 := &structs.CSISnapshotDeleteResponse{}
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.DeleteSnapshot", req1, resp1)
	require.NoError(t, err)
}

func TestCSIVolumeEndpoint_ListSnapshots(t *testing.T) {
	ci.Parallel(t)
	var err error
	srv, shutdown := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()

	testutil.WaitForLeader(t, srv.RPC)

	fake := newMockClientCSI()
	fake.NextListExternalSnapshotsResponse = &cstructs.ClientCSIControllerListSnapshotsResponse{
		Entries: []*structs.CSISnapshot{
			{
				ID:                     "snap-12345",
				ExternalSourceVolumeID: "vol-12345",
				SizeBytes:              70000,
				IsReady:                true,
			},
			{
				ID:                     "snap-abcde",
				ExternalSourceVolumeID: "vol-abcde",
				SizeBytes:              70000,
				IsReady:                false,
			},
			{
				ExternalSourceVolumeID: "you should not see me",
			},
		},
		NextToken: "page2",
	}

	client, cleanup := client.TestClientWithRPCs(t,
		func(c *cconfig.Config) {
			c.Servers = []string{srv.config.RPCAddr.String()}
		},
		map[string]interface{}{"CSI": fake},
	)
	defer cleanup()

	req0 := &structs.NodeRegisterRequest{
		Node:         client.Node(),
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp0 structs.NodeUpdateResponse
	err = client.RPC("Node.Register", req0, &resp0)
	require.NoError(t, err)

	testutil.WaitForResult(func() (bool, error) {
		nodes := srv.connectedNodes()
		return len(nodes) == 1, nil
	}, func(err error) {
		t.Fatalf("should have a client")
	})

	state := srv.fsm.State()
	codec := rpcClient(t, srv)
	index := uint64(1000)

	node := client.UpdateConfig(func(c *cconfig.Config) {
		c.Node.CSIControllerPlugins = map[string]*structs.CSIInfo{
			"minnie": {
				PluginID: "minnie",
				Healthy:  true,
				ControllerInfo: &structs.CSIControllerInfo{
					SupportsListSnapshots: true,
				},
				RequiresControllerPlugin: true,
			},
		}
	}).Node
	index++
	require.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, index, node))

	// List snapshots
	req := &structs.CSISnapshotListRequest{
		PluginID: "minnie",
		Secrets: structs.CSISecrets{
			"secret-key-1": "secret-val-1",
		},
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: structs.DefaultNamespace,
			PerPage:   2,
			NextToken: "page1",
		},
	}
	resp := &structs.CSISnapshotListResponse{}
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.ListSnapshots", req, resp)
	require.NoError(t, err)
	require.Len(t, resp.Snapshots, 2)
	require.Equal(t, "vol-12345", resp.Snapshots[0].ExternalSourceVolumeID)
	require.Equal(t, "vol-abcde", resp.Snapshots[1].ExternalSourceVolumeID)
	require.True(t, resp.Snapshots[0].IsReady)
	require.Equal(t, "page2", resp.NextToken)
}

func TestCSIVolume_expandVolume(t *testing.T) {
	ci.Parallel(t)

	srv, cleanupSrv := TestServer(t, nil)
	t.Cleanup(cleanupSrv)
	testutil.WaitForLeader(t, srv.RPC)
	t.Log("server started 👍")

	_, fake, _, fakeVolID := testClientWithCSI(t, srv)

	endpoint := NewCSIVolumeEndpoint(srv, nil)
	plug, vol, err := endpoint.volAndPluginLookup(structs.DefaultNamespace, fakeVolID)
	must.NoError(t, err)

	// ensure nil checks
	expectErr := "unexpected nil value"
	err = endpoint.expandVolume(nil, plug, &csi.CapacityRange{})
	must.EqError(t, err, expectErr)
	err = endpoint.expandVolume(vol, nil, &csi.CapacityRange{})
	must.EqError(t, err, expectErr)
	err = endpoint.expandVolume(vol, plug, nil)
	must.EqError(t, err, expectErr)

	// these tests must be run in order, as they mutate vol along the way
	cases := []struct {
		Name string

		NewMin int64
		NewMax int64

		ExpectMin      int64
		ExpectMax      int64
		ControllerResp int64 // new capacity for the mock controller response
		ExpectCapacity int64 // expected resulting capacity on the volume
		ExpectErr      string
	}{
		{
			// successful expansion from initial vol with no capacity values.
			Name:   "success",
			NewMin: 1000,
			NewMax: 2000,

			ExpectMin:      1000,
			ExpectMax:      2000,
			ControllerResp: 1000,
			ExpectCapacity: 1000,
		},
		{
			// with min/max both zero, no action should be taken,
			// so expect no change to desired or actual capacity on the volume.
			Name:   "zero",
			NewMin: 0,
			NewMax: 0,

			ExpectMin:      1000,
			ExpectMax:      2000,
			ControllerResp: 999999, // this should not come into play
			ExpectCapacity: 1000,
		},
		{
			// increasing min is what actually triggers an expand to occur.
			Name:   "increase min",
			NewMin: 1500,
			NewMax: 2000,

			ExpectMin:      1500,
			ExpectMax:      2000,
			ControllerResp: 1500,
			ExpectCapacity: 1500,
		},
		{
			// min going down is okay, but no expand should occur.
			Name:   "reduce min",
			NewMin: 500,
			NewMax: 2000,

			ExpectMin:      500,
			ExpectMax:      2000,
			ControllerResp: 999999,
			ExpectCapacity: 1500,
		},
		{
			// max going up is okay, but no expand should occur.
			Name:   "increase max",
			NewMin: 500,
			NewMax: 5000,

			ExpectMin:      500,
			ExpectMax:      5000,
			ControllerResp: 999999,
			ExpectCapacity: 1500,
		},
		{
			// max lower than min is logically impossible.
			Name:      "max below min",
			NewMin:    3,
			NewMax:    2,
			ExpectErr: "max requested capacity (2 B) less than or equal to min (3 B)",
		},
		{
			// volume size cannot be reduced.
			Name:      "max below current",
			NewMax:    2,
			ExpectErr: "max requested capacity (2 B) less than or equal to current (1.5 kB)",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			fake.NextControllerExpandVolumeResponse = &cstructs.ClientCSIControllerExpandVolumeResponse{
				CapacityBytes: tc.ControllerResp,
				// this also exercises some node expand code, incidentally
				NodeExpansionRequired: true,
			}

			err = endpoint.expandVolume(vol, plug, &csi.CapacityRange{
				RequiredBytes: tc.NewMin,
				LimitBytes:    tc.NewMax,
			})

			if tc.ExpectErr != "" {
				must.EqError(t, err, tc.ExpectErr)
				return
			}

			must.NoError(t, err)

			test.Eq(t, tc.ExpectCapacity, vol.Capacity,
				test.Sprint("unexpected capacity"))
			test.Eq(t, tc.ExpectMin, vol.RequestedCapacityMin,
				test.Sprint("unexpected min"))
			test.Eq(t, tc.ExpectMax, vol.RequestedCapacityMax,
				test.Sprint("unexpected max"))
		})
	}

	// a nodeExpandVolume error should fail expandVolume too
	t.Run("node error", func(t *testing.T) {
		expect := "sad node expand"
		fake.NextNodeExpandError = errors.New(expect)
		fake.NextControllerExpandVolumeResponse = &cstructs.ClientCSIControllerExpandVolumeResponse{
			CapacityBytes:         2000,
			NodeExpansionRequired: true,
		}
		err = endpoint.expandVolume(vol, plug, &csi.CapacityRange{
			RequiredBytes: 2000,
		})
		test.ErrorContains(t, err, expect)
	})

}

func TestCSIVolume_nodeExpandVolume(t *testing.T) {
	ci.Parallel(t)

	srv, cleanupSrv := TestServer(t, nil)
	t.Cleanup(cleanupSrv)
	testutil.WaitForLeader(t, srv.RPC)
	t.Log("server started 👍")

	c, fake, _, fakeVolID := testClientWithCSI(t, srv)
	fakeClaim := fakeCSIClaim(c.NodeID())

	endpoint := NewCSIVolumeEndpoint(srv, nil)
	plug, vol, err := endpoint.volAndPluginLookup(structs.DefaultNamespace, fakeVolID)
	must.NoError(t, err)

	// there's not a lot of logic here -- validation has been done prior,
	// in (controller) expandVolume and what preceeds it.
	cases := []struct {
		Name  string
		Error error
	}{
		{
			Name: "ok",
		},
		{
			Name:  "not ok",
			Error: errors.New("test node expand fail"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {

			fake.NextNodeExpandError = tc.Error
			capacity := &csi.CapacityRange{
				RequiredBytes: 10,
				LimitBytes:    10,
			}

			err = endpoint.nodeExpandVolume(vol, plug, capacity)

			if tc.Error == nil {
				test.NoError(t, err)
			} else {
				must.Error(t, err)
				must.ErrorContains(t, err,
					fmt.Sprintf("CSI.NodeExpandVolume error: %s", tc.Error))
			}

			req := fake.LastNodeExpandRequest
			must.NotNil(t, req, must.Sprint("request should have happened"))
			test.Eq(t, fakeVolID, req.VolumeID)
			test.Eq(t, capacity, req.Capacity)
			test.Eq(t, "fake-csi-plugin", req.PluginID)
			test.Eq(t, "fake-csi-external-id", req.ExternalID)
			test.Eq(t, fakeClaim, req.Claim)

		})
	}
}

func TestCSIPluginEndpoint_RegisterViaFingerprint(t *testing.T) {
	ci.Parallel(t)
	srv, _, shutdown := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)

	deleteNodes := state.CreateTestCSIPlugin(srv.fsm.State(), "foo")
	defer deleteNodes()

	state := srv.fsm.State()
	codec := rpcClient(t, srv)

	// Get the plugin back out
	listJob := mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob})
	policy := mock.PluginPolicy("read") + listJob
	getToken := mock.CreatePolicyAndToken(t, state, 1001, "plugin-read", policy)

	req2 := &structs.CSIPluginGetRequest{
		ID: "foo",
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			AuthToken: getToken.SecretID,
		},
	}
	resp2 := &structs.CSIPluginGetResponse{}
	err := msgpackrpc.CallWithCodec(codec, "CSIPlugin.Get", req2, resp2)
	require.NoError(t, err)

	// Get requires plugin-read, not plugin-list
	lPolicy := mock.PluginPolicy("list")
	lTok := mock.CreatePolicyAndToken(t, state, 1003, "plugin-list", lPolicy)
	req2.AuthToken = lTok.SecretID
	err = msgpackrpc.CallWithCodec(codec, "CSIPlugin.Get", req2, resp2)
	require.Error(t, err, "Permission denied")

	// List plugins
	req3 := &structs.CSIPluginListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			AuthToken: getToken.SecretID,
		},
	}
	resp3 := &structs.CSIPluginListResponse{}
	err = msgpackrpc.CallWithCodec(codec, "CSIPlugin.List", req3, resp3)
	require.NoError(t, err)
	require.Equal(t, 1, len(resp3.Plugins))

	// ensure that plugin->alloc denormalization does COW correctly
	err = msgpackrpc.CallWithCodec(codec, "CSIPlugin.List", req3, resp3)
	require.NoError(t, err)
	require.Equal(t, 1, len(resp3.Plugins))

	// List allows plugin-list
	req3.AuthToken = lTok.SecretID
	err = msgpackrpc.CallWithCodec(codec, "CSIPlugin.List", req3, resp3)
	require.NoError(t, err)
	require.Equal(t, 1, len(resp3.Plugins))

	// Deregistration works
	deleteNodes()

	// Plugin is missing
	req2.AuthToken = getToken.SecretID
	err = msgpackrpc.CallWithCodec(codec, "CSIPlugin.Get", req2, resp2)
	require.NoError(t, err)
	require.Nil(t, resp2.Plugin)
}

func TestCSIPluginEndpoint_RegisterViaJob(t *testing.T) {
	ci.Parallel(t)
	srv, shutdown := TestServer(t, nil)
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)

	codec := rpcClient(t, srv)

	// Register a job that creates the plugin
	job := mock.Job()
	job.TaskGroups[0].Tasks[0].CSIPluginConfig = &structs.TaskCSIPluginConfig{
		ID:   "foo",
		Type: structs.CSIPluginTypeNode,
	}

	req1 := &structs.JobRegisterRequest{
		Job:          job,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	resp1 := &structs.JobRegisterResponse{}
	err := msgpackrpc.CallWithCodec(codec, "Job.Register", req1, resp1)
	require.NoError(t, err)

	// Verify that the plugin exists and is unhealthy
	req2 := &structs.CSIPluginGetRequest{
		ID:           "foo",
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	resp2 := &structs.CSIPluginGetResponse{}
	err = msgpackrpc.CallWithCodec(codec, "CSIPlugin.Get", req2, resp2)
	require.NoError(t, err)
	require.NotNil(t, resp2.Plugin)
	require.Zero(t, resp2.Plugin.ControllersHealthy)
	require.Zero(t, resp2.Plugin.NodesHealthy)
	require.Equal(t, job.ID, resp2.Plugin.NodeJobs[structs.DefaultNamespace][job.ID].ID)

	// Health depends on node fingerprints
	deleteNodes := state.CreateTestCSIPlugin(srv.fsm.State(), "foo")
	defer deleteNodes()

	resp2.Plugin = nil
	err = msgpackrpc.CallWithCodec(codec, "CSIPlugin.Get", req2, resp2)
	require.NoError(t, err)
	require.NotNil(t, resp2.Plugin)
	require.NotZero(t, resp2.Plugin.ControllersHealthy)
	require.NotZero(t, resp2.Plugin.NodesHealthy)
	require.Equal(t, job.ID, resp2.Plugin.NodeJobs[structs.DefaultNamespace][job.ID].ID)

	// All fingerprints failing makes the plugin unhealthy, but does not delete it
	deleteNodes()
	err = msgpackrpc.CallWithCodec(codec, "CSIPlugin.Get", req2, resp2)
	require.NoError(t, err)
	require.NotNil(t, resp2.Plugin)
	require.Zero(t, resp2.Plugin.ControllersHealthy)
	require.Zero(t, resp2.Plugin.NodesHealthy)
	require.Equal(t, job.ID, resp2.Plugin.NodeJobs[structs.DefaultNamespace][job.ID].ID)

	// Job deregistration is necessary to gc the plugin
	req3 := &structs.JobDeregisterRequest{
		JobID: job.ID,
		Purge: true,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: structs.DefaultNamespace,
		},
	}
	resp3 := &structs.JobDeregisterResponse{}
	err = msgpackrpc.CallWithCodec(codec, "Job.Deregister", req3, resp3)
	require.NoError(t, err)

	// Plugin has been gc'ed
	resp2.Plugin = nil
	err = msgpackrpc.CallWithCodec(codec, "CSIPlugin.Get", req2, resp2)
	require.NoError(t, err)
	require.Nil(t, resp2.Plugin)
}

func TestCSIPluginEndpoint_DeleteViaGC(t *testing.T) {
	ci.Parallel(t)
	srv, _, shutdown := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)

	deleteNodes := state.CreateTestCSIPlugin(srv.fsm.State(), "foo")
	defer deleteNodes()

	state := srv.fsm.State()
	codec := rpcClient(t, srv)

	// Get the plugin back out
	listJob := mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob})
	policy := mock.PluginPolicy("read") + listJob
	getToken := mock.CreatePolicyAndToken(t, state, 1001, "plugin-read", policy)

	reqGet := &structs.CSIPluginGetRequest{
		ID: "foo",
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			AuthToken: getToken.SecretID,
		},
	}
	respGet := &structs.CSIPluginGetResponse{}
	err := msgpackrpc.CallWithCodec(codec, "CSIPlugin.Get", reqGet, respGet)
	require.NoError(t, err)
	require.NotNil(t, respGet.Plugin)

	// Delete plugin
	reqDel := &structs.CSIPluginDeleteRequest{
		ID: "foo",
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			AuthToken: getToken.SecretID,
		},
	}
	respDel := &structs.CSIPluginDeleteResponse{}

	// Improper permissions
	err = msgpackrpc.CallWithCodec(codec, "CSIPlugin.Delete", reqDel, respDel)
	require.EqualError(t, err, structs.ErrPermissionDenied.Error())

	// Retry with management permissions
	reqDel.AuthToken = srv.getLeaderAcl()
	err = msgpackrpc.CallWithCodec(codec, "CSIPlugin.Delete", reqDel, respDel)
	require.EqualError(t, err, "plugin in use")

	// Plugin was not deleted
	err = msgpackrpc.CallWithCodec(codec, "CSIPlugin.Get", reqGet, respGet)
	require.NoError(t, err)
	require.NotNil(t, respGet.Plugin)

	// Empty the plugin
	plugin := respGet.Plugin.Copy()
	plugin.Controllers = map[string]*structs.CSIInfo{}
	plugin.Nodes = map[string]*structs.CSIInfo{}

	index, _ := state.LatestIndex()
	index++
	err = state.UpsertCSIPlugin(index, plugin)
	require.NoError(t, err)

	// Retry now that it's empty
	err = msgpackrpc.CallWithCodec(codec, "CSIPlugin.Delete", reqDel, respDel)
	require.NoError(t, err)

	// Plugin is deleted
	err = msgpackrpc.CallWithCodec(codec, "CSIPlugin.Get", reqGet, respGet)
	require.NoError(t, err)
	require.Nil(t, respGet.Plugin)

	// Safe to call on already-deleted plugnis
	err = msgpackrpc.CallWithCodec(codec, "CSIPlugin.Delete", reqDel, respDel)
	require.NoError(t, err)
}

func TestCSI_RPCVolumeAndPluginLookup(t *testing.T) {
	ci.Parallel(t)

	srv, shutdown := TestServer(t, func(c *Config) {})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)

	state := srv.fsm.State()
	id0 := uuid.Generate()
	id1 := uuid.Generate()
	id2 := uuid.Generate()

	// Create a client node with a plugin
	node := mock.Node()
	node.CSIControllerPlugins = map[string]*structs.CSIInfo{
		"minnie": {PluginID: "minnie", Healthy: true, RequiresControllerPlugin: true,
			ControllerInfo: &structs.CSIControllerInfo{SupportsAttachDetach: true},
		},
	}
	node.CSINodePlugins = map[string]*structs.CSIInfo{
		"adam": {PluginID: "adam", Healthy: true},
	}
	err := state.UpsertNode(structs.MsgTypeTestSetup, 3, node)
	require.NoError(t, err)

	// Create 2 volumes
	vols := []*structs.CSIVolume{
		{
			ID:                 id0,
			Namespace:          structs.DefaultNamespace,
			PluginID:           "minnie",
			AccessMode:         structs.CSIVolumeAccessModeMultiNodeSingleWriter,
			AttachmentMode:     structs.CSIVolumeAttachmentModeFilesystem,
			ControllerRequired: true,
		},
		{
			ID:                 id1,
			Namespace:          structs.DefaultNamespace,
			PluginID:           "adam",
			AccessMode:         structs.CSIVolumeAccessModeMultiNodeSingleWriter,
			AttachmentMode:     structs.CSIVolumeAttachmentModeFilesystem,
			ControllerRequired: false,
		},
	}
	err = state.UpsertCSIVolume(1002, vols)
	require.NoError(t, err)

	// has controller
	c := NewCSIVolumeEndpoint(srv, nil)
	plugin, vol, err := c.volAndPluginLookup(structs.DefaultNamespace, id0)
	require.NotNil(t, plugin)
	require.NotNil(t, vol)
	require.NoError(t, err)

	// no controller
	plugin, vol, err = c.volAndPluginLookup(structs.DefaultNamespace, id1)
	require.Nil(t, plugin)
	require.NotNil(t, vol)
	require.NoError(t, err)

	// doesn't exist
	plugin, vol, err = c.volAndPluginLookup(structs.DefaultNamespace, id2)
	require.Nil(t, plugin)
	require.Nil(t, vol)
	require.EqualError(t, err, fmt.Sprintf("volume not found: %s", id2))
}

func TestCSI_SerializedControllerRPC(t *testing.T) {
	ci.Parallel(t)

	srv, shutdown := TestServer(t, func(c *Config) { c.NumSchedulers = 0 })
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)

	var wg sync.WaitGroup
	wg.Add(3)

	timeCh := make(chan lang.Pair[string, time.Duration])

	testFn := func(pluginID string, dur time.Duration) {
		defer wg.Done()
		c := NewCSIVolumeEndpoint(srv, nil)
		now := time.Now()
		err := c.serializedControllerRPC(pluginID, func() error {
			time.Sleep(dur)
			return nil
		})
		elapsed := time.Since(now)
		timeCh <- lang.Pair[string, time.Duration]{pluginID, elapsed}
		must.NoError(t, err)
	}

	go testFn("plugin1", 50*time.Millisecond)
	go testFn("plugin2", 50*time.Millisecond)
	go testFn("plugin1", 50*time.Millisecond)

	totals := map[string]time.Duration{}
	for i := 0; i < 3; i++ {
		pair := <-timeCh
		totals[pair.First] += pair.Second
	}

	wg.Wait()

	// plugin1 RPCs should block each other
	must.GreaterEq(t, 150*time.Millisecond, totals["plugin1"])
	must.Less(t, 200*time.Millisecond, totals["plugin1"])

	// plugin1 RPCs should not block plugin2 RPCs
	must.GreaterEq(t, 50*time.Millisecond, totals["plugin2"])
	must.Less(t, 100*time.Millisecond, totals["plugin2"])
}

// testClientWithCSI sets up a client with a fake CSI plugin.
// Much of the plugin/volume configuration is only to pass validation;
// callers should modify MockClientCSI's Next* fields.
func testClientWithCSI(t *testing.T, srv *Server) (c *client.Client, m *MockClientCSI, plugID, volID string) {
	t.Helper()

	m = newMockClientCSI()
	plugID = "fake-csi-plugin"
	volID = "fake-csi-volume"

	c, cleanup := client.TestClientWithRPCs(t,
		func(c *cconfig.Config) {
			c.Servers = []string{srv.config.RPCAddr.String()}
			c.Node.CSIControllerPlugins = map[string]*structs.CSIInfo{
				plugID: {
					PluginID: plugID,
					Healthy:  true,
					ControllerInfo: &structs.CSIControllerInfo{
						// Supports.* everything, but Next* values must be set on the mock.
						SupportsAttachDetach:             true,
						SupportsClone:                    true,
						SupportsCondition:                true,
						SupportsCreateDelete:             true,
						SupportsCreateDeleteSnapshot:     true,
						SupportsExpand:                   true,
						SupportsGet:                      true,
						SupportsGetCapacity:              true,
						SupportsListSnapshots:            true,
						SupportsListVolumes:              true,
						SupportsListVolumesAttachedNodes: true,
						SupportsReadOnlyAttach:           true,
					},
					RequiresControllerPlugin: true,
				},
			}
			c.Node.CSINodePlugins = map[string]*structs.CSIInfo{
				plugID: {
					PluginID: plugID,
					Healthy:  true,
					NodeInfo: &structs.CSINodeInfo{
						ID:                c.Node.GetID(),
						SupportsCondition: true,
						SupportsExpand:    true,
						SupportsStats:     true,
					},
				},
			}
		},
		map[string]interface{}{"CSI": m}, // MockClientCSI
	)
	t.Cleanup(func() { test.NoError(t, cleanup()) })
	testutil.WaitForClient(t, srv.RPC, c.NodeID(), c.Region())
	t.Log("client started with fake CSI plugin 👍")

	// Register a minimum-viable fake volume
	req := &structs.CSIVolumeRegisterRequest{
		Volumes: []*structs.CSIVolume{{
			PluginID:   plugID,
			ID:         volID,
			ExternalID: "fake-csi-external-id",
			Namespace:  structs.DefaultNamespace,
			RequestedCapabilities: []*structs.CSIVolumeCapability{
				{
					AccessMode:     structs.CSIVolumeAccessModeMultiNodeSingleWriter,
					AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
				},
			},
			WriteClaims: map[string]*structs.CSIVolumeClaim{
				"fake-csi-claim": fakeCSIClaim(c.NodeID()),
			},
		}},
		WriteRequest: structs.WriteRequest{Region: srv.Region()},
	}
	must.NoError(t, srv.RPC("CSIVolume.Register", req, &structs.CSIVolumeRegisterResponse{}))
	t.Logf("CSI volume %s registered 👍", volID)

	return c, m, plugID, volID
}

func fakeCSIClaim(nodeID string) *structs.CSIVolumeClaim {
	return &structs.CSIVolumeClaim{
		NodeID:         nodeID,
		AllocationID:   "fake-csi-alloc",
		ExternalNodeID: "fake-csi-external-node",
		Mode:           structs.CSIVolumeClaimWrite,
		AccessMode:     structs.CSIVolumeAccessModeMultiNodeSingleWriter,
		AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
		State:          structs.CSIVolumeClaimStateTaken,
	}
}

// TestCSIPluginEndpoint_ACLNamespaceFilterAlloc checks that plugin allocations
// are filtered by namespace when getting plugins, and enforcing that the client
// has job-read ACL access to the namespace of the plugin allocations
func TestCSIPluginEndpoint_ACLNamespaceFilterAlloc(t *testing.T) {
	ci.Parallel(t)
	srv, _, shutdown := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)
	s := srv.fsm.State()

	ns1 := mock.Namespace()
	must.NoError(t, s.UpsertNamespaces(1000, []*structs.Namespace{ns1}))

	// Setup ACLs
	codec := rpcClient(t, srv)
	listJob := mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob})
	policy := mock.PluginPolicy("read") + listJob
	getToken := mock.CreatePolicyAndToken(t, s, 1001, "plugin-read", policy)

	// Create the plugin and then some allocations to pretend to be the allocs that are
	// running the plugin tasks
	deleteNodes := state.CreateTestCSIPlugin(srv.fsm.State(), "foo")
	defer deleteNodes()

	plug, _ := s.CSIPluginByID(memdb.NewWatchSet(), "foo")
	var allocs []*structs.Allocation
	for _, info := range plug.Controllers {
		a := mock.Alloc()
		a.ID = info.AllocID
		allocs = append(allocs, a)
	}
	for _, info := range plug.Nodes {
		a := mock.Alloc()
		a.ID = info.AllocID
		allocs = append(allocs, a)
	}

	must.Eq(t, 3, len(allocs))
	allocs[0].Namespace = ns1.Name

	err := s.UpsertAllocs(structs.MsgTypeTestSetup, 1003, allocs)
	must.NoError(t, err)

	req := &structs.CSIPluginGetRequest{
		ID: "foo",
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			AuthToken: getToken.SecretID,
		},
	}
	resp := &structs.CSIPluginGetResponse{}
	err = msgpackrpc.CallWithCodec(codec, "CSIPlugin.Get", req, resp)
	must.NoError(t, err)
	must.Eq(t, 2, len(resp.Plugin.Allocations))

	for _, a := range resp.Plugin.Allocations {
		must.Eq(t, structs.DefaultNamespace, a.Namespace)
	}

	p2 := mock.PluginPolicy("read")
	t2 := mock.CreatePolicyAndToken(t, s, 1004, "plugin-read2", p2)
	req.AuthToken = t2.SecretID
	err = msgpackrpc.CallWithCodec(codec, "CSIPlugin.Get", req, resp)
	must.NoError(t, err)
	must.Eq(t, 0, len(resp.Plugin.Allocations))
}
