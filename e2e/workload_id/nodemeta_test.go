// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package workload_id

import (
	"os"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/jobspec2"
	"github.com/shoenig/test/must"
)

// TestDynamicNodeMetadata runs subtests exercising the Dynamic Node Metadata
// API. Bundled with Workload Identity as it is expected to be most used by
// jobs via Task API + Workload Identity.
func TestDynamicNodeMetadata(t *testing.T) {
	nomad := e2eutil.NomadClient(t)

	e2eutil.WaitForLeader(t, nomad)
	e2eutil.WaitForNodesReady(t, nomad, 1)

	t.Run("testDynamicNodeMetadata", testDynamicNodeMetadata)
}

// testDynamicNodeMetadata dynamically updates metadata on a node, schedules a
// job using that metadata, and has the job update that metadata.
func testDynamicNodeMetadata(t *testing.T) {
	nomad := e2eutil.NomadClient(t)

	nodes, err := e2eutil.ListLinuxClientNodes(nomad)
	must.NoError(t, err)
	if len(nodes) == 0 {
		t.Skip("requires at least 1 linux node")
	}

	node, _, err := nomad.Nodes().Info(nodes[0], nil)
	must.NoError(t, err)

	keyFoo := "foo-" + uuid.Short()
	keyEmpty := "empty-" + uuid.Short()
	keyUnset := "unset-" + uuid.Short()

	// Go ahead and submit job so it is scheduled as soon as the node metadata is
	// applied
	jobID := "node-meta-" + uuid.Short()
	jobIDs := []string{jobID}
	t.Cleanup(e2eutil.CleanupJobsAndGC(t, &jobIDs))

	t.Logf("test config: job=%s node=%s foo=%s empty=%s unset=%s",
		jobID, node.ID, keyFoo, keyEmpty, keyUnset)

	path := "./input/node-meta.nomad.hcl"
	jobBytes, err := os.ReadFile(path)
	must.NoError(t, err)
	job, err := jobspec2.ParseWithConfig(&jobspec2.ParseConfig{
		Path: path,
		Body: jobBytes,
		ArgVars: []string{
			"foo_constraint=${meta." + keyFoo + "}",
			"empty_constraint=${meta." + keyEmpty + "}",
			"unset_constraint=${meta." + keyUnset + "}",
			"foo_key=" + keyFoo,
			"empty_key=" + keyEmpty,
			"unset_key=" + keyUnset,
		},
		Strict: true,
	})
	must.NoError(t, err)
	job.ID = pointer.Of(jobID)

	// Setup ACLs
	for _, task := range job.TaskGroups[0].Tasks {
		p := e2eutil.ApplyJobPolicy(t, nomad, "default",
			jobID, *job.TaskGroups[0].Name, task.Name, `node { policy = "write" }`)

		if p == nil {
			t.Logf("skipping policy for %s as ACLs are disabled", task.Name)
		} else {
			t.Logf("created policy %s for %s", p.Name, task.Name)
		}
	}

	// Register job
	_, _, err = nomad.Jobs().Register(job, nil)
	must.NoError(t, err)

	// Update the node meta to allow the job to be placed
	req := &api.NodeMetaApplyRequest{
		NodeID: node.ID,
		Meta: map[string]*string{
			keyFoo:   pointer.Of("bar"),
			keyEmpty: pointer.Of(""),
			keyUnset: nil,
		},
	}
	resp, err := nomad.Nodes().Meta().Apply(req, nil)
	must.NoError(t, err)
	must.Eq(t, "bar", resp.Meta[keyFoo])
	must.MapContainsKey(t, resp.Meta, keyEmpty)
	must.Eq(t, "", resp.Meta[keyEmpty])
	must.MapNotContainsKey(t, resp.Meta, keyUnset)

	t.Logf("job submitted, node metadata applied, waiting for metadata to be visible...")

	// Wait up to 10 seconds (with 1s buffer) for updates to be visible to the
	// scheduler.
	qo := &api.QueryOptions{
		AllowStale: false,
		WaitIndex:  node.ModifyIndex,
		WaitTime:   2 * time.Second,
	}
	deadline := time.Now().Add(11 * time.Second)
	found := false
	for !found && time.Now().Before(deadline) {
		node, qm, err := nomad.Nodes().Info(node.ID, qo)
		must.NoError(t, err)
		qo.WaitIndex = qm.LastIndex
		t.Logf("checking node at index %d", qm.LastIndex)

		// This check races with the job scheduling, so only check keyFoo as the
		// other 2 keys are manipulated by the job itself!
		if node.Meta[keyFoo] == "bar" {
			found = true
		}
	}
	must.True(t, found, must.Sprintf("node %q did not update by deadline", node.ID))

	// Wait for the job to complete
	t.Logf("waiting for job %s to complete", jobID)
	var alloc *api.AllocationListStub
	deadline = time.Now().Add(1 * time.Minute)
	found = false
	for !found && time.Now().Before(deadline) {
		allocs, _, err := nomad.Jobs().Allocations(jobID, true, nil)
		must.NoError(t, err)
		if len(allocs) > 0 {
			for _, alloc = range allocs {
				if alloc.ClientStatus == "complete" {
					found = true
					break
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	must.True(t, found, must.Sprintf("did not find completed alloc"))

	// Ensure the job's meta updates were applied
	resp, err = nomad.Nodes().Meta().Read(node.ID, nil)
	must.NoError(t, err)
	must.Eq(t, "bar", resp.Meta[keyFoo])
	must.Eq(t, "set", resp.Meta[keyUnset])
	must.MapNotContainsKey(t, resp.Meta, keyEmpty)
	must.MapContainsKey(t, resp.Dynamic, keyEmpty)
	must.Nil(t, resp.Dynamic[keyEmpty])
}
