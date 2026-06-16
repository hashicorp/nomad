// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"maps"
	"testing"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/queues"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
	tmock "github.com/stretchr/testify/mock"
)

func TestBatchJobQueue_Status(t *testing.T) {
	ci.Parallel(t)
	s, cleanup := TestServer(t, nil)
	t.Cleanup(cleanup)
	testutil.WaitForLeader(t, s.RPC)

	s.batchJobQueue = new(queues.MockQueue)
	s.batchJobQueue.(*queues.MockQueue).On("Status", tmock.MatchedBy(func(m map[string]bool) bool {
		return m == nil
	}), tmock.Anything).Return(structs.QueueStatusResponse{
		Type: "test",
		Results: []*structs.Evaluation{
			{ID: "eval1"},
			{ID: "eval2"},
		},
	})

	req := structs.QueueStatusRequest{QueryOptions: structs.QueryOptions{
		Region: "global",
	}}

	reply := structs.QueueStatusResponse{}

	err := s.RPC("BatchJobQueue.Status", &req, &reply)
	must.NoError(t, err)
	s.batchJobQueue.(*queues.MockQueue).AssertExpectations(t)
	must.Eq(t, reply.Type, "test")
	must.Len(t, 2, reply.Results.([]*structs.Evaluation))
}

func TestBatchJobQueue_Status_WithACL(t *testing.T) {
	ci.Parallel(t)

	s1, root, cleanupS1 := TestACLServer(t, func(c *Config) {
	})
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()
	err := state.UpsertNamespaces(1001, []*structs.Namespace{{Name: "ns1"}, {Name: "ns2"}})
	must.NoError(t, err)

	testCases := []struct {
		name                      string
		req                       structs.QueueStatusRequest
		err                       string
		expectedAllowedNamespaces map[string]bool
		resp                      structs.QueueStatusResponse
	}{
		{
			name: "no token",
			req:  structs.QueueStatusRequest{QueryOptions: structs.QueryOptions{Region: "global"}},
			err:  structs.ErrPermissionDenied.Error(),
			resp: structs.QueueStatusResponse{},
		},
		{
			name:                      "management token",
			req:                       structs.QueueStatusRequest{QueryOptions: structs.QueryOptions{Region: "global", AuthToken: root.SecretID}},
			expectedAllowedNamespaces: nil,
			resp:                      structs.QueueStatusResponse{Results: []*structs.Evaluation{{ID: "eval1"}, {ID: "eval2"}}, QueryMeta: structs.QueryMeta{KnownLeader: true}},
		},
		{
			name:                      "valid token without permissions for jobs on queue",
			req:                       structs.QueueStatusRequest{QueryOptions: structs.QueryOptions{Region: "global", AuthToken: mock.CreatePolicyAndToken(t, state, 1003, "test-valid", mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs})).SecretID}},
			expectedAllowedNamespaces: map[string]bool{"default": true},
			resp:                      structs.QueueStatusResponse{Results: make([]*structs.Evaluation, 0), QueryMeta: structs.QueryMeta{KnownLeader: true}},
		},
		{
			name:                      "valid token with permissions for one namespace",
			req:                       structs.QueueStatusRequest{QueryOptions: structs.QueryOptions{Region: "global", AuthToken: mock.CreatePolicyAndToken(t, state, 1005, "test-valid-ns1", mock.NamespacePolicy("ns1", "", []string{acl.NamespaceCapabilityListJobs})).SecretID, Namespace: "ns1"}},
			expectedAllowedNamespaces: map[string]bool{"ns1": true},
			resp:                      structs.QueueStatusResponse{Results: []*structs.Evaluation{{ID: "eval1"}}, QueryMeta: structs.QueryMeta{KnownLeader: true}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s1.batchJobQueue = new(queues.MockQueue)
			s1.batchJobQueue.(*queues.MockQueue).On("Status", tmock.MatchedBy(func(m map[string]bool) bool {
				return maps.Equal(tc.expectedAllowedNamespaces, m)
			}), tmock.Anything).Return(tc.resp)

			resp := structs.QueueStatusResponse{}
			err = s1.RPC("BatchJobQueue.Status", &tc.req, &resp)
			if tc.err != "" {
				must.ErrorContains(t, err, tc.err)
				return
			}
			must.NoError(t, err)
			s1.batchJobQueue.(*queues.MockQueue).AssertExpectations(t)
			must.Eq(t, tc.resp, resp)
		})
	}
}
