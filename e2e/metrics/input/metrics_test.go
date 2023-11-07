// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package metrics

import (
	"context"
	"fmt"
	"testing"
	"time"

	nomadapi "github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/v3/cluster3"
	"github.com/hashicorp/nomad/e2e/v3/jobs3"
	promapi "github.com/prometheus/client_golang/api"
	promapi1 "github.com/prometheus/client_golang/api/prometheus/v1"
	promodel "github.com/prometheus/common/model"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
)

type metric struct {
	name   string
	filter string
	key    string
	value  float64
	sum    bool
}

func (m *metric) String() string {
	return fmt.Sprintf("%s[%s]=%v", m.name, m.key, m.value)
}

func (m *metric) Query() string {
	query := fmt.Sprintf("%s{%s=%q}", m.name, m.filter, m.key)
	if m.sum {
		query = "sum(" + query + ")"
	}
	return query
}

func TestMetrics(t *testing.T) {
	// Always run via the e2e suite; requires AWS specific attributes.

	cluster3.Establish(t,
		cluster3.Leader(),
		cluster3.LinuxClients(1),
		cluster3.WindowsClients(1),
	)

	// TODO: jobWinHP cannot export metrics until we upgrade from Windows 2016
	// _, cleanupWinHP := jobs3.Submit(t, "./input/winagent.hcl")
	// t.Cleanup(cleanupWinHP)

	jobCPU, cleanupCPU := jobs3.Submit(t, "./input/cpustress.hcl")
	t.Cleanup(cleanupCPU)

	jobHP, cleanupHP := jobs3.Submit(t, "./input/nomadagent.hcl")
	t.Cleanup(cleanupHP)

	_, cleanupProm := jobs3.Submit(t, "./input/prometheus.hcl")
	t.Cleanup(cleanupProm)

	jobPy, cleanupPy := jobs3.Submit(t, "./input/pythonhttp.hcl")
	t.Cleanup(cleanupPy)

	_, cleanupCaddy := jobs3.Submit(t, "./input/caddy.hcl")
	t.Cleanup(cleanupCaddy)

	t.Log("let the metrics collect for a bit (15s)")
	time.Sleep(15 * time.Second)

	testAllocMetrics(t, []*metric{{
		name:   "nomad_client_allocs_memory_usage",
		filter: "exported_job",
		key:    jobHP.JobID(),
	}, {
		name:   "nomad_client_allocs_cpu_user",
		filter: "exported_job",
		key:    jobCPU.JobID(),
	}, {
		name:   "nomad_client_allocs_cpu_allocated",
		filter: "exported_job",
		key:    jobPy.JobID(),
	},
	/*
		// todo: reenable windows once we upgrade and can support api.sock
		{
			name:   "nomad_client_allocs_memory_usage",
			filter: "exported_job",
			key:    jobWinHP.JobID(),
		},
	*/
	})

	testClientMetrics(t, []*metric{{
		name: "nomad_client_allocated_memory",
	}, {
		name: "nomad_client_host_cpu_user",
		sum:  true, // metric is per core
	}, {
		name: "nomad_client_host_memory_used",
	}, {
		name: "nomad_client_uptime",
	}})
}

func testAllocMetrics(t *testing.T, metrics []*metric) {
	// query metrics and update values
	query(t, metrics)

	// assert each metric has a positive value
	positives(t, metrics)
}

func testClientMetrics(t *testing.T, metrics []*metric) {
	nodes, _, err := e2eutil.NomadClient(t).Nodes().List(&nomadapi.QueryOptions{
		// TODO: unrestrict once we upgrade Windows and can support api.sock
		Filter: fmt.Sprintf("Attributes[%q] == %q", "kernel.name", "linux"),
	})
	must.NoError(t, err)

	// permute each metric per node
	results := make([]*metric, 0, len(nodes)*len(metrics))
	for _, node := range nodes {
		for _, m := range metrics {
			results = append(results, &metric{
				name:   m.name,
				filter: "node_id",
				key:    node.ID,
				sum:    m.sum,
			})
		}
	}

	// query metrics and update values
	query(t, results)

	// assert each metric has a positive value
	positives(t, results)
}

func query(t *testing.T, metrics []*metric) {
	services := e2eutil.NomadClient(t).Services()
	regs, _, err := services.Get("caddy", &nomadapi.QueryOptions{
		Filter: `Tags contains "expose"`,
	})
	must.NoError(t, err, must.Sprint("unable to query nomad for caddy service"))
	must.Len(t, 1, regs, must.Sprint("expected one caddy instance"))

	prom := regs[0] // tag[0] is public aws address
	address := fmt.Sprintf("http://%s:%d", prom.Tags[0], prom.Port)
	opts := promapi.Config{Address: address}
	t.Log("expose prometheus http address", address)

	client, err := promapi.NewClient(opts)
	must.NoError(t, err, must.Sprint("unable to create prometheus api client"))

	api1 := promapi1.NewAPI(client)

	const timeout = 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for _, m := range metrics {
		q := m.Query()
		t.Log("query for metric", q)
		result, warnings, err := api1.Query(ctx, q, time.Now(), promapi1.WithTimeout(timeout))
		must.NoError(t, err, must.Sprintf("unable to query %q", q))
		must.SliceEmpty(t, warnings, must.Sprintf("got warnings %v", warnings))

		// extract the actual value
		vector, ok := result.(promodel.Vector)
		must.True(t, ok, must.Sprint("unable to convert metric to vector"))
		must.SliceNotEmpty(t, vector, must.Sprintf("vector for %s is empty", m.String()))
		sample := vector[len(vector)-1]
		m.value = float64(sample.Value)
	}
}

func positives(t *testing.T, metrics []*metric) {
	// just ensure each metric value is positive
	for _, m := range metrics {
		test.Positive(t, m.value, test.Sprintf(
			"%s should have been positive",
			m.String(),
		))
	}
}
