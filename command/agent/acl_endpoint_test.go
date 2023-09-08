// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	capOIDC "github.com/hashicorp/cap/oidc"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTP_ACLPolicyList(t *testing.T) {
	ci.Parallel(t)
	httpACLTest(t, nil, func(s *TestAgent) {
		p1 := mock.ACLPolicy()
		p2 := mock.ACLPolicy()
		p3 := mock.ACLPolicy()
		args := structs.ACLPolicyUpsertRequest{
			Policies: []*structs.ACLPolicy{p1, p2, p3},
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				AuthToken: s.RootToken.SecretID,
			},
		}
		var resp structs.GenericResponse
		if err := s.Agent.RPC("ACL.UpsertPolicies", &args, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodGet, "/v1/acl/policies", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()
		setToken(req, s.RootToken)

		// Make the request
		obj, err := s.Server.ACLPoliciesRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check for the index
		if respW.Result().Header.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}
		if respW.Result().Header.Get("X-Nomad-KnownLeader") != "true" {
			t.Fatalf("missing known leader")
		}
		if respW.Result().Header.Get("X-Nomad-LastContact") == "" {
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
	ci.Parallel(t)
	httpACLTest(t, nil, func(s *TestAgent) {
		p1 := mock.ACLPolicy()
		args := structs.ACLPolicyUpsertRequest{
			Policies: []*structs.ACLPolicy{p1},
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				AuthToken: s.RootToken.SecretID,
			},
		}
		var resp structs.GenericResponse
		if err := s.Agent.RPC("ACL.UpsertPolicies", &args, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodGet, "/v1/acl/policy/"+p1.Name, nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()
		setToken(req, s.RootToken)

		// Make the request
		obj, err := s.Server.ACLPolicySpecificRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check for the index
		if respW.Result().Header.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}
		if respW.Result().Header.Get("X-Nomad-KnownLeader") != "true" {
			t.Fatalf("missing known leader")
		}
		if respW.Result().Header.Get("X-Nomad-LastContact") == "" {
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
	ci.Parallel(t)
	httpACLTest(t, nil, func(s *TestAgent) {
		// Make the HTTP request
		p1 := mock.ACLPolicy()
		buf := encodeReq(p1)
		req, err := http.NewRequest(http.MethodPut, "/v1/acl/policy/"+p1.Name, buf)
		must.NoError(t, err)

		respW := httptest.NewRecorder()
		setToken(req, s.RootToken)

		// Make the request
		obj, err := s.Server.ACLPolicySpecificRequest(respW, req)
		must.NoError(t, err)
		must.Nil(t, obj)

		// Check for the index
		must.StrNotEqFold(t, "", respW.Result().Header.Get("X-Nomad-Index"))

		// Check policy was created
		state := s.Agent.server.State()
		out, err := state.ACLPolicyByName(nil, p1.Name)
		must.NoError(t, err)
		must.NotNil(t, out)

		p1.CreateIndex, p1.ModifyIndex = out.CreateIndex, out.ModifyIndex
		must.Eq(t, p1.Name, out.Name)
		must.Eq(t, p1, out)

		// Create a policy that is invalid. This ensures we call the validation
		// func in the RPC handler, also that the correct code and error is
		// returned.
		aclPolicy2 := mock.ACLPolicy()
		aclPolicy2.Rules = "invalid"

		aclPolicy2Req, err := http.NewRequest(http.MethodPut, "/v1/acl/policy/"+aclPolicy2.Name, encodeReq(aclPolicy2))
		must.NoError(t, err)

		respW = httptest.NewRecorder()
		setToken(aclPolicy2Req, s.RootToken)

		// Make the request
		aclPolicy2Obj, err := s.Server.ACLPolicySpecificRequest(respW, aclPolicy2Req)
		must.ErrorContains(t, err, "400")
		must.ErrorContains(t, err, "failed to parse rules")
		must.Nil(t, aclPolicy2Obj)
	})
}

func TestHTTP_ACLPolicyDelete(t *testing.T) {
	ci.Parallel(t)
	httpACLTest(t, nil, func(s *TestAgent) {
		p1 := mock.ACLPolicy()
		args := structs.ACLPolicyUpsertRequest{
			Policies: []*structs.ACLPolicy{p1},
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				AuthToken: s.RootToken.SecretID,
			},
		}
		var resp structs.GenericResponse
		if err := s.Agent.RPC("ACL.UpsertPolicies", &args, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodDelete, "/v1/acl/policy/"+p1.Name, nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()
		setToken(req, s.RootToken)

		// Make the request
		obj, err := s.Server.ACLPolicySpecificRequest(respW, req)
		assert.Nil(t, err)
		assert.Nil(t, obj)

		// Check for the index
		if respW.Result().Header.Get("X-Nomad-Index") == "" {
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
	ci.Parallel(t)
	conf := func(c *Config) {
		c.ACL.Enabled = true
		c.ACL.PolicyTTL = 0 // Special flag to disable auto-bootstrap
	}
	httpTest(t, conf, func(s *TestAgent) {
		// Make the HTTP request
		req, err := http.NewRequest(http.MethodPut, "/v1/acl/bootstrap", nil)
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
		if respW.Result().Header.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}

		// Check the output
		n := obj.(*structs.ACLToken)
		assert.NotNil(t, n)
		assert.Equal(t, "Bootstrap Token", n.Name)
	})
}

func TestHTTP_ACLTokenBootstrapOperator(t *testing.T) {
	ci.Parallel(t)
	conf := func(c *Config) {
		c.ACL.Enabled = true
		c.ACL.PolicyTTL = 0 // Special flag to disable auto-bootstrap
	}
	httpTest(t, conf, func(s *TestAgent) {
		// Provide token
		args := structs.ACLTokenBootstrapRequest{
			BootstrapSecret: "2b778dd9-f5f1-6f29-b4b4-9a5fa948757a",
		}

		buf := encodeReq(args)

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodPut, "/v1/acl/bootstrap", buf)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Since we're not actually writing this HTTP request, we have
		// to manually set ContentLength
		req.ContentLength = -1

		respW := httptest.NewRecorder()
		// Make the request
		obj, err := s.Server.ACLTokenBootstrap(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check for the index
		if respW.Result().Header.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}

		// Check the output
		n := obj.(*structs.ACLToken)
		assert.NotNil(t, n)
		assert.Equal(t, args.BootstrapSecret, n.SecretID)
	})
}

func TestHTTP_ACLTokenList(t *testing.T) {
	ci.Parallel(t)
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
				Region:    "global",
				AuthToken: s.RootToken.SecretID,
			},
		}
		var resp structs.ACLTokenUpsertResponse
		if err := s.Agent.RPC("ACL.UpsertTokens", &args, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodGet, "/v1/acl/tokens", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()
		setToken(req, s.RootToken)

		// Make the request
		obj, err := s.Server.ACLTokensRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check for the index
		if respW.Result().Header.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}
		if respW.Result().Header.Get("X-Nomad-KnownLeader") != "true" {
			t.Fatalf("missing known leader")
		}
		if respW.Result().Header.Get("X-Nomad-LastContact") == "" {
			t.Fatalf("missing last contact")
		}

		// Check the output (includes bootstrap token)
		n := obj.([]*structs.ACLTokenListStub)
		if len(n) != 4 {
			t.Fatalf("bad: %#v", n)
		}
	})
}

