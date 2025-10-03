// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"

	memdb "github.com/hashicorp/go-memdb"
	version "github.com/hashicorp/go-version"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// Deprecated: Through Nomad v1.2 these values were configurable but
	// functionally unused. We still need to advertise them in Serf for
	// compatibility with v1.2 and earlier.
	deprecatedAPIMajorVersion    = 1
	deprecatedAPIMajorVersionStr = "1"
)

// MinVersionPlanNormalization is the minimum version to support the
// normalization of Plan in SubmitPlan, and the denormalization raft log entry committed
// in ApplyPlanResultsRequest
var MinVersionPlanNormalization = version.Must(version.NewVersion("0.9.2"))

// ensurePath is used to make sure a path exists
func ensurePath(path string, dir bool) error {
	if !dir {
		path = filepath.Dir(path)
	}
	return os.MkdirAll(path, 0755)
}

// shuffleStrings randomly shuffles the list of strings
func shuffleStrings(list []string) {
	for i := range list {
		j := rand.Intn(i + 1)
		list[i], list[j] = list[j], list[i]
	}
}

// partitionAll splits a slice of strings into a slice of slices of strings, each with a max
// size of `size`. All entries from the original slice are preserved. The last slice may be
// smaller than `size`. The input slice is unmodified
func partitionAll(size int, xs []string) [][]string {
	if size < 1 {
		return [][]string{xs}
	}

	out := [][]string{}

	for i := 0; i < len(xs); i += size {
		j := i + size
		if j > len(xs) {
			j = len(xs)
		}
		out = append(out, xs[i:j])
	}

	return out
}

// maxUint64 returns the maximum value
func maxUint64(inputs ...uint64) uint64 {
	l := len(inputs)
	if l == 0 {
		return 0
	} else if l == 1 {
		return inputs[0]
	}

	max := inputs[0]
	for i := 1; i < l; i++ {
		cur := inputs[i]
		if cur > max {
			max = cur
		}
	}
	return max
}

// getNodeForRpc returns a Node struct if the Node supports Node RPC. Otherwise
// an error is returned.
func getNodeForRpc(snap *state.StateSnapshot, nodeID string) (*structs.Node, error) {
	node, err := snap.NodeByID(nil, nodeID)
	if err != nil {
		return nil, err
	}

	if node == nil {
		return nil, fmt.Errorf("%w %s", structs.ErrUnknownNode, nodeID)
	}

	if err := nodeSupportsRpc(node); err != nil {
		return nil, err
	}

	return node, nil
}

var minNodeVersionSupportingRPC = version.Must(version.NewVersion("0.8.0-rc1"))

// nodeSupportsRpc returns a non-nil error if a Node does not support RPC.
func nodeSupportsRpc(node *structs.Node) error {
	rawNodeVer, ok := node.Attributes["nomad.version"]
	if !ok {
		return structs.ErrUnknownNomadVersion
	}

	nodeVer, err := version.NewVersion(rawNodeVer)
	if err != nil {
		return structs.ErrUnknownNomadVersion
	}

	if nodeVer.LessThan(minNodeVersionSupportingRPC) {
		return structs.ErrNodeLacksRpc
	}

	return nil
}

// AllocGetter is an interface for retrieving allocations by ID. It is
// satisfied by *state.StateStore and *state.StateSnapshot.
type AllocGetter interface {
	AllocByID(ws memdb.WatchSet, id string) (*structs.Allocation, error)
}

// getAlloc retrieves an allocation by ID and namespace. If the allocation is
// nil, an error is returned.
func getAlloc(state AllocGetter, allocID string) (*structs.Allocation, error) {
	if allocID == "" {
		return nil, structs.ErrMissingAllocID
	}

	alloc, err := state.AllocByID(nil, allocID)
	if err != nil {
		return nil, err
	}

	if alloc == nil {
		return nil, structs.NewErrUnknownAllocation(allocID)
	}

	return alloc, nil
}
