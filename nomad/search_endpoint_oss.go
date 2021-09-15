// +build !ent

package nomad

import (
	"fmt"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

var (
	// allContexts are the available contexts which are searched to find matches
	// for a given prefix
	allContexts = ossContexts
)

// contextToIndex returns the index name to lookup in the state store.
func contextToIndex(ctx structs.Context) string {
	return string(ctx)
}

// getEnterpriseMatch is a no-op in oss since there are no enterprise objects.
func getEnterpriseMatch(match interface{}) (id string, ok bool) {
	return "", false
}

// getEnterpriseResourceIter is used to retrieve an iterator over an enterprise
// only table.
func getEnterpriseResourceIter(context structs.Context, _ *acl.ACL, namespace, prefix string, ws memdb.WatchSet, state *state.StateStore) (memdb.ResultIterator, error) {
	// If we have made it here then it is an error since we have exhausted all
	// open source contexts.
	return nil, fmt.Errorf("context must be one of %v or 'all' for all contexts; got %q", allContexts, context)
}

// getEnterpriseFuzzyResourceIter is used to retrieve an iterator over an enterprise
// only table.
func getEnterpriseFuzzyResourceIter(context structs.Context, _ *acl.ACL, _ string, _ memdb.WatchSet, _ *state.StateStore) (memdb.ResultIterator, error) {
	return nil, fmt.Errorf("context must be one of %v or 'all' for all contexts; got %q", allContexts, context)
}

// sufficientSearchPerms returns true if the provided ACL has access to each
// capability required for prefix searching for the given context.
//
// Returns true if aclObj is nil.
func sufficientSearchPerms(aclObj *acl.ACL, namespace string, context structs.Context) bool {
	if aclObj == nil {
		return true
	}

	nodeRead := aclObj.AllowNodeRead()
	allowNS := aclObj.AllowNamespace(namespace)
	jobRead := aclObj.AllowNsOp(namespace, acl.NamespaceCapabilityReadJob)
	allowVolume := acl.NamespaceValidator(acl.NamespaceCapabilityCSIListVolume,
		acl.NamespaceCapabilityCSIReadVolume,
		acl.NamespaceCapabilityListJobs,
		acl.NamespaceCapabilityReadJob)
	volRead := allowVolume(aclObj, namespace)

	if !nodeRead && !jobRead && !volRead && !allowNS {
		return false
	}

	// Reject requests that explicitly specify a disallowed context. This
	// should give the user better feedback then simply filtering out all
	// results and returning an empty list.
	if !nodeRead && context == structs.Nodes {
		return false
	}
	if !allowNS && context == structs.Namespaces {
		return false
	}

	if !jobRead {
		switch context {
		case structs.Allocs, structs.Deployments, structs.Evals, structs.Jobs:
			return false
		}
	}
	if !volRead && context == structs.Volumes {
		return false
	}

	return true
}

// filteredSearchContexts returns the expanded set of contexts, filtered down
// to the subset of contexts the aclObj is valid for.
//
// If aclObj is nil, no contexts are filtered out.
func filteredSearchContexts(aclObj *acl.ACL, namespace string, context structs.Context) []structs.Context {
	desired := expandContext(context)

	// If ACLs aren't enabled return all contexts
	if aclObj == nil {
		return desired
	}

	jobRead := aclObj.AllowNsOp(namespace, acl.NamespaceCapabilityReadJob)
	allowVolume := acl.NamespaceValidator(acl.NamespaceCapabilityCSIListVolume,
		acl.NamespaceCapabilityCSIReadVolume,
		acl.NamespaceCapabilityListJobs,
		acl.NamespaceCapabilityReadJob)
	volRead := allowVolume(aclObj, namespace)
	policyRead := aclObj.AllowNsOp(namespace, acl.NamespaceCapabilityListScalingPolicies)

	// Filter contexts down to those the ACL grants access to
	available := make([]structs.Context, 0, len(desired))
	for _, c := range desired {
		switch c {
		case structs.Allocs, structs.Jobs, structs.Evals, structs.Deployments:
			if jobRead {
				available = append(available, c)
			}
		case structs.ScalingPolicies:
			if policyRead || jobRead {
				available = append(available, c)
			}
		case structs.Namespaces:
			if aclObj.AllowNamespace(namespace) {
				available = append(available, c)
			}
		case structs.Nodes:
			if aclObj.AllowNodeRead() {
				available = append(available, c)
			}
		case structs.Volumes:
			if volRead {
				available = append(available, c)
			}
		}
	}
	return available
}
