package agent

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/nomad/acl"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func TestHTTP_ClientMetaGetCurrent(t *testing.T) {
	require := require.New(t)
	path := "/v1/client/meta"
	httpTest(t,
		func(c *Config) {
			c.Client.Meta = map[string]string{
				"Foo": "bar",
			}
		},
		func(s *TestAgent) {
			// Local node, local resp
			{
				req, err := http.NewRequest("GET", path, nil)
				require.NoError(err)

				respW := httptest.NewRecorder()
				reply, err := s.Server.ClientMetaRequest(respW, req)
				require.NoError(err)
				require.NotNil(reply)
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
					require.NoError(err)
				})

				req, err := http.NewRequest("GET", fmt.Sprintf("%s?node_id=%s", path, c.NodeID()), nil)
				require.NoError(err)

				respW := httptest.NewRecorder()
				reply, err := s.Server.ClientMetaRequest(respW, req)
				require.NoError(err)
				require.NotNil(reply)

				s.client = c
			}
		})
}

func TestHTTP_ClientMetaGetCurrent_ACL(t *testing.T) {
	require := require.New(t)
	path := "/v1/client/meta?node_id=invalid"

	httpACLTest(t, nil, func(s *TestAgent) {
		state := s.Agent.server.State()

		// Make the HTTP request
		req, err := http.NewRequest("GET", path, nil)
		require.NoError(err)

		// Try request without a token and expect failure
		{
			respW := httptest.NewRecorder()
			_, err := s.Server.ClientMetaRequest(respW, req)
			require.NotNil(err)
			require.Equal(err.Error(), structs.ErrPermissionDenied.Error())
		}

		// Try request with an invalid token and expect failure
		{
			respW := httptest.NewRecorder()
			token := mock.CreatePolicyAndToken(t, state, 1005, "invalid", mock.NodePolicy(acl.PolicyDeny))
			setToken(req, token)
			_, err := s.Server.ClientMetaRequest(respW, req)
			require.NotNil(err)
			require.Equal(err.Error(), structs.ErrPermissionDenied.Error())
		}

		// Try request with a valid token
		// Still returns an error because the node does not exist
		{
			respW := httptest.NewRecorder()
			token := mock.CreatePolicyAndToken(t, state, 1007, "valid", mock.NodePolicy(acl.PolicyRead))
			setToken(req, token)
			_, err := s.Server.ClientMetaRequest(respW, req)
			require.NotNil(err)
			require.Contains(err.Error(), "node lookup failed:")
		}

		// Try request with a management token
		// Still returns an error because the node does not exist
		{
			respW := httptest.NewRecorder()
			setToken(req, s.RootToken)
			_, err := s.Server.ClientMetaRequest(respW, req)
			require.NotNil(err)
			require.Contains(err.Error(), "node lookup failed:")
		}
	})
}

func TestHTTP_ClientMetaUpdate(t *testing.T) {
	require := require.New(t)
	path := "/v1/client/meta"
	httpTest(t,
		func(c *Config) {
			c.Client.Meta = map[string]string{
				"Foo": "bar",
			}
		},
		func(s *TestAgent) {
			// Local node, local resp
			{
				body := bytes.NewBuffer([]byte("{\"updates\": {\"newkey\": \"foo\"}}\n"))
				req, err := http.NewRequest("PATCH", path, body)
				require.NoError(err)

				respW := httptest.NewRecorder()
				reply, err := s.Server.ClientMetaRequest(respW, req)
				require.NoError(err)
				require.True(reply.(*cstructs.ClientMetadataUpdateResponse).Updated)
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
					require.NoError(err)
				})

				body := bytes.NewBuffer([]byte("{\"updates\": {\"newkey\": \"foo2\"}}\n"))
				req, err := http.NewRequest("PATCH", fmt.Sprintf("%s?node_id=%s", path, c.NodeID()), body)
				require.NoError(err)

				respW := httptest.NewRecorder()
				reply, err := s.Server.ClientMetaRequest(respW, req)
				require.NoError(err)
				require.NotNil(reply)
				require.True(reply.(*cstructs.ClientMetadataUpdateResponse).Updated)

				s.client = c
			}
		})
}

