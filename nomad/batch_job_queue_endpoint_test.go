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

func TestBatchJobQueue_Jobs(t *testing.T) {
	ci.Parallel(t)
	s, cleanup := TestServer(t, nil)
	t.Cleanup(cleanup)
	testutil.WaitForLeader(t, s.RPC)

	workload1 := &structs.Evaluation{
		ID:          "eval1",
		Namespace:   "ns1",
		CreateTime:  100,
		CreateIndex: 100,
	}
	workload2 := &structs.Evaluation{
		ID:          "eval2",
		Namespace:   "ns1",
		CreateTime:  200,
		CreateIndex: 200,
	}
	workload3 := &structs.Evaluation{
		ID:          "eval3",
		Namespace:   "ns2",
		CreateTime:  300,
		CreateIndex: 300,
	}

	testCases := []struct {
		name string
		req  structs.QueueJobsRequest
		err  string
		resp structs.QueueJobsResponse
	}{
		{
			name: "list all",
			req:  structs.QueueJobsRequest{QueryOptions: structs.QueryOptions{Region: "global"}},
			resp: structs.QueueJobsResponse{Type: "test", Workloads: []structs.QueueWorkload{workload1, workload2, workload3}, QueryMeta: structs.QueryMeta{KnownLeader: true}},
		},
		{
			name: "paginate per-page=2 page=1",
			req:  structs.QueueJobsRequest{QueryOptions: structs.QueryOptions{Region: "global", PerPage: 2}},
			resp: structs.QueueJobsResponse{Type: "test", Workloads: []structs.QueueWorkload{workload1, workload2}, QueryMeta: structs.QueryMeta{KnownLeader: true, NextToken: "300.eval3"}},
		},
		{
			name: "paginate per-page=1 page=3",
			req:  structs.QueueJobsRequest{QueryOptions: structs.QueryOptions{Region: "global", PerPage: 1, NextToken: "300.eval3"}},
			resp: structs.QueueJobsResponse{Type: "test", Workloads: []structs.QueueWorkload{workload3}, QueryMeta: structs.QueryMeta{KnownLeader: true, NextToken: ""}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockQueue := new(queues.MockQueue)
			mockQueue.On("Jobs").Return(&queues.WorkloadIter{
				Workloads: []structs.QueueWorkload{workload1, workload2, workload3},
			})
			s.batchQueueMgr = queues.NewBatchQueueMgr(t.Context(), structs.BatchQueue{}, nil, nil, queues.WithQueue(mockQueue))

			reply := structs.QueueJobsResponse{}
			err := s.RPC("BatchJobQueue.Jobs", &tc.req, &reply)
			if tc.err != "" {
				must.ErrorContains(t, err, tc.err)
				return
			}
			must.NoError(t, err)
			must.Eq(t, tc.resp, reply)
		})

	}
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

	workload1 := &structs.Evaluation{
		ID:        "eval1",
		Namespace: "ns1",
	}
	workload2 := &structs.Evaluation{
		ID:        "eval2",
		Namespace: "ns1",
	}
	workload3 := &structs.Evaluation{
		ID:        "eval3",
		Namespace: "ns2",
	}

	testCases := []struct {
		name string
		req  structs.QueueJobsRequest
		err  string
		resp structs.QueueJobsResponse
	}{
		{
			name: "no token",
			req:  structs.QueueJobsRequest{QueryOptions: structs.QueryOptions{Region: "global"}},
			err:  structs.ErrPermissionDenied.Error(),
			resp: structs.QueueJobsResponse{},
		},
		{
			name: "management token",
			req:  structs.QueueJobsRequest{QueryOptions: structs.QueryOptions{Region: "global", AuthToken: root.SecretID}},
			resp: structs.QueueJobsResponse{Type: "test", Workloads: []structs.QueueWorkload{workload1, workload3, workload2}, QueryMeta: structs.QueryMeta{KnownLeader: true}},
		},
		{
			name: "valid token without permissions for jobs on queue",
			req:  structs.QueueJobsRequest{QueryOptions: structs.QueryOptions{Region: "global", AuthToken: mock.CreatePolicyAndToken(t, state, 1003, "test-valid", mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs})).SecretID}},
			resp: structs.QueueJobsResponse{Type: "test", Workloads: make([]structs.QueueWorkload, 0), QueryMeta: structs.QueryMeta{KnownLeader: true}},
		},
		{
			name: "valid token with permissions for one namespace",
			req:  structs.QueueJobsRequest{QueryOptions: structs.QueryOptions{Region: "global", AuthToken: mock.CreatePolicyAndToken(t, state, 1005, "test-valid-ns1", mock.NamespacePolicy("ns2", "", []string{acl.NamespaceCapabilityListJobs})).SecretID, Namespace: "ns2"}},
			resp: structs.QueueJobsResponse{Type: "test", Workloads: []structs.QueueWorkload{workload3}, QueryMeta: structs.QueryMeta{KnownLeader: true}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockQueue := new(queues.MockQueue)
			mockQueue.On("Jobs").Return(&queues.WorkloadIter{
				Workloads: []structs.QueueWorkload{
					workload1,
					workload3,
					workload2,
				},
			})

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
