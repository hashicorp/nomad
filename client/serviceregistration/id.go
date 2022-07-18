package serviceregistration

import (
	"crypto/md5"
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// nomadServicePrefix is the prefix that scopes all Nomad registered
	// services (both agent and task entries).
	nomadServicePrefix = "_nomad"

	// nomadTaskPrefix is the prefix that scopes Nomad registered services
	// for tasks (must match service_client.go for consul sync).
	nomadTaskPrefix = nomadServicePrefix + "-task"
)

// MakeAllocServiceID creates a unique ID for identifying an alloc service in
// a service registration provider. Both Nomad and Consul solutions use the
// same ID format to provide consistency.
//
// Format: _nomad-task-allocID-<task|`group`>-service<-port_label><-tags_hash>
//
// Example ID: _nomad-group-7f3eb69d-3a84-a0e7-2681-5f962ef522b0-database-db-tcp-f97c5d
func MakeAllocServiceID(allocID, name string, service *structs.Service) string {
	if name == "" {
		if name = service.TaskName; name == "" {
			name = "group"
		}
	}

	parts := []string{nomadTaskPrefix, allocID, name, service.Name}
	if service.PortLabel != "" {
		parts = append(parts, service.PortLabel)
	}

	if len(service.Tags) > 0 {
		h := md5.New()
		for _, tag := range service.Tags {
			h.Write([]byte(tag))
		}
		short := fmt.Sprintf("%x", h.Sum(nil))[0:6]
		parts = append(parts, short)
	}

	return strings.Join(parts, "-")
}