func TestHTTP_ClientMetaUpdate_ACL(t *testing.T) {
	require := require.New(t)
	path := "/v1/client/meta?node_id=invalid"

	httpACLTest(t, nil, func(s *TestAgent) {
		state := s.Agent.server.State()

		// Try request without a token and expect failure
		{
			respW := httptest.NewRecorder()
			body := bytes.NewBuffer([]byte("{}\n"))
			req, err := http.NewRequest("PATCH", path, body)
			require.NoError(err)
			_, err = s.Server.ClientMetaRequest(respW, req)
			require.NotNil(err)
			require.Equal(err.Error(), structs.ErrPermissionDenied.Error())
		}

		// Try request with an invalid token and expect failure
		{
			respW := httptest.NewRecorder()
			body := bytes.NewBuffer([]byte("{}\n"))
			req, err := http.NewRequest("PATCH", path, body)
			require.NoError(err)
			token := mock.CreatePolicyAndToken(t, state, 1005, "invalid", mock.NodePolicy(acl.PolicyDeny))
			setToken(req, token)
			_, err = s.Server.ClientMetaRequest(respW, req)
			require.NotNil(err)
			require.Equal(err.Error(), structs.ErrPermissionDenied.Error())
		}

		// Try request with a valid token
		// Still returns an error because the node does not exist
		{
			respW := httptest.NewRecorder()
			body := bytes.NewBuffer([]byte("{}\n"))
			req, err := http.NewRequest("PATCH", path, body)
			require.NoError(err)
			token := mock.CreatePolicyAndToken(t, state, 1007, "valid", mock.NodePolicy(acl.PolicyWrite))
			setToken(req, token)
			_, err = s.Server.ClientMetaRequest(respW, req)
			require.NotNil(err)
			require.Contains(err.Error(), "missing Updates")
		}

		// Try request with a management token
		// Still returns an error because the node does not exist
		{
			respW := httptest.NewRecorder()
			body := bytes.NewBuffer([]byte("{}\n"))
			req, err := http.NewRequest("PATCH", path, body)
			require.NoError(err)
			setToken(req, s.RootToken)
			_, err = s.Server.ClientMetaRequest(respW, req)
			require.NotNil(err)
			require.Contains(err.Error(), "missing Updates")
		}
	})
}

func TestHTTP_ClientMetaReplace(t *testing.T) {
	require := require.New(t)
	path := "/v1/client/meta"
	httpTest(t,
		func(c *Config) {
			c.Client.Meta = map[string]string{
				"Foo": "bar",
			}
		},
		func(s *TestAgent) {
			// Local node, local resp
			{
				body := bytes.NewBuffer([]byte("{}\n"))
				req, err := http.NewRequest("PUT", path, body)
				require.NoError(err)

				respW := httptest.NewRecorder()
				reply, err := s.Server.ClientMetaRequest(respW, req)
				require.NoError(err)
				require.True(reply.(*cstructs.ClientMetadataUpdateResponse).Updated)
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
					require.NoError(err)
				})

				body := bytes.NewBuffer([]byte("{\"metadata\": {}}\n"))
				req, err := http.NewRequest("PUT", fmt.Sprintf("%s?node_id=%s", path, c.NodeID()), body)
				require.NoError(err)

				respW := httptest.NewRecorder()
				reply, err := s.Server.ClientMetaRequest(respW, req)
				require.NoError(err)
				require.NotNil(reply)

				s.client = c
			}
		})
}

func TestHTTP_ClientMetaReplace_ACL(t *testing.T) {
	require := require.New(t)
	path := "/v1/client/meta?node_id=invalid"

	httpACLTest(t, nil, func(s *TestAgent) {
		state := s.Agent.server.State()

		// Make the HTTP request

		// Try request without a token and expect failure
		{
			respW := httptest.NewRecorder()
			body := bytes.NewBuffer([]byte("{}"))
			req, err := http.NewRequest("PUT", path, body)
			require.NoError(err)
			_, err = s.Server.ClientMetaRequest(respW, req)
			require.NotNil(err)
			require.Equal(err.Error(), structs.ErrPermissionDenied.Error())
		}

		// Try request with an invalid token and expect failure
		{
			respW := httptest.NewRecorder()
			body := bytes.NewBuffer([]byte("{}"))
			req, err := http.NewRequest("PUT", path, body)
			require.NoError(err)
			token := mock.CreatePolicyAndToken(t, state, 1005, "invalid", mock.NodePolicy(acl.PolicyDeny))
			setToken(req, token)
			_, err = s.Server.ClientMetaRequest(respW, req)
			require.NotNil(err)
			require.Equal(err.Error(), structs.ErrPermissionDenied.Error())
		}

		// Try request with a valid token
		// Still returns an error because the node does not exist
		{
			respW := httptest.NewRecorder()
			body := bytes.NewBuffer([]byte("{}"))
			req, err := http.NewRequest("PUT", path, body)
			require.NoError(err)
			token := mock.CreatePolicyAndToken(t, state, 1007, "valid", mock.NodePolicy(acl.PolicyWrite))
			setToken(req, token)
			_, err = s.Server.ClientMetaRequest(respW, req)
			require.NotNil(err)
			require.Contains(err.Error(), "missing Metadata")
		}

		// Try request with a management token
		// Still returns an error because the node does not exist
		{
			respW := httptest.NewRecorder()
			body := bytes.NewBuffer([]byte("{}"))
			req, err := http.NewRequest("PUT", path, body)
			require.NoError(err)
			setToken(req, s.RootToken)
			_, err = s.Server.ClientMetaRequest(respW, req)
			require.NotNil(err)
			require.Contains(err.Error(), "missing Metadata")
		}
	})
}
