// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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
