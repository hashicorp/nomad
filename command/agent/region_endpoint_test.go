package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTP_RegionList(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Make the HTTP request
		req, err := http.NewRequest("GET", "/v1/regions", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.RegionListRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		out := obj.([]string)
		if len(out) != 1 || out[0] != "global" {
			t.Fatalf("unexpected regions: %#v", out)
		}
	})
}
