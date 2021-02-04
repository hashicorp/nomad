package consul

import (
	"fmt"
	"time"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
)

type OnUpdateChecksTest struct {
	framework.TC
	jobIDs []string
}

func (tc *OnUpdateChecksTest) BeforeAll(f *framework.F) {
	// Ensure cluster has leader before running tests
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
	// Ensure that we have at least 1 client node in ready state
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 1)
}

func (tc *OnUpdateChecksTest) AfterEach(f *framework.F) {
	nomadClient := tc.Nomad()
	j := nomadClient.Jobs()

	for _, id := range tc.jobIDs {
		j.Deregister(id, true, nil)
	}
	_, err := e2eutil.Command("nomad", "system", "gc")
	f.NoError(err)
}

// TestOnUpdateCheck_IgnoreWarning_IgnoreErrors ensures that deployments
// complete successfully with service checks that warn and error when on_update
// is specified to ignore either.
func (tc *OnUpdateChecksTest) TestOnUpdateCheck_IgnoreWarning_IgnoreErrors(f *framework.F) {
	uuid := uuid.Generate()
	jobID := fmt.Sprintf("on-update-%s", uuid[0:8])
	tc.jobIDs = append(tc.jobIDs, jobID)

	f.NoError(
		e2eutil.Register(jobID, "consul/input/on_update.nomad"),
		"should have registered successfully",
	)

	wc := &e2eutil.WaitConfig{
		Interval: 1 * time.Second,
		Retries:  60,
	}
	f.NoError(
		e2eutil.WaitForLastDeploymentStatus(jobID, "", "successful", wc),
		"deployment should have completed successfully",
	)

	// register update with on_update = ignore
	// this check errors, deployment should still be successful
	f.NoError(
		e2eutil.Register(jobID, "consul/input/on_update_2.nomad"),
		"should have registered successfully",
	)

	f.NoError(
		e2eutil.WaitForLastDeploymentStatus(jobID, "", "successful", wc),
		"deployment should have completed successfully",
	)

}
