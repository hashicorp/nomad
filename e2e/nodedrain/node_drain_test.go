// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nodedrain

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-set"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestNodeDrain(t *testing.T) {

	nomadClient := e2eutil.NomadClient(t)
	e2eutil.WaitForLeader(t, nomadClient)
	e2eutil.WaitForNodesReady(t, nomadClient, 2) // needs at least 2 to test migration

	t.Run("IgnoreSystem", testIgnoreSystem)
	t.Run("EphemeralMigrate", testEphemeralMigrate)
	t.Run("KeepIneligible", testKeepIneligible)
	t.Run("KillTimeout", testKillTimeout)
	t.Run("DeadlineFlag", testDeadlineFlag)
	t.Run("ForceFlag", testForceFlag)
}

// testIgnoreSystem tests that system jobs are left behind when the
// -ignore-system flag is used.
func testIgnoreSystem(t *testing.T) {

	t.Cleanup(cleanupDrainState(t))
	nomadClient := e2eutil.NomadClient(t)

	// Figure out how many system alloc we'll expect to see
	nodes, err := e2eutil.NodeStatusListFiltered(
		func(section string) bool {
			kernelName, err := e2eutil.GetField(section, "kernel.name")
			return err == nil && kernelName == "linux"
		})
	must.NoError(t, err, must.Sprint("could not get node status listing"))
	count := len(nodes)

	// Run a system job, which will not be moved when we drain the node
	systemJobID := "test-node-drain-system-" + uuid.Short()
	t.Cleanup(cleanupJobState(t, systemJobID))

	must.NoError(t, e2eutil.Register(systemJobID, "./input/drain_ignore_system.nomad"))
	waitForRunningAllocs(t, nomadClient, systemJobID, count)

	// Also run a service job so we can verify when the drain is done
	serviceJobID := "test-node-drain-service-" + uuid.Short()
	t.Cleanup(cleanupJobState(t, serviceJobID))
	must.NoError(t, e2eutil.Register(serviceJobID, "./input/drain_simple.nomad"))
	serviceAllocs := waitForRunningAllocs(t, nomadClient, serviceJobID, 1)
	oldAllocID := serviceAllocs[0].ID
	oldNodeID := serviceAllocs[0].NodeID

	// Drain the node with -ignore-system
	out, err := e2eutil.Command(
		"nomad", "node", "drain",
		"-ignore-system", "-enable", "-yes", "-detach", oldNodeID)
	must.NoError(t, err, must.Sprintf("expected no error when marking node for drain: %v", out))

	// The service job should be drained
	newAllocs := waitForAllocDrainComplete(t, nomadClient, serviceJobID,
		oldAllocID, oldNodeID, time.Second*120)
	must.Len(t, 1, newAllocs, must.Sprint("expected 1 new service job alloc"))

	// The system job should not have been drained
	got, err := e2eutil.AllocsForJob(systemJobID, structs.DefaultNamespace)
	must.NoError(t, err, must.Sprintf("could not read allocs for system job: %v", got))
	must.Len(t, count, got, must.Sprintf("expected %d system allocs", count))

	for _, systemAlloc := range got {
		must.Eq(t, "running", systemAlloc["Status"],
			must.Sprint("expected all system allocs to be left client=running"))
		must.Eq(t, "run", systemAlloc["Desired"],
			must.Sprint("expected all system allocs to be left desired=run"))
	}
}

