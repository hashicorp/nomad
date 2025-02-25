// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"testing"
	"time"

	"github.com/hashicorp/go-memdb"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc/v2"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
)

func TestTaskGroupHostVolumeClaimEndpoint_List(t *testing.T) {
	ci.Parallel(t)

	testServer, _, testServerCleanupFn := TestACLServer(t, nil)
	defer testServerCleanupFn()
	codec := rpcClient(t, testServer)
	testutil.WaitForLeader(t, testServer.RPC)

	store := testServer.State()

	goodToken := mock.CreatePolicyAndToken(t, store, 999, "good",
		`namespace "*" { capabilities = ["host-volume-read"] }
         node { policy = "read" }`).SecretID

	stickyJob := mock.Job()
	dhvID := uuid.Generate()
	dhvName := "foo"

	// upsert claims, because we can't create them by API
	existingClaims := []*structs.TaskGroupHostVolumeClaim{
		{
			ID:            uuid.Generate(),
			Namespace:     structs.DefaultNamespace,
			JobID:         stickyJob.ID,
			TaskGroupName: stickyJob.TaskGroups[0].Name,
			VolumeID:      dhvID,
			VolumeName:    dhvName,
		},
		{
			ID:            uuid.Generate(),
			Namespace:     "foo",
			JobID:         stickyJob.ID,
			TaskGroupName: stickyJob.TaskGroups[0].Name,
			VolumeID:      dhvID,
			VolumeName:    dhvName,
		},
		{
			ID:            uuid.Generate(),
			Namespace:     structs.DefaultNamespace,
			JobID:         "fooooo",
			TaskGroupName: stickyJob.TaskGroups[0].Name,
			VolumeID:      dhvID,
			VolumeName:    dhvName,
		},
	}

	for _, claim := range existingClaims {
		must.NoError(t, store.UpsertTaskGroupHostVolumeClaim(structs.MsgTypeTestSetup, 1000, claim))
	}

	// Try listing without a valid ACL token.
	claimsReq1 := &structs.TaskGroupVolumeClaimListRequest{
		QueryOptions: structs.QueryOptions{
			Region: DefaultRegion,
		},
	}
	var claimsResp1 structs.TaskGroupVolumeClaimListResponse
	err := msgpackrpc.CallWithCodec(codec, structs.TaskGroupHostVolumeClaimListRPCMethod, claimsReq1, &claimsResp1)
	must.EqError(t, err, "Permission denied")

	// Try listing claims with a valid ACL token.
	claimsReq2 := &structs.TaskGroupVolumeClaimListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    DefaultRegion,
			AuthToken: goodToken,
		},
	}
	var claimsResp2 structs.TaskGroupVolumeClaimListResponse
	err = msgpackrpc.CallWithCodec(codec, structs.TaskGroupHostVolumeClaimListRPCMethod, claimsReq2, &claimsResp2)
	must.NoError(t, err)
	must.Len(t, 2, claimsResp2.Claims)

	// Now test a blocking query, where we wait for an update to the list which
	// is triggered by a deletion.
	type res struct {
		err   error
		reply *structs.TaskGroupVolumeClaimListResponse
	}
	resultCh := make(chan *res)

	go func(resultCh chan *res) {
		claimsReq3 := &structs.TaskGroupVolumeClaimListRequest{
			QueryOptions: structs.QueryOptions{
				Region:        DefaultRegion,
				AuthToken:     goodToken,
				MinQueryIndex: claimsResp2.Index,
				MaxQueryTime:  10 * time.Second,
			},
		}
		var claimsResp3 structs.TaskGroupVolumeClaimListResponse
		err = msgpackrpc.CallWithCodec(codec, structs.TaskGroupHostVolumeClaimListRPCMethod, claimsReq3, &claimsResp3)
		resultCh <- &res{err: err, reply: &claimsResp3}
	}(resultCh)

	// Delete a claim from state which should return the blocking query.
	must.NoError(t, testServer.fsm.State().DeleteTaskGroupHostVolumeClaim(
		claimsResp2.Index+10, existingClaims[0].ID))

	// Wait until the test within the routine is complete.
	result := <-resultCh
	must.NoError(t, result.err)
	must.Len(t, 1, result.reply.Claims)
	must.NotEq(t, result.reply.Claims[0].ID, existingClaims[0].ID)
	must.Greater(t, claimsResp2.Index, result.reply.Index)

	// Try listing claims with a filter for a wildcard ns (should only be 1
	// because existingClaims[0] has just been deleted)
	claimsReq4 := &structs.TaskGroupVolumeClaimListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    DefaultRegion,
			AuthToken: goodToken,
			Namespace: structs.AllNamespacesSentinel,
		},
		JobID: stickyJob.ID,
	}
	var claimsResp4 structs.TaskGroupVolumeClaimListResponse
	err = msgpackrpc.CallWithCodec(codec, structs.TaskGroupHostVolumeClaimListRPCMethod, claimsReq4, &claimsResp4)
	must.NoError(t, err)
	must.Len(t, 1, claimsResp4.Claims)
}

