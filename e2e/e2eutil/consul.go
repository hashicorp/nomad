// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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
	testutil.WaitForResultRetries(10, func() (bool, error) {
		defer time.Sleep(2 * time.Second)

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
	opts := &capi.WriteOptions{Namespace: namespace}

	_, err := kvClient.Put(&capi.KVPair{Key: key, Value: []byte(value)}, opts)
	require.NoError(t, err)
}

// DeleteConsulKey deletes the key from the Consul KV store from given namespace.
//
// Requires Consul Enterprise.
func DeleteConsulKey(t *testing.T, client *capi.Client, namespace, key string) {
	kvClient := client.KV()
	opts := &capi.WriteOptions{Namespace: namespace}

	_, err := kvClient.Delete(key, opts)
	require.NoError(t, err)
}

// ReadConsulConfigEntry retrieves the ConfigEntry of the given namespace, kind,
// and name.
//
// Requires Consul Enterprise.
func ReadConsulConfigEntry(t *testing.T, client *capi.Client, namespace, kind, name string) capi.ConfigEntry {
	ceClient := client.ConfigEntries()
	opts := &capi.QueryOptions{Namespace: namespace}

	ce, _, err := ceClient.Get(kind, name, opts)
	require.NoError(t, err)
	return ce
}

// DeleteConsulConfigEntry deletes the ConfigEntry of the given namespace, kind,
// and name.
//
// Requires Consul Enterprise.
func DeleteConsulConfigEntry(t *testing.T, client *capi.Client, namespace, kind, name string) {
	ceClient := client.ConfigEntries()
	opts := &capi.WriteOptions{Namespace: namespace}

	_, err := ceClient.Delete(kind, name, opts)
	require.NoError(t, err)
}

// ConsulPolicy is used for create Consul ACL policies that Consul ACL tokens
// can make use of.
type ConsulPolicy struct {
	Name  string // e.g. nomad-operator
	Rules string // e.g. service "" { policy="write" }
}

// CreateConsulPolicy is used to create a Consul ACL policy backed by the given
// ConsulPolicy in the specified namespace.
//
// Requires Consul Enterprise.
func CreateConsulPolicy(t *testing.T, client *capi.Client, namespace string, policy ConsulPolicy) string {
	aclClient := client.ACL()
	opts := &capi.WriteOptions{Namespace: namespace}

	result, _, err := aclClient.PolicyCreate(&capi.ACLPolicy{
		Name:        policy.Name,
		Rules:       policy.Rules,
		Description: fmt.Sprintf("An e2e test policy %q", policy.Name),
	}, opts)
	require.NoError(t, err, "failed to create consul acl policy")
	return result.ID
}

// DeleteConsulPolicies is used to delete a set Consul ACL policies from Consul.
//
// Requires Consul Enterprise.
func DeleteConsulPolicies(t *testing.T, client *capi.Client, policies map[string][]string) {
	aclClient := client.ACL()

	for namespace, policyIDs := range policies {
		opts := &capi.WriteOptions{Namespace: namespace}
		for _, policyID := range policyIDs {
			_, err := aclClient.PolicyDelete(policyID, opts)
			assert.NoError(t, err)
		}
	}
}

// CreateConsulToken is used to create a Consul ACL token backed by the policy of
// the given policyID in the specified namespace.
//
// Requires Consul Enterprise.
func CreateConsulToken(t *testing.T, client *capi.Client, namespace, policyID string) string {
	aclClient := client.ACL()
	opts := &capi.WriteOptions{Namespace: namespace}

	token, _, err := aclClient.TokenCreate(&capi.ACLToken{
		Policies:    []*capi.ACLTokenPolicyLink{{ID: policyID}},
		Description: "An e2e test token",
	}, opts)
	require.NoError(t, err, "failed to create consul acl token")
	return token.SecretID
}

// DeleteConsulTokens is used to delete a set of tokens from Consul.
//
// Requires Consul Enterprise.
func DeleteConsulTokens(t *testing.T, client *capi.Client, tokens map[string][]string) {
	aclClient := client.ACL()

	for namespace, tokenIDs := range tokens {
		opts := &capi.WriteOptions{Namespace: namespace}
		for _, tokenID := range tokenIDs {
			_, err := aclClient.TokenDelete(tokenID, opts)
			assert.NoError(t, err)
		}
	}
}
