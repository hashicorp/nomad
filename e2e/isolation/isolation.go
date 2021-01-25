package isolation

import (
	"fmt"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/stretchr/testify/require"
)

type IsolationTest struct {
	framework.TC

	jobIDs []string
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "Isolation",
		CanRunLocal: true,
		Cases: []framework.TestCase{
			new(IsolationTest),
		},
	})
}

func (tc *IsolationTest) BeforeAll(f *framework.F) {
	t := f.T()
	e2eutil.WaitForLeader(t, tc.Nomad())
	e2eutil.WaitForNodesReady(t, tc.Nomad(), 1)
}

func (tc *IsolationTest) AfterEach(f *framework.F) {
	for _, jobID := range tc.jobIDs {
		tc.Nomad().Jobs().Deregister(jobID, true, nil)
	}
	tc.jobIDs = []string{}
	tc.Nomad().System().GarbageCollect()
}

func (tc *IsolationTest) TestIsolation_ExecDriver_PIDNamespacing(f *framework.F) {
	t := f.T()

	clientNodes, err := e2eutil.ListLinuxClientNodes(tc.Nomad())
	require.Nil(t, err)

	if len(clientNodes) == 0 {
		t.Skip("no Linux clients")
	}

	uuid := uuid.Generate()
	jobID := "isolation-pid-namespace-" + uuid[0:8]
	file := "isolation/input/echo_pid.nomad"
	allocs := e2eutil.RegisterAndWaitForAllocs(t, tc.Nomad(), file, jobID, "")
	require.Equal(t, len(allocs), 1, fmt.Sprintf("failed to register %s", jobID))

	tc.jobIDs = append(tc.jobIDs, jobID)

	allocID := allocs[0].ID
	e2eutil.WaitForAllocStopped(t, tc.Nomad(), allocID)

	out, err := e2eutil.AllocLogs(allocID, e2eutil.LogsStdOut)
	require.NoError(t, err, fmt.Sprintf("could not get logs for alloc %s", allocID))

	require.Contains(t, out, "my pid is 1\n")
}
