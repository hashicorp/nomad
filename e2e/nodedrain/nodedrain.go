package nodedrain

import (
	"fmt"
	"os"
	"strings"
	"time"

	e2e "github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/testutil"
)

const ns = ""

type NodeDrainE2ETest struct {
	framework.TC
	jobIDs  []string
	nodeIDs []string
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "NodeDrain",
		CanRunLocal: true,
		Consul:      true,
		Cases: []framework.TestCase{
			new(NodeDrainE2ETest),
		},
	})

}

func (tc *NodeDrainE2ETest) BeforeAll(f *framework.F) {
	e2e.WaitForLeader(f.T(), tc.Nomad())
	e2e.WaitForNodesReady(f.T(), tc.Nomad(), 2) // needs at least 2 to test migration
}

func (tc *NodeDrainE2ETest) AfterEach(f *framework.F) {
	if os.Getenv("NOMAD_TEST_SKIPCLEANUP") == "1" {
		return
	}

	for _, id := range tc.jobIDs {
		_, err := e2e.Command("nomad", "job", "stop", "-purge", id)
		f.Assert().NoError(err)
	}
	tc.jobIDs = []string{}

	for _, id := range tc.nodeIDs {
		_, err := e2e.Command("nomad", "node", "drain", "-disable", "-yes", id)
		f.Assert().NoError(err)
	}
	tc.nodeIDs = []string{}

	_, err := e2e.Command("nomad", "system", "gc")
	f.Assert().NoError(err)
}

func nodesForJob(jobID string) ([]string, error) {
	allocs, err := e2e.AllocsForJob(jobID, ns)
	if err != nil {
		return nil, err
	}
	if len(allocs) < 1 {
		return nil, fmt.Errorf("no allocs found for job: %v", jobID)
	}
	nodes := []string{}
	for _, alloc := range allocs {
		nodes = append(nodes, alloc["Node ID"])
	}
	return nodes, nil
}

// waitForNodeDrain is a convenience wrapper that polls 'node status'
// until the comparison function over the state of the job's allocs on that
// node returns true
func waitForNodeDrain(nodeID string, comparison func([]map[string]string) bool, wc *e2e.WaitConfig) error {
	var got []map[string]string
	var err error
	interval, retries := wc.OrDefault()
	testutil.WaitForResultRetries(retries, func() (bool, error) {
		time.Sleep(interval)
		got, err = e2e.AllocsForNode(nodeID)
		if err != nil {
			return false, err
		}
		return comparison(got), nil
	}, func(e error) {
		err = fmt.Errorf("node drain status check failed: %v\n%#v", e, got)
	})
	return err
}

// TestNodeDrainEphemeralMigrate tests that ephermeral_disk migrations work as
// expected even during a node drain.
func (tc *NodeDrainE2ETest) TestNodeDrainEphemeralMigrate(f *framework.F) {
	jobID := "test-node-drain-" + uuid.Generate()[0:8]
	f.NoError(e2e.Register(jobID, "nodedrain/input/drain_migrate.nomad"))
	tc.jobIDs = append(tc.jobIDs, jobID)

	expected := []string{"running"}
	f.NoError(e2e.WaitForAllocStatusExpected(jobID, ns, expected), "job should be running")

	allocs, err := e2e.AllocsForJob(jobID, ns)
	f.NoError(err, "could not get allocs for job")
	f.Len(allocs, 1, "could not get allocs for job")
	oldAllocID := allocs[0]["ID"]

	nodes, err := nodesForJob(jobID)
	f.NoError(err, "could not get nodes for job")
	f.Len(nodes, 1, "could not get nodes for job")
	nodeID := nodes[0]

	out, err := e2e.Command("nomad", "node", "drain", "-enable", "-yes", "-detach", nodeID)
	f.NoError(err, fmt.Sprintf("'nomad node drain' failed: %v\n%v", err, out))
	tc.nodeIDs = append(tc.nodeIDs, nodeID)

	f.NoError(waitForNodeDrain(nodeID,
		func(got []map[string]string) bool {
			for _, alloc := range got {
				if alloc["ID"] == oldAllocID && alloc["Status"] == "complete" {
					return true
				}
			}
			return false
		}, &e2e.WaitConfig{Interval: time.Millisecond * 100, Retries: 500},
	), "node did not drain")

	// wait for the allocation to be migrated
	expected = []string{"running", "complete"}
	f.NoError(e2e.WaitForAllocStatusExpected(jobID, ns, expected), "job should be running")

	allocs, err = e2e.AllocsForJob(jobID, ns)
	f.NoError(err, "could not get allocations for job")

	// the task writes its alloc ID to a file if it hasn't been previously
	// written, so find the contents of the migrated file and make sure they
	// match the old allocation, not the running one
	var got string
	var fsErr error
	testutil.WaitForResultRetries(500, func() (bool, error) {
		time.Sleep(time.Millisecond * 100)
		for _, alloc := range allocs {
			if alloc["Status"] == "running" && alloc["Node ID"] != nodeID && alloc["ID"] != oldAllocID {
				got, fsErr = e2e.Command("nomad", "alloc", "fs",
					alloc["ID"], fmt.Sprintf("alloc/data/%s", jobID))
				if err != nil {
					return false, err
				}
				if strings.TrimSpace(got) == oldAllocID {
					return true, nil
				} else {
					return false, fmt.Errorf("expected %q, got %q", oldAllocID, got)
				}
			}
		}
		return false, fmt.Errorf("did not find a migrated alloc")
	}, func(e error) {
		fsErr = e
	})
	f.NoError(fsErr, "node drained but migration failed")
}

