// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"testing"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/queues"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
)

func TestBatchJobQueue_Status(t *testing.T) {
	ci.Parallel(t)
	s, cleanup := TestServer(t, nil)
	t.Cleanup(cleanup)
	testutil.WaitForLeader(t, s.RPC)

	s.batchJobQueue = &queues.TestQueue{}
	eval1 := mock.Eval()
	eval1.Namespace = "ns1"
	eval2 := mock.Eval()
	eval2.Namespace = "ns2"

	s.batchJobQueue.Enqueue(eval1)
	s.batchJobQueue.Enqueue(eval2)

	req := structs.QueueStatusRequest{QueryOptions: structs.QueryOptions{
		Region: "global",
	}}

	reply := structs.QueueStatusResponse{}

	err := s.RPC("BatchJobQueue.Status", &req, &reply)
	must.NoError(t, err)
	must.Eq(t, reply.Type, "test")
	must.Len(t, 2, reply.Workloads.([]*structs.Evaluation))
}

func TestBatchJobQueue_Status_WithACL(t *testing.T) {
	ci.Parallel(t)

	s1, root, cleanupS1 := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()
	err := state.UpsertNamespaces(1001, []*structs.Namespace{{Name: "ns1"}, {Name: "ns2"}})
	must.NoError(t, err)

	s1.batchJobQueue = &queues.TestQueue{}

	eval1 := mock.Eval()
	eval1.Namespace = "ns1"
	eval2 := mock.Eval()
	eval2.Namespace = "ns2"

	s1.batchJobQueue.Enqueue(eval1)
	s1.batchJobQueue.Enqueue(eval2)

	// Expect failure for request without a token
	req := structs.QueueStatusRequest{QueryOptions: structs.QueryOptions{
		Region: "global",
	}}
	resp := structs.QueueStatusResponse{}
	err = s1.RPC("BatchJobQueue.Status", req, &resp)
	must.NotNil(t, err)

	// Expect success for request with a management token
	req.AuthToken = root.SecretID
	err = s1.RPC("BatchJobQueue.Status", req, &resp)
	must.Nil(t, err)
	must.Len(t, 2, resp.Workloads.([]*structs.Evaluation))

	// Expect empty result for request with a token that doesn't have namespace permissions for any workload in the queue
	resp = structs.QueueStatusResponse{}
	validToken := mock.CreatePolicyAndToken(t, state, 1003, "test-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))
	req.AuthToken = validToken.SecretID
	err = s1.RPC("BatchJobQueue.Status", req, &resp)
	must.Nil(t, err)
	must.Len(t, 0, resp.Workloads.([]*structs.Evaluation))

	// Expect filtered result for request with a token that doesn't have namespace permissions for any workload in the queue
	validFilteredToken := mock.CreatePolicyAndToken(t, state, 1005, "test-valid",
		mock.NamespacePolicy("ns1", "", []string{acl.NamespaceCapabilityListJobs}))
	req.AuthToken = validFilteredToken.SecretID
	req.Namespace = "ns1"
	err = s1.RPC("BatchJobQueue.Status", req, &resp)
	must.Nil(t, err)
	must.Len(t, 1, resp.Workloads.([]*structs.Evaluation))
}
