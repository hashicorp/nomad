package consul

import (
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
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
		},
	})
}

func (tc *ConsulE2ETest) BeforeAll(f *framework.F) {
	// Ensure cluster has leader before running tests
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
	// Ensure that we have four client nodes in ready state
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 1)
}

type serviceNameTagPair struct {
	serviceName string
	tags        map[string]struct{}
}

// This test runs a job that registers in Consul with specific tags
func (tc *ConsulE2ETest) TestConsulRegistration(f *framework.F) {
	nomadClient := tc.Nomad()
	uuid := uuid.Generate()
	jobId := "consul" + uuid[0:8]
	tc.jobIds = append(tc.jobIds, jobId)

	allocs := e2eutil.RegisterAndWaitForAllocs(f.T(), nomadClient, "consul/input/consul_example.nomad", jobId)
	consulClient := tc.Consul()
	require := require.New(f.T())
	require.Equal(3, len(allocs))

	// Query consul catalog for service
	catalog := consulClient.Catalog()
	g := NewGomegaWithT(f.T())

	expectedTags := map[string]struct{}{}
	expectedTags["global"] = struct{}{}
	expectedTags["cache"] = struct{}{}

	g.Eventually(func() []serviceNameTagPair {
		consulService, _, err := catalog.Service("redis-cache", "", nil)
		require.Nil(err)
		var serviceInfo []serviceNameTagPair
		for _, serviceInstance := range consulService {
			tags := map[string]struct{}{}
			for _, tag := range serviceInstance.ServiceTags {
				tags[tag] = struct{}{}
			}
			serviceInfo = append(serviceInfo, serviceNameTagPair{serviceInstance.ServiceName, tags})
		}
		return serviceInfo
	}, 5*time.Second, time.Second).Should(ConsistOf([]serviceNameTagPair{
		{"redis-cache", expectedTags},
		{"redis-cache", expectedTags},
		{"redis-cache", expectedTags},
	}))

	jobs := nomadClient.Jobs()
	// Stop all jobs in test
	for _, id := range tc.jobIds {
		jobs.Deregister(id, true, nil)
	}
	// Garbage collect
	nomadClient.System().GarbageCollect()

	// Verify that services were deregistered in Consul
	g.Eventually(func() []string {
		consulService, _, err := catalog.Service("redis-cache", "", nil)
		require.Nil(err)
		var serviceIDs []string
		for _, serviceInstance := range consulService {
			serviceIDs = append(serviceIDs, serviceInstance.ServiceID)
		}
		return serviceIDs
	}, 5*time.Second, time.Second).Should(BeEmpty())
}

// This test verifies setting and unsetting canary tags
func (tc *ConsulE2ETest) TestCanaryInplaceUpgrades(f *framework.F) {
	nomadClient := tc.Nomad()
	uuid := uuid.Generate()
	jobId := "consul" + uuid[0:8]
	tc.jobIds = append(tc.jobIds, jobId)

	allocs := e2eutil.RegisterAndWaitForAllocs(f.T(), nomadClient, "consul/input/canary_tags.nomad", jobId)
	consulClient := tc.Consul()
	require := require.New(f.T())
	require.Equal(2, len(allocs))

	jobs := nomadClient.Jobs()
	g := NewGomegaWithT(f.T())

	g.Eventually(func() []string {
		deploys, _, err := jobs.Deployments(jobId, nil)
		require.Nil(err)
		healthyDeploys := make([]string, 0, len(deploys))
		for _, d := range deploys {
			if d.Status == "successful" {
				healthyDeploys = append(healthyDeploys, d.ID)
			}
		}
		return healthyDeploys
	}, 5*time.Second, 20*time.Millisecond).Should(HaveLen(1))

	// Start a deployment
	job, _, err := jobs.Info(jobId, nil)
	require.Nil(err)
	job.Meta = map[string]string{"version": "2"}
	resp, _, err := jobs.Register(job, nil)
	require.Nil(err)
	require.NotEmpty(resp.EvalID)

	// Eventually have a canary
	var deploys []*api.Deployment
	g.Eventually(func() []*api.Deployment {
		deploys, _, err = jobs.Deployments(*job.ID, nil)
		require.Nil(err)
		return deploys
	}, 2*time.Second, 20*time.Millisecond).Should(HaveLen(2))

	deployments := nomadClient.Deployments()
	var deploy *api.Deployment
	g.Eventually(func() []string {
		deploy, _, err = deployments.Info(deploys[0].ID, nil)
		require.Nil(err)
		return deploy.TaskGroups["consul_canary_test"].PlacedCanaries
	}, 2*time.Second, 20*time.Millisecond).Should(HaveLen(1))

	allocations := nomadClient.Allocations()
	g.Eventually(func() bool {
		allocID := deploy.TaskGroups["consul_canary_test"].PlacedCanaries[0]
		alloc, _, err := allocations.Info(allocID, nil)
		require.Nil(err)
		return alloc.DeploymentStatus != nil && alloc.DeploymentStatus.Healthy != nil && *alloc.DeploymentStatus.Healthy
	}, 3*time.Second, 20*time.Millisecond).Should(BeTrue())

	// Query consul catalog for service
	catalog := consulClient.Catalog()
	// Check Consul for canary tags
	g.Eventually(func() []string {
		consulServices, _, err := catalog.Service("canarytest", "", nil)
		require.Nil(err)

		for _, serviceInstance := range consulServices {
			for _, tag := range serviceInstance.ServiceTags {
				if tag == "canary" {
					return serviceInstance.ServiceTags
				}
			}
		}

		return nil
	}, 2*time.Second, 20*time.Millisecond).Should(
		Equal([]string{"foo", "canary"}))

	// Manually promote
	{
		resp, _, err := deployments.PromoteAll(deploys[0].ID, nil)
		require.Nil(err)
		require.NotEmpty(resp.EvalID)
	}

	// Eventually canary is removed
	g.Eventually(func() bool {
		allocID := deploy.TaskGroups["consul_canary_test"].PlacedCanaries[0]
		alloc, _, err := allocations.Info(allocID, nil)
		require.Nil(err)
		return alloc.DeploymentStatus.Canary
	}, 2*time.Second, 20*time.Millisecond).Should(BeFalse())

	// Verify that no instances have canary tags
	expectedTags := map[string]struct{}{}
	expectedTags["foo"] = struct{}{}
	expectedTags["bar"] = struct{}{}

	g.Eventually(func() []serviceNameTagPair {
		consulServices, _, err := catalog.Service("canarytest", "", nil)
		require.Nil(err)
		var serviceInfo []serviceNameTagPair
		for _, serviceInstance := range consulServices {
			tags := map[string]struct{}{}
			for _, tag := range serviceInstance.ServiceTags {
				tags[tag] = struct{}{}
			}
			serviceInfo = append(serviceInfo, serviceNameTagPair{serviceInstance.ServiceName, tags})
		}
		return serviceInfo
	}, 3*time.Second, 20*time.Millisecond).Should(ConsistOf([]serviceNameTagPair{
		{"canarytest", expectedTags},
		{"canarytest", expectedTags},
	}))

}

func (tc *ConsulE2ETest) AfterEach(f *framework.F) {
	nomadClient := tc.Nomad()
	jobs := nomadClient.Jobs()
	// Stop all jobs in test
	for _, id := range tc.jobIds {
		jobs.Deregister(id, true, nil)
	}
	// Garbage collect
	nomadClient.System().GarbageCollect()
}
