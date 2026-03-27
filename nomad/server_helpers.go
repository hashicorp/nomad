// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/nomad/nomad/state"
)

// lookupNodePoolForNodeID determines the node pool for a given node ID.
func lookupNodePoolForNodeID(snap *state.StateSnapshot, nodeID string) (string, bool) {
	if snap == nil || nodeID == "" {
		return "", false
	}

	node, err := snap.NodeByID(memdb.NewWatchSet(), nodeID)
	if err != nil || node == nil || node.NodePool == "" {
		return "", false
	}

	return node.NodePool, true
}
