// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package metrics

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/testutil"
	"github.com/prometheus/common/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MetricsTest struct {
	framework.TC
	jobIDs       []string
	prometheusID string
	fabioID      string
	fabioAddress string
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "Metrics",
		CanRunLocal: true,
		Cases: []framework.TestCase{
			new(MetricsTest),
		},
	})
}

// BeforeAll stands up Prometheus to collect metrics from all clients and
// allocs, with fabio as a system job in front of it so that we don't need to
// have prometheus use host networking.
func (tc *MetricsTest) BeforeAll(f *framework.F) {
	t := f.T()
	e2eutil.WaitForLeader(t, tc.Nomad())
	e2eutil.WaitForNodesReady(t, tc.Nomad(), 1)
	err := tc.setUpPrometheus(f)
	require.Nil(t, err)
}

// AfterEach CleanS up the target jobs after each test case, but keep
// fabio/prometheus for reuse between the two test cases (Windows vs Linux).
func (tc *MetricsTest) AfterEach(f *framework.F) {
	if os.Getenv("NOMAD_TEST_SKIPCLEANUP") == "1" {
		return
	}
	for _, jobID := range tc.jobIDs {
		tc.Nomad().Jobs().Deregister(jobID, true, nil)
	}
	tc.jobIDs = []string{}
	tc.Nomad().System().GarbageCollect()
}

// AfterAll cleans up fabio/prometheus.
func (tc *MetricsTest) AfterAll(f *framework.F) {
	if os.Getenv("NOMAD_TEST_SKIPCLEANUP") == "1" {
		return
	}
	tc.tearDownPrometheus(f)
}

// TestMetricsLinux runs a collection of jobs that exercise alloc metrics.
// Then we query prometheus to verify we're collecting client and alloc metrics
// and correctly presenting them to the prometheus scraper.
func (tc *MetricsTest) TestMetricsLinux(f *framework.F) {
	t := f.T()
	clientNodes, err := e2eutil.ListLinuxClientNodes(tc.Nomad())
	require.Nil(t, err)
	if len(clientNodes) == 0 {
		t.Skip("no Linux clients")
	}

	workloads := map[string]string{
		"cpustress":  "nomad_client_allocs_cpu_user",
		"diskstress": "nomad_client_allocs_memory_rss", // TODO(tgross): do we have disk stats?
		"helloworld": "nomad_client_allocs_cpu_allocated",
		"memstress":  "nomad_client_allocs_memory_usage",
		"simpleweb":  "nomad_client_allocs_memory_rss",
	}

	tc.runWorkloads(t, workloads)
	tc.queryClientMetrics(t, clientNodes)
	tc.queryAllocMetrics(t, workloads)
}

// TestMetricsWindows runs a collection of jobs that exercise alloc metrics.
// Then we query prometheus to verify we're collecting client and alloc metrics
// and correctly presenting them to the prometheus scraper.
func (tc *MetricsTest) TestMetricsWindows(f *framework.F) {
	t := f.T()
	clientNodes, err := e2eutil.ListWindowsClientNodes(tc.Nomad())
	require.Nil(t, err)
	if len(clientNodes) == 0 {
		t.Skip("no Windows clients")
	}

	workloads := map[string]string{
		"factorial_windows": "nomad_client_allocs_cpu_user",
		"mem_windows":       "nomad_client_allocs_memory_rss",
	}

	tc.runWorkloads(t, workloads)
	tc.queryClientMetrics(t, clientNodes)
	tc.queryAllocMetrics(t, workloads)
}

// run workloads and wait for allocations
func (tc *MetricsTest) runWorkloads(t *testing.T, workloads map[string]string) {
	for jobName := range workloads {
		uuid := uuid.Generate()
		jobID := "metrics-" + jobName + "-" + uuid[0:8]
		tc.jobIDs = append(tc.jobIDs, jobID)
		file := "metrics/input/" + jobName + ".nomad"
		allocs := e2eutil.RegisterAndWaitForAllocs(t, tc.Nomad(), file, jobID, "")
		require.NotZerof(t, allocs, "failed to register %s", jobID)
	}
}

// query prometheus to verify that metrics are being collected
// from clients
func (tc *MetricsTest) queryClientMetrics(t *testing.T, clientNodes []string) {
	metrics := []string{
		"nomad_client_allocated_memory",
		"nomad_client_host_cpu_user",
		"nomad_client_host_disk_available",
		"nomad_client_host_memory_used",
		"nomad_client_uptime",
	}
	// we start with a very long timeout here because it takes a while for
	// prometheus to be live and for jobs to initially register metrics.
	retries := int64(60)

	for _, metric := range metrics {

		var results model.Vector
		var err error

		testutil.WaitForResultRetries(retries, func() (bool, error) {
			defer time.Sleep(time.Second)

			results, err = tc.promQuery(metric)
			if err != nil {
				return false, err
			}

			instances := make(map[string]struct{})
			for _, result := range results {
				instances[string(result.Metric["node_id"])] = struct{}{}
			}
			// we're testing only clients for a specific OS, so we
			// want to make sure we're checking for specific node_ids
			// and not just equal lengths
			for _, clientNode := range clientNodes {
				if _, ok := instances[clientNode]; !ok {
					return false, fmt.Errorf("expected metric '%s' for all clients. got:\n%v", metric, results)
				}
			}
			return true, nil
		}, func(err error) {
			require.NoError(t, err)
		})

		// shorten the timeout after the first workload is successfully
		// queried so that we don't hang the whole test run if something's
		// wrong with only one of the jobs
		retries = 15
	}
}

// query promtheus to verify that metrics are being collected
// from allocations
func (tc *MetricsTest) queryAllocMetrics(t *testing.T, workloads map[string]string) {
	// we start with a very long timeout here because it takes a while for
	// prometheus to be live and for jobs to initially register metrics.
	timeout := 60 * time.Second
	for jobName, metric := range workloads {
		query := fmt.Sprintf("%s{exported_job=\"%s\"}", metric, jobName)
		var results model.Vector
		var err error
		ok := assert.Eventually(t, func() bool {
			results, err = tc.promQuery(query)
			if err != nil {
				return false
			}

			// make sure we didn't just collect a bunch of zero metrics
			lastResult := results[len(results)-1]
			if !(float64(lastResult.Value) > 0.0) {
				err = fmt.Errorf("expected non-zero metrics, got: %v", results)
				return false
			}
			return true
		}, timeout, 1*time.Second)
		require.Truef(t, ok, "prometheus query failed (%s): %v", query, err)

		// shorten the timeout after the first workload is successfully
		// queried so that we don't hang the whole test run if something's
		// wrong with only one of the jobs
		timeout = 15 * time.Second
	}
}
