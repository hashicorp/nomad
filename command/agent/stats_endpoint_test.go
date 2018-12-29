package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
)

func TestClientStatsRequest(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		req, err := http.NewRequest("GET", "/v1/client/stats/?since=foo", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		respW := httptest.NewRecorder()
		_, err = s.Server.ClientStatsRequest(respW, req)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
	})
}

func TestClientStatsRequest_ACL(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	httpACLTest(t, nil, func(s *TestAgent) {
		state := s.Agent.server.State()
		req, err := http.NewRequest("GET", "/v1/client/stats/", nil)
		assert.Nil(err)

		// Try request without a token and expect failure
		{
			respW := httptest.NewRecorder()
			_, err := s.Server.ClientStatsRequest(respW, req)
			assert.NotNil(err)
			assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
		}

		// Try request with an invalid token and expect failure
		{
			respW := httptest.NewRecorder()
			token := mock.CreatePolicyAndToken(t, state, 1005, "invalid", mock.NodePolicy(acl.PolicyDeny))
			setToken(req, token)
			_, err := s.Server.ClientStatsRequest(respW, req)
			assert.NotNil(err)
			assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
		}

		// Try request with a valid token
		{
			respW := httptest.NewRecorder()
			token := mock.CreatePolicyAndToken(t, state, 1007, "valid", mock.NodePolicy(acl.PolicyRead))
			setToken(req, token)
			_, err := s.Server.ClientStatsRequest(respW, req)
			assert.Nil(err)
			assert.Equal(http.StatusOK, respW.Code)
		}

		// Try request with a management token
		{
			respW := httptest.NewRecorder()
			setToken(req, s.RootToken)
			_, err := s.Server.ClientStatsRequest(respW, req)
			assert.Nil(err)
			assert.Equal(http.StatusOK, respW.Code)
		}
	})
}
