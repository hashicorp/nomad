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

func TestBatchJobQueue_Jobs(t *testing.T) {
	ci.Parallel(t)
	s, cleanup := TestServer(t, nil)
	t.Cleanup(cleanup)
	testutil.WaitForLeader(t, s.RPC)

	mockQueue := new(queues.MockQueue)
	mockQueue.On("Jobs", tmock.MatchedBy(func(m map[string]bool) bool {
		return m == nil
	})).Return(structs.QueueJobsResponse{
		Type: "test",
		Workloads: []*structs.Evaluation{
			{ID: "eval1"},
			{ID: "eval2"},
		},
	})
	mockQueue.On("Stop").Return()

	s.batchQueueMgr = queues.NewBatchQueueMgr(t.Context(), structs.BatchQueue{}, nil, nil, queues.WithQueue(mockQueue))

	req := structs.QueueJobsRequest{QueryOptions: structs.QueryOptions{
		Region: "global",
	}}

	reply := structs.QueueJobsResponse{}

	err := s.RPC("BatchJobQueue.Jobs", &req, &reply)
	must.NoError(t, err)
	must.Eq(t, reply.Type, "test")
	must.Len(t, 2, reply.Workloads.([]*structs.Evaluation))
}

func TestBatchJobQueue_Jobs_WithACL(t *testing.T) {
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
		req                       structs.QueueJobsRequest
		err                       string
		expectedAllowedNamespaces map[string]bool
		resp                      structs.QueueJobsResponse
	}{
		{
			name: "no token",
			req:  structs.QueueJobsRequest{QueryOptions: structs.QueryOptions{Region: "global"}},
			err:  structs.ErrPermissionDenied.Error(),
			resp: structs.QueueJobsResponse{},
		},
		{
			name:                      "management token",
			req:                       structs.QueueJobsRequest{QueryOptions: structs.QueryOptions{Region: "global", AuthToken: root.SecretID}},
			expectedAllowedNamespaces: nil,
			resp:                      structs.QueueJobsResponse{Workloads: []*structs.Evaluation{{ID: "eval1"}, {ID: "eval2"}}, QueryMeta: structs.QueryMeta{KnownLeader: true}},
		},
		{
			name:                      "valid token without permissions for jobs on queue",
			req:                       structs.QueueJobsRequest{QueryOptions: structs.QueryOptions{Region: "global", AuthToken: mock.CreatePolicyAndToken(t, state, 1003, "test-valid", mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs})).SecretID}},
			expectedAllowedNamespaces: map[string]bool{"default": true},
			resp:                      structs.QueueJobsResponse{Workloads: make([]*structs.Evaluation, 0), QueryMeta: structs.QueryMeta{KnownLeader: true}},
		},
		{
			name:                      "valid token with permissions for one namespace",
			req:                       structs.QueueJobsRequest{QueryOptions: structs.QueryOptions{Region: "global", AuthToken: mock.CreatePolicyAndToken(t, state, 1005, "test-valid-ns1", mock.NamespacePolicy("ns1", "", []string{acl.NamespaceCapabilityListJobs})).SecretID, Namespace: "ns1"}},
			expectedAllowedNamespaces: map[string]bool{"ns1": true},
			resp:                      structs.QueueJobsResponse{Workloads: []*structs.Evaluation{{ID: "eval1"}}, QueryMeta: structs.QueryMeta{KnownLeader: true}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockQueue := new(queues.MockQueue)
			mockQueue.On("Jobs", tmock.MatchedBy(func(m map[string]bool) bool {
				return maps.Equal(tc.expectedAllowedNamespaces, m)
			}), tmock.Anything).Return(tc.resp)
			mockQueue.On("Stop").Return()

			resp := structs.QueueJobsResponse{}
			s1.batchQueueMgr = queues.NewBatchQueueMgr(t.Context(), structs.BatchQueue{}, nil, nil, queues.WithQueue(mockQueue))

			err = s1.RPC("BatchJobQueue.Jobs", &tc.req, &resp)
			if tc.err != "" {
				must.ErrorContains(t, err, tc.err)
				return
			}
			must.NoError(t, err)
			must.Eq(t, tc.resp, resp)
		})
	}
}

