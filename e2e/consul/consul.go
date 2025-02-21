// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"fmt"
	"os"

	api "github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

const (
	consulJobBasic      = "consul/input/consul_example.nomad"
	consulJobCanaryTags = "consul/input/canary_tags.nomad"

	consulJobRegisterOnUpdatePart1 = "consul/input/services_empty.nomad"
	consulJobRegisterOnUpdatePart2 = "consul/input/services_present.nomad"
)

const (
	// unless otherwise set, tests should just use the default consul namespace
	consulNamespace = "default"
)

type ConsulE2ETest struct {
	framework.TC
	jobIds []string
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "Consul",
		CanRunLocal: true,
		Consul:      true,
		Cases: []framework.TestCase{
			new(ConsulE2ETest),
			new(ScriptChecksE2ETest),
			new(CheckRestartE2ETest),
			new(OnUpdateChecksTest),
		},
	})
}

func (tc *ConsulE2ETest) BeforeAll(f *framework.F) {
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 1)
}

func (tc *ConsulE2ETest) AfterEach(f *framework.F) {
	if os.Getenv("NOMAD_TEST_SKIPCLEANUP") == "1" {
		return
	}

	for _, id := range tc.jobIds {
		_, _, err := tc.Nomad().Jobs().Deregister(id, true, nil)
		require.NoError(f.T(), err)
	}
	tc.jobIds = []string{}
	require.NoError(f.T(), tc.Nomad().System().GarbageCollect())
}

// TestConsulRegistration asserts that a job registers services with tags in Consul.
func (tc *ConsulE2ETest) TestConsulRegistration(f *framework.F) {
	t := f.T()
	r := require.New(t)

	nomadClient := tc.Nomad()
	jobId := "consul" + uuid.Short()
	tc.jobIds = append(tc.jobIds, jobId)

	allocations := e2eutil.RegisterAndWaitForAllocs(f.T(), nomadClient, consulJobBasic, jobId, "")
	require.Equal(t, 3, len(allocations))
	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocations)
	e2eutil.WaitForAllocsRunning(t, tc.Nomad(), allocIDs)

	expectedTags := []string{
		"cache",
		"global",
	}

	// Assert services get registered
	e2eutil.RequireConsulRegistered(r, tc.Consul(), consulNamespace, "consul-example", 3)
	services, _, err := tc.Consul().Catalog().Service("consul-example", "", nil)
	require.NoError(t, err)
	for _, s := range services {
		// If we've made it this far the tags should *always* match
		require.ElementsMatch(t, expectedTags, s.ServiceTags)
	}

	// Stop the job
	e2eutil.WaitForJobStopped(t, nomadClient, jobId)

	// Verify that services were de-registered in Consul
	e2eutil.RequireConsulDeregistered(r, tc.Consul(), consulNamespace, "consul-example")
}

func (tc *ConsulE2ETest) TestConsulRegisterOnUpdate(f *framework.F) {
	t := f.T()
	r := require.New(t)

	nomadClient := tc.Nomad()
	catalog := tc.Consul().Catalog()
	jobID := "consul" + uuid.Short()
	tc.jobIds = append(tc.jobIds, jobID)

	// Initial job has no services for task.
	allocations := e2eutil.RegisterAndWaitForAllocs(t, nomadClient, consulJobRegisterOnUpdatePart1, jobID, "")
	require.Equal(t, 1, len(allocations))
	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocations)
	e2eutil.WaitForAllocsRunning(t, tc.Nomad(), allocIDs)

	// Assert service not yet registered.
	results, _, err := catalog.Service("nc-service", "", nil)
	require.NoError(t, err)
	require.Empty(t, results)

	// On update, add services for task.
	allocations = e2eutil.RegisterAndWaitForAllocs(t, nomadClient, consulJobRegisterOnUpdatePart2, jobID, "")
	require.Equal(t, 1, len(allocations))
	allocIDs = e2eutil.AllocIDsFromAllocationListStubs(allocations)
	e2eutil.WaitForAllocsRunning(t, tc.Nomad(), allocIDs)

	// Assert service is now registered.
	e2eutil.RequireConsulRegistered(r, tc.Consul(), consulNamespace, "nc-service", 1)
}

