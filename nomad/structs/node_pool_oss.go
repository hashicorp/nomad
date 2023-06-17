// copyright (c) hashicorp, inc.
// spdx-license-identifier: mpl-2.0

//go:build !ent
// +build !ent

package structs

import "errors"

// Validate returns an error if the node pool scheduler confinguration is
// invalid.
func (n *NodePoolSchedulerConfiguration) Validate() error {
	if n != nil {
		return errors.New("Node Pools Governance is unlicensed.")
	}
	return nil
}
