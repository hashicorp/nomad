package api

import (
	"testing"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test/must"
)

func TestOperator_MetricsSummary(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) {
		c.DevMode = true
	})
	defer s.Stop()

	operator := c.Operator()
	qo := &QueryOptions{
		Params: map[string]string{
			"pretty": "1",
		},
	}

	metrics, qm, err := operator.MetricsSummary(qo)
	must.NoError(t, err)
	must.NotNil(t, metrics)
	must.NotNil(t, qm)
	must.NotNil(t, metrics.Timestamp)       // should always get a TimeStamp
	must.SliceEmpty(t, metrics.Points)      // may not have points yet
	must.SliceNotEmpty(t, metrics.Gauges)   // should have at least 1 gauge
	must.SliceNotEmpty(t, metrics.Counters) // should have at least 1 counter
	must.SliceNotEmpty(t, metrics.Samples)  // should have at least 1 sample
}

func TestOperator_Metrics_Prometheus(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) {
		c.DevMode = true
		c.Telemetry = &testutil.Telemetry{PrometheusMetrics: true}
	})
	defer s.Stop()

	operator := c.Operator()
	qo := &QueryOptions{
		Params: map[string]string{
			"format": "prometheus",
		},
	}

	metrics, err := operator.Metrics(qo)
	must.NoError(t, err)
	must.NotNil(t, metrics)
	metricString := string(metrics[:])
	must.StrContains(t, metricString, "# HELP")
}
