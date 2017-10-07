// +build ent

package nomad

import (
	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

var (
	// allContexts are the available contexts which are searched to find matches
	// for a given prefix
	allContexts = append(append(ossContexts, proContexts...), entContexts...)

	// entContexts are the pro contexts which are searched to find matches
	// for a given prefix
	entContexts = []structs.Context{structs.Quotas}
)

// contextToIndex returns the index name to lookup in the state store.
func contextToIndex(ctx structs.Context) string {
	switch ctx {
	case structs.Quotas:
		return state.TableQuotaSpec
	default:
		return string(ctx)
	}
}

// getEnterpriseMatch is used to match on an object only defined in Nomad Pro or
// Premium
func getEnterpriseMatch(match interface{}) (id string, ok bool) {
	switch match.(type) {
	case *structs.QuotaSpec:
		return match.(*structs.QuotaSpec).Name, true
	default:
		return getProMatch(match)
	}
}

// getEnterpriseResourceIter is used to retrieve an iterator over an enterprise
// only table.
func getEnterpriseResourceIter(context structs.Context, namespace, prefix string, ws memdb.WatchSet, state *state.StateStore) (memdb.ResultIterator, error) {
	switch context {
	case structs.Quotas:
		return state.QuotaSpecsByNamePrefix(ws, prefix)
	default:
		return getProResourceIter(context, namespace, prefix, ws, state)
	}
}
