// +build ent

package scheduler

import (
	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
)

// StateEnterprise are the available state store methods for the enterprise
// version.
type StateEnterprise interface {
	StatePro

	// QuotaSpecByName is used to lookup a quota specification
	QuotaSpecByName(ws memdb.WatchSet, name string) (*structs.QuotaSpec, error)

	// QuotaUsageByName is used to lookup a quota usage object
	QuotaUsageByName(ws memdb.WatchSet, name string) (*structs.QuotaUsage, error)
}
