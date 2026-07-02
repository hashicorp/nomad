package allocapi

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	jobFailedLogs = "allocapi/input/failed-logs.nomad"
)

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "AllocAPI",
		CanRunLocal: true,
		Cases: []framework.TestCase{
			new(AllocLogsTest),
		},
	})
}

type AllocLogsTest struct {
	framework.TC
	jobs []string
}

func (tc *AllocLogsTest) BeforeAll(f *framework.F) {
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 1)
}

func (tc *AllocLogsTest) AfterEach(f *framework.F) {
	if os.Getenv("NOMAD_TEST_SKIPCLEANUP") == "1" {
		return
	}

	for _, id := range tc.jobs {
		tc.Nomad().Jobs().Deregister(id, true, nil)
	}
	tc.jobs = []string{}
	tc.Nomad().System().GarbageCollect()
}

// TestFailedLogs asserts that logs can be retrieved from failed allocs.
func (tc *AllocLogsTest) TestFailedLogs(f *framework.F) {
	t := f.T()
	c := tc.Nomad()

	jobID := "failed-logs-" + uuid.Generate()[0:8]
	tc.jobs = append(tc.jobs, jobID)

	// Run job
	allocs := e2eutil.RegisterAndWaitForAllocs(t, c, jobFailedLogs, jobID, "")
	require.Len(t, allocs, 1)
	allocID := allocs[0].ID
	e2eutil.WaitForAllocStopped(t, tc.Nomad(), allocID)

	// Assert that it failed
	alloc, _, err := c.Allocations().Info(allocID, nil)
	require.NoError(t, err)
	require.Equal(t, structs.AllocClientStatusFailed, alloc.ClientStatus)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Assert logs are retrievable
	for taskName := range alloc.TaskStates {
		streamCh, errCh := c.AllocFS().Logs(
			alloc, false, taskName, "stdout", "start", 0, ctx.Done(), nil)
		select {
		case frame := <-streamCh:
			expected := fmt.Sprintf("Hello %s\n", taskName)
			assert.Equal(t, expected, string(frame.Data))
		case err := <-errCh:
			assert.NoError(t, err)
		}
	}
}
