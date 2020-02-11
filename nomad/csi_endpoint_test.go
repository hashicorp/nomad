package nomad

import (
	"fmt"
	"testing"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
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
		ID:             id0,
		Namespace:      ns,
		AccessMode:     structs.CSIVolumeAccessModeMultiNodeSingleWriter,
		AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
		PluginID:       "minnie",
	}}
	err := state.CSIVolumeRegister(999, vols)
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
	state.BootstrapACLTokens(1, 0, mock.ACLManagementToken())
	srv.config.ACLEnabled = true
	policy := mock.NamespacePolicy(ns, "", []string{acl.NamespaceCapabilityCSIAccess})
	validToken := mock.CreatePolicyAndToken(t, state, 1001, "csi-access", policy)

	codec := rpcClient(t, srv)

	id0 := uuid.Generate()

	// Create the volume
	vols := []*structs.CSIVolume{{
		ID:             id0,
		Namespace:      ns,
		AccessMode:     structs.CSIVolumeAccessModeMultiNodeSingleWriter,
		AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
		PluginID:       "minnie",
	}}
	err := state.CSIVolumeRegister(999, vols)
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
	state.BootstrapACLTokens(1, 0, mock.ACLManagementToken())
	srv.config.ACLEnabled = true
	policy := mock.NamespacePolicy(ns, "", []string{acl.NamespaceCapabilityCSICreateVolume})
	validToken := mock.CreatePolicyAndToken(t, state, 1001, acl.NamespaceCapabilityCSICreateVolume, policy)

	codec := rpcClient(t, srv)

	id0 := uuid.Generate()

	// Create the volume
	vols := []*structs.CSIVolume{{
		ID:             id0,
		Namespace:      "notTheNamespace",
		PluginID:       "minnie",
		AccessMode:     structs.CSIVolumeAccessModeMultiNodeReader,
		AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
		Topologies: []*structs.CSITopology{{
			Segments: map[string]string{"foo": "bar"},
		}},
	}}

	// Create the register request
	req1 := &structs.CSIVolumeRegisterRequest{
		Volumes: vols,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: ns,
			AuthToken: validToken.SecretID,
		},
	}
	resp1 := &structs.CSIVolumeRegisterResponse{}
	err := msgpackrpc.CallWithCodec(codec, "CSIVolume.Register", req1, resp1)
	require.NoError(t, err)
	require.NotEqual(t, uint64(0), resp1.Index)

	// Get the volume back out
	policy = mock.NamespacePolicy(ns, "", []string{acl.NamespaceCapabilityCSIAccess})
	getToken := mock.CreatePolicyAndToken(t, state, 1001, "csi-access", policy)

	req2 := &structs.CSIVolumeGetRequest{
		ID: id0,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			AuthToken: getToken.SecretID,
		},
	}
	resp2 := &structs.CSIVolumeGetResponse{}
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Get", req2, resp2)
	require.NoError(t, err)
	require.Equal(t, resp1.Index, resp2.Index)
	require.Equal(t, vols[0].ID, resp2.Volume.ID)

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
			AuthToken: validToken.SecretID,
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

	ns := structs.DefaultNamespace
	state := srv.fsm.State()
	state.BootstrapACLTokens(1, 0, mock.ACLManagementToken())
	srv.config.ACLEnabled = true
	policy := mock.NamespacePolicy(ns, "",
		[]string{acl.NamespaceCapabilityCSICreateVolume, acl.NamespaceCapabilityCSIAccess})
	accessToken := mock.CreatePolicyAndToken(t, state, 1001,
		acl.NamespaceCapabilityCSIAccess, policy)
	codec := rpcClient(t, srv)
	id0 := uuid.Generate()

	// Create an initial volume claim request; we expect it to fail
	// because there's no such volume yet.
	claimReq := &structs.CSIVolumeClaimRequest{
		VolumeID:   id0,
		Allocation: mock.BatchAlloc(),
		Claim:      structs.CSIVolumeClaimWrite,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: ns,
			AuthToken: accessToken.SecretID,
		},
	}
	claimResp := &structs.CSIVolumeClaimResponse{}
	err := msgpackrpc.CallWithCodec(codec, "CSIVolume.Claim", claimReq, claimResp)
	require.EqualError(t, err, fmt.Sprintf("volume not found: %s", id0),
		"expected 'volume not found' error because volume hasn't yet been created")

	// Create a client nodes with a plugin
	node := mock.Node()
	node.CSINodePlugins = map[string]*structs.CSIInfo{
		"minnie": {PluginID: "minnie",
			Healthy:  true,
			NodeInfo: &structs.CSINodeInfo{},
		},
	}
	plugin := structs.NewCSIPlugin("minnie", 1)
	plugin.ControllerRequired = false
	plugin.AddPlugin(node.ID, &structs.CSIInfo{})
	err = state.UpsertNode(3, node)
	require.NoError(t, err)

	// Create the volume for the plugin
	vols := []*structs.CSIVolume{{
		ID:             id0,
		Namespace:      "notTheNamespace",
		PluginID:       "minnie",
		AccessMode:     structs.CSIVolumeAccessModeMultiNodeSingleWriter,
		AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
		Topologies: []*structs.CSITopology{{
			Segments: map[string]string{"foo": "bar"},
		}},
	}}

	createToken := mock.CreatePolicyAndToken(t, state, 1001,
		acl.NamespaceCapabilityCSICreateVolume, policy)
	// Register the volume
	volReq := &structs.CSIVolumeRegisterRequest{
		Volumes: vols,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: ns,
			AuthToken: createToken.SecretID,
		},
	}
	volResp := &structs.CSIVolumeRegisterResponse{}
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Register", volReq, volResp)
	require.NoError(t, err)

	// Verify we can get the volume back out
	getToken := mock.CreatePolicyAndToken(t, state, 1001,
		acl.NamespaceCapabilityCSIAccess, policy)
	volGetReq := &structs.CSIVolumeGetRequest{
		ID: id0,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			AuthToken: getToken.SecretID,
		},
	}
	volGetResp := &structs.CSIVolumeGetResponse{}
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Get", volGetReq, volGetResp)
	require.NoError(t, err)
	require.Equal(t, id0, volGetResp.Volume.ID)

	// Now our claim should succeed
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Claim", claimReq, claimResp)
	require.NoError(t, err)

	// Verify the claim was set
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Get", volGetReq, volGetResp)
	require.NoError(t, err)
	require.Equal(t, id0, volGetResp.Volume.ID)
	require.Len(t, volGetResp.Volume.ReadAllocs, 0)
	require.Len(t, volGetResp.Volume.WriteAllocs, 1)

	// Make another writer claim for a different alloc
	claimReq.Allocation = mock.BatchAlloc()
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Claim", claimReq, claimResp)
	require.EqualError(t, err, "volume max claim reached",
		"expected 'volume max claim reached' because we only allow 1 writer")

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
}

