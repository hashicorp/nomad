// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"reflect"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/stretchr/testify/require"
)

func TestShuffleStrings(t *testing.T) {
	ci.Parallel(t)
	// Generate input
	inp := make([]string, 10)
	for idx := range inp {
		inp[idx] = uuid.Generate()
	}

	// Copy the input
	orig := make([]string, len(inp))
	copy(orig, inp)

	// Shuffle
	shuffleStrings(inp)

	// Ensure order is not the same
	if reflect.DeepEqual(inp, orig) {
		t.Fatalf("shuffle failed")
	}
}

func Test_partitionAll(t *testing.T) {
	xs := []string{"a", "b", "c", "d", "e", "f"}
	// evenly divisible
	require.Equal(t, [][]string{{"a", "b"}, {"c", "d"}, {"e", "f"}}, partitionAll(2, xs))
	require.Equal(t, [][]string{{"a", "b", "c"}, {"d", "e", "f"}}, partitionAll(3, xs))
	// whole thing fits int the last part
	require.Equal(t, [][]string{{"a", "b", "c", "d", "e", "f"}}, partitionAll(7, xs))
	// odd remainder
	require.Equal(t, [][]string{{"a", "b", "c", "d"}, {"e", "f"}}, partitionAll(4, xs))
	// zero size
	require.Equal(t, [][]string{{"a", "b", "c", "d", "e", "f"}}, partitionAll(0, xs))
	// one size
	require.Equal(t, [][]string{{"a"}, {"b"}, {"c"}, {"d"}, {"e"}, {"f"}}, partitionAll(1, xs))
}

func TestMaxUint64(t *testing.T) {
	ci.Parallel(t)
	if maxUint64(1, 2) != 2 {
		t.Fatalf("bad")
	}
	if maxUint64(2, 2) != 2 {
		t.Fatalf("bad")
	}
	if maxUint64(2, 1) != 2 {
		t.Fatalf("bad")
	}
}
