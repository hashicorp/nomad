// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package vaultcompat

// role is the recommended nomad cluster role
var role = map[string]interface{}{
	"disallowed_policies": "nomad-server",
	"explicit_max_ttl":    0, // use old name for vault compatibility
	"name":                "nomad-cluster",
	"orphan":              false,
	"period":              259200, // use old name for vault compatibility
	"renewable":           true,
}
