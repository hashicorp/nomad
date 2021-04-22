package nomad

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"
	"time"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

// testBlockedEvalsRandomBlockedEval wraps an eval that is randomly generated.
type testBlockedEvalsRandomBlockedEval struct {
	eval *structs.Evaluation
}

// Generate returns a random eval.
func (t testBlockedEvalsRandomBlockedEval) Generate(rand *rand.Rand, _ int) reflect.Value {
	resourceTypes := []string{"cpu", "memory"}

	// Start with a mock eval.
	e := mock.BlockedEval()

	// Get how many task groups, datacenters and node classes to generate.
	// Add 1 to avoid 0.
	tgCount := rand.Intn(10) + 1
	dcCount := rand.Intn(3) + 1
	nodeClassCount := rand.Intn(3) + 1

	failedTGAllocs := map[string]*structs.AllocMetric{}

	for tg := 1; tg <= tgCount; tg++ {
		tgName := fmt.Sprintf("group-%d", tg)

		// Get which resource type to use for this task group.
		// Nomad stops at the first dimension that is exhausted, so only 1 is
		// added per task group.
		i := rand.Int() % len(resourceTypes)
		resourceType := resourceTypes[i]

		failedTGAllocs[tgName] = &structs.AllocMetric{
			DimensionExhausted: map[string]int{
				resourceType: 1,
			},
			NodesAvailable: map[string]int{},
			ClassExhausted: map[string]int{},
		}

		for dc := 1; dc <= dcCount; dc++ {
			dcName := fmt.Sprintf("dc%d", dc)
			failedTGAllocs[tgName].NodesAvailable[dcName] = 1
		}

		for nc := 1; nc <= nodeClassCount; nc++ {
			nodeClassName := fmt.Sprintf("node-class-%d", nc)
			failedTGAllocs[tgName].ClassExhausted[nodeClassName] = 1
		}

		// Generate resources for each task.
		taskCount := rand.Intn(5) + 1
		resourcesExhausted := map[string]*structs.Resources{}

		for t := 1; t <= taskCount; t++ {
			task := fmt.Sprintf("task-%d", t)
			resourcesExhausted[task] = &structs.Resources{}

			resourceAmount := rand.Intn(1000)
			switch resourceType {
			case "cpu":
				resourcesExhausted[task].CPU = resourceAmount
			case "memory":
				resourcesExhausted[task].MemoryMB = resourceAmount
			}
		}
		failedTGAllocs[tgName].ResourcesExhausted = resourcesExhausted
	}
	e.FailedTGAllocs = failedTGAllocs
	t.eval = e
	return reflect.ValueOf(t)
}

// clearTimestampFromBlockedResourceStats set timestamp metrics to zero to
// avoid invalid comparisons.
func clearTimestampFromBlockedResourceStats(b *BlockedResourcesStats) {
	for k, v := range b.ByJob {
		v.Timestamp = time.Time{}
		b.ByJob[k] = v
	}
	for k, v := range b.ByNodeInfo {
		v.Timestamp = time.Time{}
		b.ByNodeInfo[k] = v
	}
}

// TestBlockedEvalsStats_BlockedResources generates random evals and processes
// them using the expected code paths and a manual check of the expeceted result.
func TestBlockedEvalsStats_BlockedResources(t *testing.T) {
	t.Parallel()
	blocked, _ := testBlockedEvals(t)

	// evalHistory stores all evals generated during the test.
	evalHistory := []*structs.Evaluation{}

	// blockedEvals keeps track if evals are blocked or unblocked.
	blockedEvals := map[string]bool{}

	// blockAndUntrack processes the generated evals in order using a
	// BlockedEvals instance.
	blockAndUntrack := func(testEval testBlockedEvalsRandomBlockedEval, block bool, unblockIdx uint16) BlockedResourcesStats {
		if block || len(evalHistory) == 0 {
			blocked.Block(testEval.eval)
		} else {
			i := int(unblockIdx) % len(evalHistory)
			eval := evalHistory[i]
			blocked.Untrack(eval.JobID, eval.Namespace)
		}

		// Remove zero stats from unblocked evals.
		blocked.pruneStats(time.Now().UTC())

		result := blocked.Stats().BlockedResources
		clearTimestampFromBlockedResourceStats(&result)
		return result
	}

	// manualCount processes only the blocked evals and generate a
	// BlockedResourcesStats result directly from the eval history.
	manualCount := func(testEval testBlockedEvalsRandomBlockedEval, block bool, unblockIdx uint16) BlockedResourcesStats {
		if block || len(evalHistory) == 0 {
			evalHistory = append(evalHistory, testEval.eval)
			blockedEvals[testEval.eval.ID] = true
		} else {
			i := int(unblockIdx) % len(evalHistory)
			eval := evalHistory[i]
			blockedEvals[eval.ID] = false
		}

		result := NewBlockedResourcesStats()
		for _, e := range evalHistory {
			if !blockedEvals[e.ID] {
				continue
			}
			result = result.Add(generateResourceStats(e))
		}
		clearTimestampFromBlockedResourceStats(&result)
		return result
	}

	err := quick.CheckEqual(blockAndUntrack, manualCount, nil)
	if err != nil {
		t.Error(err)
	}
}
