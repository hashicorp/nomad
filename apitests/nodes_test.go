package apitests

import (
	"testing"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestNodes_GC(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	nodes := c.Nodes()

	err := nodes.GC(uuid.Generate(), nil)
	require.NotNil(err)
	require.True(structs.IsErrUnknownNode(err))
}

func TestNodes_GcAlloc(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	nodes := c.Nodes()

	err := nodes.GcAlloc(uuid.Generate(), nil)
	require.NotNil(err)
	require.True(structs.IsErrUnknownAllocation(err))
}