func TestTaskGroupHostVolumeClaimEndpoint_Delete(t *testing.T) {
	ci.Parallel(t)

	testServer, _, testServerCleanupFn := TestACLServer(t, nil)
	defer testServerCleanupFn()
	codec := rpcClient(t, testServer)
	testutil.WaitForLeader(t, testServer.RPC)

	store := testServer.State()

	goodToken := mock.CreatePolicyAndToken(t, store, 999, "good",
		`namespace "default" { capabilities = ["host-volume-write"] }
         node { policy = "write" }`).SecretID

	stickyJob := mock.Job()
	dhvID := uuid.Generate()
	dhvName := "foo"

	// upsert claims, because we can't create them by API
	existingClaims := []*structs.TaskGroupHostVolumeClaim{
		{
			ID:            uuid.Generate(),
			Namespace:     structs.DefaultNamespace,
			JobID:         stickyJob.ID,
			TaskGroupName: stickyJob.TaskGroups[0].Name,
			VolumeID:      dhvID,
			VolumeName:    dhvName,
		},
		{
			ID:            uuid.Generate(),
			Namespace:     "foo",
			JobID:         stickyJob.ID,
			TaskGroupName: stickyJob.TaskGroups[0].Name,
			VolumeID:      dhvID,
			VolumeName:    dhvName,
		},
		{
			ID:            uuid.Generate(),
			Namespace:     structs.DefaultNamespace,
			JobID:         "fooooo",
			TaskGroupName: stickyJob.TaskGroups[0].Name,
			VolumeID:      dhvID,
			VolumeName:    dhvName,
		},
	}

	for _, claim := range existingClaims {
		must.NoError(t, store.UpsertTaskGroupHostVolumeClaim(structs.MsgTypeTestSetup, 1000, claim))
	}

	// Try deleting without a valid ACL token.
	claimsReq1 := &structs.TaskGroupVolumeClaimDeleteRequest{
		ClaimID: existingClaims[0].ID,
		WriteRequest: structs.WriteRequest{
			Region: DefaultRegion,
		},
	}
	var claimsResp1 structs.TaskGroupVolumeClaimDeleteResponse
	err := msgpackrpc.CallWithCodec(codec, structs.TaskGroupHostVolumeClaimDeleteRPCMethod, claimsReq1, &claimsResp1)
	must.EqError(t, err, "Permission denied")

	// Try deleting claim with a valid ACL token.
	claimsReq2 := &structs.TaskGroupVolumeClaimDeleteRequest{
		ClaimID: existingClaims[0].ID,
		WriteRequest: structs.WriteRequest{
			Region:    DefaultRegion,
			AuthToken: goodToken,
		},
	}
	var claimsResp2 structs.TaskGroupVolumeClaimDeleteResponse
	err = msgpackrpc.CallWithCodec(codec, structs.TaskGroupHostVolumeClaimDeleteRPCMethod, claimsReq2, &claimsResp2)
	must.NoError(t, err)

	// Ensure the claim is gone
	ws := memdb.NewWatchSet()
	iter, err := testServer.State().GetTaskGroupHostVolumeClaims(ws)
	must.NoError(t, err)

	var claimsLookup []*structs.TaskGroupHostVolumeClaim
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		claimsLookup = append(claimsLookup, raw.(*structs.TaskGroupHostVolumeClaim))
	}

	must.Len(t, 2, claimsLookup)
	must.SliceNotContains(t, claimsLookup, existingClaims[0])

	// Try to delete the previously deleted claim; this should fail.
	claimsReq3 := &structs.TaskGroupVolumeClaimDeleteRequest{
		ClaimID: existingClaims[0].ID,
		WriteRequest: structs.WriteRequest{
			Region:    DefaultRegion,
			AuthToken: goodToken,
		},
	}
	var claimsResp3 structs.TaskGroupVolumeClaimDeleteResponse
	err = msgpackrpc.CallWithCodec(codec, structs.TaskGroupHostVolumeClaimDeleteRPCMethod, claimsReq3, &claimsResp3)
	must.EqError(t, err, "Task group volume claim does not exist")
}
