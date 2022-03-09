package nomad

import (
	"fmt"
	"strings"
	"testing"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/client"
	cconfig "github.com/hashicorp/nomad/client/config"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func TestCSIVolumeEndpoint_Get(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	srv, shutdown := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)

	ns := structs.DefaultNamespace

	state := srv.fsm.State()
	state.BootstrapACLTokens(structs.MsgTypeTestSetup, 1, 0, mock.ACLManagementToken())
	srv.config.ACLEnabled = true
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

func TestCSIVolumeEndpoint_Register(t *testing.T) {
	t.Parallel()
	srv, shutdown := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)

	ns := structs.DefaultNamespace

	state := srv.fsm.State()
	codec := rpcClient(t, srv)

	id0 := uuid.Generate()

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
	require.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 1000, node))

	// Create the volume
	vols := []*structs.CSIVolume{{
		ID:             id0,
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
			Namespace: ns,
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
			Region: "global",
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
			Namespace: ns,
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
	t.Parallel()
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
	require.EqualError(t, err, fmt.Sprintf("controller publish: volume not found: %s", id0),
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
	t.Parallel()
	srv, shutdown := TestServer(t, func(c *Config) {
		c.ACLEnabled = true
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)

	ns := structs.DefaultNamespace
	state := srv.fsm.State()
	state.BootstrapACLTokens(structs.MsgTypeTestSetup, 1, 0, mock.ACLManagementToken())

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
	require.EqualError(t, err, "controller publish: attach volume: controller attach volume: No path to node")

	// The node SecretID is authorized for all policies
	claimReq.AuthToken = node.SecretID
	claimReq.Namespace = ""
	claimResp = &structs.CSIVolumeClaimResponse{}
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Claim", claimReq, claimResp)
	require.EqualError(t, err, "controller publish: attach volume: controller attach volume: No path to node")
}

func TestCSIVolumeEndpoint_Unpublish(t *testing.T) {
	t.Parallel()
	srv, shutdown := TestServer(t, func(c *Config) { c.NumSchedulers = 0 })
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)

	var err error
	index := uint64(1000)
	ns := structs.DefaultNamespace
	state := srv.fsm.State()
	state.BootstrapACLTokens(structs.MsgTypeTestSetup, 1, 0, mock.ACLManagementToken())

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
	require.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, index, node))

	type tc struct {
		name           string
		startingState  structs.CSIVolumeClaimState
		expectedErrMsg string
	}
	testCases := []tc{
		{
			name:          "success",
			startingState: structs.CSIVolumeClaimStateControllerDetached,
		},
		{
			name:           "unpublish previously detached node",
			startingState:  structs.CSIVolumeClaimStateNodeDetached,
			expectedErrMsg: "could not detach from controller: controller detach volume: No path to node",
		},
		{
			name:           "first unpublish",
			startingState:  structs.CSIVolumeClaimStateTaken,
			expectedErrMsg: "could not detach from node: No path to node",
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
			require.NoError(t, err)

			// setup: create an alloc that will claim our volume
			alloc := mock.BatchAlloc()
			alloc.NodeID = node.ID
			alloc.ClientStatus = structs.AllocClientStatusFailed

			index++
			require.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, index, []*structs.Allocation{alloc}))

			// setup: claim the volume for our alloc
			claim := &structs.CSIVolumeClaim{
				AllocationID:   alloc.ID,
				NodeID:         node.ID,
				ExternalNodeID: "i-example",
				Mode:           structs.CSIVolumeClaimRead,
			}

			index++
			claim.State = structs.CSIVolumeClaimStateTaken
			err = state.CSIVolumeClaim(index, ns, volID, claim)
			require.NoError(t, err)

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

			err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Unpublish", req,
				&structs.CSIVolumeUnpublishResponse{})

			if tc.expectedErrMsg == "" {
				require.NoError(t, err)
				vol, err = state.CSIVolumeByID(nil, ns, volID)
				require.NoError(t, err)
				require.NotNil(t, vol)
				require.Len(t, vol.ReadAllocs, 0)
			} else {
				require.Error(t, err)
				require.True(t, strings.Contains(err.Error(), tc.expectedErrMsg),
					"error message %q did not contain %q", err.Error(), tc.expectedErrMsg)
			}
		})
	}

}

func TestCSIVolumeEndpoint_List(t *testing.T) {
	t.Parallel()
	srv, shutdown := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)

	state := srv.fsm.State()
	state.BootstrapACLTokens(structs.MsgTypeTestSetup, 1, 0, mock.ACLManagementToken())
	srv.config.ACLEnabled = true
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
	t.Parallel()
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

