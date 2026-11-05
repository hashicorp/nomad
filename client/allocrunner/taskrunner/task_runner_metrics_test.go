// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"fmt"
	"sync"
	"testing"

	metrics "github.com/hashicorp/go-metrics"
	metricsprom "github.com/hashicorp/go-metrics/prometheus"
	"github.com/hashicorp/nomad/client/config"
	cstate "github.com/hashicorp/nomad/client/state"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/shoenig/test/must"
)

// Replaces the process-wide metrics sink.
func TestTaskRunner_OOMCounter(t *testing.T) {
	registry := prometheus.NewRegistry()
	sink, err := metricsprom.NewPrometheusSinkFrom(metricsprom.PrometheusOpts{
		Registerer: registry,
	})
	must.NoError(t, err)

	previous := metrics.Default()
	_, err = metrics.NewGlobal(&metrics.Config{ServiceName: "nomad", FilterDefault: true}, sink)
	must.NoError(t, err)
	t.Cleanup(func() {
		_, err := metrics.NewGlobal(&metrics.Config{FilterDefault: true}, previous)
		must.NoError(t, err)
	})

	logger := testlog.HCLogger(t)
	tr := &TaskRunner{
		clientConfig: &config.Config{PublishAllocationMetrics: true},
		baseLabels:   []metrics.Label{{Name: "task", Value: "oom-counter-test"}},
		logger:       logger,
		state:        structs.NewTaskState(),
		stateDB:      cstate.NewMemDB(logger),
		stateUpdater: NewMockTaskStateUpdater(),
		maxEvents:    10,
	}
	ru := &cstructs.TaskResourceUsage{ResourceUsage: &cstructs.ResourceUsage{}}

	checkCounter := func(expected float64) {
		t.Helper()
		families, err := registry.Gather()
		must.NoError(t, err)
		for _, family := range families {
			if family.GetName() == "nomad_client_allocs_oom_killed" {
				must.Len(t, 1, family.Metric)
				must.Eq(t, expected, family.Metric[0].GetCounter().GetValue())
				must.Len(t, 1, family.Metric[0].Label)
				must.Eq(t, "task", family.Metric[0].Label[0].GetName())
				must.Eq(t, "oom-counter-test", family.Metric[0].Label[0].GetValue())
				return
			}
		}
		t.Fatal("OOM counter missing from Prometheus metrics")
	}

	tr.UpdateStats(ru)
	checkCounter(0)
	tr.UpdateStats(ru)
	checkCounter(0)

	tr.emitExitResultEvent(&drivers.ExitResult{OOMKilled: true})
	tr.UpdateStats(ru)
	checkCounter(1)

	tr.clientConfig.PublishAllocationMetrics = false
	tr.baseLabels = []metrics.Label{{Name: "task", Value: "metrics-disabled"}}
	tr.UpdateStats(ru)
	checkCounter(1)
	// First stats and OOM updates can arrive together for a new task.
	const trials = 10000
	for i := range trials {
		concurrent := &TaskRunner{
			baseLabels: []metrics.Label{{Name: "task", Value: fmt.Sprintf("concurrent-%d", i)}},
		}
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			concurrent.emitOOMCounter(0)
		}()
		go func() {
			defer wg.Done()
			<-start
			concurrent.emitOOMCounter(1)
		}()
		close(start)
		wg.Wait()
	}

	families, err := registry.Gather()
	must.NoError(t, err)
	for _, family := range families {
		if family.GetName() == "nomad_client_allocs_oom_killed" {
			must.Len(t, trials+1, family.Metric)
			for _, metric := range family.Metric {
				must.Eq(t, float64(1), metric.GetCounter().GetValue(),
					must.Sprintf("lost OOM increment for %s", metric.Label[0].GetValue()))
			}
		}
	}

}
