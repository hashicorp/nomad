package e2eutil

import (
	"time"

	capi "github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/require"
)

// RequireConsulStatus asserts the aggregate health of the service converges to the expected status.
func RequireConsulStatus(require *require.Assertions, client *capi.Client, serviceName, expectedStatus string) {
	require.Eventually(func() bool {
		_, status := serviceStatus(require, client, serviceName)
		return status == expectedStatus
	}, 30*time.Second, time.Second, // needs a long time for killing tasks/clients
		"timed out expecting %q to become %q",
		serviceName, expectedStatus,
	)
}

// serviceStatus gets the aggregate health of the service and returns the []ServiceEntry for further checking.
func serviceStatus(require *require.Assertions, client *capi.Client, serviceName string) ([]*capi.ServiceEntry, string) {
	services, _, err := client.Health().Service(serviceName, "", false, nil)
	require.NoError(err, "expected no error for %q, got %v", serviceName, err)
	if len(services) > 0 {
		return services, services[0].Checks.AggregatedStatus()
	}
	return nil, "(unknown status)"
}

// RequireConsulDeregistered asserts that the service eventually is de-registered from Consul.
func RequireConsulDeregistered(require *require.Assertions, client *capi.Client, service string) {
	require.Eventually(func() bool {
		services, _, err := client.Health().Service(service, "", false, nil)
		require.NoError(err)
		return len(services) == 0
	}, 5*time.Second, time.Second)
}

// RequireConsulRegistered assert that the service is registered in Consul.
func RequireConsulRegistered(require *require.Assertions, client *capi.Client, service string, count int) {
	require.Eventually(func() bool {
		services, _, err := client.Catalog().Service(service, "", nil)
		require.NoError(err)
		return len(services) == count
	}, 5*time.Second, time.Second)
}
