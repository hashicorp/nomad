// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !ent
// +build !ent

package nomad

import (
	"errors"

	"github.com/hashicorp/nomad/nomad/structs"
)

func (n *NodePool) validateLicense(pool *structs.NodePool) error {
	if pool != nil && pool.SchedulerConfiguration != nil {
		return errors.New(`Feature "Node Pools Governance" is unlicensed`)
	}

	return nil
}