func TestCSIVolumeEndpoint_Create(t *testing.T) {
	t.Parallel()
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

	node := client.Node()
	node.Attributes["nomad.version"] = "0.11.0" // client RPCs not supported on early versions

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

	node.CSIControllerPlugins = map[string]*structs.CSIInfo{
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
	node.CSINodePlugins = map[string]*structs.CSIInfo{
		"minnie": {
			PluginID: "minnie",
			Healthy:  true,
			NodeInfo: &structs.CSINodeInfo{},
		},
	}
	index++
	require.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, index, node))

	// Create the volume
	volID := uuid.Generate()
	vols := []*structs.CSIVolume{{
		ID:             volID,
		Name:           "vol",
		Namespace:      "notTheNamespace", // overriden by WriteRequest
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
	t.Parallel()
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

	node := client.Node()
	node.Attributes["nomad.version"] = "0.11.0" // client RPCs not supported on early versions

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
	index++
	require.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, index, node))

	volID := uuid.Generate()
	vols := []*structs.CSIVolume{{
		ID:        volID,
		Namespace: structs.DefaultNamespace,
		PluginID:  "minnie",
		Secrets:   structs.CSISecrets{"mysecret": "secretvalue"},
	}}
	index++
	err = state.UpsertCSIVolume(index, vols)
	require.NoError(t, err)

	// Delete volumes

	// Create an invalid delete request, ensure it doesn't hit the plugin
	req1 := &structs.CSIVolumeDeleteRequest{
		VolumeIDs: []string{"bad", volID},
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: ns,
		},
	}
	resp1 := &structs.CSIVolumeCreateResponse{}
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Delete", req1, resp1)
	require.EqualError(t, err, "volume not found: bad")

	// Make sure the valid volume wasn't deleted
	req2 := &structs.CSIVolumeGetRequest{
		ID: volID,
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}
	resp2 := &structs.CSIVolumeGetResponse{}
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Get", req2, resp2)
	require.NoError(t, err)
	require.NotNil(t, resp2.Volume)

	// Fix the delete request
	fake.NextDeleteError = nil
	req1.VolumeIDs = []string{volID}
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Delete", req1, resp1)
	require.NoError(t, err)

	// Make sure it was deregistered
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Get", req2, resp2)
	require.NoError(t, err)
	require.Nil(t, resp2.Volume)
}

func TestCSIVolumeEndpoint_ListExternal(t *testing.T) {
	t.Parallel()
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

	node := client.Node()
	node.Attributes["nomad.version"] = "0.11.0" // client RPCs not supported on early versions

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

	node.CSIControllerPlugins = map[string]*structs.CSIInfo{
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
	node.CSINodePlugins = map[string]*structs.CSIInfo{
		"minnie": {
			PluginID: "minnie",
			Healthy:  true,
			NodeInfo: &structs.CSINodeInfo{},
		},
	}
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
	t.Parallel()
	var err error
	srv, shutdown := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()

	testutil.WaitForLeader(t, srv.RPC)

	fake := newMockClientCSI()
	fake.NextCreateSnapshotError = nil
	fake.NextCreateSnapshotResponse = &cstructs.ClientCSIControllerCreateSnapshotResponse{
		ID:                     "snap-12345",
		ExternalSourceVolumeID: "vol-12345",
		SizeBytes:              42,
		IsReady:                true,
	}

	client, cleanup := client.TestClientWithRPCs(t,
		func(c *cconfig.Config) {
			c.Servers = []string{srv.config.RPCAddr.String()}
		},
		map[string]interface{}{"CSI": fake},
	)
	defer cleanup()

	node := client.Node()

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

	node.CSIControllerPlugins = map[string]*structs.CSIInfo{
		"minnie": {
			PluginID: "minnie",
			Healthy:  true,
			ControllerInfo: &structs.CSIControllerInfo{
				SupportsCreateDeleteSnapshot: true,
			},
			RequiresControllerPlugin: true,
		},
	}
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
	t.Parallel()
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

	node := client.Node()

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

	node.CSIControllerPlugins = map[string]*structs.CSIInfo{
		"minnie": {
			PluginID: "minnie",
			Healthy:  true,
			ControllerInfo: &structs.CSIControllerInfo{
				SupportsCreateDeleteSnapshot: true,
			},
			RequiresControllerPlugin: true,
		},
	}
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
	t.Parallel()
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

	node := client.Node()
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

	node.CSIControllerPlugins = map[string]*structs.CSIInfo{
		"minnie": {
			PluginID: "minnie",
			Healthy:  true,
			ControllerInfo: &structs.CSIControllerInfo{
				SupportsListSnapshots: true,
			},
			RequiresControllerPlugin: true,
		},
	}
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

func TestCSIPluginEndpoint_RegisterViaFingerprint(t *testing.T) {
	t.Parallel()
	srv, shutdown := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)

	deleteNodes := state.CreateTestCSIPlugin(srv.fsm.State(), "foo")
	defer deleteNodes()

	state := srv.fsm.State()
	state.BootstrapACLTokens(structs.MsgTypeTestSetup, 1, 0, mock.ACLManagementToken())
	srv.config.ACLEnabled = true
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
	t.Parallel()
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
	t.Parallel()
	srv, shutdown := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)

	deleteNodes := state.CreateTestCSIPlugin(srv.fsm.State(), "foo")
	defer deleteNodes()

	state := srv.fsm.State()
	state.BootstrapACLTokens(structs.MsgTypeTestSetup, 1, 0, mock.ACLManagementToken())
	srv.config.ACLEnabled = true
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
	c := srv.staticEndpoints.CSIVolume
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
