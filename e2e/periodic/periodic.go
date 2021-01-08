package periodic

import (
	"fmt"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
)

type PeriodicTest struct {
	framework.TC
	jobIDs []string
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "Periodic",
		CanRunLocal: true,
		Cases: []framework.TestCase{
			new(PeriodicTest),
		},
	})
}

func (tc *PeriodicTest) BeforeAll(f *framework.F) {
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
}

func (tc *PeriodicTest) AfterEach(f *framework.F) {
	nomadClient := tc.Nomad()
	j := nomadClient.Jobs()

	for _, id := range tc.jobIDs {
		j.Deregister(id, true, nil)
	}
	_, err := e2eutil.Command("nomad", "system", "gc")
	f.NoError(err)
}

func (tc *PeriodicTest) TestPeriodicDispatch_Basic(f *framework.F) {
	t := f.T()

	nomadClient := tc.Nomad()

	uuid := uuid.Generate()
	jobID := fmt.Sprintf("deployment-%s", uuid[0:8])
	tc.jobIDs = append(tc.jobIDs, jobID)

	// register job
	e2eutil.RegisterAndWaitForAllocs(t, nomadClient, "periodic/input/simple.nomad", jobID, "")
}
