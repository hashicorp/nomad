// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestHTTP_ScalingPoliciesList(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		for i := 0; i < 3; i++ {
			// Create the job
			job, _ := mock.JobWithScalingPolicy()

			args := structs.JobRegisterRequest{
				Job: job,
				WriteRequest: structs.WriteRequest{
					Region:    "global",
					Namespace: structs.DefaultNamespace,
				},
			}
			var resp structs.JobRegisterResponse
			require.NoError(s.Agent.RPC("Job.Register", &args, &resp))
		}

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodGet, "/v1/scaling/policies", nil)
		require.NoError(err)

		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.ScalingPoliciesRequest(respW, req)
		require.NoError(err)

		// Check for the index
		require.NotEmpty(respW.Header().Get("X-Nomad-Index"), "missing index")
		require.NotEmpty(respW.Header().Get("X-Nomad-KnownLeader"), "missing known leader")
		require.NotEmpty(respW.Header().Get("X-Nomad-LastContact"), "missing last contact")

		// Check the list
		l := obj.([]*structs.ScalingPolicyListStub)
		require.Len(l, 3)
	})
}

func TestHTTP_ScalingPoliciesList_Filter(t *testing.T) {
	require := require.New(t)
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		var job *structs.Job
		for i := 0; i < 3; i++ {
			// Create the job
			job, _ = mock.JobWithScalingPolicy()

			args := structs.JobRegisterRequest{
				Job: job,
				WriteRequest: structs.WriteRequest{
					Region:    "global",
					Namespace: structs.DefaultNamespace,
				},
			}
			var resp structs.JobRegisterResponse
			require.NoError(s.Agent.RPC("Job.Register", &args, &resp))
		}

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodGet, "/v1/scaling/policies?job="+job.ID, nil)
		require.NoError(err)
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.ScalingPoliciesRequest(respW, req)
		require.NoError(err)

		// Check the list
		l := obj.([]*structs.ScalingPolicyListStub)
		require.Len(l, 1)

		// Request again, with policy type filter
		req, err = http.NewRequest(http.MethodGet, "/v1/scaling/policies?type=cluster", nil)
		require.NoError(err)
		respW = httptest.NewRecorder()

		// Make the request
		obj, err = s.Server.ScalingPoliciesRequest(respW, req)
		require.NoError(err)

		// Check the list
		l = obj.([]*structs.ScalingPolicyListStub)
		require.Len(l, 0)
	})
}

func TestHTTP_ScalingPolicyGet(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Create the job
		job, p := mock.JobWithScalingPolicy()
		args := structs.JobRegisterRequest{
			Job: job,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: structs.DefaultNamespace,
			},
		}
		var resp structs.JobRegisterResponse
		err := s.Agent.RPC("Job.Register", &args, &resp)
		require.NoError(err)

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodGet, "/v1/scaling/policy/"+p.ID, nil)
		require.NoError(err)
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.ScalingPolicySpecificRequest(respW, req)
		require.NoError(err)

		// Check for the index
		require.NotEmpty(respW.Header().Get("X-Nomad-Index"), "missing index")
		require.NotEmpty(respW.Header().Get("X-Nomad-KnownLeader"), "missing known leader")
		require.NotEmpty(respW.Header().Get("X-Nomad-LastContact"), "missing last contact")

		// Check the policy
		require.Equal(p.ID, obj.(*structs.ScalingPolicy).ID)
	})
}
