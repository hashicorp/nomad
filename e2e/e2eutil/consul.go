package e2eutil

import (
	"fmt"
	"testing"
	"time"

	capi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/testutil"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RequireConsulStatus asserts the aggregate health of the service converges to the expected status.
func RequireConsulStatus(require *require.Assertions, client *capi.Client, namespace, service, expectedStatus string) {
	testutil.WaitForResultRetries(30, func() (bool, error) {
		defer time.Sleep(time.Second) // needs a long time for killing tasks/clients

		_, status := serviceStatus(require, client, namespace, service)
		return status == expectedStatus, fmt.Errorf("service %s/%s: expected %s but found %s", namespace, service, expectedStatus, status)
	}, func(err error) {
		require.NoError(err, "timedout waiting for consul status")
	})
}

// serviceStatus gets the aggregate health of the service and returns the []ServiceEntry for further checking.
func serviceStatus(require *require.Assertions, client *capi.Client, namespace, service string) ([]*capi.ServiceEntry, string) {
	services, _, err := client.Health().Service(service, "", false, &capi.QueryOptions{Namespace: namespace})
	require.NoError(err, "expected no error for %s/%s, got %s", namespace, service, err)
	if len(services) > 0 {
		return services, services[0].Checks.AggregatedStatus()
	}
	return nil, "(unknown status)"
}

// RequireConsulDeregistered asserts that the service eventually is de-registered from Consul.
func RequireConsulDeregistered(require *require.Assertions, client *capi.Client, namespace, service string) {
	testutil.WaitForResultRetries(5, func() (bool, error) {
		defer time.Sleep(time.Second)

		services, _, err := client.Health().Service(service, "", false, &capi.QueryOptions{Namespace: namespace})
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
func RequireConsulRegistered(require *require.Assertions, client *capi.Client, namespace, service string, count int) {
	testutil.WaitForResultRetries(5, func() (bool, error) {
		defer time.Sleep(time.Second)

		services, _, err := client.Catalog().Service(service, "", &capi.QueryOptions{Namespace: namespace})
		require.NoError(err)
		if len(services) != count {
			return false, fmt.Errorf("service %v: expected %v services but found %v %v", service, count, len(services), pretty.Sprint(services))
		}
		return true, nil
	}, func(err error) {
		require.NoError(err)
	})
}

// CreateConsulNamespaces will create each namespace in Consul, with a description
// containing the namespace name.
//
// Requires Consul Enterprise.
func CreateConsulNamespaces(t *testing.T, client *capi.Client, namespaces []string) {
	nsClient := client.Namespaces()
	for _, namespace := range namespaces {
		_, _, err := nsClient.Create(&capi.Namespace{
			Name:        namespace,
			Description: fmt.Sprintf("An e2e namespace called %q", namespace),
		}, nil)
		require.NoError(t, err)
	}
}

// DeleteConsulNamespaces will delete each namespace from Consul.
//
// Requires Consul Enterprise.
func DeleteConsulNamespaces(t *testing.T, client *capi.Client, namespaces []string) {
	nsClient := client.Namespaces()
	for _, namespace := range namespaces {
		_, err := nsClient.Delete(namespace, nil)
		assert.NoError(t, err) // be lenient; used in cleanup
	}
}

// ListConsulNamespaces will list the namespaces in Consul.
//
// Requires Consul Enterprise.
func ListConsulNamespaces(t *testing.T, client *capi.Client) []string {
	nsClient := client.Namespaces()
	namespaces, _, err := nsClient.List(nil)
	require.NoError(t, err)
	result := make([]string, 0, len(namespaces))
	for _, namespace := range namespaces {
		result = append(result, namespace.Name)
	}
	return result
}

// PutConsulKey sets key:value in the Consul KV store under given namespace.
//
// Requires Consul Enterprise.
func PutConsulKey(t *testing.T, client *capi.Client, namespace, key, value string) {
	kvClient := client.KV()
	_, err := kvClient.Put(&capi.KVPair{Key: key, Value: []byte(value)}, &capi.WriteOptions{Namespace: namespace})
	require.NoError(t, err)
}

// DeleteConsulKey deletes the key from the Consul KV store from given namespace.
//
// Requires Consul Enterprise.
func DeleteConsulKey(t *testing.T, client *capi.Client, namespace, key string) {
	kvClient := client.KV()
	_, err := kvClient.Delete(key, &capi.WriteOptions{Namespace: namespace})
	require.NoError(t, err)
}

// ReadConsulConfigEntry retrieves the ConfigEntry of the given namespace, kind,
// and name.
//
// Requires Consul Enterprise.
func ReadConsulConfigEntry(t *testing.T, client *capi.Client, namespace, kind, name string) capi.ConfigEntry {
	ceClient := client.ConfigEntries()
	ce, _, err := ceClient.Get(kind, name, &capi.QueryOptions{Namespace: namespace})
	require.NoError(t, err)
	return ce
}

// DeleteConsulConfigEntry deletes the ConfigEntry of the given namespace, kind,
// and name.
//
// Requires Consul Enterprise.
func DeleteConsulConfigEntry(t *testing.T, client *capi.Client, namespace, kind, name string) {
	ceClient := client.ConfigEntries()
	_, err := ceClient.Delete(kind, name, &capi.WriteOptions{Namespace: namespace})
	require.NoError(t, err)
}
