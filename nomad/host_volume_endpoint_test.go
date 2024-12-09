// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc/v2"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client"
	"github.com/hashicorp/nomad/client/config"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/hashicorp/nomad/version"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

func TestHostVolumeEndpoint_CreateRegisterGetDelete(t *testing.T) {
	ci.Parallel(t)

	srv, _, cleanupSrv := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	t.Cleanup(cleanupSrv)
	testutil.WaitForLeader(t, srv.RPC)
	store := srv.fsm.State()

	c1, node1 := newMockHostVolumeClient(t, srv, "prod")
	c2, _ := newMockHostVolumeClient(t, srv, "default")
	c2.setCreate(nil, errors.New("this node should never receive create RPC"))
	c2.setDelete("this node should never receive delete RPC")

	index := uint64(1001)

	token := mock.CreatePolicyAndToken(t, store, index, "volume-manager",
		`namespace "apps" { capabilities = ["host-volume-register"] }
         node { policy = "read" }`).SecretID

	index++
	otherToken := mock.CreatePolicyAndToken(t, store, index, "other",
		`namespace "foo" { capabilities = ["host-volume-register"] }
         node { policy = "read" }`).SecretID

	index++
	powerToken := mock.CreatePolicyAndToken(t, store, index, "cluster-admin",
		`namespace "*" { capabilities = ["host-volume-write"] }
         node { policy = "read" }`).SecretID

	index++
	ns := "apps"
	nspace := mock.Namespace()
	nspace.Name = ns
	must.NoError(t, store.UpsertNamespaces(index, []*structs.Namespace{nspace}))

	codec := rpcClient(t, srv)

	req := &structs.HostVolumeCreateRequest{
		Volumes: []*structs.HostVolume{},
		WriteRequest: structs.WriteRequest{
			Region:    srv.Region(),
			AuthToken: token},
	}

	t.Run("invalid create", func(t *testing.T) {

		// TODO(1.10.0): once validation logic for updating an existing volume is in
		// place, fully test it here

		req.Namespace = ns
		var resp structs.HostVolumeCreateResponse
		err := msgpackrpc.CallWithCodec(codec, "HostVolume.Create", req, &resp)
		must.EqError(t, err, "missing volume definition")
	})

	var vol1ID, vol2ID string
	var expectIndex uint64

	c1.setCreate(&cstructs.ClientHostVolumeCreateResponse{
		HostPath:      "/var/nomad/alloc_mounts/foo",
		CapacityBytes: 150000,
	}, nil)

	vol1 := mock.HostVolumeRequest()
	vol1.Namespace = "apps"
	vol1.Name = "example1"
	vol1.NodePool = "prod"
	vol2 := mock.HostVolumeRequest()
	vol2.Namespace = "apps"
	vol2.Name = "example2"
	vol2.NodePool = "prod"
	req.Volumes = []*structs.HostVolume{vol1, vol2}

	t.Run("invalid permissions", func(t *testing.T) {
		var resp structs.HostVolumeCreateResponse
		req.AuthToken = otherToken
		err := msgpackrpc.CallWithCodec(codec, "HostVolume.Create", req, &resp)
		must.EqError(t, err, "Permission denied")
	})

	t.Run("valid create", func(t *testing.T) {
		var resp structs.HostVolumeCreateResponse
		req.AuthToken = token
		err := msgpackrpc.CallWithCodec(codec, "HostVolume.Create", req, &resp)
		must.NoError(t, err)
		must.Len(t, 2, resp.Volumes)
		vol1ID = resp.Volumes[0].ID
		vol2ID = resp.Volumes[1].ID
		expectIndex = resp.Index

		getReq := &structs.HostVolumeGetRequest{
			ID: vol1ID,
			QueryOptions: structs.QueryOptions{
				Region:    srv.Region(),
				Namespace: ns,
				AuthToken: otherToken,
			},
		}
		var getResp structs.HostVolumeGetResponse
		err = msgpackrpc.CallWithCodec(codec, "HostVolume.Get", getReq, &getResp)
		must.EqError(t, err, "Permission denied")

		getReq.AuthToken = token
		err = msgpackrpc.CallWithCodec(codec, "HostVolume.Get", getReq, &getResp)
		must.NoError(t, err)
		must.NotNil(t, getResp.Volume)
	})

	t.Run("blocking Get unblocks on write", func(t *testing.T) {
		vol1, err := store.HostVolumeByID(nil, ns, vol1ID, false)
		must.NoError(t, err)
		must.NotNil(t, vol1)
		nextVol1 := vol1.Copy()
		nextVol1.RequestedCapacityMaxBytes = 300000
		registerReq := &structs.HostVolumeRegisterRequest{
			Volumes: []*structs.HostVolume{nextVol1},
			WriteRequest: structs.WriteRequest{
				Region:    srv.Region(),
				Namespace: ns,
				AuthToken: token},
		}

		c1.setCreate(nil, errors.New("should not call this endpoint on register RPC"))

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		t.Cleanup(cancel)
		volCh := make(chan *structs.HostVolume)
		errCh := make(chan error)

		getReq := &structs.HostVolumeGetRequest{
			ID: vol1ID,
			QueryOptions: structs.QueryOptions{
				Region:        srv.Region(),
				Namespace:     ns,
				AuthToken:     token,
				MinQueryIndex: expectIndex,
			},
		}

		go func() {
			codec := rpcClient(t, srv)
			var getResp structs.HostVolumeGetResponse
			err := msgpackrpc.CallWithCodec(codec, "HostVolume.Get", getReq, &getResp)
			if err != nil {
				errCh <- err
			}
			volCh <- getResp.Volume
		}()

		var registerResp structs.HostVolumeRegisterResponse
		err = msgpackrpc.CallWithCodec(codec, "HostVolume.Register", registerReq, &registerResp)
		must.NoError(t, err)

		select {
		case <-ctx.Done():
			t.Fatal("timeout or cancelled")
		case vol := <-volCh:
			must.Greater(t, expectIndex, vol.ModifyIndex)
		case err := <-errCh:
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("delete blocked by allocation claims", func(t *testing.T) {
		vol2, err := store.HostVolumeByID(nil, ns, vol2ID, false)
		must.NoError(t, err)
		must.NotNil(t, vol2)

		// claim one of the volumes with a pending allocation
		alloc := mock.MinAlloc()
		alloc.NodeID = node1.ID
		alloc.Job.TaskGroups[0].Volumes = map[string]*structs.VolumeRequest{"example": {
			Name:   "example",
			Type:   structs.VolumeTypeHost,
			Source: vol2.Name,
		}}
		index++
		must.NoError(t, store.UpsertAllocs(structs.MsgTypeTestSetup,
			index, []*structs.Allocation{alloc}))

		delReq := &structs.HostVolumeDeleteRequest{
			VolumeIDs: []string{vol1ID, vol2ID},
			WriteRequest: structs.WriteRequest{
				Region:    srv.Region(),
				Namespace: ns,
				AuthToken: token},
		}
		var delResp structs.HostVolumeDeleteResponse

		err = msgpackrpc.CallWithCodec(codec, "HostVolume.Delete", delReq, &delResp)
		must.EqError(t, err, "Permission denied")

		delReq.AuthToken = powerToken
		err = msgpackrpc.CallWithCodec(codec, "HostVolume.Delete", delReq, &delResp)
		must.EqError(t, err, fmt.Sprintf("volume %s in use by allocations: [%s]", vol2ID, alloc.ID))

		// volume not in use will be deleted even if we got an error
		getReq := &structs.HostVolumeGetRequest{
			ID: vol1ID,
			QueryOptions: structs.QueryOptions{
				Region:    srv.Region(),
				Namespace: ns,
				AuthToken: token,
			},
		}
		var getResp structs.HostVolumeGetResponse
		err = msgpackrpc.CallWithCodec(codec, "HostVolume.Get", getReq, &getResp)
		must.NoError(t, err)
		must.Nil(t, getResp.Volume)

		// update the allocations terminal so the delete works
		alloc = alloc.Copy()
		alloc.ClientStatus = structs.AllocClientStatusFailed
		nArgs := &structs.AllocUpdateRequest{
			Alloc: []*structs.Allocation{alloc},
			WriteRequest: structs.WriteRequest{
				Region:    srv.Region(),
				AuthToken: node1.SecretID},
		}
		err = msgpackrpc.CallWithCodec(codec, "Node.UpdateAlloc", nArgs, &structs.GenericResponse{})

		delReq.VolumeIDs = []string{vol2ID}
		err = msgpackrpc.CallWithCodec(codec, "HostVolume.Delete", delReq, &delResp)
		must.NoError(t, err)

		getReq.ID = vol2ID
		err = msgpackrpc.CallWithCodec(codec, "HostVolume.Get", getReq, &getResp)
		must.NoError(t, err)
		must.Nil(t, getResp.Volume)
	})
}

func TestHostVolumeEndpoint_List(t *testing.T) {

	srv, rootToken, cleanupSrv := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	t.Cleanup(cleanupSrv)
	testutil.WaitForLeader(t, srv.RPC)
	store := srv.fsm.State()
	codec := rpcClient(t, srv)

	index := uint64(1001)

	token := mock.CreatePolicyAndToken(t, store, index, "volume-manager",
		`namespace "apps" { capabilities = ["host-volume-register"] }
	     node { policy = "read" }`).SecretID

	index++
	otherToken := mock.CreatePolicyAndToken(t, store, index, "other",
		`namespace "foo" { capabilities = ["host-volume-read"] }
         node { policy = "read" }`).SecretID

	index++
	ns1 := "apps"
	ns2 := "system"
	nspace1, nspace2 := mock.Namespace(), mock.Namespace()
	nspace1.Name = ns1
	nspace2.Name = ns2
	must.NoError(t, store.UpsertNamespaces(index, []*structs.Namespace{nspace1, nspace2}))

	nodes := []*structs.Node{
		mock.Node(),
		mock.Node(),
		mock.Node(),
	}
	nodes[2].NodePool = "prod"
	index++
	must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup,
		index, nodes[0], state.NodeUpsertWithNodePool))
	must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup,
		index, nodes[1], state.NodeUpsertWithNodePool))
	must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup,
		index, nodes[2], state.NodeUpsertWithNodePool))

	vol1, vol2 := mock.HostVolume(), mock.HostVolume()
	vol1.NodeID = nodes[0].ID
	vol1.Name = "foobar-example"
	vol1.Namespace = ns1
	vol2.NodeID = nodes[1].ID
	vol2.Name = "foobaz-example"
	vol2.Namespace = ns1

	vol3, vol4 := mock.HostVolume(), mock.HostVolume()
	vol3.NodeID = nodes[2].ID
	vol3.NodePool = "prod"
	vol3.Namespace = ns2
	vol3.Name = "foobar-example"
	vol4.Namespace = ns2
	vol4.NodeID = nodes[1].ID
	vol4.Name = "foobaz-example"

	// we need to register these rather than upsert them so we have the correct
	// indexes for unblocking later
	registerReq := &structs.HostVolumeRegisterRequest{
		Volumes: []*structs.HostVolume{vol1, vol2, vol3, vol4},
		WriteRequest: structs.WriteRequest{
			Region:    srv.Region(),
			AuthToken: rootToken.SecretID},
	}

	var registerResp structs.HostVolumeRegisterResponse
	err := msgpackrpc.CallWithCodec(codec, "HostVolume.Register", registerReq, &registerResp)
	must.NoError(t, err)

	testCases := []struct {
		name         string
		req          *structs.HostVolumeListRequest
		expectVolIDs []string
	}{
		{
			name: "wrong namespace for token",
			req: &structs.HostVolumeListRequest{
				QueryOptions: structs.QueryOptions{
					Region:    srv.Region(),
					Namespace: ns1,
					AuthToken: otherToken,
				},
			},
			expectVolIDs: []string{},
		},
		{
			name: "query by namespace",
			req: &structs.HostVolumeListRequest{
				QueryOptions: structs.QueryOptions{
					Region:    srv.Region(),
					Namespace: ns1,
					AuthToken: token,
				},
			},
			expectVolIDs: []string{vol1.ID, vol2.ID},
		},
		{
			name: "wildcard namespace",
			req: &structs.HostVolumeListRequest{
				QueryOptions: structs.QueryOptions{
					Region:    srv.Region(),
					Namespace: structs.AllNamespacesSentinel,
					AuthToken: token,
				},
			},
			expectVolIDs: []string{vol1.ID, vol2.ID, vol3.ID, vol4.ID},
		},
		{
			name: "query by prefix",
			req: &structs.HostVolumeListRequest{
				QueryOptions: structs.QueryOptions{
					Region:    srv.Region(),
					Namespace: ns1,
					AuthToken: token,
					Prefix:    "foobar",
				},
			},
			expectVolIDs: []string{vol1.ID},
		},
		{
			name: "query by node",
			req: &structs.HostVolumeListRequest{
				NodeID: nodes[1].ID,
				QueryOptions: structs.QueryOptions{
					Region:    srv.Region(),
					Namespace: structs.AllNamespacesSentinel,
					AuthToken: token,
				},
			},
			expectVolIDs: []string{vol2.ID, vol4.ID},
		},
		{
			name: "query by node pool",
			req: &structs.HostVolumeListRequest{
				NodePool: "prod",
				QueryOptions: structs.QueryOptions{
					Region:    srv.Region(),
					Namespace: structs.AllNamespacesSentinel,
					AuthToken: token,
				},
			},
			expectVolIDs: []string{vol3.ID},
		},
		{
			name: "query by incompatible node ID and pool",
			req: &structs.HostVolumeListRequest{
				NodeID:   nodes[1].ID,
				NodePool: "prod",
				QueryOptions: structs.QueryOptions{
					Region:    srv.Region(),
					Namespace: structs.AllNamespacesSentinel,
					AuthToken: token,
				},
			},
			expectVolIDs: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var resp structs.HostVolumeListResponse
			err := msgpackrpc.CallWithCodec(codec, "HostVolume.List", tc.req, &resp)
			must.NoError(t, err)

			gotIDs := helper.ConvertSlice(resp.Volumes,
				func(v *structs.HostVolumeStub) string { return v.ID })
			must.SliceContainsAll(t, tc.expectVolIDs, gotIDs,
				must.Sprintf("got: %v", gotIDs))
		})
	}

	t.Run("blocking query unblocks", func(t *testing.T) {

		// Get response will include the volume's Index to block on
		getReq := &structs.HostVolumeGetRequest{
			ID: vol1.ID,
			QueryOptions: structs.QueryOptions{
				Region:    srv.Region(),
				Namespace: vol1.Namespace,
				AuthToken: token,
			},
		}
		var getResp structs.HostVolumeGetResponse
		err = msgpackrpc.CallWithCodec(codec, "HostVolume.Get", getReq, &getResp)

		nextVol := getResp.Volume.Copy()
		nextVol.RequestedCapacityMaxBytes = 300000
		registerReq.Volumes = []*structs.HostVolume{nextVol}
		registerReq.Namespace = nextVol.Namespace

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		t.Cleanup(cancel)
		respCh := make(chan *structs.HostVolumeListResponse)
		errCh := make(chan error)

		// prepare the blocking List query

		req := &structs.HostVolumeListRequest{
			QueryOptions: structs.QueryOptions{
				Region:        srv.Region(),
				Namespace:     ns1,
				AuthToken:     token,
				MinQueryIndex: getResp.Index,
			},
		}

		go func() {
			codec := rpcClient(t, srv)
			var listResp structs.HostVolumeListResponse
			err := msgpackrpc.CallWithCodec(codec, "HostVolume.List", req, &listResp)
			if err != nil {
				errCh <- err
			}
			respCh <- &listResp
		}()

		err = msgpackrpc.CallWithCodec(codec, "HostVolume.Register", registerReq, &registerResp)
		must.NoError(t, err)

		select {
		case <-ctx.Done():
			t.Fatal("timeout or cancelled")
		case listResp := <-respCh:
			must.Greater(t, req.MinQueryIndex, listResp.Index)
		case err := <-errCh:
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// mockHostVolumeClient models client RPCs that have side-effects on the
// client host
type mockHostVolumeClient struct {
	lock               sync.Mutex
	nextCreateResponse *cstructs.ClientHostVolumeCreateResponse
	nextCreateErr      error
	nextDeleteErr      error
}

// newMockHostVolumeClient configures a RPC-only Nomad test agent and returns a
// mockHostVolumeClient so we can send it client RPCs
func newMockHostVolumeClient(t *testing.T, srv *Server, pool string) (*mockHostVolumeClient, *structs.Node) {
	t.Helper()

	mockClientEndpoint := &mockHostVolumeClient{}

	c1, cleanup := client.TestRPCOnlyClient(t, func(c *config.Config) {
		c.Node.NodePool = pool
		// TODO(1.10.0): we'll want to have a version gate for this feature
		c.Node.Attributes["nomad.version"] = version.Version
	}, srv.config.RPCAddr, map[string]any{"HostVolume": mockClientEndpoint})
	t.Cleanup(cleanup)

	must.Wait(t, wait.InitialSuccess(wait.BoolFunc(func() bool {
		node, err := srv.fsm.State().NodeByID(nil, c1.NodeID())
		if err != nil {
			return false
		}
		if node != nil && node.Status == structs.NodeStatusReady {
			return true
		}
		return false
	}),
		wait.Timeout(time.Second*5),
		wait.Gap(time.Millisecond),
	), must.Sprint("client did not fingerprint before timeout"))

	return mockClientEndpoint, c1.Node()
}

func (v *mockHostVolumeClient) setCreate(
	resp *cstructs.ClientHostVolumeCreateResponse, err error) {
	v.lock.Lock()
	defer v.lock.Unlock()
	v.nextCreateResponse = resp
	v.nextCreateErr = err
}

func (v *mockHostVolumeClient) setDelete(errMsg string) {
	v.lock.Lock()
	defer v.lock.Unlock()
	v.nextDeleteErr = errors.New(errMsg)
}

func (v *mockHostVolumeClient) Create(
	req *cstructs.ClientHostVolumeCreateRequest,
	resp *cstructs.ClientHostVolumeCreateResponse) error {
	v.lock.Lock()
	defer v.lock.Unlock()
	*resp = *v.nextCreateResponse
	return v.nextCreateErr
}

func (v *mockHostVolumeClient) Delete(
	req *cstructs.ClientHostVolumeDeleteRequest,
	resp *cstructs.ClientHostVolumeDeleteResponse) error {
	v.lock.Lock()
	defer v.lock.Unlock()
	return v.nextDeleteErr
}
