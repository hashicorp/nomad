// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package servicediscovery

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/stretchr/testify/require"
)

const (
	jobNomadProvider    = "./input/nomad_provider.nomad"
	jobConsulProvider   = "./input/consul_provider.nomad"
	jobMultiProvider    = "./input/multi_provider.nomad"
	jobSimpleLBReplicas = "./input/simple_lb_replicas.nomad"
	jobSimpleLBClients  = "./input/simple_lb_clients.nomad"
	jobChecksHappy      = "./input/checks_happy.nomad"
	jobChecksSad        = "./input/checks_sad.nomad"
)

const (
	defaultWaitForTime = 5 * time.Second
	defaultTickTime    = 200 * time.Millisecond
)

// TestServiceDiscovery runs a number of tests which exercise Nomads service
// discovery functionality. It does not test subsystems of service discovery
// such as Consul Connect, which have their own test suite.
func TestServiceDiscovery(t *testing.T) {

	// Wait until we have a usable cluster before running the tests.
	nomadClient := e2eutil.NomadClient(t)
	e2eutil.WaitForLeader(t, nomadClient)
	e2eutil.WaitForNodesReady(t, nomadClient, 1)

	// Run our test cases.
	t.Run("TestServiceDiscovery_MultiProvider", testMultiProvider)
	t.Run("TestServiceDiscovery_UpdateProvider", testUpdateProvider)
	t.Run("TestServiceDiscovery_SimpleLoadBalancing", testSimpleLoadBalancing)
	t.Run("TestServiceDiscovery_ChecksHappy", testChecksHappy)
	t.Run("TestServiceDiscovery_ChecksSad", testChecksSad)
	t.Run("TestServiceDiscovery_ServiceRegisterAfterCheckRestart", testChecksServiceReRegisterAfterCheckRestart)
}

