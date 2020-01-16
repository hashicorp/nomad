package nomad

import (
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
		Driver:         "minnie",
	}}
	err := state.CSIVolumeRegister(0, vols)
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
	require.NotEqual(t, 0, resp.Index)
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
		Driver:         "minnie",
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
	require.NotEqual(t, 0, resp1.Index)

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
	require.NotEqual(t, 0, resp2.Index)
	require.Equal(t, vols[0].ID, resp2.Volume.ID)

	// Registration does not update
	req1.Volumes[0].Driver = "adam"
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
	require.Error(t, err, "missing")
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
		Driver:         "minnie",
	}, {
		ID:             id1,
		Namespace:      ns,
		AccessMode:     structs.CSIVolumeAccessModeMultiNodeSingleWriter,
		AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
		Driver:         "adam",
	}, {
		ID:             id2,
		Namespace:      ms,
		AccessMode:     structs.CSIVolumeAccessModeMultiNodeSingleWriter,
		AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
		Driver:         "paddy",
	}}
	err := state.CSIVolumeRegister(0, vols)
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
	require.NotEqual(t, 0, resp.Index)
	require.Equal(t, 2, len(resp.Volumes))
	ids := map[string]bool{vols[0].ID: true, vols[1].ID: true}
	for _, v := range resp.Volumes {
		delete(ids, v.ID)
	}
	require.Equal(t, 0, len(ids))

	// Query by Driver
	req = &structs.CSIVolumeListRequest{
		Driver: "adam",
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

	// Query by Driver, ACL filters all results
	req = &structs.CSIVolumeListRequest{
		Driver: "paddy",
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
