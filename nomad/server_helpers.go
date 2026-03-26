// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// resolveNodePoolFromClaims determines the node pool directly from authenticated
// identity claims.
//
// it returns the resolved node pool name, or an empty string if resolution
// fails.
func resolveNodePoolFromClaims(identity *structs.AuthenticatedIdentity) string {
	if identity == nil || identity.Claims == nil {
		return ""
	}

	if identity.Claims.IsNode() {
		if p := identity.Claims.NodeIdentityClaims.NodePool; p != "" {
			return p
		}
	}
	if identity.Claims.IsNodeIntroduction() {
		if p := identity.Claims.NodeIntroductionIdentityClaims.NodePool; p != "" {
			return p
		}
	}

	return ""
}

// resolveNodePoolForNodeID determines the node pool for a given node ID.
//
// it returns a permission denied error if the node or its pool cannot be
// resolved.
func resolveNodePoolForNodeID(snap *state.StateSnapshot, nodeID string) (string, error) {
	if snap == nil || nodeID == "" {
		return "", structs.ErrPermissionDenied
	}

	node, err := snap.NodeByID(memdb.NewWatchSet(), nodeID)
	if err != nil || node == nil || node.NodePool == "" {
		return "", structs.ErrPermissionDenied
	}

	return node.NodePool, nil
}

// resolveNodeIDForAllocID determines the node ID for a given allocation ID.
//
// it returns a permission denied error if the allocation or its node ID cannot
// be resolved.
func resolveNodeIDForAllocID(snap *state.StateSnapshot, allocID string) (string, error) {
	if snap == nil || allocID == "" {
		return "", structs.ErrPermissionDenied
	}

	alloc, err := snap.AllocByID(memdb.NewWatchSet(), allocID)
	if err != nil || alloc == nil || alloc.NodeID == "" {
		return "", structs.ErrPermissionDenied
	}

	return alloc.NodeID, nil
}

// resolveCallerNodePool determines the node pool of the caller.
//
// it returns the resolved node pool name. if no pool can be resolved it returns
// a permission denied error.
func resolveCallerNodePool(s *Server, ctx *RPCContext, identity *structs.AuthenticatedIdentity) (string, error) {
	// 1) Use workload identity claims if present (no state lookup)
	if pool := resolveNodePoolFromClaims(identity); pool != "" {
		return pool, nil
	}

	snap, err := s.fsm.State().Snapshot()
	if err != nil {
		// fail fast if we can't get the state; this means other lookups would
		// fail anyway and we should return an error
		return "", structs.ErrPermissionDenied
	}

	// 2) If RPC context has a NodeID (connected client), prefer that.
	if ctx != nil && ctx.NodeID != "" && s != nil {
		if pool, err := resolveNodePoolForNodeID(snap, ctx.NodeID); err == nil {
			return pool, nil
		}
	}

	// 3) If the authenticated identity has a ClientID (node secret), look up
	// node.
	if identity != nil && identity.ClientID != "" && s != nil {
		if pool, err := resolveNodePoolForNodeID(snap, identity.ClientID); err == nil {
			return pool, nil
		}
	}

	return "", structs.ErrPermissionDenied
}
