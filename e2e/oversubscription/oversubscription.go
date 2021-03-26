package oversubscription

import (
	"fmt"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
)

type OversubscriptionTest struct {
	framework.TC
	jobIDs []string
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "oversubscription",
		CanRunLocal: true,
		Cases: []framework.TestCase{
			new(OversubscriptionTest),
		},
	})
}

func (tc *OversubscriptionTest) BeforeAll(f *framework.F) {
	// Ensure cluster has leader before running tests
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
}

func (tc *OversubscriptionTest) AfterEach(f *framework.F) {
	nomadClient := tc.Nomad()
	j := nomadClient.Jobs()

	for _, id := range tc.jobIDs {
		j.Deregister(id, true, nil)
	}
	tc.Nomad().System().GarbageCollect()
}

func (tc *OverSubscriptionTest) TestDocker(f *framework.F) {
	tc.runTest(f, "oversubscription-docker-", "docker.nomad")
}

func (tc *OverSubscriptionTest) TestExec(f *framework.F) {
	tc.runTest(f, "oversubscription-exec-", "exec.nomad")
}

func (tc *OversubscriptionTest) runTest(f *framework.F, jobPrefix, jobfile string) {
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 1)

	// register a job
	jobID := jobPrefix + uuid.Generate()[:8]
	tc.jobIDs = append(tc.jobIDs, jobID)

	allocs := e2eutil.RegisterAndWaitForAllocs(f.T(), tc.Nomad(), "oversubscription/testdata/"+jobfile, jobID, "")
	f.Len(allocs, 1)

	e2eutil.WaitForAllocRunning(f.T(), tc.Nomad(), allocs[0].ID)

	alloc, _, err := tc.Nomad().Allocations().Info(allocs[0].ID, nil)
	f.NoError(err)

	resources := alloc.AllocatedResources.Tasks["task"]
	f.Equal(int64(20), resources.Memory.MemoryMB)
	f.Equal(int64(30), resources.Memory.MemoryMaxMB)

	// check that cgroup reports the memoryMaxMB as the limit
	stdout, err := e2eutil.AllocLogs(alloc.ID, e2eutil.LogsStdOut)
	f.NoError(err)
	f.Equal(fmt.Sprintf("%d\n", 30*1024*1024), stdout)
}
