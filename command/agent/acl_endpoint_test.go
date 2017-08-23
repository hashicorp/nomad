package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
)

func TestHTTP_ACLPolicyList(t *testing.T) {
	t.Parallel()
	httpACLTest(t, nil, func(s *TestAgent) {
		p1 := mock.ACLPolicy()
		p2 := mock.ACLPolicy()
		p3 := mock.ACLPolicy()
		args := structs.ACLPolicyUpsertRequest{
			Policies: []*structs.ACLPolicy{p1, p2, p3},
			WriteRequest: structs.WriteRequest{
				Region:   "global",
				SecretID: s.Token.SecretID,
			},
		}
		var resp structs.GenericResponse
		if err := s.Agent.RPC("ACL.UpsertPolicies", &args, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make the HTTP request
		req, err := http.NewRequest("GET", "/v1/acl/policies", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()
		setToken(req, s.Token)

		// Make the request
		obj, err := s.Server.ACLPoliciesRequest(respW, req)
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
		n := obj.([]*structs.ACLPolicyListStub)
		if len(n) != 3 {
			t.Fatalf("bad: %#v", n)
		}
	})
}

func TestHTTP_ACLPolicyQuery(t *testing.T) {
	t.Parallel()
	httpACLTest(t, nil, func(s *TestAgent) {
		p1 := mock.ACLPolicy()
		args := structs.ACLPolicyUpsertRequest{
			Policies: []*structs.ACLPolicy{p1},
			WriteRequest: structs.WriteRequest{
				Region:   "global",
				SecretID: s.Token.SecretID,
			},
		}
		var resp structs.GenericResponse
		if err := s.Agent.RPC("ACL.UpsertPolicies", &args, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make the HTTP request
		req, err := http.NewRequest("GET", "/v1/acl/policy/"+p1.Name, nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()
		setToken(req, s.Token)

		// Make the request
		obj, err := s.Server.ACLPolicySpecificRequest(respW, req)
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
		n := obj.(*structs.ACLPolicy)
		if n.Name != p1.Name {
			t.Fatalf("bad: %#v", n)
		}
	})
}

func TestHTTP_ACLPolicyCreate(t *testing.T) {
	t.Parallel()
	httpACLTest(t, nil, func(s *TestAgent) {
		// Make the HTTP request
		p1 := mock.ACLPolicy()
		buf := encodeReq(p1)
		req, err := http.NewRequest("PUT", "/v1/acl/policy/"+p1.Name, buf)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()
		setToken(req, s.Token)

		// Make the request
		obj, err := s.Server.ACLPolicySpecificRequest(respW, req)
		assert.Nil(t, err)
		assert.Nil(t, obj)

		// Check for the index
		if respW.HeaderMap.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}

		// Check policy was created
		state := s.Agent.server.State()
		out, err := state.ACLPolicyByName(nil, p1.Name)
		assert.Nil(t, err)
		assert.NotNil(t, out)

		p1.CreateIndex, p1.ModifyIndex = out.CreateIndex, out.ModifyIndex
		assert.Equal(t, p1.Name, out.Name)
		assert.Equal(t, p1, out)
	})
}

func TestHTTP_ACLPolicyDelete(t *testing.T) {
	t.Parallel()
	httpACLTest(t, nil, func(s *TestAgent) {
		p1 := mock.ACLPolicy()
		args := structs.ACLPolicyUpsertRequest{
			Policies: []*structs.ACLPolicy{p1},
			WriteRequest: structs.WriteRequest{
				Region:   "global",
				SecretID: s.Token.SecretID,
			},
		}
		var resp structs.GenericResponse
		if err := s.Agent.RPC("ACL.UpsertPolicies", &args, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make the HTTP request
		req, err := http.NewRequest("DELETE", "/v1/acl/policy/"+p1.Name, nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()
		setToken(req, s.Token)

		// Make the request
		obj, err := s.Server.ACLPolicySpecificRequest(respW, req)
		assert.Nil(t, err)
		assert.Nil(t, obj)

		// Check for the index
		if respW.HeaderMap.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}

		// Check policy was created
		state := s.Agent.server.State()
		out, err := state.ACLPolicyByName(nil, p1.Name)
		assert.Nil(t, err)
		assert.Nil(t, out)
	})
}

func TestHTTP_ACLTokenBootstrap(t *testing.T) {
	t.Parallel()
	conf := func(c *Config) {
		c.ACL.Enabled = true
		c.ACL.PolicyTTL = 0 // Special flag to disable auto-bootstrap
	}
	httpTest(t, conf, func(s *TestAgent) {
		// Make the HTTP request
		req, err := http.NewRequest("PUT", "/v1/acl/bootstrap", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.ACLTokenBootstrap(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check for the index
		if respW.HeaderMap.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}

		// Check the output
		n := obj.(*structs.ACLToken)
		assert.NotNil(t, n)
		assert.Equal(t, "Bootstrap Token", n.Name)
	})
}

func TestHTTP_ACLTokenList(t *testing.T) {
	t.Parallel()
	httpACLTest(t, nil, func(s *TestAgent) {
		p1 := mock.ACLToken()
		p1.AccessorID = ""
		p2 := mock.ACLToken()
		p2.AccessorID = ""
		p3 := mock.ACLToken()
		p3.AccessorID = ""
		args := structs.ACLTokenUpsertRequest{
			Tokens: []*structs.ACLToken{p1, p2, p3},
			WriteRequest: structs.WriteRequest{
				Region:   "global",
				SecretID: s.Token.SecretID,
			},
		}
		var resp structs.ACLTokenUpsertResponse
		if err := s.Agent.RPC("ACL.UpsertTokens", &args, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make the HTTP request
		req, err := http.NewRequest("GET", "/v1/acl/tokens", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()
		setToken(req, s.Token)

		// Make the request
		obj, err := s.Server.ACLTokensRequest(respW, req)
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

		// Check the output (includes boostrap token)
		n := obj.([]*structs.ACLTokenListStub)
		if len(n) != 4 {
			t.Fatalf("bad: %#v", n)
		}
	})
}

func TestHTTP_ACLTokenQuery(t *testing.T) {
	t.Parallel()
	httpACLTest(t, nil, func(s *TestAgent) {
		p1 := mock.ACLToken()
		p1.AccessorID = ""
		args := structs.ACLTokenUpsertRequest{
			Tokens: []*structs.ACLToken{p1},
			WriteRequest: structs.WriteRequest{
				Region:   "global",
				SecretID: s.Token.SecretID,
			},
		}
		var resp structs.ACLTokenUpsertResponse
		if err := s.Agent.RPC("ACL.UpsertTokens", &args, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}
		out := resp.Tokens[0]

		// Make the HTTP request
		req, err := http.NewRequest("GET", "/v1/acl/token/"+out.AccessorID, nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()
		setToken(req, s.Token)

		// Make the request
		obj, err := s.Server.ACLTokenSpecificRequest(respW, req)
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
		n := obj.(*structs.ACLToken)
		assert.Equal(t, out, n)
	})
}

func TestHTTP_ACLTokenCreate(t *testing.T) {
	t.Parallel()
	httpACLTest(t, nil, func(s *TestAgent) {
		// Make the HTTP request
		p1 := mock.ACLToken()
		p1.AccessorID = ""
		buf := encodeReq(p1)
		req, err := http.NewRequest("PUT", "/v1/acl/token", buf)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()
		setToken(req, s.Token)

		// Make the request
		obj, err := s.Server.ACLTokenSpecificRequest(respW, req)
		assert.Nil(t, err)
		assert.NotNil(t, obj)
		outTK := obj.(*structs.ACLToken)

		// Check for the index
		if respW.HeaderMap.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}

		// Check token was created
		state := s.Agent.server.State()
		out, err := state.ACLTokenByAccessorID(nil, outTK.AccessorID)
		assert.Nil(t, err)
		assert.NotNil(t, out)
		assert.Equal(t, outTK, out)
	})
}

func TestHTTP_ACLTokenDelete(t *testing.T) {
	t.Parallel()
	httpACLTest(t, nil, func(s *TestAgent) {
		p1 := mock.ACLToken()
		p1.AccessorID = ""
		args := structs.ACLTokenUpsertRequest{
			Tokens: []*structs.ACLToken{p1},
			WriteRequest: structs.WriteRequest{
				Region:   "global",
				SecretID: s.Token.SecretID,
			},
		}
		var resp structs.ACLTokenUpsertResponse
		if err := s.Agent.RPC("ACL.UpsertTokens", &args, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}
		ID := resp.Tokens[0].AccessorID

		// Make the HTTP request
		req, err := http.NewRequest("DELETE", "/v1/acl/token/"+ID, nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()
		setToken(req, s.Token)

		// Make the request
		obj, err := s.Server.ACLTokenSpecificRequest(respW, req)
		assert.Nil(t, err)
		assert.Nil(t, obj)

		// Check for the index
		if respW.HeaderMap.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}

		// Check token was created
		state := s.Agent.server.State()
		out, err := state.ACLTokenByAccessorID(nil, ID)
		assert.Nil(t, err)
		assert.Nil(t, out)
	})
}
