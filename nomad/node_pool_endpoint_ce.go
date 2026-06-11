// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

package nomad

import (
	"errors"

	"github.com/hashicorp/nomad/v2/nomad/structs"
)

func (n *NodePool) validateLicense(pool *structs.NodePool) error {
	if pool != nil && pool.SchedulerConfiguration != nil {
		return errors.New(`Feature "Node Pools Governance" is unlicensed`)
	}

	return nil
}
