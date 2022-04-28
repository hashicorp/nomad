package api

import (
	"testing"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/stretchr/testify/require"
)

func TestOperator_MetricsSummary(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	operator := c.Operator()
	qo := &QueryOptions{
		Params: map[string]string{
			"pretty": "1",
		},
	}

	metrics, qm, err := operator.MetricsSummary(qo)
	require.NoError(t, err)
	require.NotNil(t, metrics)
	require.NotNil(t, qm)
	require.NotNil(t, metrics.Timestamp)                // should always get a TimeStamp
	require.GreaterOrEqual(t, len(metrics.Points), 0)   // may not have points yet
	require.GreaterOrEqual(t, len(metrics.Gauges), 1)   // should have at least 1 gauge
	require.GreaterOrEqual(t, len(metrics.Counters), 1) // should have at least 1 counter
	require.GreaterOrEqual(t, len(metrics.Samples), 1)  // should have at least 1 sample
}

func TestOperator_Metrics_Prometheus(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) {
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
	require.NoError(t, err)
	require.NotNil(t, metrics)
	metricString := string(metrics[:])
	require.Containsf(t, metricString, "# HELP", "expected Prometheus format containing \"# HELP\", got: \n%s", metricString)
}
