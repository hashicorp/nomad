package deployment

import (
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/helper/uuid"
)

type DeploymentTest struct {
	framework.TC
	jobIds []string
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "Deployment",
		CanRunLocal: true,
		Cases: []framework.TestCase{
			new(DeploymentTest),
		},
	})
}

func (tc *DeploymentTest) BeforeAll(f *framework.F) {
	// Ensure cluster has leader before running tests
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 4)
}

func (tc *DeploymentTest) TestDeploymentAutoPromote(f *framework.F) {
	t := f.T()
	nomadClient := tc.Nomad()
	uuid := uuid.Generate()
	jobId := "deployment" + uuid[0:8]
	tc.jobIds = append(tc.jobIds, jobId)
	e2eutil.RegisterAndWaitForAllocs(t, nomadClient, "deployment/input/deployment_auto0.nomad", jobId)

	// Upgrade
	e2eutil.RegisterAllocs(t, nomadClient, "deployment/input/deployment_auto1.nomad", jobId)
	var deploy *api.Deployment
	ds, _, err := nomadClient.Deployments().List(nil)
	require.NoError(t, err)

	// Find the deployment
	for _, d := range ds {
		if d.JobID == jobId {
			deploy = d
			break
		}
	}

	// Deployment is auto pending the upgrade of "two" which has a longer time to health
	run := structs.DeploymentStatusRunning
	require.Equal(t, run, deploy.Status)
	require.Equal(t, structs.DeploymentStatusDescriptionRunningAutoPromotion, deploy.StatusDescription)

	// Deployment is eventually running
	e2eutil.WaitForDeployment(t, nomadClient, deploy.ID, run, structs.DeploymentStatusDescriptionRunning)

	deploy, _, _ = nomadClient.Deployments().Info(deploy.ID, nil)
	require.Equal(t, run, deploy.Status)
	require.Equal(t, structs.DeploymentStatusDescriptionRunning, deploy.StatusDescription)
}

func (tc *DeploymentTest) AfterEach(f *framework.F) {
	nomadClient := tc.Nomad()
	jobs := nomadClient.Jobs()
	// Stop all jobs in test
	for _, id := range tc.jobIds {
		jobs.Deregister(id, true, nil)
	}
	// Garbage collect
	nomadClient.System().GarbageCollect()
}
