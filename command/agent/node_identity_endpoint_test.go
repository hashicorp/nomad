// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
)

func TestHTTPServer_NodeIdentityGetRequest(t *testing.T) {
	ci.Parallel(t)

	t.Run("200 ok", func(t *testing.T) {
		httpTest(t, cb, func(s *TestAgent) {
			respW := httptest.NewRecorder()

			req, err := http.NewRequest(http.MethodGet, "/v1/client/identity", nil)
			must.NoError(t, err)

			obj, err := s.Server.NodeIdentityGetRequest(respW, req)
			must.NoError(t, err)
			must.Eq(t, http.StatusOK, respW.Code)

			resp, ok := obj.(structs.NodeIdentityGetResp)
			must.True(t, ok)

			must.MapLen(t, 9, resp.Claims)

			must.MapContainsKeys(t, resp.Claims, []string{
				"aud",
				"exp",
				"jti",
				"nbf",
				"sub",
				"iat",
				"nomad_node_datacenter",
				"nomad_node_id",
				"nomad_node_pool",
			})

			must.MapContainsValues(t, resp.Claims, []any{
				"nomadproject.io",
				s.client.NodeID(),
				s.client.Datacenter(),
				s.client.Node().NodePool,
			})
		})
	})

	t.Run("405 invalid method", func(t *testing.T) {
		httpTest(t, cb, func(s *TestAgent) {
			respW := httptest.NewRecorder()

			badMethods := []string{
				http.MethodConnect,
				http.MethodDelete,
				http.MethodHead,
				http.MethodOptions,
				http.MethodPatch,
				http.MethodPost,
				http.MethodPut,
				http.MethodTrace,
			}

			for _, method := range badMethods {
				req, err := http.NewRequest(method, "/v1/client/identity", nil)
				must.NoError(t, err)

				_, err = s.Server.NodeIdentityGetRequest(respW, req)
				must.ErrorContains(t, err, "Invalid method")

				codedErr, ok := err.(HTTPCodedError)
				must.True(t, ok)
				must.Eq(t, http.StatusMethodNotAllowed, codedErr.Code())
				must.Eq(t, ErrInvalidMethod, codedErr.Error())
			}
		})
	})
}

func TestHTTPServer_NodeIdentityRenewRequest(t *testing.T) {
	ci.Parallel(t)

	t.Run("405 invalid method", func(t *testing.T) {
		httpTest(t, nil, func(s *TestAgent) {
			respW := httptest.NewRecorder()

			badMethods := []string{
				http.MethodConnect,
				http.MethodDelete,
				http.MethodGet,
				http.MethodHead,
				http.MethodOptions,
				http.MethodPatch,
				http.MethodTrace,
			}

			for _, method := range badMethods {
				req, err := http.NewRequest(method, "/v1/client/identity/renew", nil)
				must.NoError(t, err)

				_, err = s.Server.NodeIdentityRenewRequest(respW, req)
				must.ErrorContains(t, err, "Invalid method")

				codedErr, ok := err.(HTTPCodedError)
				must.True(t, ok)
				must.Eq(t, http.StatusMethodNotAllowed, codedErr.Code())
				must.Eq(t, ErrInvalidMethod, codedErr.Error())
			}
		})
	})

	t.Run("400 no node", func(t *testing.T) {
		httpTest(t, nil, func(s *TestAgent) {

			reqObj := structs.NodeIdentityRenewReq{NodeID: uuid.Generate()}

			buf := encodeReq(reqObj)

			respW := httptest.NewRecorder()

			req, err := http.NewRequest(http.MethodPost, "/v1/client/identity/renew", buf)
			must.NoError(t, err)

			_, err = s.Server.NodeIdentityRenewRequest(respW, req)
			must.ErrorContains(t, err, "Unknown node")
		})
	})

	t.Run("200 ok", func(t *testing.T) {

		// Enable the client, so we have something to renew.
		configFn := func(c *Config) { c.Client.Enabled = true }

		httpTest(t, configFn, func(s *TestAgent) {

			testutil.WaitForClient(t, s.RPC, s.client.NodeID(), s.config().Region)

			reqObj := structs.NodeIdentityRenewReq{NodeID: s.client.NodeID()}

			buf := encodeReq(reqObj)

			respW := httptest.NewRecorder()

			req, err := http.NewRequest(http.MethodPost, "/v1/client/identity/renew", buf)
			must.NoError(t, err)

			obj, err := s.Server.NodeIdentityRenewRequest(respW, req)
			must.NoError(t, err)

			_, ok := obj.(structs.NodeIdentityRenewResp)
			must.True(t, ok)
		})
	})
}
