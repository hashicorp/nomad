package e2eutil

import (
	"fmt"
	"time"

	capi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/testutil"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/require"
)

// RequireConsulStatus asserts the aggregate health of the service converges to the expected status.
func RequireConsulStatus(require *require.Assertions, client *capi.Client, serviceName, expectedStatus string) {
	testutil.WaitForResultRetries(30, func() (bool, error) {
		defer time.Sleep(time.Second) // needs a long time for killing tasks/clients

		_, status := serviceStatus(require, client, serviceName)
		return status == expectedStatus, fmt.Errorf("service %v: expected %v but found %v", serviceName, expectedStatus, status)
	}, func(err error) {
		require.NoError(err, "timedout waiting for consul status")
	})
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
	testutil.WaitForResultRetries(5, func() (bool, error) {
		defer time.Sleep(time.Second)

		services, _, err := client.Health().Service(service, "", false, nil)
		require.NoError(err)
		if len(services) != 0 {
			return false, fmt.Errorf("service %v: expected empty services but found %v %v", service, len(services), pretty.Sprint(services))
		}
		return true, nil
	}, func(err error) {
		require.NoError(err)
	})
}

// RequireConsulRegistered assert that the service is registered in Consul.
func RequireConsulRegistered(require *require.Assertions, client *capi.Client, service string, count int) {
	testutil.WaitForResultRetries(5, func() (bool, error) {
		defer time.Sleep(time.Second)

		services, _, err := client.Catalog().Service(service, "", nil)
		require.NoError(err)
		if len(services) != count {
			return false, fmt.Errorf("service %v: expected %v services but found %v %v", service, count, len(services), pretty.Sprint(services))
		}
		return true, nil
	}, func(err error) {
		require.NoError(err)
	})
}
