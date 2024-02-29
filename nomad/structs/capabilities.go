// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

type Capabilities struct {
	ACL              bool
	ACLEnabled       bool
	OIDC             bool
	NodePools        bool
	WorkloadIdentity bool
	ConsulVaultWI    bool
	Enterprise       bool
}

type CapabilitiesListRequest struct {
	QueryOptions
}

type CapabilitiesListResponse struct {
	Capabilities *Capabilities
}
