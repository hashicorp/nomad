// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package auth

import (
	"github.com/hashicorp/nomad/nomad/structs"
)

type Identity struct {
	// Claims is the format of this Identity suitable for selection
	// with a binding rule.
	Claims interface{}

	// ClaimMappings is the format of this Identity suitable for interpolation in a
	// bind name within a binding rule.
	ClaimMappings map[string]string
}

// NewIdentity builds a new Identity that can be used to generate bindings via
// Bind for ACL token creation.
func NewIdentity(
	authMethodConfig *structs.ACLAuthMethodConfig, authClaims *structs.ACLAuthClaims) *Identity {

	claimMappings := make(map[string]string)

	// Populate claimMappings vars with empty values so HIL works.
	for _, k := range authMethodConfig.ClaimMappings {
		claimMappings["value."+k] = ""
	}
	for k, val := range authClaims.Value {
		claimMappings["value."+k] = val
	}

	return &Identity{
		Claims:        authClaims,
		ClaimMappings: claimMappings,
	}
}
