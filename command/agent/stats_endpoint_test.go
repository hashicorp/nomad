package agent

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientStatsRequest(t *testing.T) {
	httpTest(t, nil, func(s *TestServer) {
		req, err := http.NewRequest("GET", "/v1/client/stats/?since=foo", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		respW := httptest.NewRecorder()
		_, err = s.Server.ClientStatsRequest(respW, req)
		if !strings.ContainsAny(err.Error(), invalidSinceErrPrefix) {
			t.Fatalf("unexpected err: %v", err)
		}
	})
}
