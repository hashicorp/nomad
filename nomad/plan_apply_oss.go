// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

package nomad

import (
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// refreshIndex returns the index the scheduler should refresh to as the maximum
// of both the allocation and node tables.
func refreshIndex(snap *state.StateSnapshot) (uint64, error) {
	allocIndex, err := snap.Index("allocs")
	if err != nil {
		return 0, err
	}
	nodeIndex, err := snap.Index("nodes")
	if err != nil {
		return 0, err
	}
	return maxUint64(nodeIndex, allocIndex), nil
}

// evaluatePlanQuota returns whether the plan would be over quota
func evaluatePlanQuota(_ *state.StateSnapshot, _ *structs.Plan) (bool, error) {
	return false, nil
}