// TestNodeDrainIgnoreSystem tests that system jobs are left behind when the
// -ignore-system flag is used.
func (tc *NodeDrainE2ETest) TestNodeDrainIgnoreSystem(f *framework.F) {

	nodes, err := e2e.NodeStatusListFiltered(
		func(section string) bool {
			kernelName, err := e2e.GetField(section, "kernel.name")
			return err == nil && kernelName == "linux"
		})
	f.NoError(err, "could not get node status listing")

	serviceJobID := "test-node-drain-service-" + uuid.Generate()[0:8]
	systemJobID := "test-node-drain-system-" + uuid.Generate()[0:8]

	f.NoError(e2e.Register(serviceJobID, "nodedrain/input/drain_simple.nomad"))
	tc.jobIDs = append(tc.jobIDs, serviceJobID)

	allocs, err := e2e.AllocsForJob(serviceJobID, ns)
	f.NoError(err, "could not get allocs for service job")
	f.Len(allocs, 1, "could not get allocs for service job")
	oldAllocID := allocs[0]["ID"]

	f.NoError(e2e.Register(systemJobID, "nodedrain/input/drain_ignore_system.nomad"))
	tc.jobIDs = append(tc.jobIDs, systemJobID)

	expected := []string{"running"}
	f.NoError(e2e.WaitForAllocStatusExpected(serviceJobID, ns, expected),
		"service job should be running")

	// can't just give it a static list because the number of nodes can vary
	f.NoError(
		e2e.WaitForAllocStatusComparison(
			func() ([]string, error) { return e2e.AllocStatuses(systemJobID, ns) },
			func(got []string) bool {
				if len(got) != len(nodes) {
					return false
				}
				for _, status := range got {
					if status != "running" {
						return false
					}
				}
				return true
			}, nil,
		),
		"system job should be running on every node",
	)

	jobNodes, err := nodesForJob(serviceJobID)
	f.NoError(err, "could not get nodes for job")
	f.Len(jobNodes, 1, "could not get nodes for job")
	nodeID := jobNodes[0]

	out, err := e2e.Command(
		"nomad", "node", "drain",
		"-ignore-system", "-enable", "-yes", "-detach", nodeID)
	f.NoError(err, fmt.Sprintf("'nomad node drain' failed: %v\n%v", err, out))
	tc.nodeIDs = append(tc.nodeIDs, nodeID)

	f.NoError(waitForNodeDrain(nodeID,
		func(got []map[string]string) bool {
			for _, alloc := range got {
				if alloc["ID"] == oldAllocID && alloc["Status"] == "complete" {
					return true
				}
			}
			return false
		}, &e2e.WaitConfig{Interval: time.Millisecond * 100, Retries: 500},
	), "node did not drain")

	allocs, err = e2e.AllocsForJob(systemJobID, ns)
	f.NoError(err, "could not query allocs for system job")
	f.Equal(len(nodes), len(allocs), "system job should still be running on every node")
	for _, alloc := range allocs {
		f.Equal("run", alloc["Desired"], "no system allocs should be draining")
		f.Equal("running", alloc["Status"], "no system allocs should be draining")
	}
}

