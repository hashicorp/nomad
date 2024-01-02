// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
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
	switch ctx {
	// Handle cases where context name and state store table name do not match
	case structs.Variables:
		return state.TableVariables
	default:
		return string(ctx)
	}
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

func filteredSearchContextsEnt(aclObj *acl.ACL, namespace string, context structs.Context) bool {
	return true
}