// testEphemeralMigrate tests that ephermeral_disk migrations work as expected
// even during a node drain.
func testEphemeralMigrate(t *testing.T) {

	t.Cleanup(cleanupDrainState(t))

	nomadClient := e2eutil.NomadClient(t)
	jobID := "drain-migrate-" + uuid.Short()

	must.NoError(t, e2eutil.Register(jobID, "./input/drain_migrate.nomad"))
	allocs := waitForRunningAllocs(t, nomadClient, jobID, 1)
	t.Cleanup(cleanupJobState(t, jobID))
	oldAllocID := allocs[0].ID
	oldNodeID := allocs[0].NodeID

	// make sure the allocation has written its ID to disk so we have something to migrate
	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(func() error {
			got, err := e2eutil.Command("nomad", "alloc", "fs", oldAllocID,
				"alloc/data/migrate.txt")
			if err != nil {
				return fmt.Errorf("did not expect error reading alloc fs: %v", err)
			}
			if !strings.Contains(got, oldAllocID) {
				return fmt.Errorf("expected data to be written for alloc %q", oldAllocID)
			}
			return nil
		}),
		wait.Timeout(10*time.Second),
		wait.Gap(500*time.Millisecond),
	))

	out, err := e2eutil.Command("nomad", "node", "drain", "-enable", "-yes", "-detach", oldNodeID)
	must.NoError(t, err, must.Sprintf("expected no error when marking node for drain: %v", out))

	newAllocs := waitForAllocDrainComplete(t, nomadClient, jobID,
		oldAllocID, oldNodeID, time.Second*120)
	must.Len(t, 1, newAllocs, must.Sprint("expected 1 new alloc"))
	newAllocID := newAllocs[0].ID
	newNodeID := newAllocs[0].NodeID

	// although migrate=true implies sticky=true, the drained node is ineligible
	// for scheduling so the alloc should have been migrated
	must.NotEq(t, oldNodeID, newNodeID, must.Sprint("new alloc was placed on draining node"))

	// once the new allocation is running, it should quickly have the right data
	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(func() error {
			got, err := e2eutil.Command("nomad", "alloc", "fs", newAllocID,
				"alloc/data/migrate.txt")
			if err != nil {
				return fmt.Errorf("did not expect error reading alloc fs: %v", err)
			}
			if !strings.Contains(got, oldAllocID) || !strings.Contains(got, newAllocID) {
				return fmt.Errorf(
					"expected data to be migrated from alloc=%s on node=%s to alloc=%s on node=%s but got:\n%q",
					oldAllocID[:8], oldNodeID[:8], newAllocID[:8], newNodeID[:8], got)
			}
			return nil
		}),
		wait.Timeout(10*time.Second),
		wait.Gap(500*time.Millisecond),
	))
}

// testKeepIneligible tests that nodes can be kept ineligible for scheduling after
// disabling drain.
func testKeepIneligible(t *testing.T) {

	nodes, err := e2eutil.NodeStatusList()
	must.NoError(t, err, must.Sprint("expected no error when listing nodes"))

	nodeID := nodes[0]["ID"]

	t.Cleanup(cleanupDrainState(t))

	out, err := e2eutil.Command("nomad", "node", "drain", "-enable", "-yes", "-detach", nodeID)
	must.NoError(t, err, must.Sprintf("expected no error when marking node for drain: %v", out))

	out, err = e2eutil.Command(
		"nomad", "node", "drain",
		"-disable", "-keep-ineligible", "-yes", nodeID)
	must.NoError(t, err, must.Sprintf("expected no error when disabling drain for node: %v", out))

	nodes, err = e2eutil.NodeStatusList()
	must.NoError(t, err, must.Sprint("expected no error when listing nodes"))

	for _, node := range nodes {
		if node["ID"] == nodeID {
			must.Eq(t, "ineligible", nodes[0]["Eligibility"])
			must.Eq(t, "false", nodes[0]["Drain"])
		}
	}
}

// testKillTimeout tests that we block drains until the client status has been
// updated, not the server status.
func testKillTimeout(t *testing.T) {

	nomadClient := e2eutil.NomadClient(t)
	t.Cleanup(cleanupDrainState(t))

	jobID := "test-node-drain-" + uuid.Short()

	must.NoError(t, e2eutil.Register(jobID, "./input/drain_killtimeout.nomad"))
	allocs := waitForRunningAllocs(t, nomadClient, jobID, 1)

	t.Cleanup(cleanupJobState(t, jobID))
	oldAllocID := allocs[0].ID
	oldNodeID := allocs[0].NodeID

	t.Logf("draining node %v", oldNodeID)
	out, err := e2eutil.Command(
		"nomad", "node", "drain",
		"-enable", "-yes", "-detach", oldNodeID)
	must.NoError(t, err, must.Sprintf("'nomad node drain %v' failed: %v\n%v", oldNodeID, err, out))

	// the job will hang with kill_timeout for up to 30s, so we want to assert
	// that we don't complete draining before that window expires. But we also
	// can't guarantee we've started this assertion with exactly 30s left on the
	// clock, so cut the deadline close without going over to avoid test
	// flakiness
	t.Log("waiting for kill_timeout to expire")
	must.Wait(t, wait.ContinualSuccess(
		wait.BoolFunc(func() bool {
			node, _, err := nomadClient.Nodes().Info(oldNodeID, nil)
			must.NoError(t, err)
			return node.DrainStrategy != nil
		}),
		wait.Timeout(time.Second*25),
		wait.Gap(500*time.Millisecond),
	))

	// the allocation will then get force-killed, so wait for the alloc
	// eventually be migrated and for the node's drain to be complete
	t.Log("waiting for migration to complete")
	newAllocs := waitForAllocDrainComplete(t, nomadClient, jobID,
		oldAllocID, oldNodeID, time.Second*60)
	must.Len(t, 1, newAllocs, must.Sprint("expected 1 new alloc"))

	must.Wait(t, wait.InitialSuccess(
		wait.BoolFunc(func() bool {
			node, _, err := nomadClient.Nodes().Info(oldNodeID, nil)
			must.NoError(t, err)
			return node.DrainStrategy == nil
		}),
		wait.Timeout(time.Second*5),
		wait.Gap(500*time.Millisecond),
	))
}

