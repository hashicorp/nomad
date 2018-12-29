package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTP_SystemGarbageCollect(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Make the HTTP request
		req, err := http.NewRequest("PUT", "/v1/system/gc", nil)
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
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Make the HTTP request
		req, err := http.NewRequest("PUT", "/v1/system/reconcile/summaries", nil)
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
