// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

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
