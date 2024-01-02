// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package workload_id

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
)

// TestTaskAPI runs subtests exercising the Task API related functionality.
// Bundled with Workload Identity as that's a prereq for the Task API to work.
func TestTaskAPI(t *testing.T) {
	nomad := e2eutil.NomadClient(t)

	e2eutil.WaitForLeader(t, nomad)
	e2eutil.WaitForNodesReady(t, nomad, 1)

	t.Run("testTaskAPI_Auth", testTaskAPIAuth)
	t.Run("testTaskAPI_Windows", testTaskAPIWindows)
}

func testTaskAPIAuth(t *testing.T) {
	nomad := e2eutil.NomadClient(t)
	jobID := "api-auth-" + uuid.Short()
	jobIDs := []string{jobID}
	t.Cleanup(e2eutil.CleanupJobsAndGC(t, &jobIDs))

	// start job
	allocs := e2eutil.RegisterAndWaitForAllocs(t, nomad, "./input/api-auth.nomad.hcl", jobID, "")
	must.Len(t, 1, allocs)
	allocID := allocs[0].ID

	// wait for batch alloc to complete
	alloc := e2eutil.WaitForAllocStopped(t, nomad, allocID)
	must.Eq(t, alloc.ClientStatus, "complete")

	assertions := []struct {
		task   string
		suffix string
	}{
		{
			task:   "none",
			suffix: http.StatusText(http.StatusUnauthorized),
		},
		{
			task:   "bad",
			suffix: http.StatusText(http.StatusForbidden),
		},
		{
			task:   "docker-wid",
			suffix: `"ok":true}}`,
		},
		{
			task:   "exec-wid",
			suffix: `"ok":true}}`,
		},
	}

	// Ensure the assertions and input file match
	must.Len(t, len(assertions), alloc.Job.TaskGroups[0].Tasks,
		must.Sprintf("test and jobspec mismatch"))

	for _, tc := range assertions {
		logFile := fmt.Sprintf("alloc/logs/%s.stdout.0", tc.task)
		fd, err := nomad.AllocFS().Cat(alloc, logFile, nil)
		must.NoError(t, err)
		logBytes, err := io.ReadAll(fd)
		must.NoError(t, err)
		logs := string(logBytes)

		ps := must.Sprintf("Task: %s Logs: <<EOF\n%sEOF", tc.task, logs)

		must.StrHasSuffix(t, tc.suffix, logs, ps)
	}
}

func testTaskAPIWindows(t *testing.T) {
	nomad := e2eutil.NomadClient(t)
	winNodes, err := e2eutil.ListWindowsClientNodes(nomad)
	must.NoError(t, err)
	if len(winNodes) == 0 {
		t.Skip("no Windows clients")
	}

	found := false
	for _, nodeID := range winNodes {
		node, _, err := nomad.Nodes().Info(nodeID, nil)
		must.NoError(t, err)
		if name := node.Attributes["os.name"]; strings.Contains(name, "2016") {
			t.Logf("Node %s is too old to support unix sockets: %s", nodeID, name)
			continue
		}

		found = true
		break
	}
	if !found {
		t.Skip("no Windows clients with unix socket support")
	}

	jobID := "api-win-" + uuid.Short()
	jobIDs := []string{jobID}
	t.Cleanup(e2eutil.CleanupJobsAndGC(t, &jobIDs))

	// start job
	allocs := e2eutil.RegisterAndWaitForAllocs(t, nomad, "./input/api-win.nomad.hcl", jobID, "")
	must.Len(t, 1, allocs)
	allocID := allocs[0].ID

	// wait for batch alloc to complete
	alloc := e2eutil.WaitForAllocStopped(t, nomad, allocID)
	test.Eq(t, alloc.ClientStatus, "complete")

	logFile := "alloc/logs/win.stdout.0"
	fd, err := nomad.AllocFS().Cat(alloc, logFile, nil)
	must.NoError(t, err)
	logBytes, err := io.ReadAll(fd)
	must.NoError(t, err)
	logs := string(logBytes)

	must.StrHasSuffix(t, `"ok":true}}`, logs)
}
