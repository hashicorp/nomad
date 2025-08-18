// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/structs"
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
