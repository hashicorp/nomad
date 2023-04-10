// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

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
	registerAndWaitForRunning(t, nomadClient, systemJobID, "./input/drain_ignore_system.nomad", count)

	// Also run a service job so we can verify when the drain is done
	serviceJobID := "test-node-drain-service-" + uuid.Short()
	t.Cleanup(cleanupJobState(t, serviceJobID))
	serviceAllocs := registerAndWaitForRunning(t, nomadClient, serviceJobID, "./input/drain_simple.nomad", 1)
	oldAllocID := serviceAllocs[0].ID
	oldNodeID := serviceAllocs[0].NodeID

	// Drain the node with -ignore-system
	out, err := e2eutil.Command(
		"nomad", "node", "drain",
		"-ignore-system", "-enable", "-yes", "-detach", oldNodeID)
	must.NoError(t, err, must.Sprintf("expected no error when marking node for drain: %v", out))

	// The service job should be drained
	newAllocs := waitForAllocDrain(t, nomadClient, serviceJobID, oldAllocID, oldNodeID)
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

	allocs := registerAndWaitForRunning(t, nomadClient, jobID, "./input/drain_migrate.nomad", 1)
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

	newAllocs := waitForAllocDrain(t, nomadClient, jobID, oldAllocID, oldNodeID)
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

// registerAndWaitForRunning registers a job and waits for the expected number
// of allocations to be in a running state. Returns the allocations.
func registerAndWaitForRunning(t *testing.T, nomadClient *api.Client, jobID, jobSpec string, expectedCount int) []*api.AllocationListStub {
	t.Helper()

	var allocs []*api.AllocationListStub
	var err error
	must.NoError(t, e2eutil.Register(jobID, jobSpec))

	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(func() error {
			allocs, _, err = nomadClient.Jobs().Allocations(jobID, false, nil)
			if err != nil {
				return fmt.Errorf("expected no error listing allocs: %v", err)
			}
			if len(allocs) != expectedCount {
				return fmt.Errorf("expected %d allocs but found %d", expectedCount, len(allocs))
			}
			for _, alloc := range allocs {
				if alloc.ClientStatus != structs.AllocClientStatusRunning {
					return fmt.Errorf("alloc %q was %q, not running", alloc.ID, alloc.ClientStatus)
				}
			}
			return nil
		}),
		wait.Timeout(60*time.Second),
		wait.Gap(500*time.Millisecond),
	))
	return allocs
}

// waitForAllocDrain polls the allocation statues for a job until we've finished
// migrating:
// - the old alloc should be stopped
// - the new alloc should be running
func waitForAllocDrain(t *testing.T, nomadClient *api.Client, jobID, oldAllocID, oldNodeID string) []*api.AllocationListStub {

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
		wait.Timeout(120*time.Second),
		wait.Gap(500*time.Millisecond),
	))

	return newAllocs.Slice()
}

func cleanupJobState(t *testing.T, jobID string) func() {
	return func() {
		_, err := e2eutil.Command("nomad", "job", "stop", "-purge", jobID)
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
