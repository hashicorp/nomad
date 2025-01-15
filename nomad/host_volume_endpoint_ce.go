// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

package nomad

import (
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// enforceEnterprisePolicy is the CE stub for Enterprise governance via
// Sentinel policy and quotas
func (v *HostVolume) enforceEnterprisePolicy(
	_ *state.StateSnapshot,
	_ *structs.HostVolume,
	_ *structs.ACLToken,
	_ bool,
) (error, error) {
	return nil, nil
}

// enterpriseNodePoolFilter is the CE stub for filtering nodes during placement
// via Enterprise node pool governance.
func (v *HostVolume) enterpriseNodePoolFilter(_ *state.StateSnapshot, _ *structs.HostVolume) (func(string) bool, error) {
	return func(_ string) bool { return true }, nil
}
