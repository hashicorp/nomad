// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/nomad/ci"
)

func TestHTTP_StatusLeader(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Make the HTTP request
		req, err := http.NewRequest(http.MethodGet, "/v1/status/leader", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.StatusLeaderRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		leader := obj.(string)
		if leader == "" {
			t.Fatalf("bad: %#v", leader)
		}
	})
}

func TestHTTP_StatusPeers(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Make the HTTP request
		req, err := http.NewRequest(http.MethodGet, "/v1/status/peers", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.StatusPeersRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check the job
		peers := obj.([]string)
		if len(peers) == 0 {
			t.Fatalf("bad: %#v", peers)
		}
	})
}