// testDeadlineFlag tests the enforcement of the node drain deadline so that
// allocations are moved even if max_parallel says we should be waiting
func testDeadlineFlag(t *testing.T) {

	nomadClient := e2eutil.NomadClient(t)
	t.Cleanup(cleanupDrainState(t))

	jobID := "test-node-drain-" + uuid.Short()
	must.NoError(t, e2eutil.Register(jobID, "./input/drain_deadline.nomad"))
	allocs := waitForRunningAllocs(t, nomadClient, jobID, 2)

	t.Cleanup(cleanupJobState(t, jobID))
	oldAllocID1 := allocs[0].ID
	oldNodeID1 := allocs[0].NodeID
	oldAllocID2 := allocs[1].ID
	oldNodeID2 := allocs[1].NodeID

	t.Logf("draining nodes %s, %s", oldNodeID1, oldNodeID2)
	out, err := e2eutil.Command("nomad", "node", "eligibility", "-disable", oldNodeID1)
	must.NoError(t, err, must.Sprintf("nomad node eligibility -disable failed: %v\n%v", err, out))
	out, err = e2eutil.Command("nomad", "node", "eligibility", "-disable", oldNodeID2)
	must.NoError(t, err, must.Sprintf("nomad node eligibility -disable failed: %v\n%v", err, out))

	out, err = e2eutil.Command(
		"nomad", "node", "drain",
		"-deadline", "1s",
		"-enable", "-yes", "-detach", oldNodeID1)
	must.NoError(t, err, must.Sprintf("'nomad node drain %v' failed: %v\n%v", oldNodeID1, err, out))

	out, err = e2eutil.Command(
		"nomad", "node", "drain",
		"-deadline", "1s",
		"-enable", "-yes", "-detach", oldNodeID2)
	must.NoError(t, err, must.Sprintf("'nomad node drain %v' failed: %v\n%v", oldNodeID2, err, out))

	// with max_parallel=1 and min_healthy_time=30s we'd expect it to take ~60
	// for both to be marked complete. Instead, because of the -deadline flag
	// we'll expect the allocs to be stoppped almost immediately (give it 10s to
	// avoid flakiness), and then the new allocs should come up and get marked
	// healthy after ~30s
	t.Log("waiting for old allocs to stop")
	waitForAllocsStop(t, nomadClient, time.Second*10, oldAllocID1, oldAllocID2)

	t.Log("waiting for running allocs")
	waitForRunningAllocs(t, nomadClient, jobID, 2)
}

// testForceFlag tests the enforcement of the node drain -force flag so that
// allocations are terminated immediately.
func testForceFlag(t *testing.T) {

	nomadClient := e2eutil.NomadClient(t)
	t.Cleanup(cleanupDrainState(t))

	jobID := "test-node-drain-" + uuid.Short()
	must.NoError(t, e2eutil.Register(jobID, "./input/drain_deadline.nomad"))
	allocs := waitForRunningAllocs(t, nomadClient, jobID, 2)

	t.Cleanup(cleanupJobState(t, jobID))
	oldAllocID1 := allocs[0].ID
	oldNodeID1 := allocs[0].NodeID
	oldAllocID2 := allocs[1].ID
	oldNodeID2 := allocs[1].NodeID

	t.Logf("draining nodes %s, %s", oldNodeID1, oldNodeID2)
	out, err := e2eutil.Command("nomad", "node", "eligibility", "-disable", oldNodeID1)
	must.NoError(t, err, must.Sprintf("nomad node eligibility -disable failed: %v\n%v", err, out))
	out, err = e2eutil.Command("nomad", "node", "eligibility", "-disable", oldNodeID2)
	must.NoError(t, err, must.Sprintf("nomad node eligibility -disable failed: %v\n%v", err, out))

	out, err = e2eutil.Command(
		"nomad", "node", "drain", "-force",
		"-enable", "-yes", "-detach", oldNodeID1)
	must.NoError(t, err, must.Sprintf("'nomad node drain %v' failed: %v\n%v", oldNodeID1, err, out))

	out, err = e2eutil.Command(
		"nomad", "node", "drain", "-force",
		"-enable", "-yes", "-detach", oldNodeID2)
	must.NoError(t, err, must.Sprintf("'nomad node drain %v' failed: %v\n%v", oldNodeID2, err, out))

	// with max_parallel=1 and min_healthy_time=30s we'd expect it to take ~60
	// for both to be marked complete. Instead, because of the -force flag
	// we'll expect the allocs to be stoppped almost immediately (give it 10s to
	// avoid flakiness), and then the new allocs should come up and get marked
	// healthy after ~30s
	t.Log("waiting for old allocs to stop")
	waitForAllocsStop(t, nomadClient, time.Second*10, oldAllocID1, oldAllocID2)

	t.Log("waiting for running allocs")
	waitForRunningAllocs(t, nomadClient, jobID, 2)
}