func TestHTTP_ACLTokenQuery(t *testing.T) {
	ci.Parallel(t)
	httpACLTest(t, nil, func(s *TestAgent) {
		p1 := mock.ACLToken()
		p1.AccessorID = ""
		args := structs.ACLTokenUpsertRequest{
			Tokens: []*structs.ACLToken{p1},
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				AuthToken: s.RootToken.SecretID,
			},
		}
		var resp structs.ACLTokenUpsertResponse
		if err := s.Agent.RPC("ACL.UpsertTokens", &args, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}
		out := resp.Tokens[0]

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodGet, "/v1/acl/token/"+out.AccessorID, nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()
		setToken(req, s.RootToken)

		// Make the request
		obj, err := s.Server.ACLTokenSpecificRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check for the index
		if respW.Result().Header.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}
		if respW.Result().Header.Get("X-Nomad-KnownLeader") != "true" {
			t.Fatalf("missing known leader")
		}
		if respW.Result().Header.Get("X-Nomad-LastContact") == "" {
			t.Fatalf("missing last contact")
		}

		// Check the output
		n := obj.(*structs.ACLToken)
		assert.Equal(t, out, n)
	})
}

func TestHTTP_ACLTokenSelf(t *testing.T) {
	ci.Parallel(t)
	httpACLTest(t, nil, func(s *TestAgent) {
		p1 := mock.ACLToken()
		p1.AccessorID = ""
		args := structs.ACLTokenUpsertRequest{
			Tokens: []*structs.ACLToken{p1},
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				AuthToken: s.RootToken.SecretID,
			},
		}
		var resp structs.ACLTokenUpsertResponse
		if err := s.Agent.RPC("ACL.UpsertTokens", &args, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}
		out := resp.Tokens[0]

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodGet, "/v1/acl/token/self", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()
		setToken(req, out)

		// Make the request
		obj, err := s.Server.ACLTokenSpecificRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check for the index
		if respW.Result().Header.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}
		if respW.Result().Header.Get("X-Nomad-KnownLeader") != "true" {
			t.Fatalf("missing known leader")
		}
		if respW.Result().Header.Get("X-Nomad-LastContact") == "" {
			t.Fatalf("missing last contact")
		}

		// Check the output
		n := obj.(*structs.ACLToken)
		assert.Equal(t, out, n)
	})
}

func TestHTTP_ACLTokenCreate(t *testing.T) {
	ci.Parallel(t)
	httpACLTest(t, nil, func(s *TestAgent) {
		// Make the HTTP request
		p1 := mock.ACLToken()
		p1.AccessorID = ""
		buf := encodeReq(p1)
		req, err := http.NewRequest(http.MethodPut, "/v1/acl/token", buf)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()
		setToken(req, s.RootToken)

		// Make the request
		obj, err := s.Server.ACLTokenSpecificRequest(respW, req)
		assert.Nil(t, err)
		assert.NotNil(t, obj)
		outTK := obj.(*structs.ACLToken)

		// Check for the index
		if respW.Result().Header.Get("X-Nomad-Index") == "" {
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

func TestHTTP_ACLTokenCreateExpirationTTL(t *testing.T) {
	ci.Parallel(t)
	httpACLTest(t, nil, func(s *TestAgent) {

		// Generate an example token which has an expiration TTL in string
		// format.
		aclToken := `
{
  "Name": "Readonly token",
  "Type": "client",
  "Policies": ["readonly"],
  "ExpirationTTL": "10h",
  "Global": false
}`

		req, err := http.NewRequest(http.MethodPut, "/v1/acl/token", bytes.NewReader([]byte(aclToken)))
		must.NoError(t, err)

		respW := httptest.NewRecorder()
		setToken(req, s.RootToken)

		// Make the request.
		obj, err := s.Server.ACLTokenSpecificRequest(respW, req)
		must.NoError(t, err)
		must.NotNil(t, obj)

		// Ensure the returned token includes expiration.
		createdTokenResp := obj.(*structs.ACLToken)
		must.Eq(t, "10h0m0s", createdTokenResp.ExpirationTTL.String())
		must.False(t, createdTokenResp.CreateTime.IsZero())

		// Check for the index.
		must.StrNotEqFold(t, "", respW.Result().Header.Get("X-Nomad-Index"))

		// Check token was created and stored properly within state.
		out, err := s.Agent.server.State().ACLTokenByAccessorID(nil, createdTokenResp.AccessorID)
		must.NoError(t, err)
		must.NotNil(t, out)
		must.Eq(t, createdTokenResp, out)
	})
}

func TestHTTP_ACLTokenDelete(t *testing.T) {
	ci.Parallel(t)
	httpACLTest(t, nil, func(s *TestAgent) {
		p1 := mock.ACLToken()
		p1.AccessorID = ""
		args := structs.ACLTokenUpsertRequest{
			Tokens: []*structs.ACLToken{p1},
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				AuthToken: s.RootToken.SecretID,
			},
		}
		var resp structs.ACLTokenUpsertResponse
		if err := s.Agent.RPC("ACL.UpsertTokens", &args, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}
		ID := resp.Tokens[0].AccessorID

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodDelete, "/v1/acl/token/"+ID, nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()
		setToken(req, s.RootToken)

		// Make the request
		obj, err := s.Server.ACLTokenSpecificRequest(respW, req)
		assert.Nil(t, err)
		assert.Nil(t, obj)

		// Check for the index
		if respW.Result().Header.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}

		// Check token was created
		state := s.Agent.server.State()
		out, err := state.ACLTokenByAccessorID(nil, ID)
		assert.Nil(t, err)
		assert.Nil(t, out)
	})
}

func TestHTTP_OneTimeToken(t *testing.T) {
	ci.Parallel(t)
	httpACLTest(t, nil, func(s *TestAgent) {

		// Setup the ACL token

		p1 := mock.ACLToken()
		p1.AccessorID = ""
		args := structs.ACLTokenUpsertRequest{
			Tokens: []*structs.ACLToken{p1},
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				AuthToken: s.RootToken.SecretID,
			},
		}
		var resp structs.ACLTokenUpsertResponse
		err := s.Agent.RPC("ACL.UpsertTokens", &args, &resp)
		require.NoError(t, err)
		aclID := resp.Tokens[0].AccessorID
		aclSecret := resp.Tokens[0].SecretID

		// Make a HTTP request to get a one-time token

		req, err := http.NewRequest(http.MethodPost, "/v1/acl/token/onetime", nil)
		require.NoError(t, err)
		req.Header.Set("X-Nomad-Token", aclSecret)
		respW := httptest.NewRecorder()

		obj, err := s.Server.UpsertOneTimeToken(respW, req)
		require.NoError(t, err)
		require.NotNil(t, obj)

		ott := obj.(structs.OneTimeTokenUpsertResponse)
		require.Equal(t, aclID, ott.OneTimeToken.AccessorID)
		require.NotEqual(t, "", ott.OneTimeToken.OneTimeSecretID)

		// Make a HTTP request to exchange that token

		buf := encodeReq(structs.OneTimeTokenExchangeRequest{
			OneTimeSecretID: ott.OneTimeToken.OneTimeSecretID})
		req, err = http.NewRequest(http.MethodPost, "/v1/acl/token/onetime/exchange", buf)
		require.NoError(t, err)
		respW = httptest.NewRecorder()

		obj, err = s.Server.ExchangeOneTimeToken(respW, req)
		require.NoError(t, err)
		require.NotNil(t, obj)

		token := obj.(structs.OneTimeTokenExchangeResponse)
		require.Equal(t, aclID, token.Token.AccessorID)
		require.Equal(t, aclSecret, token.Token.SecretID)

		// Making the same request a second time should return an error

		buf = encodeReq(structs.OneTimeTokenExchangeRequest{
			OneTimeSecretID: ott.OneTimeToken.OneTimeSecretID})
		req, err = http.NewRequest(http.MethodPost, "/v1/acl/token/onetime/exchange", buf)
		require.NoError(t, err)
		respW = httptest.NewRecorder()

		obj, err = s.Server.ExchangeOneTimeToken(respW, req)
		require.EqualError(t, err, structs.ErrPermissionDenied.Error())
	})
}

func TestHTTPServer_ACLRoleListRequest(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name   string
		testFn func(srv *TestAgent)
	}{
		{
			name: "no auth token set",
			testFn: func(srv *TestAgent) {

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodGet, "/v1/acl/roles", nil)
				require.NoError(t, err)
				respW := httptest.NewRecorder()

				// Send the HTTP request.
				obj, err := srv.Server.ACLRoleListRequest(respW, req)
				require.NoError(t, err)
				require.Empty(t, obj)
			},
		},
		{
			name: "invalid method",
			testFn: func(srv *TestAgent) {

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodConnect, "/v1/acl/roles", nil)
				require.NoError(t, err)
				respW := httptest.NewRecorder()

				// Ensure we have a token set.
				setToken(req, srv.RootToken)

				// Send the HTTP request.
				obj, err := srv.Server.ACLRoleListRequest(respW, req)
				require.Error(t, err)
				require.ErrorContains(t, err, "Invalid method")
				require.Nil(t, obj)
			},
		},
		{
			name: "no roles in state",
			testFn: func(srv *TestAgent) {

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodGet, "/v1/acl/roles", nil)
				require.NoError(t, err)
				respW := httptest.NewRecorder()

				// Ensure we have a token set.
				setToken(req, srv.RootToken)

				// Send the HTTP request.
				obj, err := srv.Server.ACLRoleListRequest(respW, req)
				require.NoError(t, err)
				require.Empty(t, obj.([]*structs.ACLRoleListStub))
			},
		},
		{
			name: "roles in state",
			testFn: func(srv *TestAgent) {

				// Create the policies our ACL roles wants to link to.
				policy1 := mock.ACLPolicy()
				policy1.Name = "mocked-test-policy-1"
				policy2 := mock.ACLPolicy()
				policy2.Name = "mocked-test-policy-2"

				require.NoError(t, srv.server.State().UpsertACLPolicies(
					structs.MsgTypeTestSetup, 10, []*structs.ACLPolicy{policy1, policy2}))

				// Create two ACL roles and put these directly into state.
				aclRoles := []*structs.ACLRole{mock.ACLRole(), mock.ACLRole()}
				require.NoError(t, srv.server.State().UpsertACLRoles(structs.MsgTypeTestSetup, 20, aclRoles, false))

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodGet, "/v1/acl/roles", nil)
				require.NoError(t, err)
				respW := httptest.NewRecorder()

				// Ensure we have a token set.
				setToken(req, srv.RootToken)

				// Send the HTTP request.
				obj, err := srv.Server.ACLRoleListRequest(respW, req)
				require.NoError(t, err)
				require.Len(t, obj.([]*structs.ACLRoleListStub), 2)
			},
		},
		{
			name: "roles in state using prefix",
			testFn: func(srv *TestAgent) {

				// Create the policies our ACL roles wants to link to.
				policy1 := mock.ACLPolicy()
				policy1.Name = "mocked-test-policy-1"
				policy2 := mock.ACLPolicy()
				policy2.Name = "mocked-test-policy-2"

				require.NoError(t, srv.server.State().UpsertACLPolicies(
					structs.MsgTypeTestSetup, 10, []*structs.ACLPolicy{policy1, policy2}))

				// Create two ACL roles and put these directly into state, one
				// using a custom prefix.
				aclRoles := []*structs.ACLRole{mock.ACLRole(), mock.ACLRole()}
				aclRoles[1].ID = "badger-badger-badger-" + uuid.Generate()
				require.NoError(t, srv.server.State().UpsertACLRoles(structs.MsgTypeTestSetup, 20, aclRoles, false))

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodGet, "/v1/acl/roles?prefix=badger-badger-badger", nil)
				require.NoError(t, err)
				respW := httptest.NewRecorder()

				// Ensure we have a token set.
				setToken(req, srv.RootToken)

				// Send the HTTP request.
				obj, err := srv.Server.ACLRoleListRequest(respW, req)
				require.NoError(t, err)
				require.Len(t, obj.([]*structs.ACLRoleListStub), 1)
				require.Contains(t, obj.([]*structs.ACLRoleListStub)[0].ID, "badger-badger-badger")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			httpACLTest(t, nil, tc.testFn)
		})
	}
}

