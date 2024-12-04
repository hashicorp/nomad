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
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
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
		WriteRequest: structs.WriteRequest{
			Region:    srv.Region(),
			AuthToken: token},
	}

	t.Run("invalid create", func(t *testing.T) {

		req.Namespace = ns
		var resp structs.HostVolumeCreateResponse
		err := msgpackrpc.CallWithCodec(codec, "HostVolume.Create", req, &resp)
		must.EqError(t, err, "missing volume definition")

		req.Volume = &structs.HostVolume{}
		err = msgpackrpc.CallWithCodec(codec, "HostVolume.Create", req, &resp)
		must.EqError(t, err, `volume validation failed: 2 errors occurred:
	* missing name
	* must include at least one capability block

`)

		req.Volume = &structs.HostVolume{
			Name:     "example",
			PluginID: "example_plugin",
			Constraints: []*structs.Constraint{{
				RTarget: "r1",
				Operand: "=",
			}},
			RequestedCapacityMinBytes: 200000,
			RequestedCapacityMaxBytes: 100000,
			RequestedCapabilities: []*structs.HostVolumeCapability{
				{
					AttachmentMode: structs.HostVolumeAttachmentModeFilesystem,
					AccessMode:     structs.HostVolumeAccessModeSingleNodeWriter,
				},
				{
					AttachmentMode: "bad",
					AccessMode:     "invalid",
				},
			},
		}

		err = msgpackrpc.CallWithCodec(codec, "HostVolume.Create", req, &resp)
		must.EqError(t, err, `volume validation failed: 3 errors occurred:
	* capacity_max (100000) must be larger than capacity_min (200000)
	* invalid attachment mode: "bad"
	* invalid constraint: 1 error occurred:
	* No LTarget provided but is required by constraint



`)

		invalidNode := &structs.Node{ID: uuid.Generate(), NodePool: "does-not-exist"}
		volOnInvalidNode := mock.HostVolumeRequestForNode(ns, invalidNode)
		req.Volume = volOnInvalidNode
		err = msgpackrpc.CallWithCodec(codec, "HostVolume.Create", req, &resp)
		must.EqError(t, err, fmt.Sprintf(
			`validating volume "example" against state failed: node %q does not exist`,
			invalidNode.ID))
	})

	var expectIndex uint64

	c1.setCreate(&cstructs.ClientHostVolumeCreateResponse{
		HostPath:      "/var/nomad/alloc_mounts/foo",
		CapacityBytes: 150000,
	}, nil)

	vol1 := mock.HostVolumeRequest("apps")
	vol1.Name = "example1"
	vol1.NodePool = "prod"
	vol2 := mock.HostVolumeRequest("apps")
	vol2.Name = "example2"
	vol2.NodePool = "prod"

	t.Run("invalid permissions", func(t *testing.T) {
		var resp structs.HostVolumeCreateResponse
		req.AuthToken = otherToken

		req.Volume = vol1
		err := msgpackrpc.CallWithCodec(codec, "HostVolume.Create", req, &resp)
		must.EqError(t, err, "Permission denied")
	})

	t.Run("invalid node constraints", func(t *testing.T) {
		vol1.Constraints[0].RTarget = "r2"
		vol2.Constraints[0].RTarget = "r2"

		defer func() {
			vol1.Constraints[0].RTarget = "r1"
			vol2.Constraints[0].RTarget = "r1"
		}()

		req.Volume = vol1.Copy()
		var resp structs.HostVolumeCreateResponse
		req.AuthToken = token
		err := msgpackrpc.CallWithCodec(codec, "HostVolume.Create", req, &resp)
		must.EqError(t, err, `could not place volume "example1": no node meets constraints`)

		req.Volume = vol2.Copy()
		resp = structs.HostVolumeCreateResponse{}
		err = msgpackrpc.CallWithCodec(codec, "HostVolume.Create", req, &resp)
		must.EqError(t, err, `could not place volume "example2": no node meets constraints`)
	})

	t.Run("valid create", func(t *testing.T) {
		var resp structs.HostVolumeCreateResponse
		req.AuthToken = token
		req.Volume = vol1.Copy()
		err := msgpackrpc.CallWithCodec(codec, "HostVolume.Create", req, &resp)
		must.NoError(t, err)
		must.NotNil(t, resp.Volume)
		vol1 = resp.Volume

		expectIndex = resp.Index
		req.Volume = vol2.Copy()
		resp = structs.HostVolumeCreateResponse{}
		err = msgpackrpc.CallWithCodec(codec, "HostVolume.Create", req, &resp)
		must.NoError(t, err)
		must.NotNil(t, resp.Volume)
		vol2 = resp.Volume

		getReq := &structs.HostVolumeGetRequest{
			ID: vol1.ID,
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

	t.Run("invalid updates", func(t *testing.T) {

		invalidVol1 := vol1.Copy()
		invalidVol2 := &structs.HostVolume{}

		createReq := &structs.HostVolumeCreateRequest{
			Volume: invalidVol2,
			WriteRequest: structs.WriteRequest{
				Region:    srv.Region(),
				Namespace: ns,
				AuthToken: token},
		}
		c1.setCreate(nil, errors.New("should not call this endpoint on invalid RPCs"))
		var createResp structs.HostVolumeCreateResponse
		err := msgpackrpc.CallWithCodec(codec, "HostVolume.Create", createReq, &createResp)
		must.EqError(t, err, `volume validation failed: 2 errors occurred:
	* missing name
	* must include at least one capability block

`, must.Sprint("initial validation failures should exit early"))

		invalidVol1.NodeID = uuid.Generate()
		invalidVol1.RequestedCapacityMinBytes = 100
		invalidVol1.RequestedCapacityMaxBytes = 200
		registerReq := &structs.HostVolumeRegisterRequest{
			Volume: invalidVol1,
			WriteRequest: structs.WriteRequest{
				Region:    srv.Region(),
				Namespace: ns,
				AuthToken: token},
		}
		var registerResp structs.HostVolumeRegisterResponse
		err = msgpackrpc.CallWithCodec(codec, "HostVolume.Register", registerReq, &registerResp)
		must.EqError(t, err, fmt.Sprintf(`validating volume %q update failed: 2 errors occurred:
	* node ID cannot be updated
	* capacity_max (200) cannot be less than existing provisioned capacity (150000)

`, invalidVol1.ID), must.Sprint("update validation checks should have failed"))

	})

	t.Run("blocking Get unblocks on write", func(t *testing.T) {
		nextVol1 := vol1.Copy()
		nextVol1.RequestedCapacityMaxBytes = 300000
		registerReq := &structs.HostVolumeRegisterRequest{
			Volume: nextVol1,
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
			ID: vol1.ID,
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

		// re-register the volume long enough later that we can be sure we won't
		// win a race with the get RPC goroutine
		time.AfterFunc(200*time.Millisecond, func() {
			codec := rpcClient(t, srv)
			var registerResp structs.HostVolumeRegisterResponse
			err := msgpackrpc.CallWithCodec(codec, "HostVolume.Register", registerReq, &registerResp)
			must.NoError(t, err)
		})

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
			VolumeID: vol2.ID,
			WriteRequest: structs.WriteRequest{
				Region:    srv.Region(),
				Namespace: ns,
				AuthToken: token},
		}
		var delResp structs.HostVolumeDeleteResponse

		err := msgpackrpc.CallWithCodec(codec, "HostVolume.Delete", delReq, &delResp)
		must.EqError(t, err, "Permission denied")

		delReq.AuthToken = powerToken
		err = msgpackrpc.CallWithCodec(codec, "HostVolume.Delete", delReq, &delResp)
		must.EqError(t, err, fmt.Sprintf("volume %s in use by allocations: [%s]", vol2.ID, alloc.ID))

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

		err = msgpackrpc.CallWithCodec(codec, "HostVolume.Delete", delReq, &delResp)
		must.NoError(t, err)

		getReq := &structs.HostVolumeGetRequest{
			ID: vol2.ID,
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
	})

	// delete vol1 to finish cleaning up
	var delResp structs.HostVolumeDeleteResponse
	err := msgpackrpc.CallWithCodec(codec, "HostVolume.Delete", &structs.HostVolumeDeleteRequest{
		VolumeID: vol1.ID,
		WriteRequest: structs.WriteRequest{
			Region:    srv.Region(),
			Namespace: vol1.Namespace,
			AuthToken: powerToken,
		},
	}, &delResp)
	must.NoError(t, err)

	// should be no volumes left
	var listResp structs.HostVolumeListResponse
	err = msgpackrpc.CallWithCodec(codec, "HostVolume.List", &structs.HostVolumeListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    srv.Region(),
			Namespace: "*",
			AuthToken: token,
		},
	}, &listResp)
	must.NoError(t, err)
	must.Len(t, 0, listResp.Volumes, must.Sprintf("expect no volumes to remain, got: %+v", listResp))
}

func TestHostVolumeEndpoint_List(t *testing.T) {
	ci.Parallel(t)

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

	vol1 := mock.HostVolumeRequestForNode(ns1, nodes[0])
	vol1.Name = "foobar-example"

	vol2 := mock.HostVolumeRequestForNode(ns1, nodes[1])
	vol2.Name = "foobaz-example"

	vol3 := mock.HostVolumeRequestForNode(ns2, nodes[2])
	vol3.Name = "foobar-example"

	vol4 := mock.HostVolumeRequestForNode(ns2, nodes[1])
	vol4.Name = "foobaz-example"

	// we need to register these rather than upsert them so we have the correct
	// indexes for unblocking later.
	registerReq := &structs.HostVolumeRegisterRequest{
		WriteRequest: structs.WriteRequest{
			Region:    srv.Region(),
			AuthToken: rootToken.SecretID},
	}

	var registerResp structs.HostVolumeRegisterResponse

	// write the volumes in reverse order so our later test can get a blocking
	// query index from a Get it has access to

	registerReq.Volume = vol4
	err := msgpackrpc.CallWithCodec(codec, "HostVolume.Register", registerReq, &registerResp)
	must.NoError(t, err)
	vol4 = registerResp.Volume

	registerReq.Volume = vol3
	registerResp = structs.HostVolumeRegisterResponse{}
	err = msgpackrpc.CallWithCodec(codec, "HostVolume.Register", registerReq, &registerResp)
	must.NoError(t, err)
	vol3 = registerResp.Volume

	registerReq.Volume = vol2
	registerResp = structs.HostVolumeRegisterResponse{}
	err = msgpackrpc.CallWithCodec(codec, "HostVolume.Register", registerReq, &registerResp)
	must.NoError(t, err)
	vol2 = registerResp.Volume

	registerReq.Volume = vol1
	registerResp = structs.HostVolumeRegisterResponse{}
	err = msgpackrpc.CallWithCodec(codec, "HostVolume.Register", registerReq, &registerResp)
	must.NoError(t, err)
	vol1 = registerResp.Volume

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

		// the Get response from the most-recently written volume will have the
		// index we want to block on
		getReq := &structs.HostVolumeGetRequest{
			ID: vol1.ID,
			QueryOptions: structs.QueryOptions{
				Region:    srv.Region(),
				Namespace: ns1,
				AuthToken: token,
			},
		}
		var getResp structs.HostVolumeGetResponse
		err = msgpackrpc.CallWithCodec(codec, "HostVolume.Get", getReq, &getResp)
		must.NoError(t, err)
		must.NotNil(t, getResp.Volume)

		nextVol := getResp.Volume.Copy()
		nextVol.RequestedCapacityMaxBytes = 300000
		registerReq.Volume = nextVol
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

		// re-register the volume long enough later that we can be sure we won't
		// win a race with the get RPC goroutine
		time.AfterFunc(200*time.Millisecond, func() {
			codec := rpcClient(t, srv)
			var registerResp structs.HostVolumeRegisterResponse
			err = msgpackrpc.CallWithCodec(codec, "HostVolume.Register", registerReq, &registerResp)
			must.NoError(t, err)
		})

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

func TestHostVolumeEndpoint_placeVolume(t *testing.T) {
	srv, _, cleanupSrv := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	t.Cleanup(cleanupSrv)
	testutil.WaitForLeader(t, srv.RPC)
	store := srv.fsm.State()

	endpoint := &HostVolume{
		srv:    srv,
		logger: testlog.HCLogger(t),
	}

	node0, node1, node2, node3 := mock.Node(), mock.Node(), mock.Node(), mock.Node()
	node0.NodePool = structs.NodePoolDefault
	node0.Attributes["plugins.host_volume.version.mkdir"] = "0.0.1"

	node1.NodePool = "dev"
	node1.Meta["rack"] = "r2"
	node1.Attributes["plugins.host_volume.version.mkdir"] = "0.0.1"

	node2.NodePool = "prod"
	node2.Attributes["plugins.host_volume.version.mkdir"] = "0.0.1"

	node3.NodePool = "prod"
	node3.Meta["rack"] = "r3"
	node3.HostVolumes = map[string]*structs.ClientHostVolumeConfig{"example": {
		Name: "example",
		Path: "/srv",
	}}
	node3.Attributes["plugins.host_volume.version.mkdir"] = "0.0.1"

	must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, 1000, node0))
	must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, 1000, node1))
	must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, 1000, node2))
	must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, 1000, node3))

	testCases := []struct {
		name      string
		vol       *structs.HostVolume
		expect    *structs.Node
		expectErr string
	}{
		{
			name:   "only one in node pool",
			vol:    &structs.HostVolume{NodePool: "default", PluginID: "mkdir"},
			expect: node0,
		},
		{
			name: "only one that matches constraints",
			vol: &structs.HostVolume{
				PluginID: "mkdir",
				Constraints: []*structs.Constraint{
					{
						LTarget: "${meta.rack}",
						RTarget: "r2",
						Operand: "=",
					},
				}},
			expect: node1,
		},
		{
			name:   "only one available in pool",
			vol:    &structs.HostVolume{NodePool: "prod", Name: "example", PluginID: "mkdir"},
			expect: node2,
		},
		{
			name: "no matching constraint",
			vol: &structs.HostVolume{
				PluginID: "mkdir",
				Constraints: []*structs.Constraint{
					{
						LTarget: "${meta.rack}",
						RTarget: "r6",
						Operand: "=",
					},
				}},
			expectErr: "no node meets constraints",
		},
		{
			name:      "no matching plugin",
			vol:       &structs.HostVolume{PluginID: "not-mkdir"},
			expectErr: "no node meets constraints",
		},
		{
			name: "match already has a volume with the same name",
			vol: &structs.HostVolume{
				Name:     "example",
				PluginID: "mkdir",
				Constraints: []*structs.Constraint{
					{
						LTarget: "${meta.rack}",
						RTarget: "r3",
						Operand: "=",
					},
				}},
			expectErr: "no node meets constraints",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			snap, _ := store.Snapshot()
			node, err := endpoint.placeHostVolume(snap, tc.vol)
			if tc.expectErr == "" {
				must.NoError(t, err)
				must.Eq(t, tc.expect, node)
			} else {
				must.EqError(t, err, tc.expectErr)
				must.Nil(t, node)
			}
		})
	}
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
		c.Node.Attributes["nomad.version"] = version.Version
		c.Node.Attributes["plugins.host_volume.version.mkdir"] = "0.0.1"
		c.Node.Meta["rack"] = "r1"
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
	if v.nextCreateResponse == nil {
		return nil // prevents panics from incorrect tests
	}
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