func TestCSIVolumeEndpoint_List(t *testing.T) {
	t.Parallel()
	srv, shutdown := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)

	ns := structs.DefaultNamespace
	ms := "altNamespace"

	state := srv.fsm.State()
	state.BootstrapACLTokens(1, 0, mock.ACLManagementToken())
	srv.config.ACLEnabled = true

	policy := mock.NamespacePolicy(ns, "", []string{acl.NamespaceCapabilityCSIAccess})
	nsTok := mock.CreatePolicyAndToken(t, state, 1001, "csi-access", policy)
	codec := rpcClient(t, srv)

	id0 := uuid.Generate()
	id1 := uuid.Generate()
	id2 := uuid.Generate()

	// Create the volume
	vols := []*structs.CSIVolume{{
		ID:             id0,
		Namespace:      ns,
		AccessMode:     structs.CSIVolumeAccessModeMultiNodeReader,
		AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
		PluginID:       "minnie",
	}, {
		ID:             id1,
		Namespace:      ns,
		AccessMode:     structs.CSIVolumeAccessModeMultiNodeSingleWriter,
		AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
		PluginID:       "adam",
	}, {
		ID:             id2,
		Namespace:      ms,
		AccessMode:     structs.CSIVolumeAccessModeMultiNodeSingleWriter,
		AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
		PluginID:       "paddy",
	}}
	err := state.CSIVolumeRegister(999, vols)
	require.NoError(t, err)

	var resp structs.CSIVolumeListResponse

	// Query all, ACL only allows ns
	req := &structs.CSIVolumeListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			AuthToken: nsTok.SecretID,
		},
	}
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.List", req, &resp)
	require.NoError(t, err)
	require.Equal(t, uint64(999), resp.Index)
	require.Equal(t, 2, len(resp.Volumes))
	ids := map[string]bool{vols[0].ID: true, vols[1].ID: true}
	for _, v := range resp.Volumes {
		delete(ids, v.ID)
	}
	require.Equal(t, 0, len(ids))

	// Query by PluginID
	req = &structs.CSIVolumeListRequest{
		PluginID: "adam",
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: ns,
			AuthToken: nsTok.SecretID,
		},
	}
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.List", req, &resp)
	require.NoError(t, err)
	require.Equal(t, 1, len(resp.Volumes))
	require.Equal(t, vols[1].ID, resp.Volumes[0].ID)

	// Query by PluginID, ACL filters all results
	req = &structs.CSIVolumeListRequest{
		PluginID: "paddy",
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: ms,
			AuthToken: nsTok.SecretID,
		},
	}
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.List", req, &resp)
	require.NoError(t, err)
	require.Equal(t, 0, len(resp.Volumes))
}

