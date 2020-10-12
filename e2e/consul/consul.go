package consul

import (
	"fmt"
	"os"

	"github.com/hashicorp/nomad/api"
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
		tc.Nomad().Jobs().Deregister(id, true, nil)
	}
	tc.jobIds = []string{}
	tc.Nomad().System().GarbageCollect()
}

// TestConsulRegistration asserts that a job registers services with tags in Consul.
func (tc *ConsulE2ETest) TestConsulRegistration(f *framework.F) {
	t := f.T()

	nomadClient := tc.Nomad()
	catalog := tc.Consul().Catalog()
	jobId := "consul" + uuid.Generate()[0:8]
	tc.jobIds = append(tc.jobIds, jobId)

	allocs := e2eutil.RegisterAndWaitForAllocs(f.T(), nomadClient, consulJobBasic, jobId, "")
	require.Equal(t, 3, len(allocs))
	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocs)
	e2eutil.WaitForAllocsRunning(t, tc.Nomad(), allocIDs)

	expectedTags := []string{
		"cache",
		"global",
	}

	// Assert services get registered
	testutil.WaitForResult(func() (bool, error) {
		services, _, err := catalog.Service("consul-example", "", nil)
		if err != nil {
			return false, fmt.Errorf("error contacting Consul: %v", err)
		}
		if expected := 3; len(services) != expected {
			return false, fmt.Errorf("expected %d services but found %d", expected, len(services))
		}
		for _, s := range services {
			// If we've made it this far the tags should *always* match
			require.True(t, helper.CompareSliceSetString(expectedTags, s.ServiceTags))
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("error waiting for services to be registered: %v", err)
	})

	// Stop the job
	e2eutil.WaitForJobStopped(t, nomadClient, jobId)

	// Verify that services were deregistered in Consul
	testutil.WaitForResult(func() (bool, error) {
		s, _, err := catalog.Service("consul-example", "", nil)
		if err != nil {
			return false, err
		}

		return len(s) == 0, fmt.Errorf("expected 0 services but found: %v", s)
	}, func(err error) {
		t.Fatalf("error waiting for services to be deregistered: %v", err)
	})
}

// TestCanaryInplaceUpgrades verifies setting and unsetting canary tags
func (tc *ConsulE2ETest) TestCanaryInplaceUpgrades(f *framework.F) {
	t := f.T()
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
		t.Fatalf("error while waiting for deploys: %v", err)
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
		t.Fatalf("error waiting for canary to be healthy: %v", err)
	})

	// Check Consul for canary tags
	testutil.WaitForResult(func() (bool, error) {
		consulServices, _, err := consulClient.Catalog().Service("canarytest", "", nil)
		if err != nil {
			return false, err
		}
		for _, s := range consulServices {
			if helper.CompareSliceSetString([]string{"canary", "foo"}, s.ServiceTags) {
				return true, nil
			}
		}
		return false, fmt.Errorf(`could not find service tags {"canary", "foo"}: %#v`, consulServices)
	}, func(err error) {
		t.Fatalf("error waiting for canary tags: %v", err)
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
		t.Fatalf("error waiting for canary to be promoted: %v", err)
	})

	// Verify that no instances have canary tags
	expected := []string{"foo", "bar"}
	testutil.WaitForResult(func() (bool, error) {
		consulServices, _, err := consulClient.Catalog().Service("canarytest", "", nil)
		if err != nil {
			return false, err
		}
		for _, s := range consulServices {
			if !helper.CompareSliceSetString(expected, s.ServiceTags) {
				return false, fmt.Errorf("expected %#v Consul tags but found %#v",
					expected, s.ServiceTags)
			}
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("error waiting for non-canary tags: %v", err)
	})

}