func TestHTTPServer_ACLRoleRequest(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name   string
		testFn func(srv *TestAgent)
	}{
		{
			name: "no auth token set",
			testFn: func(srv *TestAgent) {

				// Create a mock role to use in the request body.
				mockACLRole := mock.ACLRole()
				mockACLRole.ID = ""

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodPut, "/v1/acl/role", encodeReq(mockACLRole))
				require.NoError(t, err)
				respW := httptest.NewRecorder()

				// Send the HTTP request.
				obj, err := srv.Server.ACLRoleRequest(respW, req)
				require.Error(t, err)
				require.ErrorContains(t, err, "Permission denied")
				require.Nil(t, obj)
			},
		},
		{
			name: "invalid method",
			testFn: func(srv *TestAgent) {

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodConnect, "/v1/acl/role", nil)
				require.NoError(t, err)
				respW := httptest.NewRecorder()

				// Ensure we have a token set.
				setToken(req, srv.RootToken)

				// Send the HTTP request.
				obj, err := srv.Server.ACLRoleRequest(respW, req)
				require.Error(t, err)
				require.ErrorContains(t, err, "Invalid method")
				require.Nil(t, obj)
			},
		},
		{
			name: "successful upsert",
			testFn: func(srv *TestAgent) {

				// Create the policies our ACL roles wants to link to.
				policy1 := mock.ACLPolicy()
				policy1.Name = "mocked-test-policy-1"
				policy2 := mock.ACLPolicy()
				policy2.Name = "mocked-test-policy-2"

				require.NoError(t, srv.server.State().UpsertACLPolicies(
					structs.MsgTypeTestSetup, 10, []*structs.ACLPolicy{policy1, policy2}))

				// Create a mock role to use in the request body.
				mockACLRole := mock.ACLRole()
				mockACLRole.ID = ""

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodPut, "/v1/acl/role", encodeReq(mockACLRole))
				require.NoError(t, err)
				respW := httptest.NewRecorder()

				// Ensure we have a token set.
				setToken(req, srv.RootToken)

				// Send the HTTP request.
				obj, err := srv.Server.ACLRoleRequest(respW, req)
				require.NoError(t, err)
				require.NotNil(t, obj)
				require.Equal(t, obj.(*structs.ACLRole).Hash, mockACLRole.Hash)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			httpACLTest(t, nil, tc.testFn)
		})
	}
}

