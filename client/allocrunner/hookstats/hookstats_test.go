// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hookstats

import (
	"errors"
	"testing"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestHandler(t *testing.T) {
	ci.Parallel(t)

	// Generate base labels that represent what an operator would see and then
	// create out new handler to interact with.
	baseLabels := []metrics.Label{
		{Name: "datacenter", Value: "dc1"},
		{Name: "node_class", Value: "none"},
		{Name: "node_pool", Value: "default"},
		{Name: "namespace", Value: "default"},
		{Name: "host", Value: "client-5d3c"},
		{Name: "node_id", Value: "35db24e7-0a55-80d2-2279-e022c37cc591"},
	}
	newHandler := NewHandler(baseLabels, "test_hook")

	// The data stored is within the in-memory sink as map entries, so we need
	// to know the key names to pull this out correctly. Build those now.
	var metricKeySuffix, sampleName, counterSuccessName, counterFailureName string

	for _, label := range baseLabels {
		metricKeySuffix += ";" + label.Name + "=" + label.Value
	}

	metricKeySuffix += ";" + "hook_name=test_hook_name"
	sampleName = "nomad_test.client.test_hook.prerun.elapsed" + metricKeySuffix
	counterSuccessName = "nomad_test.client.test_hook.prerun.success" + metricKeySuffix
	counterFailureName = "nomad_test.client.test_hook.prerun.failed" + metricKeySuffix

	// Create an in-memory sink and global, so we can actually look at and test
	// the metrics we emit.
	inMemorySink := metrics.NewInmemSink(10*time.Millisecond, 50*time.Millisecond)

	_, err := metrics.NewGlobal(metrics.DefaultConfig("nomad_test"), inMemorySink)
	must.NoError(t, err)

	// Emit hook related metrics where the supplied error is nil and check that
	// the data is as expected.
	newHandler.Emit(time.Now(), "test_hook_name", "prerun", nil)

	sinkData := inMemorySink.Data()
	must.Len(t, 1, sinkData)
	must.MapContainsKey(t, sinkData[0].Counters, counterSuccessName)
	must.MapContainsKey(t, sinkData[0].Samples, sampleName)

	successCounter := sinkData[0].Counters[counterSuccessName]
	must.Eq(t, 1, successCounter.Count)
	must.Eq(t, 1, successCounter.Sum)

	sample1 := sinkData[0].Samples[sampleName]
	must.Eq(t, 1, sample1.Count)
	must.True(t, sample1.Sum > 0)

	// Create a new in-memory sink and global collector to ensure we don't have
	// leftovers from the previous test.
	inMemorySink = metrics.NewInmemSink(10*time.Millisecond, 50*time.Millisecond)

	_, err = metrics.NewGlobal(metrics.DefaultConfig("nomad_test"), inMemorySink)
	must.NoError(t, err)

	// Emit a hook related metrics where the supplied error is non-nil and
	// check that the data is as expected.
	newHandler.Emit(time.Now(), "test_hook_name", "prerun", errors.New("test error"))

	sinkData = inMemorySink.Data()
	must.Len(t, 1, sinkData)
	must.MapContainsKey(t, sinkData[0].Counters, counterFailureName)
	must.MapContainsKey(t, sinkData[0].Samples, sampleName)

	failureCounter := sinkData[0].Counters[counterFailureName]
	must.Eq(t, 1, failureCounter.Count)
	must.Eq(t, 1, failureCounter.Sum)

	sample2 := sinkData[0].Samples[sampleName]
	must.Eq(t, 1, sample2.Count)
	must.True(t, sample2.Sum > 0)
}

func TestNoOpHandler(t *testing.T) {
	ci.Parallel(t)

	newHandler := NewNoOpHandler()

	// Create a new in-memory sink and global collector, so we can test that no
	// metrics are emitted.
	inMemorySink := metrics.NewInmemSink(10*time.Millisecond, 50*time.Millisecond)

	_, err := metrics.NewGlobal(metrics.DefaultConfig("nomad_test"), inMemorySink)
	must.NoError(t, err)

	// Call the function with a non-nil error and check the results of the
	// in-memory sink.
	newHandler.Emit(time.Now(), "test_hook_name", "prerun", errors.New("test error"))

	sinkData := inMemorySink.Data()
	must.Len(t, 1, sinkData)
	must.MapLen(t, 0, sinkData[0].Counters)
	must.MapLen(t, 0, sinkData[0].Samples)
}
