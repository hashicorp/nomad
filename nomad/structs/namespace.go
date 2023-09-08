// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

// NamespaceVaultConfiguration stores configuration about permissions to Vault
// clusters for a namespace, for use with Nomad Enterprise.
type NamespaceVaultConfiguration struct {
	// Default is the Vault cluster used by jobs in this namespace that don't
	// specify a cluster of their own.
	Default string

	// Allowed specifies the Vault clusters that are allowed to be used by jobs
	// in this namespace. By default, all clusters are allowed. If an empty list
	// is provided only the namespace's default cluster is allowed. This field
	// supports wildcard globbing through the use of `*` for multi-character
	// matching. This field cannot be used with Denied.
	Allowed []string

	// Denied specifies the Vault clusters that are not allowed to be used by
	// jobs in this namespace. This field supports wildcard globbing through the
	// use of `*` for multi-character matching. If specified, any cluster is
	// allowed to be used, except for those that match any of these patterns.
	// This field cannot be used with Allowed.
	Denied []string
}

// NamespaceConsulConfiguration stores configuration about permissions to Consul
// clusters for a namespace, for use with Nomad Enterprise.
type NamespaceConsulConfiguration struct {
	// Default is the Consul cluster used by jobs in this namespace that don't
	// specify a cluster of their own.
	Default string

	// Allowed specifies the Consul clusters that are allowed to be used by jobs
	// in this namespace. By default, all clusters are allowed. If an empty list
	// is provided only the namespace's default cluster is allowed. This field
	// supports wildcard globbing through the use of `*` for multi-character
	// matching. This field cannot be used with Denied.
	Allowed []string

	// Denied specifies the Consul clusters that are not allowed to be used by
	// jobs in this namespace. This field supports wildcard globbing through the
	// use of `*` for multi-character matching. If specified, any cluster is
	// allowed to be used, except for those that match any of these patterns.
	// This field cannot be used with Allowed.
	Denied []string
}
