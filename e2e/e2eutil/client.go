package e2eutil

import (
	"testing"

	capi "github.com/hashicorp/consul/api"
	napi "github.com/hashicorp/nomad/api"
	vapi "github.com/hashicorp/vault/api"

	"github.com/stretchr/testify/require"
)

// NomadClient creates a default Nomad client based on the env vars
// from the test environment. Fails the test if it can't be created
func NomadClient(t *testing.T) *napi.Client {
	client, err := napi.NewClient(napi.DefaultConfig())
	require.NoError(t, err, "could not create Nomad client")
	return client
}

// ConsulClient creates a default Consul client based on the env vars
// from the test environment. Fails the test if it can't be created
func ConsulClient(t *testing.T) *capi.Client {
	client, err := capi.NewClient(capi.DefaultConfig())
	require.NoError(t, err, "could not create Consul client")
	return client
}

// VaultClient creates a default Vault client based on the env vars
// from the test environment. Fails the test if it can't be created
func VaultClient(t *testing.T) *vapi.Client {
	client, err := vapi.NewClient(vapi.DefaultConfig())
	require.NoError(t, err, "could not create Vault client")
	return client
}