func waitForRunningAllocs(t *testing.T, nomadClient *api.Client, jobID string, expectedRunningCount int) []*api.AllocationListStub {
	t.Helper()

	runningAllocs := set.From([]*api.AllocationListStub{})

	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(func() error {
			allocs, _, err := nomadClient.Jobs().Allocations(jobID, false, nil)
			must.NoError(t, err)
			count := 0
			for _, alloc := range allocs {
				if alloc.ClientStatus == structs.AllocClientStatusRunning {
					runningAllocs.Insert(alloc)
				}
			}
			if runningAllocs.Size() < expectedRunningCount {
				return fmt.Errorf("expected %d running allocs, got %d", expectedRunningCount, count)
			}
			return nil
		}),
		wait.Timeout(60*time.Second),
		wait.Gap(500*time.Millisecond),
	))
	return runningAllocs.Slice()
}

// waitForAllocDrainComplete polls the allocation statues for a job until we've finished
// migrating:
// - the old alloc should be stopped
// - the new alloc should be running
func waitForAllocDrainComplete(t *testing.T, nomadClient *api.Client, jobID, oldAllocID, oldNodeID string, deadline time.Duration) []*api.AllocationListStub {

	t.Helper()
	newAllocs := set.From([]*api.AllocationListStub{})
	start := time.Now()

	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(func() error {
			allocs, _, err := nomadClient.Jobs().Allocations(jobID, false, nil)
			if err != nil {
				return fmt.Errorf("could not read allocations for node: %w", err)
			}
			if len(allocs) == 1 {
				return fmt.Errorf("no new alloc started")
			}

			for _, alloc := range allocs {
				if alloc.ID == oldAllocID {
					if alloc.ClientStatus != structs.AllocClientStatusComplete {
						return fmt.Errorf("old alloc was not marked complete")
					}
				} else {
					if alloc.ClientStatus != structs.AllocClientStatusRunning {
						return fmt.Errorf("new alloc was not marked running")
					}
					newAllocs.Insert(alloc)
				}
			}
			t.Logf("alloc has drained from node=%s after %v",
				oldNodeID[:8], time.Now().Sub(start))
			return nil
		}),
		wait.Timeout(deadline),
		wait.Gap(500*time.Millisecond),
	))

	return newAllocs.Slice()
}

// waitForAllocsStop polls the allocation statues for specific allocations until
// they've stopped
func waitForAllocsStop(t *testing.T, nomadClient *api.Client, deadline time.Duration, oldAllocIDs ...string) {
	t.Helper()

	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(func() error {
			for _, allocID := range oldAllocIDs {
				alloc, _, err := nomadClient.Allocations().Info(allocID, nil)
				must.NoError(t, err)
				if alloc.ClientStatus != structs.AllocClientStatusComplete {
					return fmt.Errorf("expected alloc %s to be complete, got %q",
						allocID[:8], alloc.ClientStatus)
				}
			}
			return nil
		}),
		wait.Timeout(deadline),
		wait.Gap(500*time.Millisecond),
	))
}

func cleanupJobState(t *testing.T, jobID string) func() {
	return func() {
		if os.Getenv("NOMAD_TEST_SKIPCLEANUP") == "1" {
			return
		}

		// we can't use the CLI here because some tests will stop the job during
		// a running deployment, which returns a non-zero exit code
		nomadClient := e2eutil.NomadClient(t)
		_, _, err := nomadClient.Jobs().Deregister(jobID, true, nil)
		test.NoError(t, err)
	}
}

func cleanupDrainState(t *testing.T) func() {
	return func() {
		if os.Getenv("NOMAD_TEST_SKIPCLEANUP") == "1" {
			return
		}

		nomadClient := e2eutil.NomadClient(t)
		nodes, _, err := nomadClient.Nodes().List(nil)
		must.NoError(t, err, must.Sprint("expected no error when listing nodes"))
		for _, node := range nodes {
			_, err := e2eutil.Command("nomad", "node", "drain", "-disable", "-yes", node.ID)
			test.NoError(t, err)
			_, err = e2eutil.Command("nomad", "node", "eligibility", "-enable", node.ID)
			test.NoError(t, err)
		}
	}
}