// TestCanaryInplaceUpgrades verifies setting and unsetting canary tags
func (tc *ConsulE2ETest) TestCanaryInplaceUpgrades(f *framework.F) {
	t := f.T()

	// TODO(shoenig) https://github.com/hashicorp/nomad/issues/9627
	t.Skip("THIS TEST IS BROKEN (#9627)")

	nomadClient := tc.Nomad()
	consulClient := tc.Consul()
	jobId := "consul" + uuid.Generate()[0:8]
	tc.jobIds = append(tc.jobIds, jobId)

	allocs := e2eutil.RegisterAndWaitForAllocs(f.T(), nomadClient, consulJobCanaryTags, jobId, "")
	require.Equal(t, 2, len(allocs))

	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocs)
	e2eutil.WaitForAllocsRunning(t, nomadClient, allocIDs)

	// Start a deployment
	job, _, err := nomadClient.Jobs().Info(jobId, nil)
	require.NoError(t, err)
	job.Meta = map[string]string{"version": "2"}
	resp, _, err := nomadClient.Jobs().Register(job, nil)
	require.NoError(t, err)
	require.NotEmpty(t, resp.EvalID)

	// Eventually have a canary
	var activeDeploy *api.Deployment
	testutil.WaitForResult(func() (bool, error) {
		deploys, _, err := nomadClient.Jobs().Deployments(jobId, false, nil)
		if err != nil {
			return false, err
		}
		if expected := 2; len(deploys) != expected {
			return false, fmt.Errorf("expected 2 deploys but found %v", deploys)
		}

		for _, d := range deploys {
			if d.Status == structs.DeploymentStatusRunning {
				activeDeploy = d
				break
			}
		}
		if activeDeploy == nil {
			return false, fmt.Errorf("no running deployments: %v", deploys)
		}
		if expected := 1; len(activeDeploy.TaskGroups["consul_canary_test"].PlacedCanaries) != expected {
			return false, fmt.Errorf("expected %d placed canaries but found %#v",
				expected, activeDeploy.TaskGroups["consul_canary_test"])
		}

		return true, nil
	}, func(err error) {
		f.NoError(err, "error while waiting for deploys")
	})

	allocID := activeDeploy.TaskGroups["consul_canary_test"].PlacedCanaries[0]
	testutil.WaitForResult(func() (bool, error) {
		alloc, _, err := nomadClient.Allocations().Info(allocID, nil)
		if err != nil {
			return false, err
		}

		if alloc.DeploymentStatus == nil {
			return false, fmt.Errorf("canary alloc %s has no deployment status", allocID)
		}
		if alloc.DeploymentStatus.Healthy == nil {
			return false, fmt.Errorf("canary alloc %s has no deployment health: %#v",
				allocID, alloc.DeploymentStatus)
		}
		return *alloc.DeploymentStatus.Healthy, fmt.Errorf("expected healthy canary but found: %#v",
			alloc.DeploymentStatus)
	}, func(err error) {
		f.NoError(err, "error waiting for canary to be healthy")
	})

	// Check Consul for canary tags
	testutil.WaitForResult(func() (bool, error) {
		consulServices, _, err := consulClient.Catalog().Service("canarytest", "", nil)
		if err != nil {
			return false, err
		}
		for _, s := range consulServices {
			if helper.SliceSetEq([]string{"canary", "foo"}, s.ServiceTags) {
				return true, nil
			}
		}
		return false, fmt.Errorf(`could not find service tags {"canary", "foo"}: %#v`, consulServices)
	}, func(err error) {
		f.NoError(err, "error waiting for canary tags")
	})

	// Promote canary
	{
		resp, _, err := nomadClient.Deployments().PromoteAll(activeDeploy.ID, nil)
		require.NoError(t, err)
		require.NotEmpty(t, resp.EvalID)
	}

	// Eventually canary is promoted
	testutil.WaitForResult(func() (bool, error) {
		alloc, _, err := nomadClient.Allocations().Info(allocID, nil)
		if err != nil {
			return false, err
		}
		return !alloc.DeploymentStatus.Canary, fmt.Errorf("still a canary")
	}, func(err error) {
		require.NoError(t, err, "error waiting for canary to be promoted")
	})

	// Verify that no instances have canary tags
	expected := []string{"foo", "bar"}
	testutil.WaitForResult(func() (bool, error) {
		consulServices, _, err := consulClient.Catalog().Service("canarytest", "", nil)
		if err != nil {
			return false, err
		}
		for _, s := range consulServices {
			if !helper.SliceSetEq(expected, s.ServiceTags) {
				return false, fmt.Errorf("expected %#v Consul tags but found %#v",
					expected, s.ServiceTags)
			}
		}
		return true, nil
	}, func(err error) {
		require.NoError(t, err, "error waiting for non-canary tags")
	})

}