// testMultiProvider tests service discovery where multi providers are used
// within a single job.
func testMultiProvider(t *testing.T) {

	nomadClient := e2eutil.NomadClient(t)
	consulClient := e2eutil.ConsulClient(t)

	// Generate our job ID which will be used for the entire test.
	jobID := "service-discovery-multi-provider-" + uuid.Short()
	jobIDs := []string{jobID}

	// Defer a cleanup function to remove the job. This will trigger if the
	// test fails, unless the cancel function is called.
	ctx, cancel := context.WithCancel(context.Background())
	defer e2eutil.CleanupJobsAndGCWithContext(t, ctx, &jobIDs)

	// Register the job which contains two groups, each with a single service
	// that use different providers.
	allocStubs := e2eutil.RegisterAndWaitForAllocs(t, nomadClient, jobMultiProvider, jobID, "")
	require.Len(t, allocStubs, 2)

	// We need to understand which allocation belongs to which group so we can
	// test the service registrations properly.
	var nomadProviderAllocID, consulProviderAllocID string

	for _, allocStub := range allocStubs {
		switch allocStub.TaskGroup {
		case "service_discovery":
			consulProviderAllocID = allocStub.ID
		case "service_discovery_secondary":
			nomadProviderAllocID = allocStub.ID
		default:
			t.Fatalf("unknown task group allocation found: %q", allocStub.TaskGroup)
		}
	}

	require.NotEmpty(t, nomadProviderAllocID)
	require.NotEmpty(t, consulProviderAllocID)

	// Services are registered on by the client, so we need to wait for the
	// alloc to be running before continue safely.
	e2eutil.WaitForAllocsRunning(t, nomadClient, []string{nomadProviderAllocID, consulProviderAllocID})

	// Lookup the service registration in Nomad and assert this matches what we
	// expected.
	expectedNomadService := api.ServiceRegistration{
		ServiceName: "http-api-nomad",
		Namespace:   api.DefaultNamespace,
		Datacenter:  "dc1",
		JobID:       jobID,
		AllocID:     nomadProviderAllocID,
		Tags:        []string{"foo", "bar"},
	}
	requireEventuallyNomadService(t, &expectedNomadService, "")

	// Lookup the service registration in Consul and assert this matches what
	// we expected.
	require.Eventually(t, func() bool {
		consulServices, _, err := consulClient.Catalog().Service("http-api", "", nil)
		if err != nil {
			return false
		}

		// Perform the checks.
		if len(consulServices) != 1 {
			return false
		}
		if consulServices[0].ServiceName != "http-api" {
			return false
		}
		if !strings.Contains(consulServices[0].ServiceID, consulProviderAllocID) {
			return false
		}
		if !reflect.DeepEqual(consulServices[0].ServiceTags, []string{"foo", "bar"}) {
			return false
		}
		return reflect.DeepEqual(consulServices[0].ServiceMeta, map[string]string{"external-source": "nomad"})
	}, defaultWaitForTime, defaultTickTime)

	// Register a "modified" job which removes the second task group and
	// therefore the service registration that is within Nomad.
	allocStubs = e2eutil.RegisterAndWaitForAllocs(t, nomadClient, jobConsulProvider, jobID, "")
	require.Len(t, allocStubs, 2)

	// Check the allocations have the expected.
	require.Eventually(t, func() bool {
		allocStubs, _, err := nomadClient.Jobs().Allocations(jobID, true, nil)
		if err != nil {
			return false
		}
		if len(allocStubs) != 2 {
			return false
		}

		var correctStatus bool

		for _, allocStub := range allocStubs {
			switch allocStub.TaskGroup {
			case "service_discovery":
				correctStatus = correctStatus || api.AllocClientStatusRunning == allocStub.ClientStatus
			case "service_discovery_secondary":
				correctStatus = correctStatus || api.AllocClientStatusComplete == allocStub.ClientStatus
			default:
				t.Fatalf("unknown task group allocation found: %q", allocStub.TaskGroup)
			}
		}
		return correctStatus
	}, defaultWaitForTime, defaultTickTime)

	// We should now have zero service registrations for the given serviceName
	// within Nomad.
	require.Eventually(t, func() bool {
		services, _, err := nomadClient.Services().Get("http-api-nomad", nil)
		if err != nil {
			return false
		}
		return len(services) == 0
	}, defaultWaitForTime, defaultTickTime)

	// The service registration should still exist within Consul.
	require.Eventually(t, func() bool {
		consulServices, _, err := consulClient.Catalog().Service("http-api", "", nil)
		if err != nil {
			return false
		}

		// Perform the checks.
		if len(consulServices) != 1 {
			return false
		}
		if consulServices[0].ServiceName != "http-api" {
			return false
		}
		if !strings.Contains(consulServices[0].ServiceID, consulProviderAllocID) {
			return false
		}
		if !reflect.DeepEqual(consulServices[0].ServiceTags, []string{"foo", "bar"}) {
			return false
		}
		return reflect.DeepEqual(consulServices[0].ServiceMeta, map[string]string{"external-source": "nomad"})
	}, defaultWaitForTime, defaultTickTime)

	// Purge the job and ensure the service is removed. If this completes
	// successfully, cancel the deferred cleanup.
	e2eutil.CleanupJobsAndGC(t, &jobIDs)()
	cancel()

	// Ensure the service has now been removed from Consul. Wrap this in an
	// eventual as Consul updates are a-sync.
	require.Eventually(t, func() bool {
		consulServices, _, err := consulClient.Catalog().Service("http-api", "", nil)
		if err != nil {
			return false
		}
		return len(consulServices) == 0
	}, defaultWaitForTime, defaultTickTime)
}

