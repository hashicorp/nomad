package example

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/helper/uuid"
)

func TestExample(t *testing.T) {
	nomad := e2eutil.NomadClient(t)

	e2eutil.WaitForLeader(t, nomad)
	e2eutil.WaitForNodesReady(t, nomad, 2)

	t.Run("TestExample_Simple", testExample_Simple)
	t.Run("TestExample_WithCleanup", testExample_WithCleanup)
}

func testExample_Simple(t *testing.T) {
	t.Logf("Logging %s", t.Name())
	out, err := e2eutil.Command("nomad", "node", "status")
	require.NoError(t, err, "failed to run `nomad node status`")

	rows, err := e2eutil.ParseColumns(out)
	require.NoError(t, err, "failed to parse `nomad node status`")
	for _, row := range rows {
		require.Equal(t, "ready", row["Status"])
	}
}

func testExample_WithCleanup(t *testing.T) {

	t.Logf("Logging %s", t.Name())
	nomad := e2eutil.NomadClient(t)

	_, err := e2eutil.Command("nomad", "job", "init", "-short", "./input/example.nomad")
	require.NoError(t, err, "failed to run `nomad job init -short`")
	t.Cleanup(func() { os.Remove("input/example.nomad") })

	jobIDs := []string{}
	t.Cleanup(e2eutil.CleanupJobsAndGC(t, &jobIDs))

	jobID := "example-" + uuid.Short()
	jobIDs = append(jobIDs, jobID)
	e2eutil.RegisterAndWaitForAllocs(t, nomad, "./input/example.nomad", jobID, "")

	jobs, _, err := nomad.Jobs().List(nil)
	require.NoError(t, err)
	require.NotEmpty(t, jobs)
}