func TestBatchJobQueue_Tenants(t *testing.T) {
	ci.Parallel(t)
	s, cleanup := TestServer(t, nil)
	t.Cleanup(cleanup)
	testutil.WaitForLeader(t, s.RPC)

	mockQueue := new(queues.MockQueue)
	mockQueue.On("Tenants").Return(structs.QueueTenantsResponse{
		Type: "test",
		Tenants: []string{
			"ns1", "ns2",
		},
	})
	mockQueue.On("Stop").Return()

	req := structs.QueueTenantsRequest{QueryOptions: structs.QueryOptions{
		Region: "global",
	}}

	reply := structs.QueueTenantsResponse{}

	s.batchQueueMgr = queues.NewBatchQueueMgr(t.Context(), structs.BatchQueue{}, nil, nil, queues.WithQueue(mockQueue))

	err := s.RPC("BatchJobQueue.Tenants", &req, &reply)
	must.NoError(t, err)
	must.Eq(t, reply.Type, "test")
	must.Len(t, 2, reply.Tenants.([]string))
}

func TestBatchJobQueue_Tenants_WithACL(t *testing.T) {
	ci.Parallel(t)
	s1, root, cleanupS1 := TestACLServer(t, func(c *Config) {
	})
	defer cleanupS1()
	state := s1.fsm.State()
	testutil.WaitForLeader(t, s1.RPC)

	testCases := []struct {
		name string
		req  structs.QueueTenantsRequest
		err  string
		resp structs.QueueTenantsResponse
	}{
		{
			name: "no token",
			req:  structs.QueueTenantsRequest{QueryOptions: structs.QueryOptions{Region: "global"}},
			err:  structs.ErrPermissionDenied.Error(),
			resp: structs.QueueTenantsResponse{},
		},
		{
			name: "management token",
			req:  structs.QueueTenantsRequest{QueryOptions: structs.QueryOptions{Region: "global", AuthToken: root.SecretID}},
			resp: structs.QueueTenantsResponse{Tenants: []string{"ns1", "ns2"}, QueryMeta: structs.QueryMeta{KnownLeader: true}},
		},
		{
			name: "valid token without operator read permissions",
			req:  structs.QueueTenantsRequest{QueryOptions: structs.QueryOptions{Region: "global", AuthToken: mock.CreatePolicyAndToken(t, state, 1003, "test-valid", mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs})).SecretID}},
			err:  structs.ErrPermissionDenied.Error(),
			resp: structs.QueueTenantsResponse{},
		},
		{
			name: "valid token with operator read permissions",
			req:  structs.QueueTenantsRequest{QueryOptions: structs.QueryOptions{Region: "global", AuthToken: mock.CreatePolicyAndToken(t, state, 1005, "test-valid-operator", mock.OperatorPolicy("read")).SecretID}},
			resp: structs.QueueTenantsResponse{Tenants: []string{"ns1", "ns2"}, QueryMeta: structs.QueryMeta{KnownLeader: true}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockQueue := new(queues.MockQueue)
			mockQueue.On("Tenants").Return(tc.resp)
			mockQueue.On("Stop").Return()

			reply := structs.QueueTenantsResponse{}

			s1.batchQueueMgr = queues.NewBatchQueueMgr(t.Context(), structs.BatchQueue{}, nil, nil, queues.WithQueue(mockQueue))

			err := s1.RPC("BatchJobQueue.Tenants", &tc.req, &reply)

			if tc.err != "" {
				must.ErrorContains(t, err, tc.err)
				return
			}

			must.NoError(t, err)
			must.Eq(t, tc.resp, reply)
		})
	}
}
