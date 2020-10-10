package api

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOperator_RaftGetConfiguration(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	operator := c.Operator()
	out, err := operator.RaftGetConfiguration(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(out.Servers) != 1 ||
		!out.Servers[0].Leader ||
		!out.Servers[0].Voter {
		t.Fatalf("bad: %v", out)
	}
}

func TestOperator_RaftRemovePeerByAddress(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	// If we get this error, it proves we sent the address all the way
	// through.
	operator := c.Operator()
	err := operator.RaftRemovePeerByAddress("nope", nil)
	if err == nil || !strings.Contains(err.Error(),
		"address \"nope\" was not found in the Raft configuration") {
		t.Fatalf("err: %v", err)
	}
}

func TestOperator_RaftRemovePeerByID(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	// If we get this error, it proves we sent the address all the way
	// through.
	operator := c.Operator()
	err := operator.RaftRemovePeerByID("nope", nil)
	if err == nil || !strings.Contains(err.Error(),
		"id \"nope\" was not found in the Raft configuration") {
		t.Fatalf("err: %v", err)
	}
}

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
	c, s := makeClient(t, nil, nil)
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