func TestHTTPServer_ACLRoleSpecificRequest(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name   string
		testFn func(srv *TestAgent)
	}{
		{
			name: "invalid URI",
			testFn: func(srv *TestAgent) {

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodGet, "/v1/acl/role/name/this/is/will/not/work", nil)
				require.NoError(t, err)
				respW := httptest.NewRecorder()

				// Send the HTTP request.
				obj, err := srv.Server.ACLRoleSpecificRequest(respW, req)
				require.Error(t, err)
				require.ErrorContains(t, err, "invalid URI")
				require.Nil(t, obj)
			},
		},
		{
			name: "invalid role name lookalike URI",
			testFn: func(srv *TestAgent) {

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodGet, "/v1/acl/role/foobar/rolename", nil)
				require.NoError(t, err)
				respW := httptest.NewRecorder()

				// Send the HTTP request.
				obj, err := srv.Server.ACLRoleSpecificRequest(respW, req)
				require.Error(t, err)
				require.ErrorContains(t, err, "invalid URI")
				require.Nil(t, obj)
			},
		},
		{
			name: "missing role name",
			testFn: func(srv *TestAgent) {

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodGet, "/v1/acl/role/name/", nil)
				require.NoError(t, err)
				respW := httptest.NewRecorder()

				// Send the HTTP request.
				obj, err := srv.Server.ACLRoleSpecificRequest(respW, req)
				require.Error(t, err)
				require.ErrorContains(t, err, "missing ACL role name")
				require.Nil(t, obj)
			},
		},
		{
			name: "missing role ID",
			testFn: func(srv *TestAgent) {

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodGet, "/v1/acl/role/", nil)
				require.NoError(t, err)
				respW := httptest.NewRecorder()

				// Send the HTTP request.
				obj, err := srv.Server.ACLRoleSpecificRequest(respW, req)
				require.Error(t, err)
				require.ErrorContains(t, err, "missing ACL role ID")
				require.Nil(t, obj)
			},
		},
		{
			name: "role name incorrect method",
			testFn: func(srv *TestAgent) {

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodConnect, "/v1/acl/role/name/foobar", nil)
				require.NoError(t, err)
				respW := httptest.NewRecorder()

				// Send the HTTP request.
				obj, err := srv.Server.ACLRoleSpecificRequest(respW, req)
				require.Error(t, err)
				require.ErrorContains(t, err, "Invalid method")
				require.Nil(t, obj)
			},
		},
		{
			name: "role ID incorrect method",
			testFn: func(srv *TestAgent) {

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodConnect, "/v1/acl/role/foobar", nil)
				require.NoError(t, err)
				respW := httptest.NewRecorder()

				// Send the HTTP request.
				obj, err := srv.Server.ACLRoleSpecificRequest(respW, req)
				require.Error(t, err)
				require.ErrorContains(t, err, "Invalid method")
				require.Nil(t, obj)
			},
		},
		{
			name: "get role by name",
			testFn: func(srv *TestAgent) {

				// Create the policies our ACL roles wants to link to.
				policy1 := mock.ACLPolicy()
				policy1.Name = "mocked-test-policy-1"
				policy2 := mock.ACLPolicy()
				policy2.Name = "mocked-test-policy-2"

				require.NoError(t, srv.server.State().UpsertACLPolicies(
					structs.MsgTypeTestSetup, 10, []*structs.ACLPolicy{policy1, policy2}))

				// Create a mock role and put directly into state.
				mockACLRole := mock.ACLRole()
				require.NoError(t, srv.server.State().UpsertACLRoles(
					structs.MsgTypeTestSetup, 20, []*structs.ACLRole{mockACLRole}, false))

				url := fmt.Sprintf("/v1/acl/role/name/%s", mockACLRole.Name)

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodGet, url, nil)
				require.NoError(t, err)
				respW := httptest.NewRecorder()

				// Ensure we have a token set.
				setToken(req, srv.RootToken)

				// Send the HTTP request.
				obj, err := srv.Server.ACLRoleSpecificRequest(respW, req)
				require.NoError(t, err)
				require.Equal(t, obj.(*structs.ACLRole).Hash, mockACLRole.Hash)
			},
		},
		{
			name: "get, update, and delete role by ID",
			testFn: func(srv *TestAgent) {

				// Create the policies our ACL roles wants to link to.
				policy1 := mock.ACLPolicy()
				policy1.Name = "mocked-test-policy-1"
				policy2 := mock.ACLPolicy()
				policy2.Name = "mocked-test-policy-2"

				require.NoError(t, srv.server.State().UpsertACLPolicies(
					structs.MsgTypeTestSetup, 10, []*structs.ACLPolicy{policy1, policy2}))

				// Create a mock role and put directly into state.
				mockACLRole := mock.ACLRole()
				require.NoError(t, srv.server.State().UpsertACLRoles(
					structs.MsgTypeTestSetup, 20, []*structs.ACLRole{mockACLRole}, false))

				url := fmt.Sprintf("/v1/acl/role/%s", mockACLRole.ID)

				// Build the HTTP request to read the role using its ID.
				req, err := http.NewRequest(http.MethodGet, url, nil)
				require.NoError(t, err)
				respW := httptest.NewRecorder()

				// Ensure we have a token set.
				setToken(req, srv.RootToken)

				// Send the HTTP request.
				obj, err := srv.Server.ACLRoleSpecificRequest(respW, req)
				require.NoError(t, err)
				require.Equal(t, obj.(*structs.ACLRole).Hash, mockACLRole.Hash)

				// Update the role policy list and make the request via the
				// HTTP API.
				mockACLRole.Policies = []*structs.ACLRolePolicyLink{{Name: "mocked-test-policy-1"}}

				req, err = http.NewRequest(http.MethodPost, url, encodeReq(mockACLRole))
				require.NoError(t, err)
				respW = httptest.NewRecorder()

				// Ensure we have a token set.
				setToken(req, srv.RootToken)

				// Send the HTTP request.
				obj, err = srv.Server.ACLRoleSpecificRequest(respW, req)
				require.NoError(t, err)
				require.Equal(t, obj.(*structs.ACLRole).Policies, mockACLRole.Policies)

				// Delete the ACL role using its ID.
				req, err = http.NewRequest(http.MethodDelete, url, nil)
				require.NoError(t, err)
				respW = httptest.NewRecorder()

				// Ensure we have a token set.
				setToken(req, srv.RootToken)

				// Send the HTTP request.
				obj, err = srv.Server.ACLRoleSpecificRequest(respW, req)
				require.NoError(t, err)
				require.Nil(t, obj)

				// Ensure the ACL role is no longer stored within state.
				aclRole, err := srv.server.State().GetACLRoleByID(nil, mockACLRole.ID)
				require.NoError(t, err)
				require.Nil(t, aclRole)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			httpACLTest(t, nil, tc.testFn)
		})
	}
}

func TestHTTPServer_ACLAuthMethodListRequest(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name   string
		testFn func(srv *TestAgent)
	}{
		{
			name: "no auth token set",
			testFn: func(srv *TestAgent) {

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodGet, "/v1/acl/auth-methods", nil)
				must.NoError(t, err)
				respW := httptest.NewRecorder()

				// Send the HTTP request.
				obj, err := srv.Server.ACLAuthMethodListRequest(respW, req)
				must.NoError(t, err)
				must.Len(t, 0, obj.([]*structs.ACLAuthMethodStub))
			},
		},
		{
			name: "invalid method",
			testFn: func(srv *TestAgent) {

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodConnect, "/v1/acl/auth-methods", nil)
				must.NoError(t, err)
				respW := httptest.NewRecorder()

				// Ensure we have a token set.
				setToken(req, srv.RootToken)

				// Send the HTTP request.
				obj, err := srv.Server.ACLAuthMethodListRequest(respW, req)
				must.Error(t, err)
				must.StrContains(t, err.Error(), "Invalid method")
				must.Nil(t, obj)
			},
		},
		{
			name: "no auth-methods in state",
			testFn: func(srv *TestAgent) {

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodGet, "/v1/acl/auth-methods", nil)
				must.NoError(t, err)
				respW := httptest.NewRecorder()

				// Ensure we have a token set.
				setToken(req, srv.RootToken)

				// Send the HTTP request.
				obj, err := srv.Server.ACLAuthMethodListRequest(respW, req)
				must.NoError(t, err)
				must.Len(t, 0, obj.([]*structs.ACLAuthMethodStub))
			},
		},
		{
			name: "auth-methods in state",
			testFn: func(srv *TestAgent) {

				// Upsert two auth-methods into state.
				must.NoError(t, srv.server.State().UpsertACLAuthMethods(
					10, []*structs.ACLAuthMethod{mock.ACLOIDCAuthMethod(), mock.ACLOIDCAuthMethod()}))

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodGet, "/v1/acl/auth-methods", nil)
				must.NoError(t, err)
				respW := httptest.NewRecorder()

				// Ensure we have a token set.
				setToken(req, srv.RootToken)

				// Send the HTTP request.
				obj, err := srv.Server.ACLAuthMethodListRequest(respW, req)
				must.NoError(t, err)
				must.Len(t, 2, obj.([]*structs.ACLAuthMethodStub))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			httpACLTest(t, nil, tc.testFn)
		})
	}
}

