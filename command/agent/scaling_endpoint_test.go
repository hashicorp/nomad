package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestHTTP_ScalingPoliciesList(t *testing.T) {
	t.Parallel()
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
			if err := s.Agent.RPC("Job.Register", &args, &resp); err != nil {
				t.Fatalf("err: %v", err)
			}
		}

		// Make the HTTP request
		req, err := http.NewRequest("GET", "/v1/scaling/policies", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.ScalingPoliciesRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check for the index
		if respW.Header().Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}
		if respW.Header().Get("X-Nomad-KnownLeader") != "true" {
			t.Fatalf("missing known leader")
		}
		if respW.Header().Get("X-Nomad-LastContact") == "" {
			t.Fatalf("missing last contact")
		}

		// Check the list
		l := obj.([]*structs.ScalingPolicyListStub)
		if len(l) != 3 {
			t.Fatalf("bad: %#v", l)
		}
	})
}

func TestHTTP_ScalingPolicyGet(t *testing.T) {
	t.Parallel()
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
		req, err := http.NewRequest("GET", "/v1/scaling/policy/"+p.ID, nil)
		require.NoError(err)
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.ScalingPolicySpecificRequest(respW, req)
		require.NoError(err)

		// Check for the index
		if respW.Header().Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}
		if respW.Header().Get("X-Nomad-KnownLeader") != "true" {
			t.Fatalf("missing known leader")
		}
		if respW.Header().Get("X-Nomad-LastContact") == "" {
			t.Fatalf("missing last contact")
		}

		// Check the policy
		require.Equal(p.ID, obj.(*structs.ScalingPolicy).ID)
	})
}
