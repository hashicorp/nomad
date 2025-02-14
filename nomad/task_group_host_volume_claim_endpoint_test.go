package nomad

import (
	"testing"
	"time"

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

	testServer, aclRootToken, testServerCleanupFn := TestACLServer(t, nil)
	defer testServerCleanupFn()
	codec := rpcClient(t, testServer)
	testutil.WaitForLeader(t, testServer.RPC)

	store := testServer.State()

	goodToken := mock.CreatePolicyAndToken(t, store, 999, "good",
		`namespace "default" { capabilities = ["host-volume-read"] }
         node { policy = "read" }`).SecretID

	// upsert claims, because we can't create them by API
	stickyRequests := map[string]*structs.VolumeRequest{
		"foo": {
			Type:           "host",
			Source:         "foo",
			Sticky:         true,
			AccessMode:     structs.CSIVolumeAccessModeSingleNodeWriter,
			AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
		},
	}
	stickyJob := mock.Job()
	stickyJob.TaskGroups[0].Volumes = stickyRequests

	dhvID := uuid.Generate()
	dhvName := "foo"

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
	must.Len(t, 3, claimsResp2.Claims)

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
				AuthToken:     aclRootToken.SecretID,
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
	must.Len(t, 2, result.reply.Claims)
	must.NotEq(t, result.reply.Claims[0].ID, existingClaims[0].ID)
	must.Greater(t, claimsResp2.Index, result.reply.Index)
}