func TestHTTPServer_ACLAuthMethodRequest(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name   string
		testFn func(srv *TestAgent)
	}{
		{
			name: "no auth token set",
			testFn: func(srv *TestAgent) {

				// Create a mock role to use in the request body.
				mockACLRole := mock.ACLRole()
				mockACLRole.ID = ""

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodPut, "/v1/acl/auth-method", encodeReq(mockACLRole))
				must.NoError(t, err)
				respW := httptest.NewRecorder()

				// Send the HTTP request.
				obj, err := srv.Server.ACLAuthMethodRequest(respW, req)
				must.Error(t, err)
				must.StrContains(t, err.Error(), "Permission denied")
				must.Nil(t, obj)
			},
		},
		{
			name: "invalid method",
			testFn: func(srv *TestAgent) {

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodConnect, "/v1/acl/auth-method", nil)
				must.NoError(t, err)
				respW := httptest.NewRecorder()

				// Ensure we have a token set.
				setToken(req, srv.RootToken)

				// Send the HTTP request.
				obj, err := srv.Server.ACLAuthMethodRequest(respW, req)
				must.Error(t, err)
				must.StrContains(t, err.Error(), "Invalid method")
				must.Nil(t, obj)
			},
		},
		{
			name: "successful upsert",
			testFn: func(srv *TestAgent) {

				// Create a mock auth-method to use in the request body.
				mockACLAuthMethod := mock.ACLOIDCAuthMethod()

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodPut, "/v1/acl/auth-method", encodeReq(mockACLAuthMethod))
				must.NoError(t, err)
				respW := httptest.NewRecorder()

				// Ensure we have a token set.
				setToken(req, srv.RootToken)

				// Send the HTTP request.
				obj, err := srv.Server.ACLAuthMethodRequest(respW, req)
				must.NoError(t, err)

				result := obj.(*structs.ACLAuthMethod)
				must.Eq(t, result, mockACLAuthMethod)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			httpACLTest(t, nil, tc.testFn)
		})
	}
}

func TestHTTPServer_ACLAuthMethodSpecificRequest(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name   string
		testFn func(srv *TestAgent)
	}{
		{
			name: "missing auth-method name",
			testFn: func(srv *TestAgent) {

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodGet, "/v1/acl/auth-method/", nil)
				must.NoError(t, err)
				respW := httptest.NewRecorder()

				// Send the HTTP request.
				obj, err := srv.Server.ACLAuthMethodSpecificRequest(respW, req)
				must.Error(t, err)
				must.StrContains(t, err.Error(), "missing ACL auth-method name")
				must.Nil(t, obj)
			},
		},
		{
			name: "incorrect method",
			testFn: func(srv *TestAgent) {

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodConnect, "/v1/acl/auth-method/foobar", nil)
				must.NoError(t, err)
				respW := httptest.NewRecorder()

				// Send the HTTP request.
				obj, err := srv.Server.ACLAuthMethodSpecificRequest(respW, req)
				must.Error(t, err)
				must.StrContains(t, err.Error(), "Invalid method")
				must.Nil(t, obj)
			},
		},
		{
			name: "get auth-method",
			testFn: func(srv *TestAgent) {

				// Create a mock auth-method and put directly into state.
				mockACLAuthMethod := mock.ACLOIDCAuthMethod()
				must.NoError(t, srv.server.State().UpsertACLAuthMethods(
					20, []*structs.ACLAuthMethod{mockACLAuthMethod}))

				authMethodURL := "/v1/acl/auth-method/" + mockACLAuthMethod.Name

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodGet, authMethodURL, nil)
				must.NoError(t, err)
				respW := httptest.NewRecorder()

				// Ensure we have a token set.
				setToken(req, srv.RootToken)

				// Send the HTTP request.
				obj, err := srv.Server.ACLAuthMethodSpecificRequest(respW, req)
				must.NoError(t, err)
				must.Eq(t, obj.(*structs.ACLAuthMethod).Hash, mockACLAuthMethod.Hash)
			},
		},
		{
			name: "get, update, and delete auth-method",
			testFn: func(srv *TestAgent) {

				// Create a mock auth-method and put directly into state.
				mockACLAuthMethod := mock.ACLOIDCAuthMethod()
				must.NoError(t, srv.server.State().UpsertACLAuthMethods(
					20, []*structs.ACLAuthMethod{mockACLAuthMethod}))

				authMethodURL := "/v1/acl/auth-method/" + mockACLAuthMethod.Name

				// Build the HTTP request to read the auth-method.
				req, err := http.NewRequest(http.MethodGet, authMethodURL, nil)
				must.NoError(t, err)
				respW := httptest.NewRecorder()

				// Ensure we have a token set.
				setToken(req, srv.RootToken)

				// Send the HTTP request.
				obj, err := srv.Server.ACLAuthMethodSpecificRequest(respW, req)
				must.NoError(t, err)
				must.Eq(t, obj.(*structs.ACLAuthMethod).Hash, mockACLAuthMethod.Hash)

				// Update the auth-method and make the request via the HTTP
				// API.
				mockACLAuthMethod.MaxTokenTTL = 3600 * time.Hour
				mockACLAuthMethod.SetHash()

				req, err = http.NewRequest(http.MethodPost, authMethodURL, encodeReq(mockACLAuthMethod))
				must.NoError(t, err)
				respW = httptest.NewRecorder()

				// Ensure we have a token set.
				setToken(req, srv.RootToken)

				// Send the HTTP request.
				_, err = srv.Server.ACLAuthMethodSpecificRequest(respW, req)
				must.NoError(t, err)

				// Delete the ACL auth-method.
				req, err = http.NewRequest(http.MethodDelete, authMethodURL, nil)
				must.NoError(t, err)
				respW = httptest.NewRecorder()

				// Ensure we have a token set.
				setToken(req, srv.RootToken)

				// Send the HTTP request.
				obj, err = srv.Server.ACLAuthMethodSpecificRequest(respW, req)
				must.NoError(t, err)
				must.Nil(t, obj)

				// Ensure the ACL auth-method is no longer stored within state.
				aclAuthMethod, err := srv.server.State().GetACLAuthMethodByName(nil, mockACLAuthMethod.Name)
				must.NoError(t, err)
				must.Nil(t, aclAuthMethod)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cb := func(c *Config) { c.NomadConfig.ACLTokenMaxExpirationTTL = 3600 * time.Hour }
			httpACLTest(t, cb, tc.testFn)
		})
	}
}