// testUpdateProvider tests updating the service provider within a running job
// to ensure the backend providers are updated as expected.
func testUpdateProvider(t *testing.T) {

	nomadClient := e2eutil.NomadClient(t)
	const serviceName = "http-api"

	// Generate our job ID which will be used for the entire test.
	jobID := "service-discovery-update-provider-" + uuid.Short()
	jobIDs := []string{jobID}

	// Defer a cleanup function to remove the job. This will trigger if the
	// test fails, unless the cancel function is called.
	ctx, cancel := context.WithCancel(context.Background())
	defer e2eutil.CleanupJobsAndGCWithContext(t, ctx, &jobIDs)

	// We want to capture this for use outside the test func routine.
	var nomadProviderAllocID string

	// Capture the Nomad mini-test as a function, so we can call this twice
	// during this test.
	nomadServiceTestFn := func() {

		// Register the job and get our allocation ID which we can use for later
		// tests.
		allocStubs := e2eutil.RegisterAndWaitForAllocs(t, nomadClient, jobNomadProvider, jobID, "")
		require.Len(t, allocStubs, 1)
		nomadProviderAllocID = allocStubs[0].ID

		// Services are registered on by the client, so we need to wait for the
		// alloc to be running before continue safely.
		e2eutil.WaitForAllocRunning(t, nomadClient, nomadProviderAllocID)

		// List all registrations using the service name and check the return
		// object is as expected. There are some details we cannot assert, such as
		// node ID and address.
		expectedNomadService := api.ServiceRegistration{
			ServiceName: serviceName,
			Namespace:   api.DefaultNamespace,
			Datacenter:  "dc1",
			JobID:       jobID,
			AllocID:     nomadProviderAllocID,
			Tags:        []string{"foo", "bar"},
		}
		requireEventuallyNomadService(t, &expectedNomadService, "")
	}
	nomadServiceTestFn()

	// Register the "modified" job which changes the service provider from
	// Nomad to Consul. Updating the provider should be an in-place update to
	// the allocation.
	allocStubs := e2eutil.RegisterAndWaitForAllocs(t, nomadClient, jobConsulProvider, jobID, "")
	require.Len(t, allocStubs, 1)
	require.Equal(t, nomadProviderAllocID, allocStubs[0].ID)

	// We should now have zero service registrations for the given serviceName.
	require.Eventually(t, func() bool {
		services, _, err := nomadClient.Services().Get(serviceName, nil)
		if err != nil {
			return false
		}
		return len(services) == 0
	}, defaultWaitForTime, defaultTickTime)

	// Grab the Consul client for use.
	consulClient := e2eutil.ConsulClient(t)

	// List all registrations using the service name and check the return
	// object is as expected. There are some details we cannot assert.
	require.Eventually(t, func() bool {
		consulServices, _, err := consulClient.Catalog().Service(serviceName, "", nil)
		if err != nil {
			return false
		}

		// Perform the checks.
		if len(consulServices) != 1 {
			return false
		}
		if consulServices[0].ServiceName != "http-api" {
			return false
		}
		if !strings.Contains(consulServices[0].ServiceID, nomadProviderAllocID) {
			return false
		}
		if !reflect.DeepEqual(consulServices[0].ServiceTags, []string{"foo", "bar"}) {
			return false
		}
		return reflect.DeepEqual(consulServices[0].ServiceMeta, map[string]string{"external-source": "nomad"})
	}, defaultWaitForTime, defaultTickTime)

	// Rerun the Nomad test function. This will register the service back with
	// the Nomad provider and make sure it is found as expected.
	nomadServiceTestFn()

	// Ensure the service has now been removed from Consul. Wrap this in an
	// eventual as Consul updates are a-sync.
	require.Eventually(t, func() bool {
		consulServices, _, err := consulClient.Catalog().Service(serviceName, "", nil)
		if err != nil {
			return false
		}
		return len(consulServices) == 0
	}, defaultWaitForTime, defaultTickTime)

	// Purge the job and ensure the service is removed. If this completes
	// successfully, cancel the deferred cleanup.
	e2eutil.CleanupJobsAndGC(t, &jobIDs)()
	cancel()

	require.Eventually(t, func() bool {
		services, _, err := nomadClient.Services().Get(serviceName, nil)
		if err != nil {
			return false
		}
		return len(services) == 0
	}, defaultWaitForTime, defaultTickTime)
}

// requireEventuallyNomadService is a helper which performs an eventual check
// against Nomad for a single service. Test cases which expect more than a
// single response should implement their own assertion, to handle ordering
// problems.
func requireEventuallyNomadService(t *testing.T, expected *api.ServiceRegistration, filter string) {
	opts := (*api.QueryOptions)(nil)
	if filter != "" {
		opts = &api.QueryOptions{
			Filter: filter,
		}
	}

	require.Eventually(t, func() bool {
		services, _, err := e2eutil.NomadClient(t).Services().Get(expected.ServiceName, opts)
		if err != nil {
			return false
		}

		if len(services) != 1 {
			return false
		}

		// ensure each matching service meets expectations
		if services[0].ServiceName != expected.ServiceName {
			return false
		}
		if services[0].Namespace != api.DefaultNamespace {
			return false
		}
		if services[0].Datacenter != "dc1" {
			return false
		}
		if services[0].JobID != expected.JobID {
			return false
		}
		if services[0].AllocID != expected.AllocID {
			return false
		}
		if !slices.Equal(services[0].Tags, expected.Tags) {
			return false
		}

		return true

	}, defaultWaitForTime, defaultTickTime)
}
