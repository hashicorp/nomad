package nomad

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJobNamespaceConstraintCheckHook_Name(t *testing.T) {
	t.Parallel()

	require.Equal(t, "namespace-constraint-check", new(jobNamespaceConstraintCheckHook).Name())
}

// TODO: More tests
