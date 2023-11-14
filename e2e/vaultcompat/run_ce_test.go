// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent

package vaultcompat

import (
	"context"
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/shoenig/test/must"
)

// usable is used by the downloader to verify that we're getting the right
// versions of Vault CE
func usable(v, minimum *version.Version) bool {
	switch {
	case v.Prerelease() != "":
		return false
	case v.Metadata() != "":
		return false
	case v.LessThan(minimum):
		return false
	default:
		return true
	}
}

func testVaultLegacy(t *testing.T, b build) {
	vStop, vc := startVault(t, b)
	defer vStop()
	setupVaultLegacy(t, vc)

	nStop, nc := startNomad(t, configureNomadVaultLegacy(vc))
	defer nStop()
	runJob(t, nc, "input/cat.hcl", "default", validateLegacyAllocs)
}

func testVaultJWT(t *testing.T, b build) {
	vStop, vc := startVault(t, b)
	defer vStop()

	// Start Nomad without access to the Vault token.
	vaultToken := vc.Token()
	vc.SetToken("")
	nStop, nc := startNomad(t, configureNomadVaultJWT(vc))
	defer nStop()

	// Restore token and configure Vault for JWT login.
	vc.SetToken(vaultToken)
	setupVaultJWT(t, vc, nc.Address()+"/.well-known/jwks.json")

	// Write secrets for test job.
	_, err := vc.KVv2("secret").Put(context.Background(), "default/cat_jwt", map[string]any{
		"secret": "workload",
	})
	must.NoError(t, err)

	_, err = vc.KVv2("secret").Put(context.Background(), "restricted", map[string]any{
		"secret": "restricted",
	})
	must.NoError(t, err)

	// Run test job.
	runJob(t, nc, "input/cat_jwt.hcl", "default", validateJWTAllocs)
}
