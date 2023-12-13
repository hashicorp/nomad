// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package e2eutil

import (
	"testing"

	capi "github.com/hashicorp/consul/api"
	napi "github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper/useragent"
	vapi "github.com/hashicorp/vault/api"
	"github.com/shoenig/test/must"
)

// NomadClient creates a default Nomad client based on the env vars
// from the test environment. Fails the test if it can't be created
func NomadClient(t *testing.T) *napi.Client {
	client, err := napi.NewClient(napi.DefaultConfig())
	must.NoError(t, err)
	return client
}

// ConsulClient creates a default Consul client based on the env vars
// from the test environment. Fails the test if it can't be created
func ConsulClient(t *testing.T) *capi.Client {
	client, err := capi.NewClient(capi.DefaultConfig())
	must.NoError(t, err)
	return client
}

// VaultClient creates a default Vault client based on the env vars
// from the test environment. Fails the test if it can't be created
func VaultClient(t *testing.T) *vapi.Client {
	client, err := vapi.NewClient(vapi.DefaultConfig())
	useragent.SetHeaders(client)
	must.NoError(t, err)
	return client
}
