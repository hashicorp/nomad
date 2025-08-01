// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

package nomad

import (
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

func (v *CSIVolume) enforceEnterprisePolicy(_ *state.StateSnapshot, _ *structs.CSIVolume, _ *structs.CSIVolume, _ *structs.ACLToken, _ bool) (error, error) {
	return nil, nil
}