func TestCSIPluginEndpoint_RegisterViaJob(t *testing.T) {
	t.Parallel()
	srv, shutdown := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)

	ns := structs.DefaultNamespace

	job := mock.Job()
	job.TaskGroups[0].Tasks[0].CSIPluginConfig = &structs.TaskCSIPluginConfig{
		ID:       "foo",
		Type:     structs.CSIPluginTypeMonolith,
		MountDir: "non-empty",
	}

	state := srv.fsm.State()
	state.BootstrapACLTokens(1, 0, mock.ACLManagementToken())
	srv.config.ACLEnabled = true
	policy := mock.NamespacePolicy(ns, "", []string{
		acl.NamespaceCapabilityCSICreateVolume,
		acl.NamespaceCapabilitySubmitJob,
	})
	validToken := mock.CreatePolicyAndToken(t, state, 1001, acl.NamespaceCapabilityCSICreateVolume, policy)

	codec := rpcClient(t, srv)

	// Create the register request
	req1 := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: ns,
			AuthToken: validToken.SecretID,
		},
	}
	resp1 := &structs.JobRegisterResponse{}
	err := msgpackrpc.CallWithCodec(codec, "Job.Register", req1, resp1)
	require.NoError(t, err)
	require.NotEqual(t, uint64(0), resp1.Index)

	// Get the plugin back out
	policy = mock.NamespacePolicy(ns, "", []string{acl.NamespaceCapabilityCSIAccess})
	getToken := mock.CreatePolicyAndToken(t, state, 1001, "csi-access", policy)

	req2 := &structs.CSIPluginGetRequest{
		ID: "foo",
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			AuthToken: getToken.SecretID,
		},
	}
	resp2 := &structs.CSIPluginGetResponse{}
	err = msgpackrpc.CallWithCodec(codec, "CSIPlugin.Get", req2, resp2)
	require.NoError(t, err)
	// The job is created with a higher index than the plugin, there's an extra raft write
	require.Greater(t, resp1.Index, resp2.Index)

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

	// Deregistration works
	req4 := &structs.JobDeregisterRequest{
		JobID: job.ID,
		Purge: true,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: ns,
			AuthToken: validToken.SecretID,
		},
	}
	resp4 := &structs.JobDeregisterResponse{}
	err = msgpackrpc.CallWithCodec(codec, "Job.Deregister", req4, resp4)
	require.NoError(t, err)
	require.Less(t, resp2.Index, resp4.Index)

	// Plugin is missing
	err = msgpackrpc.CallWithCodec(codec, "CSIPlugin.Get", req2, resp2)
	require.NoError(t, err)
	require.Nil(t, resp2.Plugin)
}
