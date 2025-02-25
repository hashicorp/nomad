// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

package nomad

import (
	"testing"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc/v2"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
)

func TestJobEndpoint_Register_NodePool(t *testing.T) {
	ci.Parallel(t)

	s, cleanupS := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Create test namespace.
	ns := mock.Namespace()
	nsReq := &structs.NamespaceUpsertRequest{
		Namespaces:   []*structs.Namespace{ns},
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var nsResp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "Namespace.UpsertNamespaces", nsReq, &nsResp)
	must.NoError(t, err)

	// Create test node pool.
	pool := mock.NodePool()
	poolReq := &structs.NodePoolUpsertRequest{
		NodePools:    []*structs.NodePool{pool},
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var poolResp structs.GenericResponse
	err = msgpackrpc.CallWithCodec(codec, "NodePool.UpsertNodePools", poolReq, &poolResp)
	must.NoError(t, err)

	testCases := []struct {
		name         string
		namespace    string
		nodePool     string
		expectedPool string
		expectedErr  string
	}{
		{
			name:         "job in default namespace uses default node pool",
			namespace:    structs.DefaultNamespace,
			nodePool:     "",
			expectedPool: structs.NodePoolDefault,
		},
		{
			name:         "job without node pool uses default node pool",
			namespace:    ns.Name,
			nodePool:     "",
			expectedPool: structs.NodePoolDefault,
		},
		{
			name:         "job can set node pool",
			namespace:    ns.Name,
			nodePool:     pool.Name,
			expectedPool: pool.Name,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			job := mock.Job()
			job.Namespace = tc.namespace
			job.NodePool = tc.nodePool

			req := &structs.JobRegisterRequest{
				Job: job,
				WriteRequest: structs.WriteRequest{
					Region:    "global",
					Namespace: job.Namespace,
				},
			}
			var resp structs.JobRegisterResponse
			err = msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)

			if tc.expectedErr != "" {
				must.ErrorContains(t, err, tc.expectedErr)
			} else {
				must.NoError(t, err)

				got, err := s.State().JobByID(nil, job.Namespace, job.ID)
				must.NoError(t, err)
				must.Eq(t, tc.expectedPool, got.NodePool)
			}
		})
	}
}
