package agent

import (
	"github.com/hashicorp/nomad/testutil/openapi"
	"github.com/stretchr/testify/require"
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

		client, ctx := openapi.NewClientAndContext(s.Config.BindAddr, strconv.Itoa(s.Config.Ports.HTTP))
		regionsRequest := client.RegionsApi.RegionsGet(ctx)

		regions, _, err := client.RegionsApi.RegionsGetExecute(regionsRequest)
		require.NoError(t, err)
		require.Len(t, regions, 1)
		require.Equal(t, "global", regions[0])
	})
}
