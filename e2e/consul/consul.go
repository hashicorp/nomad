package consul

import (
	"time"

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

	type serviceNameTagPair struct {
		serviceName string
		tags        map[string]struct{}
	}

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
