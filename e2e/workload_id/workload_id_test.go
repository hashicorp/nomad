package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/shoenig/test/must"
)

// TestWorkloadIdentity runs subtests exercising workload identity related
// functionality.
func TestWorkloadIdentity(t *testing.T) {
	nomad := e2eutil.NomadClient(t)

	e2eutil.WaitForLeader(t, nomad)
	e2eutil.WaitForNodesReady(t, nomad, 1)

	t.Run("testIdentity", testIdentity)
}

// testIdentity asserts that the various combinations of identity block
// parameteres produce the expected results.
func testIdentity(t *testing.T) {
	nomad := e2eutil.NomadClient(t)

	jobID := "identity-" + uuid.Short()
	jobIDs := []string{jobID}
	t.Cleanup(e2eutil.CleanupJobsAndGC(t, &jobIDs))

	// start job
	allocs := e2eutil.RegisterAndWaitForAllocs(t, nomad, "./input/identity.nomad", jobID, "")
	must.Len(t, 1, allocs)
	allocID := allocs[0].ID

	// wait for batch alloc to complete
	alloc := e2eutil.WaitForAllocStopped(t, nomad, allocID)
	must.Eq(t, alloc.ClientStatus, "complete")

	assertions := []struct {
		task string
		env  bool
		file bool
	}{
		{
			task: "none",
			env:  false,
			file: false,
		},
		{
			task: "empty",
			env:  true,
			file: true,
		},
		{
			task: "env",
			env:  true,
			file: false,
		},
		{
			task: "falsey",
			env:  false,
			file: false,
		},
	}

	for _, tc := range assertions {
		logFile := fmt.Sprintf("alloc/logs/%s.stdout.0", tc.task)
		fd, err := nomad.AllocFS().Cat(alloc, logFile, nil)
		must.NoError(t, err)
		logBytes, err := io.ReadAll(fd)
		must.NoError(t, err)
		logs := string(logBytes)

		ps := must.Sprintf("Task: %s Logs: <<EOF\n%sEOF", tc.task, logs)

		must.StrHasSuffix(t, "done\n", logs, ps)

		lines := strings.Split(logs, "\n")
		switch {
		case tc.env && tc.file:
			must.Len(t, 4, lines, ps)

			// Parse the env first
			token := parseEnv(t, lines[1], ps)

			// Assert the file length matches
			n, err := strconv.Atoi(lines[0])
			must.NoError(t, err, ps)
			must.Eq(t, n, len(token), ps)

		case !tc.env && tc.file:
			must.Len(t, 3, lines, ps)

			// Assert the length is > 10
			n, err := strconv.Atoi(lines[0])
			must.NoError(t, err, ps)
			must.Greater(t, 10, n, ps)

		case tc.env && !tc.file:
			must.Len(t, 3, lines, ps)

			parseEnv(t, lines[0], ps)

		case !tc.env && !tc.file:
			must.Len(t, 2, lines, ps)
		}
	}

}

func parseEnv(t *testing.T, line string, ps must.Setting) string {
	must.StrHasPrefix(t, "NOMAD_TOKEN=", line, ps)
	token := strings.Split(line, "=")[1]
	must.Positive(t, len(token), ps)
	return token
}
