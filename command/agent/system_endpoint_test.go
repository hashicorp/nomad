package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTP_SystemGarbageCollect(t *testing.T) {
	httpTest(t, nil, func(s *TestServer) {
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