func TestHTTPServer_ACLBindingRuleListRequest(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name   string
		testFn func(srv *TestAgent)
	}{
		{
			name: "no auth token set",
			testFn: func(srv *TestAgent) {

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodGet, "/v1/acl/binding-rules", nil)
				must.NoError(t, err)
				respW := httptest.NewRecorder()

				// Send the HTTP request.
				obj, err := srv.Server.ACLBindingRuleListRequest(respW, req)
				must.EqError(t, err, "Permission denied")
				must.Nil(t, obj)
			},
		},
		{
			name: "invalid method",
			testFn: func(srv *TestAgent) {

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodConnect, "/v1/acl/binding-rules", nil)
				must.NoError(t, err)
				respW := httptest.NewRecorder()

				// Ensure we have a token set.
				setToken(req, srv.RootToken)

				// Send the HTTP request.
				obj, err := srv.Server.ACLBindingRuleListRequest(respW, req)
				must.EqError(t, err, "Invalid method")
				must.Nil(t, obj)
			},
		},
		{
			name: "no binding rules in state",
			testFn: func(srv *TestAgent) {

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodGet, "/v1/acl/binding-rules", nil)
				must.NoError(t, err)
				respW := httptest.NewRecorder()

				// Ensure we have a token set.
				setToken(req, srv.RootToken)

				// Send the HTTP request.
				obj, err := srv.Server.ACLBindingRuleListRequest(respW, req)
				must.NoError(t, err)
				must.Len(t, 0, obj.([]*structs.ACLBindingRuleListStub))
			},
		},
		{
			name: "binding rules in state",
			testFn: func(srv *TestAgent) {

				// Upsert two binding rules into state.
				must.NoError(t, srv.server.State().UpsertACLBindingRules(
					10, []*structs.ACLBindingRule{mock.ACLBindingRule(), mock.ACLBindingRule()}, true))

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodGet, "/v1/acl/binding-rules", nil)
				must.NoError(t, err)
				respW := httptest.NewRecorder()

				// Ensure we have a token set.
				setToken(req, srv.RootToken)

				// Send the HTTP request.
				obj, err := srv.Server.ACLBindingRuleListRequest(respW, req)
				must.NoError(t, err)
				must.Len(t, 2, obj.([]*structs.ACLBindingRuleListStub))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			httpACLTest(t, nil, tc.testFn)
		})
	}
}

func TestHTTPServer_ACLBindingRuleRequest(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name   string
		testFn func(srv *TestAgent)
	}{
		{
			name: "no auth token set",
			testFn: func(srv *TestAgent) {

				// Create a mock binding rule to use in the request body.
				mockACLBindingRule := mock.ACLBindingRule()
				mockACLBindingRule.ID = ""

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodPut, "/v1/acl/binding-rule", encodeReq(mockACLBindingRule))
				must.NoError(t, err)
				respW := httptest.NewRecorder()

				// Send the HTTP request.
				obj, err := srv.Server.ACLBindingRuleRequest(respW, req)
				must.Error(t, err)
				must.StrContains(t, err.Error(), "Permission denied")
				must.Nil(t, obj)
			},
		},
		{
			name: "invalid method",
			testFn: func(srv *TestAgent) {

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodConnect, "/v1/acl/binding-rule", nil)
				must.NoError(t, err)
				respW := httptest.NewRecorder()

				// Ensure we have a token set.
				setToken(req, srv.RootToken)

				// Send the HTTP request.
				obj, err := srv.Server.ACLBindingRuleRequest(respW, req)
				must.Error(t, err)
				must.StrContains(t, err.Error(), "Invalid method")
				must.Nil(t, obj)
			},
		},
		{
			name: "successful upsert",
			testFn: func(srv *TestAgent) {

				// Upsert the auth method that the binding rule will associate
				// with.
				mockACLAuthMethod := mock.ACLOIDCAuthMethod()
				must.NoError(t, srv.server.State().UpsertACLAuthMethods(
					10, []*structs.ACLAuthMethod{mockACLAuthMethod}))

				// Create a mock binding rule to use in the request body.
				mockACLBindingRule := mock.ACLBindingRule()
				mockACLBindingRule.AuthMethod = mockACLAuthMethod.Name
				mockACLBindingRule.ID = ""

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodPut, "/v1/acl/binding-rule", encodeReq(mockACLBindingRule))
				must.NoError(t, err)
				respW := httptest.NewRecorder()

				// Ensure we have a token set.
				setToken(req, srv.RootToken)

				// Send the HTTP request.
				obj, err := srv.Server.ACLBindingRuleRequest(respW, req)
				must.NoError(t, err)

				result := obj.(*structs.ACLBindingRule)
				must.Eq(t, mockACLBindingRule.Selector, result.Selector)
				must.Eq(t, mockACLBindingRule.AuthMethod, result.AuthMethod)
				must.Eq(t, mockACLBindingRule.Description, result.Description)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			httpACLTest(t, nil, tc.testFn)
		})
	}
}

func TestHTTPServer_ACLBindingRuleSpecificRequest(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name   string
		testFn func(srv *TestAgent)
	}{
		{
			name: "missing binding rule ID",
			testFn: func(srv *TestAgent) {

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodGet, "/v1/acl/binding-rule/", nil)
				must.NoError(t, err)
				respW := httptest.NewRecorder()

				// Send the HTTP request.
				obj, err := srv.Server.ACLBindingRuleSpecificRequest(respW, req)
				must.EqError(t, err, "missing ACL binding rule ID")
				must.Nil(t, obj)
			},
		},
		{
			name: "incorrect method",
			testFn: func(srv *TestAgent) {

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodConnect, "/v1/acl/binding-rule/foobar", nil)
				must.NoError(t, err)
				respW := httptest.NewRecorder()

				// Send the HTTP request.
				obj, err := srv.Server.ACLBindingRuleSpecificRequest(respW, req)
				must.EqError(t, err, "Invalid method")
				must.Nil(t, obj)
			},
		},
		{
			name: "get binding rule",
			testFn: func(srv *TestAgent) {

				// Upsert a binding rule into state.
				aclBindingRules := []*structs.ACLBindingRule{mock.ACLBindingRule()}
				must.NoError(t, srv.server.State().UpsertACLBindingRules(10, aclBindingRules, true))

				url := "/v1/acl/binding-rule/" + aclBindingRules[0].ID

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodGet, url, nil)
				must.NoError(t, err)
				respW := httptest.NewRecorder()

				// Ensure we have a token set.
				setToken(req, srv.RootToken)

				// Send the HTTP request.
				obj, err := srv.Server.ACLBindingRuleSpecificRequest(respW, req)
				must.NoError(t, err)

				result := obj.(*structs.ACLBindingRule)
				must.Eq(t, aclBindingRules[0].ID, result.ID)
				must.Eq(t, aclBindingRules[0].Selector, result.Selector)
				must.Eq(t, aclBindingRules[0].AuthMethod, result.AuthMethod)
				must.Eq(t, aclBindingRules[0].Description, result.Description)
			},
		},
		{
			name: "get, update, and delete binding rule",
			testFn: func(srv *TestAgent) {

				// Upsert the auth method that the binding rule will associate
				// with.
				mockACLAuthMethod := mock.ACLOIDCAuthMethod()
				must.NoError(t, srv.server.State().UpsertACLAuthMethods(
					10, []*structs.ACLAuthMethod{mockACLAuthMethod}))

				// Upsert a binding rule into state.
				mockedAclBindingRule := mock.ACLBindingRule()
				mockedAclBindingRule.AuthMethod = mockACLAuthMethod.Name

				must.NoError(t, srv.server.State().UpsertACLBindingRules(
					10, []*structs.ACLBindingRule{mockedAclBindingRule}, true))

				url := "/v1/acl/binding-rule/" + mockedAclBindingRule.ID

				// Build the HTTP request to read the binding rule.
				req, err := http.NewRequest(http.MethodGet, url, nil)
				must.NoError(t, err)
				respW := httptest.NewRecorder()

				// Ensure we have a token set.
				setToken(req, srv.RootToken)

				// Send the HTTP request.
				obj, err := srv.Server.ACLBindingRuleSpecificRequest(respW, req)
				must.NoError(t, err)

				result := obj.(*structs.ACLBindingRule)
				must.Eq(t, mockedAclBindingRule.ID, result.ID)
				must.Eq(t, mockedAclBindingRule.Selector, result.Selector)
				must.Eq(t, mockedAclBindingRule.AuthMethod, result.AuthMethod)
				must.Eq(t, mockedAclBindingRule.Description, result.Description)

				// Update the binding rule and make the request via the HTTP
				// API.
				result.Description = "new description"

				req, err = http.NewRequest(http.MethodPost, url, encodeReq(result))
				must.NoError(t, err)
				respW = httptest.NewRecorder()

				// Ensure we have a token set.
				setToken(req, srv.RootToken)

				// Send the HTTP request.
				_, err = srv.Server.ACLBindingRuleSpecificRequest(respW, req)
				must.NoError(t, err)

				// Delete the ACL binding rule.
				req, err = http.NewRequest(http.MethodDelete, url, nil)
				must.NoError(t, err)
				respW = httptest.NewRecorder()

				// Ensure we have a token set.
				setToken(req, srv.RootToken)

				// Send the HTTP request.
				obj, err = srv.Server.ACLBindingRuleSpecificRequest(respW, req)
				must.NoError(t, err)
				must.Nil(t, obj)

				// Ensure the ACL binding rule is no longer stored within
				// state.
				aclBindingRule, err := srv.server.State().GetACLBindingRule(nil, mockedAclBindingRule.ID)
				must.NoError(t, err)
				must.Nil(t, aclBindingRule)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cb := func(c *Config) { c.NomadConfig.ACLTokenMaxExpirationTTL = 3600 * time.Hour }
			httpACLTest(t, cb, tc.testFn)
		})
	}
}

