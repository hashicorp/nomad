// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package spread

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/assert"
)

const (
	evenJobFilePath     = "./input/even_spread.nomad"
	multipleJobFilePath = "./input/multiple_spread.nomad"
)

func TestSpread(t *testing.T) {

	nomadClient := e2eutil.NomadClient(t)
	e2eutil.WaitForLeader(t, nomadClient)
	e2eutil.WaitForNodesReady(t, nomadClient, 4)

	// Run our test cases.
	t.Run("TestSpread_Even", testSpreadEven)
	t.Run("TestSpread_Multiple", testSpreadMultiple)
}

func testSpreadEven(t *testing.T) {

	nomadClient := e2eutil.NomadClient(t)

	// Generate a job ID and register the test job.
	jobID := "spread-" + uuid.Short()
	allocs := e2eutil.RegisterAndWaitForAllocs(t, nomadClient, evenJobFilePath, jobID, "")

	// Ensure the test cleans its own job and allocations fully, so it does not
	// impact other spread tests.
	t.Cleanup(func() { cleanupJob(t, nomadClient, jobID, allocs) })

	dcToAllocs := make(map[string]int)

	for _, allocStub := range allocs {
		alloc, _, err := nomadClient.Allocations().Info(allocStub.ID, nil)
		must.NoError(t, err)
		must.Greater(t, 0, len(alloc.Metrics.ScoreMetaData))
		node, _, err := nomadClient.Nodes().Info(alloc.NodeID, nil)
		must.NoError(t, err)
		dcToAllocs[node.Datacenter]++
	}

	must.Eq(t, map[string]int{"dc1": 3, "dc2": 3}, dcToAllocs)
}

func testSpreadMultiple(t *testing.T) {

	nomadClient := e2eutil.NomadClient(t)

	// Generate a job ID and register the test job.
	jobID := "spread-" + uuid.Short()
	allocs := e2eutil.RegisterAndWaitForAllocs(t, nomadClient, multipleJobFilePath, jobID, "")

	// Ensure the test cleans its own job and allocations fully, so it does not
	// impact other spread tests.
	t.Cleanup(func() { cleanupJob(t, nomadClient, jobID, allocs) })

	// Verify spread score and alloc distribution
	dcToAllocs := make(map[string]int)
	rackToAllocs := make(map[string]int)
	allocMetrics := make(map[string]*api.AllocationMetric)

	for _, allocStub := range allocs {
		alloc, _, err := nomadClient.Allocations().Info(allocStub.ID, nil)
		must.NoError(t, err)
		must.Greater(t, 0, len(alloc.Metrics.ScoreMetaData))
		allocMetrics[allocStub.ID] = alloc.Metrics

		node, _, err := nomadClient.Nodes().Info(alloc.NodeID, nil)
		must.NoError(t, err)
		dcToAllocs[node.Datacenter]++

		if rack := node.Meta["rack"]; rack != "" {
			rackToAllocs[rack]++
		}
	}

	failureReport := report(allocMetrics)

	must.Eq(t, map[string]int{"dc1": 5, "dc2": 5}, dcToAllocs, failureReport)
	must.Eq(t, map[string]int{"r1": 7, "r2": 3}, rackToAllocs, failureReport)
}

func cleanupJob(t *testing.T, nomadClient *api.Client, jobID string, allocs []*api.AllocationListStub) {

	_, _, err := nomadClient.Jobs().Deregister(jobID, true, nil)
	assert.NoError(t, err)

	// Ensure that all allocations have been removed from state. This is an
	// important aspect of the cleaning required which allows the spread
	// test to run successfully.
	assert.Eventually(t, func() bool {

		// Run the garbage collector to remove all terminal allocations.
		must.NoError(t, nomadClient.System().GarbageCollect())

		for _, allocStub := range allocs {
			_, _, err := nomadClient.Allocations().Info(allocStub.ID, nil)
			if err == nil {
				return false
			} else {
				if !strings.Contains(err.Error(), "alloc not found") {
					return false
				}
			}
		}
		return true
	}, 10*time.Second, 200*time.Millisecond)
}

func report(metrics map[string]*api.AllocationMetric) must.Setting {
	var s strings.Builder
	for allocID, m := range metrics {
		s.WriteString("Alloc ID: " + allocID + "\n")
		s.WriteString(fmt.Sprintf("  NodesEvaluated: %d\n", m.NodesEvaluated))
		s.WriteString(fmt.Sprintf("  NodesAvailable: %#v\n", m.NodesAvailable))
		s.WriteString(fmt.Sprintf("  ClassFiltered: %#v\n", m.ClassFiltered))
		s.WriteString(fmt.Sprintf("  ConstraintFiltered: %#v\n", m.ConstraintFiltered))
		s.WriteString(fmt.Sprintf("  NodesExhausted: %d\n", m.NodesExhausted))
		s.WriteString(fmt.Sprintf("  ClassExhausted: %#v\n", m.ClassExhausted))
		s.WriteString(fmt.Sprintf("  DimensionExhausted: %#v\n", m.DimensionExhausted))
		s.WriteString(fmt.Sprintf("  QuotaExhausted: %#v\n", m.QuotaExhausted))
		for _, nodeMeta := range m.ScoreMetaData {
			s.WriteString(fmt.Sprintf("    NodeID: %s, NormScore: %f, Scores: %#v\n",
				nodeMeta.NodeID, nodeMeta.NormScore, nodeMeta.Scores))
		}
	}
	return must.Sprint(s.String())
}
