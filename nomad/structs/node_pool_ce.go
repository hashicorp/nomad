// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

package structs

import "errors"

// Validate returns an error if the node pool scheduler configuration is
// invalid.
func (n *NodePoolSchedulerConfiguration) Validate() error {
	if n != nil {
		return errors.New("Node Pools Governance is unlicensed.")
	}
	return nil
}
