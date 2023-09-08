// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTP_MetricsWithIllegalMethod(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)

	httpTest(t, nil, func(s *TestAgent) {
		req, err := http.NewRequest(http.MethodDelete, "/v1/metrics", nil)
		assert.Nil(err)
		respW := httptest.NewRecorder()

		_, err = s.Server.MetricsRequest(respW, req)
		assert.NotNil(err, "HTTP DELETE should not be accepted for this endpoint")
	})
}

func TestHTTP_MetricsPrometheusDisabled(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)

	httpTest(t, func(c *Config) { c.Telemetry.PrometheusMetrics = false }, func(s *TestAgent) {
		req, err := http.NewRequest(http.MethodGet, "/v1/metrics?format=prometheus", nil)
		assert.Nil(err)

		resp, err := s.Server.MetricsRequest(nil, req)
		assert.Nil(resp)
		assert.Error(err, "Prometheus is not enabled")
	})
}

func TestHTTP_MetricsPrometheusEnabled(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)

	httpTest(t, nil, func(s *TestAgent) {
		req, err := http.NewRequest(http.MethodGet, "/v1/metrics?format=prometheus", nil)
		assert.Nil(err)
		respW := httptest.NewRecorder()

		resp, err := s.Server.MetricsRequest(respW, req)
		assert.Nil(resp)
		assert.Nil(err)

		// Ensure the response body is not empty and that it contains something
		// that looks like a metric we expect.
		assert.NotNil(respW.Body)
		assert.Contains(respW.Body.String(), "HELP go_gc_duration_seconds")
	})
}

func TestHTTP_Metrics(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)

	httpTest(t, nil, func(s *TestAgent) {
		// make a separate HTTP request first, to ensure Nomad has written metrics
		// and prevent a race condition
		req, err := http.NewRequest(http.MethodGet, "/v1/agent/self", nil)
		assert.Nil(err)
		respW := httptest.NewRecorder()
		s.Server.AgentSelfRequest(respW, req)

		// now make a metrics endpoint request, which should be already initialized
		// and written to
		req, err = http.NewRequest(http.MethodGet, "/v1/metrics", nil)
		assert.Nil(err)
		respW = httptest.NewRecorder()

		testutil.WaitForResult(func() (bool, error) {
			resp, err := s.Server.MetricsRequest(respW, req)
			if err != nil {
				return false, err
			}
			respW.Flush()

			res := resp.(metrics.MetricsSummary)
			return len(res.Gauges) != 0, nil
		}, func(err error) {
			t.Fatalf("should have metrics: %v", err)
		})
	})
}

// When emitting metrics, the client should use the local copy of the allocs with
// updated task states (not the copy submitted by the server).
//
// **Cannot** be run in parallel as metrics are global.
func TestHTTP_FreshClientAllocMetrics(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)
	numTasks := 10

	httpTest(t, func(c *Config) {
		c.Telemetry.PublishAllocationMetrics = true
		c.Telemetry.PublishNodeMetrics = true
	}, func(s *TestAgent) {
		// Create the job, wait for it to finish
		job := mock.BatchJob()
		job.TaskGroups[0].Count = numTasks
		testutil.RegisterJob(t, s.RPC, job)
		testutil.WaitForResult(func() (bool, error) {
			time.Sleep(200 * time.Millisecond)
			args := &structs.JobSpecificRequest{}
			args.JobID = job.ID
			args.QueryOptions.Region = "global"
			var resp structs.SingleJobResponse
			err := s.RPC("Job.GetJob", args, &resp)
			return err == nil && resp.Job.Status == "dead", err
		}, func(err error) {
			require.Fail("timed-out waiting for job to complete")
		})

		nodeID := s.client.NodeID()

		// wait for metrics to converge
		var pending, running, terminal float32 = -1.0, -1.0, -1.0
		testutil.WaitForResultRetries(100, func() (bool, error) {
			time.Sleep(100 * time.Millisecond)
			req, err := http.NewRequest(http.MethodGet, "/v1/metrics", nil)
			require.NoError(err)
			respW := httptest.NewRecorder()

			obj, err := s.Server.MetricsRequest(respW, req)
			if err != nil {
				return false, err
			}

			metrics := obj.(metrics.MetricsSummary)
			for _, g := range metrics.Gauges {

				// ignore client metrics belonging to other test nodes
				// from other tests that contaminate go-metrics reporting
				if g.DisplayLabels["node_id"] != nodeID {
					continue
				}

				if strings.HasSuffix(g.Name, "client.allocations.pending") {
					pending = g.Value
				}
				if strings.HasSuffix(g.Name, "client.allocations.running") {
					running = g.Value
				}
				if strings.HasSuffix(g.Name, "client.allocations.terminal") {
					terminal = g.Value
				}
			}
			// client alloc metrics should reflect that there is numTasks terminal allocs and no other allocs
			return pending == float32(0) && running == float32(0) &&
				terminal == float32(numTasks), nil
		}, func(err error) {
			require.Fail("timed out waiting for metrics to converge",
				"expected: (pending: 0, running: 0, terminal: %v), got: (pending: %v, running: %v, terminal: %v)", numTasks, pending, running, terminal)
		})
	})
}
