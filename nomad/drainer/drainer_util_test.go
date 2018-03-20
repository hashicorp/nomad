package drainer

import (
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestDrainer_PartitionAllocDrain(t *testing.T) {
	// Set the max ids per reap to something lower.
	old := maxIdsPerTxn
	defer func() { maxIdsPerTxn = old }()
	maxIdsPerTxn = 2

	require := require.New(t)
	transistions := map[string]*structs.DesiredTransition{"a": nil, "b": nil, "c": nil}
	evals := []*structs.Evaluation{nil, nil, nil}
	requests := partitionAllocDrain(transistions, evals)
	require.Len(requests, 3)

	first := requests[0]
	require.Len(first.Transistions, 2)
	require.Len(first.Evals, 0)

	second := requests[1]
	require.Len(second.Transistions, 1)
	require.Len(second.Evals, 1)

	third := requests[2]
	require.Len(third.Transistions, 0)
	require.Len(third.Evals, 2)
}

func TestDrainer_PartitionIds(t *testing.T) {
	require := require.New(t)

	// Set the max ids per reap to something lower.
	old := maxIdsPerTxn
	defer func() { maxIdsPerTxn = old }()
	maxIdsPerTxn = 2

	ids := []string{"1", "2", "3", "4", "5"}
	requests := partitionIds(ids)
	require.Len(requests, 3)
	require.Len(requests[0], 2)
	require.Len(requests[1], 2)
	require.Len(requests[2], 1)
	require.Equal(requests[0][0], ids[0])
	require.Equal(requests[0][1], ids[1])
	require.Equal(requests[1][0], ids[2])
	require.Equal(requests[1][1], ids[3])
	require.Equal(requests[2][0], ids[4])
}