// TestNodeDrainDeadline tests the enforcement of the node drain deadline so
// that allocations are terminated even if they haven't gracefully exited.
func (tc *NodeDrainE2ETest) TestNodeDrainDeadline(f *framework.F) {

	jobID := "test-node-drain-" + uuid.Generate()[0:8]
	f.NoError(e2e.Register(jobID, "nodedrain/input/drain_deadline.nomad"))
	tc.jobIDs = append(tc.jobIDs, jobID)

	expected := []string{"running"}
	f.NoError(e2e.WaitForAllocStatusExpected(jobID, ns, expected), "job should be running")

	nodes, err := nodesForJob(jobID)
	f.NoError(err, "could not get nodes for job")
	f.Len(nodes, 1, "could not get nodes for job")
	nodeID := nodes[0]

	f.T().Logf("draining node %v", nodeID)
	out, err := e2e.Command(
		"nomad", "node", "drain",
		"-deadline", "5s",
		"-enable", "-yes", "-detach", nodeID)
	f.NoError(err, fmt.Sprintf("'nomad node drain %v' failed: %v\n%v", nodeID, err, out))
	tc.nodeIDs = append(tc.nodeIDs, nodeID)

	// the deadline is 40s but we can't guarantee its instantly terminated at
	// that point, so we give it 30s which is well under the 2m kill_timeout in
	// the job.
	// deadline here needs to account for scheduling and propagation delays.
	f.NoError(waitForNodeDrain(nodeID,
		func(got []map[string]string) bool {
			for _, alloc := range got {
				if alloc["Status"] == "complete" {
					return true
				}
			}
			return false
		}, &e2e.WaitConfig{Interval: time.Second, Retries: 40},
	), "node did not drain immediately following deadline")
}

// TestNodeDrainDeadline tests the enforcement of the node drain -force flag
// so that allocations are terminated immediately.
func (tc *NodeDrainE2ETest) TestNodeDrainForce(f *framework.F) {

	jobID := "test-node-drain-" + uuid.Generate()[0:8]
	f.NoError(e2e.Register(jobID, "nodedrain/input/drain_deadline.nomad"))
	tc.jobIDs = append(tc.jobIDs, jobID)

	expected := []string{"running"}
	f.NoError(e2e.WaitForAllocStatusExpected(jobID, ns, expected), "job should be running")

	nodes, err := nodesForJob(jobID)
	f.NoError(err, "could not get nodes for job")
	f.Len(nodes, 1, "could not get nodes for job")
	nodeID := nodes[0]

	out, err := e2e.Command(
		"nomad", "node", "drain",
		"-force",
		"-enable", "-yes", "-detach", nodeID)
	f.NoError(err, fmt.Sprintf("'nomad node drain' failed: %v\n%v", err, out))
	tc.nodeIDs = append(tc.nodeIDs, nodeID)

	// we've passed -force but we can't guarantee its instantly terminated at
	// that point, so we give it 30s which is under the 2m kill_timeout in
	// the job
	f.NoError(waitForNodeDrain(nodeID,
		func(got []map[string]string) bool {
			for _, alloc := range got {
				if alloc["Status"] == "complete" {
					return true
				}
			}
			return false
		}, &e2e.WaitConfig{Interval: time.Second, Retries: 40},
	), "node did not drain immediately when forced")

}

// TestNodeDrainKeepIneligible tests that nodes can be kept ineligible for
// scheduling after disabling drain.
func (tc *NodeDrainE2ETest) TestNodeDrainKeepIneligible(f *framework.F) {

	nodes, err := e2e.NodeStatusList()
	f.NoError(err, "could not get node status listing")

	nodeID := nodes[0]["ID"]

	out, err := e2e.Command("nomad", "node", "drain", "-enable", "-yes", "-detach", nodeID)
	f.NoError(err, fmt.Sprintf("'nomad node drain' failed: %v\n%v", err, out))
	tc.nodeIDs = append(tc.nodeIDs, nodeID)

	_, err = e2e.Command(
		"nomad", "node", "drain",
		"-disable", "-keep-ineligible", "-yes", nodeID)
	f.NoError(err, fmt.Sprintf("'nomad node drain' failed: %v\n%v", err, out))

	nodes, err = e2e.NodeStatusList()
	f.NoError(err, "could not get updated node status listing")

	f.Equal("ineligible", nodes[0]["Eligibility"])
	f.Equal("false", nodes[0]["Drain"])
}
