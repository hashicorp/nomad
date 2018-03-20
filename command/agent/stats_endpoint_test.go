package agent

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientStatsRequest(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	httpTest(t, nil, func(s *TestAgent) {

		// Local node, local resp
		{
			req, err := http.NewRequest("GET", "/v1/client/stats/?since=foo", nil)
			if err != nil {
				t.Fatalf("err: %v", err)
			}

			respW := httptest.NewRecorder()
			_, err = s.Server.ClientStatsRequest(respW, req)
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
		}

		// Local node, server resp
		{
			srv := s.server
			s.server = nil

			req, err := http.NewRequest("GET", fmt.Sprintf("/v1/client/stats?node_id=%s", uuid.Generate()), nil)
			require.Nil(err)

			respW := httptest.NewRecorder()
			_, err = s.Server.ClientStatsRequest(respW, req)
			require.NotNil(err)
			require.Contains(err.Error(), "Unknown node")

			s.server = srv
		}

		// no client, server resp
		{
			c := s.client
			s.client = nil

			testutil.WaitForResult(func() (bool, error) {
				n, err := s.server.State().NodeByID(nil, c.NodeID())
				if err != nil {
					return false, err
				}
				return n != nil, nil
			}, func(err error) {
				t.Fatalf("should have client: %v", err)
			})

			req, err := http.NewRequest("GET", fmt.Sprintf("/v1/client/stats?node_id=%s", c.NodeID()), nil)
			require.Nil(err)

			respW := httptest.NewRecorder()
			_, err = s.Server.ClientStatsRequest(respW, req)
			require.NotNil(err)

			// The dev agent uses in-mem RPC so just assert the no route error
			require.Contains(err.Error(), structs.ErrNoNodeConn.Error())

			s.client = c
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
