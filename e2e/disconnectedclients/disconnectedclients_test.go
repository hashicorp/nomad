package disconnectedclients

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

const ns = ""

// typical wait times for this test package
var wait30s = &e2eutil.WaitConfig{Interval: time.Second, Retries: 30}
var wait60s = &e2eutil.WaitConfig{Interval: time.Second, Retries: 60}

type expectedAllocStatus struct {
	disconnected string
	unchanged    string
	replacement  string
}

func TestDisconnectedClients(t *testing.T) {

	nomad := e2eutil.NomadClient(t)
	e2eutil.WaitForLeader(t, nomad)
	e2eutil.WaitForNodesReady(t, nomad, 2) // needs at least 2 to test replacement

	testCases := []struct {
		name                    string
		jobFile                 string
		disconnectFn            func(string, time.Duration) (string, error)
		expectedAfterDisconnect expectedAllocStatus
		expectedAfterReconnect  expectedAllocStatus
	}{
		{
			// test that allocations on clients that are netsplit and
			// marked disconnected are replaced
			name:         "netsplit client no max disconnect",
			jobFile:      "./input/lost_simple.nomad",
			disconnectFn: e2eutil.AgentDisconnect,
			expectedAfterDisconnect: expectedAllocStatus{
				disconnected: "lost",
				unchanged:    "running",
				replacement:  "running",
			},
			expectedAfterReconnect: expectedAllocStatus{
				disconnected: "complete",
				unchanged:    "running",
				replacement:  "running",
			},
		},

		{
			// test that allocations on clients that are netsplit and
			// marked disconnected are replaced but that the
			// replacements are rolled back after reconnection
			name:         "netsplit client with max disconnect",
			jobFile:      "./input/lost_max_disconnect.nomad",
			disconnectFn: e2eutil.AgentDisconnect,
			expectedAfterDisconnect: expectedAllocStatus{
				disconnected: "unknown",
				unchanged:    "running",
				replacement:  "running",
			},
			expectedAfterReconnect: expectedAllocStatus{
				disconnected: "running",
				unchanged:    "running",
				replacement:  "complete",
			},
		},

		{
			// test that allocations on clients that are shutdown and
			// marked disconnected are replaced
			name:         "shutdown client no max disconnect",
			jobFile:      "./input/lost_simple.nomad",
			disconnectFn: e2eutil.AgentDisconnect,
			expectedAfterDisconnect: expectedAllocStatus{
				disconnected: "lost",
				unchanged:    "running",
				replacement:  "running",
			},
			expectedAfterReconnect: expectedAllocStatus{
				disconnected: "complete",
				unchanged:    "running",
				replacement:  "running",
			},
		},

		{
			// test that allocations on clients that are shutdown and
			// marked disconnected are replaced
			name:         "shutdown client with max disconnect",
			jobFile:      "./input/lost_max_disconnect.nomad",
			disconnectFn: e2eutil.AgentDisconnect,
			expectedAfterDisconnect: expectedAllocStatus{
				disconnected: "unknown",
				unchanged:    "running",
				replacement:  "running",
			},
			expectedAfterReconnect: expectedAllocStatus{
				disconnected: "running",
				unchanged:    "running",
				replacement:  "complete",
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {

			jobIDs := []string{}
			t.Cleanup(disconnectedClientsCleanup(t))
			t.Cleanup(e2eutil.CleanupJobsAndGC(t, &jobIDs))

			jobID := "test-disconnected-clients-" + uuid.Short()

			err := e2eutil.Register(jobID, tc.jobFile)
			require.NoError(t, err)
			jobIDs = append(jobIDs, jobID)

			err = e2eutil.WaitForAllocStatusExpected(jobID, ns,
				[]string{"running", "running"})
			require.NoError(t, err, "job should be running")

			err = e2eutil.WaitForLastDeploymentStatus(jobID, ns, "successful", nil)
			require.NoError(t, err, "success", "deployment did not complete")

			// pick one alloc to make our disconnected alloc (and its node)
			allocs, err := e2eutil.AllocsForJob(jobID, ns)
			require.NoError(t, err, "could not query allocs for job")
			require.Len(t, allocs, 2, "could not find 2 allocs for job")

			disconnectedAllocID := allocs[0]["ID"]
			disconnectedNodeID := allocs[0]["Node ID"]
			unchangedAllocID := allocs[1]["ID"]

			// disconnect the node and wait for the results

			restartJobID, err := tc.disconnectFn(disconnectedNodeID, 30*time.Second)
			require.NoError(t, err, "expected agent disconnect job to register")
			jobIDs = append(jobIDs, restartJobID)

			err = e2eutil.WaitForNodeStatus(disconnectedNodeID, "disconnected", wait60s)
			require.NoError(t, err, "expected node to go down")

			require.NoError(t, waitForAllocStatusMap(
				jobID, disconnectedAllocID, unchangedAllocID, tc.expectedAfterDisconnect, wait60s))

			allocs, err = e2eutil.AllocsForJob(jobID, ns)
			require.NoError(t, err, "could not query allocs for job")
			require.Len(t, allocs, 3, "could not find 3 allocs for job")

			// wait for the reconnect and wait for the results

			err = e2eutil.WaitForNodeStatus(disconnectedNodeID, "ready", wait30s)
			require.NoError(t, err, "expected node to come back up")
			require.NoError(t, waitForAllocStatusMap(
				jobID, disconnectedAllocID, unchangedAllocID, tc.expectedAfterReconnect, wait60s))
		})
	}

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
		t.Logf("waiting for %d nodes to become ready again", len(nodeIDs))
		e2eutil.WaitForNodesReady(t, nomad, len(nodeIDs))
	}
}

