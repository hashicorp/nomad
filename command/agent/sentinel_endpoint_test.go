// +build ent

package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
)

func TestHTTP_SentinelPolicyList(t *testing.T) {
	t.Parallel()
	httpACLTest(t, nil, func(s *TestAgent) {
		p1 := mock.SentinelPolicy()
		p2 := mock.SentinelPolicy()
		p3 := mock.SentinelPolicy()
		args := structs.SentinelPolicyUpsertRequest{
			Policies: []*structs.SentinelPolicy{p1, p2, p3},
			WriteRequest: structs.WriteRequest{
				Region:   "global",
				SecretID: s.Token.SecretID,
			},
		}
		var resp structs.GenericResponse
		if err := s.Agent.RPC("Sentinel.UpsertPolicies", &args, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make the HTTP request
		req, err := http.NewRequest("GET", "/v1/sentinel/policies", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()
		setToken(req, s.Token)

		// Make the request
		obj, err := s.Server.SentinelPoliciesRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check for the index
		if respW.HeaderMap.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}
		if respW.HeaderMap.Get("X-Nomad-KnownLeader") != "true" {
			t.Fatalf("missing known leader")
		}
		if respW.HeaderMap.Get("X-Nomad-LastContact") == "" {
			t.Fatalf("missing last contact")
		}

		// Check the output
		n := obj.([]*structs.SentinelPolicyListStub)
		if len(n) != 3 {
			t.Fatalf("bad: %#v", n)
		}
	})
}

func TestHTTP_SentinelPolicyQuery(t *testing.T) {
	t.Parallel()
	httpACLTest(t, nil, func(s *TestAgent) {
		p1 := mock.SentinelPolicy()
		args := structs.SentinelPolicyUpsertRequest{
			Policies: []*structs.SentinelPolicy{p1},
			WriteRequest: structs.WriteRequest{
				Region:   "global",
				SecretID: s.Token.SecretID,
			},
		}
		var resp structs.GenericResponse
		if err := s.Agent.RPC("Sentinel.UpsertPolicies", &args, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make the HTTP request
		req, err := http.NewRequest("GET", "/v1/sentinel/policy/"+p1.Name, nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()
		setToken(req, s.Token)

		// Make the request
		obj, err := s.Server.SentinelPolicySpecificRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check for the index
		if respW.HeaderMap.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}
		if respW.HeaderMap.Get("X-Nomad-KnownLeader") != "true" {
			t.Fatalf("missing known leader")
		}
		if respW.HeaderMap.Get("X-Nomad-LastContact") == "" {
			t.Fatalf("missing last contact")
		}

		// Check the output
		n := obj.(*structs.SentinelPolicy)
		if n.Name != p1.Name {
			t.Fatalf("bad: %#v", n)
		}
	})
}

func TestHTTP_SentinelPolicyCreate(t *testing.T) {
	t.Parallel()
	httpACLTest(t, nil, func(s *TestAgent) {
		// Make the HTTP request
		p1 := mock.SentinelPolicy()
		buf := encodeReq(p1)
		req, err := http.NewRequest("PUT", "/v1/sentinel/policy/"+p1.Name, buf)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()
		setToken(req, s.Token)

		// Make the request
		obj, err := s.Server.SentinelPolicySpecificRequest(respW, req)
		assert.Nil(t, err)
		assert.Nil(t, obj)

		// Check for the index
		if respW.HeaderMap.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}

		// Check policy was created
		state := s.Agent.server.State()
		out, err := state.SentinelPolicyByName(nil, p1.Name)
		assert.Nil(t, err)
		assert.NotNil(t, out)

		p1.CreateIndex, p1.ModifyIndex = out.CreateIndex, out.ModifyIndex
		assert.Equal(t, p1.Name, out.Name)
		assert.Equal(t, p1, out)
	})
}

func TestHTTP_SentinelPolicyDelete(t *testing.T) {
	t.Parallel()
	httpACLTest(t, nil, func(s *TestAgent) {
		p1 := mock.SentinelPolicy()
		args := structs.SentinelPolicyUpsertRequest{
			Policies: []*structs.SentinelPolicy{p1},
			WriteRequest: structs.WriteRequest{
				Region:   "global",
				SecretID: s.Token.SecretID,
			},
		}
		var resp structs.GenericResponse
		if err := s.Agent.RPC("Sentinel.UpsertPolicies", &args, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make the HTTP request
		req, err := http.NewRequest("DELETE", "/v1/sentinel/policy/"+p1.Name, nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()
		setToken(req, s.Token)

		// Make the request
		obj, err := s.Server.SentinelPolicySpecificRequest(respW, req)
		assert.Nil(t, err)
		assert.Nil(t, obj)

		// Check for the index
		if respW.HeaderMap.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}

		// Check policy was created
		state := s.Agent.server.State()
		out, err := state.SentinelPolicyByName(nil, p1.Name)
		assert.Nil(t, err)
		assert.Nil(t, out)
	})
}
