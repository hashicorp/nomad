// +build pro ent

package scheduler

import (
	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
)

// StatePro contains state methods that are shared across the enterprise
// versions.
type StatePro interface {
	// NamespaceByName is used to lookup a namespace
	NamespaceByName(ws memdb.WatchSet, name string) (*structs.Namespace, error)
}
