// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

type Capabilities struct {
	ACL              bool
	ACLEnabled       bool
	OIDC             bool
	WorkloadIdentity bool
	ConsulVaultWI    bool
	NUMA             bool
	Plugins          []string
}

type CapabilitiesListRequest struct {
	QueryOptions
}

type CapabilitiesListResponse struct {
	Capabilities *Capabilities
}
