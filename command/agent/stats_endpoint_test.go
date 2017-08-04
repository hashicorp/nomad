package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
