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
// Sentinel policy, quotas, and node pools
func (v *HostVolume) enforceEnterprisePolicy(
	_ *state.StateSnapshot,
	_ *structs.HostVolume,
	_ *structs.ACLToken,
	_ bool,
) (error, error) {
	return nil, nil
}
