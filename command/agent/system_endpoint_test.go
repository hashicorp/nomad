// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/nomad/ci"
)

func TestHTTP_SystemGarbageCollect(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Make the HTTP request
		req, err := http.NewRequest(http.MethodPut, "/v1/system/gc", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		if _, err := s.Server.GarbageCollectRequest(respW, req); err != nil {
			t.Fatalf("err: %v", err)
		}
	})
}

func TestHTTP_ReconcileJobSummaries(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Make the HTTP request
		req, err := http.NewRequest(http.MethodPut, "/v1/system/reconcile/summaries", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		if _, err := s.Server.ReconcileJobSummaries(respW, req); err != nil {
			t.Fatalf("err: %v", err)
		}

		if respW.Code != 200 {
			t.Fatalf("expected: %v, actual: %v", 200, respW.Code)
		}
	})
}
