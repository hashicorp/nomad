package nomad

import (
	"github.com/hashicorp/nomad/nomad/structs"
)

// resolveCallerNodePool determines the node pool of the caller.
//
// it returns the resolved node pool name. if no pool can be resolved it returns
// structs.NodePoolDefault.
func resolveCallerNodePool(s *Server, ctx *RPCContext, identity *structs.AuthenticatedIdentity) string {
	// 1) Use workload identity claims if present (no state lookup)
	if identity != nil && identity.Claims != nil {
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
	}

	// 2) If RPC context has a NodeID (connected client), prefer that.
	if ctx != nil && ctx.NodeID != "" && s != nil {
		if snap, err := s.fsm.State().Snapshot(); err == nil {
			if node, err := snap.NodeByID(nil, ctx.NodeID); err == nil && node != nil && node.NodePool != "" {
				return node.NodePool
			}
		}
	}

	// 3) If the authenticated identity has a ClientID (node secret), look up
	// node.
	if identity != nil && identity.ClientID != "" && s != nil {
		if snap, err := s.fsm.State().Snapshot(); err == nil {
			if node, err := snap.NodeByID(nil, identity.ClientID); err == nil && node != nil && node.NodePool != "" {
				return node.NodePool
			}
		}
	}

	// fallback to default node pool
	return structs.NodePoolDefault
}
