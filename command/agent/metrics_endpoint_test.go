package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"

	metrics "github.com/armon/go-metrics"
	"github.com/stretchr/testify/assert"
)

func TestHTTP_MetricsWithIllegalMethod(t *testing.T) {
	assert := assert.New(t)

	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		req, err := http.NewRequest("DELETE", "/v1/metrics", nil)
		assert.Nil(err)
		respW := httptest.NewRecorder()

		_, err = s.Server.MetricsRequest(respW, req)
		assert.NotNil(err, "HTTP DELETE should not be accepted for this endpoint")
	})
}

func TestHTTP_Metrics(t *testing.T) {
	assert := assert.New(t)

	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// make a separate HTTP request first, to ensure Nomad has written metrics
		// and prevent a race condition
		req, err := http.NewRequest("GET", "/v1/agent/self", nil)
		assert.Nil(err)
		respW := httptest.NewRecorder()
		s.Server.AgentSelfRequest(respW, req)

		// now make a metrics endpoint request, which should be already initialized
		// and written to
		req, err = http.NewRequest("GET", "/v1/metrics", nil)
		assert.Nil(err)
		respW = httptest.NewRecorder()

		resp, err := s.Server.MetricsRequest(respW, req)
		assert.Nil(err)

		res := resp.(metrics.MetricsSummary)

		gauges := res.Gauges
		assert.NotEqual(0, len(gauges))
	})
}
