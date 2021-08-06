package agent

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
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

		client, ctx := v1api.NewClientAndContext(s.Config.BindAddr, strconv.Itoa(s.Config.Ports.HTTP))
		regionsRequest := client.RegionsApi.RegionsGet(ctx)

		regions, _, err := client.RegionsApi.RegionsGetExecute(regionsRequest)
		if err != nil {
			v1api.Fatalf("GetRegions", err, t)
		}

		// Check the response
		if len(regions) != 1 && regions[0] != "global" {
			v1api.Fatalf("GetRegions", errors.New("bad response"), t)
		}
	})
}
