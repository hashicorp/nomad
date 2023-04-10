// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nodedrain

import (
	"fmt"
	"os"
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
		_, err = e2e.Command("nomad", "node", "eligibility", "-enable", id)
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

// TestNodeDrainDeadline tests the enforcement of the node drain deadline so
// that allocations are terminated even if they haven't gracefully exited.
func (tc *NodeDrainE2ETest) TestNodeDrainDeadline(f *framework.F) {
	f.T().Skip("The behavior is unclear and test assertions don't capture intent.  Issue 9902")

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
			// FIXME: check the drain job alloc specifically. test
			// may pass if client had another completed alloc
			for _, alloc := range got {
				if alloc["Status"] == "complete" {
					return true
				}
			}
			return false
		}, &e2e.WaitConfig{Interval: time.Second, Retries: 40},
	), "node did not drain immediately following deadline")
}

// TestNodeDrainForce tests the enforcement of the node drain -force flag so
// that allocations are terminated immediately.
func (tc *NodeDrainE2ETest) TestNodeDrainForce(f *framework.F) {
	f.T().Skip("The behavior is unclear and test assertions don't capture intent.  Issue 9902")

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
			// FIXME: check the drain job alloc specifically. test
			// may pass if client had another completed alloc
			for _, alloc := range got {
				if alloc["Status"] == "complete" {
					return true
				}
			}
			return false
		}, &e2e.WaitConfig{Interval: time.Second, Retries: 40},
	), "node did not drain immediately when forced")

}