func TestHTTPServer_ACLOIDCAuthURLRequest(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name   string
		testFn func(srv *TestAgent)
	}{
		{
			name: "incorrect method",
			testFn: func(testAgent *TestAgent) {

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodConnect, "/v1/acl/oidc/auth-url", nil)
				must.NoError(t, err)
				respW := httptest.NewRecorder()

				// Send the HTTP request.
				obj, err := testAgent.Server.ACLOIDCAuthURLRequest(respW, req)
				must.Error(t, err)
				must.StrContains(t, err.Error(), "Invalid method")
				must.Nil(t, obj)
			},
		},
		{
			name: "success",
			testFn: func(testAgent *TestAgent) {

				// Set up the test OIDC provider.
				oidcTestProvider := capOIDC.StartTestProvider(t)
				defer oidcTestProvider.Stop()

				// Generate and upsert an ACL auth method for use. Certain values must be
				// taken from the cap OIDC provider just like real world use.
				mockedAuthMethod := mock.ACLOIDCAuthMethod()
				mockedAuthMethod.Config.AllowedRedirectURIs = []string{"http://127.0.0.1:4649/oidc/callback"}
				mockedAuthMethod.Config.OIDCDiscoveryURL = oidcTestProvider.Addr()
				mockedAuthMethod.Config.SigningAlgs = []string{"ES256"}
				mockedAuthMethod.Config.DiscoveryCaPem = []string{oidcTestProvider.CACert()}

				must.NoError(t, testAgent.server.State().UpsertACLAuthMethods(
					10, []*structs.ACLAuthMethod{mockedAuthMethod}))

				// Generate the request body.
				requestBody := structs.ACLOIDCAuthURLRequest{
					AuthMethodName: mockedAuthMethod.Name,
					RedirectURI:    mockedAuthMethod.Config.AllowedRedirectURIs[0],
					ClientNonce:    "fpSPuaodKevKfDU3IeXa",
					WriteRequest: structs.WriteRequest{
						Region: "global",
					},
				}

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodPost, "/v1/acl/oidc/auth-url", encodeReq(&requestBody))
				must.NoError(t, err)
				respW := httptest.NewRecorder()

				// Send the HTTP request.
				obj, err := testAgent.Server.ACLOIDCAuthURLRequest(respW, req)
				must.NoError(t, err)

				// The response URL comes encoded, so decode this and check we have each
				// component we expect.
				escapedURL, err := url.PathUnescape(obj.(structs.ACLOIDCAuthURLResponse).AuthURL)
				must.NoError(t, err)
				must.StrContains(t, escapedURL, "/authorize?client_id=mock")
				must.StrContains(t, escapedURL, "&nonce=fpSPuaodKevKfDU3IeXa")
				must.StrContains(t, escapedURL, "&redirect_uri=http://127.0.0.1:4649/oidc/callback")
				must.StrContains(t, escapedURL, "&response_type=code")
				must.StrContains(t, escapedURL, "&scope=openid")
				must.StrContains(t, escapedURL, "&state=st_")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			httpACLTest(t, nil, tc.testFn)
		})
	}
}

func TestHTTPServer_ACLOIDCCompleteAuthRequest(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name   string
		testFn func(srv *TestAgent)
	}{
		{
			name: "incorrect method",
			testFn: func(testAgent *TestAgent) {

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodConnect, "/v1/acl/oidc/complete-auth", nil)
				must.NoError(t, err)
				respW := httptest.NewRecorder()

				// Send the HTTP request.
				obj, err := testAgent.Server.ACLOIDCCompleteAuthRequest(respW, req)
				must.Error(t, err)
				must.StrContains(t, err.Error(), "Invalid method")
				must.Nil(t, obj)
			},
		},
		{
			name: "success",
			testFn: func(testAgent *TestAgent) {

				// Set up the test OIDC provider.
				oidcTestProvider := capOIDC.StartTestProvider(t)
				defer oidcTestProvider.Stop()
				oidcTestProvider.SetAllowedRedirectURIs([]string{"http://127.0.0.1:4649/oidc/callback"})

				// Generate and upsert an ACL auth method for use. Certain values must be
				// taken from the cap OIDC provider just like real world use.
				mockedAuthMethod := mock.ACLOIDCAuthMethod()
				mockedAuthMethod.Config.BoundAudiences = []string{"mock"}
				mockedAuthMethod.Config.AllowedRedirectURIs = []string{"http://127.0.0.1:4649/oidc/callback"}
				mockedAuthMethod.Config.OIDCDiscoveryURL = oidcTestProvider.Addr()
				mockedAuthMethod.Config.SigningAlgs = []string{"ES256"}
				mockedAuthMethod.Config.DiscoveryCaPem = []string{oidcTestProvider.CACert()}
				mockedAuthMethod.Config.ClaimMappings = map[string]string{}
				mockedAuthMethod.Config.ListClaimMappings = map[string]string{
					"http://nomad.internal/roles":    "roles",
					"http://nomad.internal/policies": "policies",
				}

				must.NoError(t, testAgent.server.State().UpsertACLAuthMethods(
					10, []*structs.ACLAuthMethod{mockedAuthMethod}))

				// Set our custom data and some expected values, so we can make the RPC and
				// use the test provider.
				oidcTestProvider.SetExpectedAuthNonce("fpSPuaodKevKfDU3IeXa")
				oidcTestProvider.SetExpectedAuthCode("codeABC")
				oidcTestProvider.SetCustomAudience("mock")
				oidcTestProvider.SetExpectedState("st_someweirdstateid")
				oidcTestProvider.SetCustomClaims(map[string]interface{}{
					"azp":                            "mock",
					"http://nomad.internal/policies": []string{"engineering"},
					"http://nomad.internal/roles":    []string{"engineering"},
				})

				// Generate the request body.
				requestBody := structs.ACLOIDCCompleteAuthRequest{
					AuthMethodName: mockedAuthMethod.Name,
					ClientNonce:    "fpSPuaodKevKfDU3IeXa",
					State:          "st_someweirdstateid",
					Code:           "codeABC",
					RedirectURI:    mockedAuthMethod.Config.AllowedRedirectURIs[0],
					WriteRequest: structs.WriteRequest{
						Region: "global",
					},
				}

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodPost, "/v1/acl/oidc/complete-auth", encodeReq(&requestBody))
				must.NoError(t, err)
				respW := httptest.NewRecorder()

				// Send the HTTP request.
				_, err = testAgent.Server.ACLOIDCCompleteAuthRequest(respW, req)
				must.ErrorContains(t, err, "no role or policy bindings matched")

				// Upsert an ACL policy and role, so that we can reference this within our
				// OIDC claims.
				mockACLPolicy := mock.ACLPolicy()
				must.NoError(t, testAgent.server.State().UpsertACLPolicies(
					structs.MsgTypeTestSetup, 20, []*structs.ACLPolicy{mockACLPolicy}))

				mockACLRole := mock.ACLRole()
				mockACLRole.Policies = []*structs.ACLRolePolicyLink{{Name: mockACLPolicy.Name}}
				must.NoError(t, testAgent.server.State().UpsertACLRoles(
					structs.MsgTypeTestSetup, 30, []*structs.ACLRole{mockACLRole}, true))

				// Generate and upsert two binding rules, so we can test both ACL Policy
				// and Role claim mapping.
				mockBindingRule1 := mock.ACLBindingRule()
				mockBindingRule1.AuthMethod = mockedAuthMethod.Name
				mockBindingRule1.BindType = structs.ACLBindingRuleBindTypePolicy
				mockBindingRule1.Selector = "engineering in list.policies"
				mockBindingRule1.BindName = mockACLPolicy.Name

				mockBindingRule2 := mock.ACLBindingRule()
				mockBindingRule2.AuthMethod = mockedAuthMethod.Name
				mockBindingRule2.BindName = mockACLRole.Name

				must.NoError(t, testAgent.server.State().UpsertACLBindingRules(
					40, []*structs.ACLBindingRule{mockBindingRule1, mockBindingRule2}, true))

				// Build the HTTP request.
				req, err = http.NewRequest(http.MethodPost, "/v1/acl/oidc/complete-auth", encodeReq(&requestBody))
				must.NoError(t, err)
				respW = httptest.NewRecorder()

				// Send the HTTP request.
				obj, err := testAgent.Server.ACLOIDCCompleteAuthRequest(respW, req)
				must.NoError(t, err)

				aclTokenResp, ok := obj.(*structs.ACLToken)
				must.True(t, ok)
				must.NotNil(t, aclTokenResp)
				must.Len(t, 1, aclTokenResp.Policies)
				must.Eq(t, mockACLPolicy.Name, aclTokenResp.Policies[0])
				must.Len(t, 1, aclTokenResp.Roles)
				must.Eq(t, mockACLRole.Name, aclTokenResp.Roles[0].Name)
				must.Eq(t, mockACLRole.ID, aclTokenResp.Roles[0].ID)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			httpACLTest(t, nil, tc.testFn)
		})
	}
}