func waitForAllocStatusMap(jobID, disconnectedAllocID, unchangedAllocID string, expected expectedAllocStatus, wc *e2eutil.WaitConfig) error {
	var err error
	interval, retries := wc.OrDefault()
	testutil.WaitForResultRetries(retries, func() (bool, error) {
		time.Sleep(interval)
		allocs, err := e2eutil.AllocsForJob(jobID, ns)
		if err != nil {
			return false, err
		}

		var merr *multierror.Error

		for _, alloc := range allocs {
			switch allocID, allocStatus := alloc["ID"], alloc["Status"]; allocID {
			case disconnectedAllocID:
				if allocStatus != expected.disconnected {
					merr = multierror.Append(merr, fmt.Errorf(
						"disconnected alloc %q on node %q should be %q, got %q",
						allocID, alloc["Node ID"], expected.disconnected, allocStatus))
				}
			case unchangedAllocID:
				if allocStatus != expected.unchanged {
					merr = multierror.Append(merr, fmt.Errorf(
						"unchanged alloc %q on node %q should be %q, got %q",
						allocID, alloc["Node ID"], expected.unchanged, allocStatus))
				}
			default:
				if allocStatus != expected.replacement {
					merr = multierror.Append(merr, fmt.Errorf(
						"replacement alloc %q on node %q should be %q, got %q",
						allocID, alloc["Node ID"], expected.replacement, allocStatus))
				}
			}
		}
		if merr != nil {
			return false, merr.ErrorOrNil()
		}
		return true, nil
	}, func(e error) {
		err = e
	})

	// TODO(tgross): remove this block once this test has stabilized
	if err != nil {
		fmt.Printf("test failed, printing allocation status of all %q allocs for analysis\n", jobID)
		fmt.Println("----------------")
		allocs, _ := e2eutil.AllocsForJob(jobID, ns)
		for _, alloc := range allocs {
			out, _ := e2eutil.Command("nomad", "alloc", "status", alloc["ID"])
			fmt.Println(out)
			fmt.Println("----------------")
		}
	}

	return err
}
