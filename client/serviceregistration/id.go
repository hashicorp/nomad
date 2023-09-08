// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package serviceregistration

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// nomadServicePrefix is the prefix that scopes all Nomad registered
	// services (both agent and task entries).
	nomadServicePrefix = "_nomad"

	// nomadTaskPrefix is the prefix that scopes Nomad registered services
	// for tasks.
	nomadTaskPrefix = nomadServicePrefix + "-task-"
)

// MakeAllocServiceID creates a unique ID for identifying an alloc service in
// a service registration provider. Both Nomad and Consul solutions use the
// same ID format to provide consistency.
//
// Example Service ID: _nomad-task-b4e61df9-b095-d64e-f241-23860da1375f-redis-http-http
func MakeAllocServiceID(allocID, taskName string, service *structs.Service) string {
	return fmt.Sprintf("%s%s-%s-%s-%s",
		nomadTaskPrefix, allocID, taskName, service.Name, service.PortLabel)
}
