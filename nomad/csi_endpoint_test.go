package nomad

import (
	"fmt"
	"testing"

	memdb "github.com/hashicorp/go-memdb"
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
	require.NoError(t, state.UpsertNode(1000, node))

	// Create the volume
	vols := []*structs.CSIVolume{{
		ID:             id0,
		Namespace:      "notTheNamespace",
		PluginID:       "minnie",
		AccessMode:     structs.CSIVolumeAccessModeMultiNodeReader,
		AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
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

	ns := structs.DefaultNamespace
	state := srv.fsm.State()
	codec := rpcClient(t, srv)
	id0 := uuid.Generate()
	alloc := mock.BatchAlloc()

	// Create an initial volume claim request; we expect it to fail
	// because there's no such volume yet.
	claimReq := &structs.CSIVolumeClaimRequest{
		VolumeID:     id0,
		AllocationID: alloc.ID,
		Claim:        structs.CSIVolumeClaimWrite,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: ns,
		},
	}
	claimResp := &structs.CSIVolumeClaimResponse{}
	err := msgpackrpc.CallWithCodec(codec, "CSIVolume.Claim", claimReq, claimResp)
	require.EqualError(t, err, fmt.Sprintf("volume not found: %s", id0),
		"expected 'volume not found' error because volume hasn't yet been created")

	// Create a client node, plugin, alloc, and volume
	node := mock.Node()
	node.CSINodePlugins = map[string]*structs.CSIInfo{
		"minnie": {
			PluginID: "minnie",
			Healthy:  true,
			NodeInfo: &structs.CSINodeInfo{},
		},
	}
	err = state.UpsertNode(1002, node)
	require.NoError(t, err)

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
	err = state.CSIVolumeRegister(1003, vols)
	require.NoError(t, err)

	// Upsert the job and alloc
	alloc.NodeID = node.ID
	summary := mock.JobSummary(alloc.JobID)
	require.NoError(t, state.UpsertJobSummary(1004, summary))
	require.NoError(t, state.UpsertAllocs(1005, []*structs.Allocation{alloc}))

	// Now our claim should succeed
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Claim", claimReq, claimResp)
	require.NoError(t, err)

	// Verify the claim was set
	volGetReq := &structs.CSIVolumeGetRequest{
		ID: id0,
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}
	volGetResp := &structs.CSIVolumeGetResponse{}
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Get", volGetReq, volGetResp)
	require.NoError(t, err)
	require.Equal(t, id0, volGetResp.Volume.ID)
	require.Len(t, volGetResp.Volume.ReadAllocs, 0)
	require.Len(t, volGetResp.Volume.WriteAllocs, 1)

	// Make another writer claim for a different alloc
	alloc2 := mock.Alloc()
	summary = mock.JobSummary(alloc2.JobID)
	require.NoError(t, state.UpsertJobSummary(1005, summary))
	require.NoError(t, state.UpsertAllocs(1006, []*structs.Allocation{alloc2}))
	claimReq.AllocationID = alloc2.ID
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
	state.BootstrapACLTokens(1, 0, mock.ACLManagementToken())
	policy := mock.NamespacePolicy(ns, "",
		[]string{acl.NamespaceCapabilityCSICreateVolume, acl.NamespaceCapabilityCSIAccess})
	accessToken := mock.CreatePolicyAndToken(t, state, 1001,
		acl.NamespaceCapabilityCSIAccess, policy)
	codec := rpcClient(t, srv)
	id0 := uuid.Generate()

	// Create a client node, plugin, alloc, and volume
	node := mock.Node()
	node.Attributes["nomad.version"] = "0.11.0" // client RPCs not supported on early version
	node.CSIControllerPlugins = map[string]*structs.CSIInfo{
		"minnie": {PluginID: "minnie",
			Healthy:                  true,
			ControllerInfo:           &structs.CSIControllerInfo{},
			NodeInfo:                 &structs.CSINodeInfo{},
			RequiresControllerPlugin: true,
		},
	}
	node.CSINodePlugins = map[string]*structs.CSIInfo{
		"minnie": {PluginID: "minnie",
			Healthy:                  true,
			ControllerInfo:           &structs.CSIControllerInfo{},
			NodeInfo:                 &structs.CSINodeInfo{},
			RequiresControllerPlugin: true,
		},
	}
	err := state.UpsertNode(1002, node)
	require.NoError(t, err)
	vols := []*structs.CSIVolume{{
		ID:                 id0,
		Namespace:          "notTheNamespace",
		PluginID:           "minnie",
		ControllerRequired: true,
		AccessMode:         structs.CSIVolumeAccessModeMultiNodeSingleWriter,
		AttachmentMode:     structs.CSIVolumeAttachmentModeFilesystem,
	}}
	err = state.CSIVolumeRegister(1003, vols)

	alloc := mock.BatchAlloc()
	alloc.NodeID = node.ID
	summary := mock.JobSummary(alloc.JobID)
	require.NoError(t, state.UpsertJobSummary(1004, summary))
	require.NoError(t, state.UpsertAllocs(1005, []*structs.Allocation{alloc}))

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
	require.EqualError(t, err, "No path to node")
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

func TestCSIPluginEndpoint_RegisterViaFingerprint(t *testing.T) {
	t.Parallel()
	srv, shutdown := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)

	ns := structs.DefaultNamespace

	deleteNodes := CreateTestCSIPlugin(srv.fsm.State(), "foo")
	defer deleteNodes()

	state := srv.fsm.State()
	state.BootstrapACLTokens(1, 0, mock.ACLManagementToken())
	srv.config.ACLEnabled = true
	codec := rpcClient(t, srv)

	// Get the plugin back out
	policy := mock.NamespacePolicy(ns, "", []string{acl.NamespaceCapabilityCSIAccess})
	getToken := mock.CreatePolicyAndToken(t, state, 1001, "csi-access", policy)

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
	deleteNodes()

	// Plugin is missing
	err = msgpackrpc.CallWithCodec(codec, "CSIPlugin.Get", req2, resp2)
	require.NoError(t, err)
	require.Nil(t, resp2.Plugin)
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
	node.CSINodePlugins = map[string]*structs.CSIInfo{
		"minnie": {PluginID: "minnie", Healthy: true, RequiresControllerPlugin: true},
		"adam":   {PluginID: "adam", Healthy: true},
	}
	err := state.UpsertNode(3, node)
	require.NoError(t, err)

	// Create 2 volumes
	vols := []*structs.CSIVolume{
		{
			ID:                 id0,
			Namespace:          "notTheNamespace",
			PluginID:           "minnie",
			AccessMode:         structs.CSIVolumeAccessModeMultiNodeSingleWriter,
			AttachmentMode:     structs.CSIVolumeAttachmentModeFilesystem,
			ControllerRequired: true,
		},
		{
			ID:                 id1,
			Namespace:          "notTheNamespace",
			PluginID:           "adam",
			AccessMode:         structs.CSIVolumeAccessModeMultiNodeSingleWriter,
			AttachmentMode:     structs.CSIVolumeAttachmentModeFilesystem,
			ControllerRequired: false,
		},
	}
	err = state.CSIVolumeRegister(1002, vols)
	require.NoError(t, err)

	// has controller
	plugin, vol, err := srv.volAndPluginLookup(id0)
	require.NotNil(t, plugin)
	require.NotNil(t, vol)
	require.NoError(t, err)

	// no controller
	plugin, vol, err = srv.volAndPluginLookup(id1)
	require.Nil(t, plugin)
	require.NotNil(t, vol)
	require.NoError(t, err)

	// doesn't exist
	plugin, vol, err = srv.volAndPluginLookup(id2)
	require.Nil(t, plugin)
	require.Nil(t, vol)
	require.EqualError(t, err, fmt.Sprintf("volume not found: %s", id2))
}

func TestCSI_NodeForControllerPlugin(t *testing.T) {
	t.Parallel()
	srv, shutdown := TestServer(t, func(c *Config) {})
	testutil.WaitForLeader(t, srv.RPC)
	defer shutdown()

	plugins := map[string]*structs.CSIInfo{
		"minnie": {PluginID: "minnie",
			Healthy:                  true,
			ControllerInfo:           &structs.CSIControllerInfo{},
			NodeInfo:                 &structs.CSINodeInfo{},
			RequiresControllerPlugin: true,
		},
	}
	state := srv.fsm.State()

	node1 := mock.Node()
	node1.Attributes["nomad.version"] = "0.11.0" // client RPCs not supported on early versions
	node1.CSIControllerPlugins = plugins
	node2 := mock.Node()
	node2.CSIControllerPlugins = plugins
	node2.ID = uuid.Generate()
	node3 := mock.Node()
	node3.ID = uuid.Generate()

	err := state.UpsertNode(1002, node1)
	require.NoError(t, err)
	err = state.UpsertNode(1003, node2)
	require.NoError(t, err)
	err = state.UpsertNode(1004, node3)
	require.NoError(t, err)

	ws := memdb.NewWatchSet()

	plugin, err := state.CSIPluginByID(ws, "minnie")
	require.NoError(t, err)
	nodeID, err := srv.nodeForControllerPlugin(plugin)

	// only node1 has both the controller and a recent Nomad version
	require.Equal(t, nodeID, node1.ID)
}
