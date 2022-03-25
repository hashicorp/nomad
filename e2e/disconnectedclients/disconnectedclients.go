package disconnectedclients

import (
	"fmt"
	"os"
	"time"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/testutil"
)

type DisconnectedClientsE2ETest struct {
	framework.TC
	jobIDs  []string
	nodeIDs []string
}

const ns = ""

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "DisconnectedClients",
		CanRunLocal: true,
		Cases: []framework.TestCase{
			new(DisconnectedClientsE2ETest),
		},
	})

}

func (tc *DisconnectedClientsE2ETest) BeforeAll(f *framework.F) {
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 2) // needs at least 2 to test replacement

	nodeStatuses, err := e2eutil.NodeStatusList()
	f.NoError(err)
	for _, nodeStatus := range nodeStatuses {
		tc.nodeIDs = append(tc.nodeIDs, nodeStatus["ID"])
	}
}

func (tc *DisconnectedClientsE2ETest) AfterEach(f *framework.F) {
	if os.Getenv("NOMAD_TEST_SKIPCLEANUP") == "1" {
		return
	}

	for _, id := range tc.jobIDs {
		_, err := e2eutil.Command("nomad", "job", "stop", "-purge", id)
		f.Assert().NoError(err)
	}
	tc.jobIDs = []string{}

	_, err := e2eutil.Command("nomad", "system", "gc")
	f.Assert().NoError(err)

	// make sure we've waited for all the nodes to come back up
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), len(tc.nodeIDs))
}

// TestDisconnectedClients_AllocReplacement tests that allocations on
// disconnected clients are replaced
func (tc *DisconnectedClientsE2ETest) TestDisconnectedClients_AllocReplacment(f *framework.F) {
	jobID := "test-lost-allocs-" + uuid.Generate()[0:8]

	f.NoError(e2eutil.Register(jobID, "disconnectedclients/input/lost_simple.nomad"))
	tc.jobIDs = append(tc.jobIDs, jobID)
	f.NoError(e2eutil.WaitForAllocStatusExpected(jobID, ns,
		[]string{"running", "running"}), "job should be running")

	// pick a node to make our lost node
	allocs, err := e2eutil.AllocsForJob(jobID, ns)
	f.NoError(err, "could not query allocs for job")
	f.Len(allocs, 2, "could not find 2 allocs for job")

	lostAlloc := allocs[0]
	lostAllocID := lostAlloc["ID"]
	disconnectedNodeID := lostAlloc["Node ID"]
	otherAllocID := allocs[0]["ID"]

	restartJobID, err := e2eutil.AgentRestartAfter(disconnectedNodeID, 30*time.Second)
	f.NoError(err, "expected agent restart job to register")
	tc.jobIDs = append(tc.jobIDs, restartJobID)

	err = e2eutil.WaitForNodeStatus(disconnectedNodeID, "down", nil)
	f.NoError(err, "expected node to go down")

	err = waitForAllocStatusMap(jobID, map[string]string{
		lostAllocID:  "lost",
		otherAllocID: "running",
		"":           "running",
	}, &e2eutil.WaitConfig{Interval: time.Second, Retries: 60})
	f.NoError(err, "expected alloc on disconnected client to be marked lost and replaced")

	allocs, err = e2eutil.AllocsForJob(jobID, ns)
	f.NoError(err, "could not query allocs for job")
	f.Len(allocs, 3, "could not find 3 allocs for job")

	err = e2eutil.WaitForNodeStatus(disconnectedNodeID, "ready", nil)
	f.NoError(err, "expected node to come back up")

	err = waitForAllocStatusMap(jobID, map[string]string{
		lostAllocID:  "dead",
		otherAllocID: "running",
		"":           "running",
	}, &e2eutil.WaitConfig{Interval: time.Second, Retries: 30})
	f.NoError(err, "expected lost alloc on reconnected client to be marked dead and replaced")
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
