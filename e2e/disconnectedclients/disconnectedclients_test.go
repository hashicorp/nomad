package disconnectedclients

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

const ns = ""

// typical wait times for this test package
var wait30s = &e2eutil.WaitConfig{Interval: time.Second, Retries: 30}
var wait60s = &e2eutil.WaitConfig{Interval: time.Second, Retries: 60}

func TestDisconnectedClients(t *testing.T) {
	nomad := e2eutil.NomadClient(t)

	e2eutil.WaitForLeader(t, nomad)
	e2eutil.WaitForNodesReady(t, nomad, 2) // needs at least 2 to test replacement

	t.Run("AllocReplacementOnShutdown", testDisconnected_AllocReplacementOnShutdown)
}

// disconnectedClientsCleanup sets up a cleanup function to make sure
// we've waited for all the nodes to come back up between tests
func disconnectedClientsCleanup(t *testing.T) func() {
	nodeIDs := []string{}
	nodeStatuses, err := e2eutil.NodeStatusList()
	require.NoError(t, err)
	for _, nodeStatus := range nodeStatuses {
		nodeIDs = append(nodeIDs, nodeStatus["ID"])
	}
	return func() {
		nomad := e2eutil.NomadClient(t)
		e2eutil.WaitForNodesReady(t, nomad, len(nodeIDs))
	}
}

// testDisconnected_AllocReplacementOnShutdown tests that allocations on
// clients that are shut down and marked disconnected are replaced
func testDisconnected_AllocReplacementOnShutdown(t *testing.T) {

	jobIDs := []string{}
	t.Cleanup(disconnectedClientsCleanup(t))
	t.Cleanup(e2eutil.CleanupJobsAndGC(t, &jobIDs))

	jobID := "test-lost-allocs-" + uuid.Short()

	err := e2eutil.Register(jobID, "./input/lost_simple.nomad")
	require.NoError(t, err)
	jobIDs = append(jobIDs, jobID)

	err = e2eutil.WaitForAllocStatusExpected(jobID, ns,
		[]string{"running", "running"})
	require.NoError(t, err, "job should be running")

	// pick a node to make our lost node
	allocs, err := e2eutil.AllocsForJob(jobID, ns)
	require.NoError(t, err, "could not query allocs for job")
	require.Len(t, allocs, 2, "could not find 2 allocs for job")

	lostAlloc := allocs[0]
	lostAllocID := lostAlloc["ID"]
	disconnectedNodeID := lostAlloc["Node ID"]
	otherAllocID := allocs[1]["ID"]

	restartJobID, err := e2eutil.AgentRestartAfter(disconnectedNodeID, 30*time.Second)
	require.NoError(t, err, "expected agent restart job to register")
	jobIDs = append(jobIDs, restartJobID)

	err = e2eutil.WaitForNodeStatus(disconnectedNodeID, "down", wait30s)
	require.NoError(t, err, "expected node to go down")

	err = waitForAllocStatusMap(jobID, map[string]string{
		lostAllocID:  "lost",
		otherAllocID: "running",
		"":           "running",
	}, wait60s)
	require.NoError(t, err, "expected alloc on disconnected client to be marked lost and replaced")

	allocs, err = e2eutil.AllocsForJob(jobID, ns)
	require.NoError(t, err, "could not query allocs for job")
	require.Len(t, allocs, 3, "could not find 3 allocs for job")

	err = e2eutil.WaitForNodeStatus(disconnectedNodeID, "ready", wait30s)
	require.NoError(t, err, "expected node to come back up")

	err = waitForAllocStatusMap(jobID, map[string]string{
		lostAllocID:  "complete",
		otherAllocID: "running",
		"":           "running",
	}, wait30s)
	require.NoError(t, err, "expected lost alloc on reconnected client to be marked complete and replaced")
}

func waitForAllocStatusMap(jobID string, allocsToStatus map[string]string, wc *e2eutil.WaitConfig) error {
	var err error
	interval, retries := wc.OrDefault()
	testutil.WaitForResultRetries(retries, func() (bool, error) {
		time.Sleep(interval)
		allocs, err := e2eutil.AllocsForJob(jobID, ns)
		if err != nil {
			return false, err
		}
		for _, alloc := range allocs {
			if expectedAllocStatus, ok := allocsToStatus[alloc["ID"]]; ok {
				if alloc["Status"] != expectedAllocStatus {
					return false, fmt.Errorf("expected status of alloc %q to be %q, got %q",
						alloc["ID"], expectedAllocStatus, alloc["Status"])
				}
			} else {
				if alloc["Status"] != allocsToStatus[""] {
					return false, fmt.Errorf("expected status of alloc %q to be %q, got %q",
						alloc["ID"], expectedAllocStatus, alloc["Status"])
				}
			}
		}
		return true, nil
	}, func(e error) {
		err = e
	})
	return err
}
