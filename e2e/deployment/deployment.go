// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package deployment

import (
	"fmt"

	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
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
	run := structs.DeploymentStatusRunning
	uuid := uuid.Generate()
	// unique each run, cluster could have previous jobs
	jobId := "deployment" + uuid[0:8]
	tc.jobIds = append(tc.jobIds, jobId)
	e2eutil.RegisterAndWaitForAllocs(t, nomadClient, "deployment/input/deployment_auto0.nomad", jobId, "")
	ds := e2eutil.DeploymentsForJob(t, nomadClient, jobId)
	require.Equal(t, 1, len(ds))
	deploy := ds[0]

	// Upgrade
	e2eutil.RegisterAllocs(t, nomadClient, "deployment/input/deployment_auto1.nomad", jobId, "")

	// Find the deployment we don't already have
	testutil.WaitForResult(func() (bool, error) {
		ds = e2eutil.DeploymentsForJob(t, nomadClient, jobId)
		for _, d := range ds {
			if d.ID != deploy.ID {
				deploy = d
				return true, nil
			}
		}
		return false, fmt.Errorf("missing update deployment for job %s", jobId)
	}, func(e error) {
		require.NoError(t, e)
	})

	// Deployment is auto pending the upgrade of "two" which has a longer time to health
	e2eutil.WaitForDeployment(t, nomadClient, deploy.ID, run, structs.DeploymentStatusDescriptionRunningAutoPromotion)

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
	tc.jobIds = []string{}
	// Garbage collect
	nomadClient.System().GarbageCollect()
}