func TestHTTPServer_ACLLoginRequest(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name   string
		testFn func(srv *TestAgent)
	}{
		{
			name: "incorrect method",
			testFn: func(testAgent *TestAgent) {

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodConnect, "/v1/acl/login", nil)
				must.NoError(t, err)
				respW := httptest.NewRecorder()

				// Send the HTTP request.
				obj, err := testAgent.Server.ACLOIDCCompleteAuthRequest(respW, req)
				must.Error(t, err)
				must.StrContains(t, err.Error(), "Invalid method")
				must.Nil(t, obj)
			},
		},
		{
			name: "success",
			testFn: func(testAgent *TestAgent) {

				// Generate a sample JWT
				iat := time.Now().Unix()
				nbf := time.Now().Unix()
				exp := time.Now().Add(time.Hour).Unix()
				claims := jwt.MapClaims{
					"iss":                            "nomad test suite",
					"iat":                            iat,
					"nbf":                            nbf,
					"exp":                            exp,
					"aud":                            "engineering",
					"http://nomad.internal/policies": []string{"engineering"},
					"http://nomad.internal/roles":    []string{"engineering"},
				}

				token, pubKey, err := mock.SampleJWTokenWithKeys(claims, nil)
				must.NoError(t, err)

				// Generate and upsert a JWT ACL auth method for use.
				mockedAuthMethod := mock.ACLJWTAuthMethod()
				mockedAuthMethod.Config.BoundAudiences = []string{"engineering"}
				mockedAuthMethod.Config.JWTValidationPubKeys = []string{pubKey}
				mockedAuthMethod.Config.BoundIssuer = []string{"nomad test suite"}
				mockedAuthMethod.Config.ExpirationLeeway = time.Duration(3600)
				mockedAuthMethod.Config.ClockSkewLeeway = time.Duration(3600)
				mockedAuthMethod.Config.ClaimMappings = map[string]string{}
				mockedAuthMethod.Config.ListClaimMappings = map[string]string{
					"http://nomad.internal/roles":    "roles",
					"http://nomad.internal/policies": "policies",
				}

				must.NoError(t, testAgent.server.State().UpsertACLAuthMethods(
					10, []*structs.ACLAuthMethod{mockedAuthMethod}))

				// Generate the request body.
				requestBody := structs.ACLLoginRequest{
					AuthMethodName: mockedAuthMethod.Name,
					LoginToken:     token,
					WriteRequest: structs.WriteRequest{
						Region: "global",
					},
				}

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodPost, "/v1/acl/login", encodeReq(&requestBody))
				must.NoError(t, err)
				respW := httptest.NewRecorder()

				// Send the HTTP request.
				_, err = testAgent.Server.ACLLoginRequest(respW, req)
				must.ErrorContains(t, err, "no role or policy bindings matched")

				// Upsert an ACL policy and role, so that we can reference this within our
				// OIDC claims.
				mockACLPolicy := mock.ACLPolicy()
				must.NoError(t, testAgent.server.State().UpsertACLPolicies(
					structs.MsgTypeTestSetup, 20, []*structs.ACLPolicy{mockACLPolicy}))

				mockACLRole := mock.ACLRole()
				mockACLRole.Policies = []*structs.ACLRolePolicyLink{{Name: mockACLPolicy.Name}}
				must.NoError(t, testAgent.server.State().UpsertACLRoles(
					structs.MsgTypeTestSetup, 30, []*structs.ACLRole{mockACLRole}, true))

				// Generate and upsert two binding rules, so we can test both ACL Policy
				// and Role claim mapping.
				mockBindingRule1 := mock.ACLBindingRule()
				mockBindingRule1.AuthMethod = mockedAuthMethod.Name
				mockBindingRule1.BindType = structs.ACLBindingRuleBindTypePolicy
				mockBindingRule1.Selector = "engineering in list.policies"
				mockBindingRule1.BindName = mockACLPolicy.Name

				mockBindingRule2 := mock.ACLBindingRule()
				mockBindingRule2.AuthMethod = mockedAuthMethod.Name
				mockBindingRule2.BindName = mockACLRole.Name

				must.NoError(t, testAgent.server.State().UpsertACLBindingRules(
					40, []*structs.ACLBindingRule{mockBindingRule1, mockBindingRule2}, true))

				// Build the HTTP request.
				req, err = http.NewRequest(http.MethodPost, "/v1/acl/login", encodeReq(&requestBody))
				must.NoError(t, err)
				respW = httptest.NewRecorder()

				// Send the HTTP request.
				obj, err := testAgent.Server.ACLLoginRequest(respW, req)
				must.NoError(t, err)

				aclTokenResp, ok := obj.(*structs.ACLToken)
				must.True(t, ok)
				must.NotNil(t, aclTokenResp)
				must.Len(t, 1, aclTokenResp.Policies)
				must.Eq(t, mockACLPolicy.Name, aclTokenResp.Policies[0])
				must.Len(t, 1, aclTokenResp.Roles)
				must.Eq(t, mockACLRole.Name, aclTokenResp.Roles[0].Name)
				must.Eq(t, mockACLRole.ID, aclTokenResp.Roles[0].ID)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			httpACLTest(t, nil, tc.testFn)
		})
	}
}
