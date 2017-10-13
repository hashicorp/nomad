// +build pro ent

package nomad

import (
	"fmt"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

var (
	// proContexts are the pro contexts which are searched to find matches
	// for a given prefix
	proContexts = []structs.Context{structs.Namespaces}
)

// getProMatch is used to match on an object only defined in Nomad Pro or
// Premium
func getProMatch(match interface{}) (id string, ok bool) {
	switch match.(type) {
	case *structs.Namespace:
		return match.(*structs.Namespace).Name, true
	default:
		return "", false
	}
}

// getProResourceIter is used to retrieve an iterator over an enterprise
// only table.
func getProResourceIter(context structs.Context, aclObj *acl.ACL, namespace, prefix string, ws memdb.WatchSet, state *state.StateStore) (memdb.ResultIterator, error) {
	switch context {
	case structs.Namespaces:
		iter, err := state.NamespacesByNamePrefix(ws, prefix)
		if err != nil {
			return nil, err
		}
		if aclObj == nil {
			return iter, nil
		}
		return memdb.NewFilterIterator(iter, namespaceFilter(aclObj)), nil
	default:
		return nil, fmt.Errorf("context must be one of %v or 'all' for all contexts; got %q", allContexts, context)
	}
}

// namespaceFilter wraps a namespace iterator with a filter for removing
// namespaces the ACL can't access.
func namespaceFilter(aclObj *acl.ACL) memdb.FilterFunc {
	return func(v interface{}) bool {
		return !aclObj.AllowNamespace(v.(*structs.Namespace).Name)
	}
}
