// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package numalib

import (
	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/hashicorp/nomad/client/lib/numalib/hw"
)

// MockTopology is a constructor for the Topology object, only used in tests for
// mocking.
func MockTopology(nodeIDs *idset.Set[hw.NodeID], distances SLIT, cores []Core) *Topology {
	t := &Topology{
		nodeIDs:   nodeIDs,
		Distances: distances, Cores: cores}
	t.SetNodes(nodeIDs)
	return t
}
