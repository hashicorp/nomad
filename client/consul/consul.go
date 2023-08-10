// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"github.com/hashicorp/nomad/nomad/structs"
)

// TokenDeriverFunc takes an allocation and a set of tasks and derives a
// service identity token for each. Requests go through nomad server.
type TokenDeriverFunc func(*structs.Allocation, []string) (map[string]string, error)

// ServiceIdentityAPI is the interface the Nomad Client uses to request Consul
// Service Identity tokens through Nomad Server.
//
// ACL requirements
// - acl:write (used by Server only)
type ServiceIdentityAPI interface {
	// DeriveSITokens contacts the nomad server and requests consul service
	// identity tokens be generated for tasks in the allocation.
	DeriveSITokens(alloc *structs.Allocation, tasks []string) (map[string]string, error)
}

// SupportedProxiesAPI is the interface the Nomad Client uses to request from
// Consul the set of supported proxied to use for Consul Connect.
//
// No ACL requirements
type SupportedProxiesAPI interface {
	Proxies() (map[string][]string, error)
}
