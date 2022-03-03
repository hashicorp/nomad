package structs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAllocServiceRegistrationsRequest_StaleReadSupport(t *testing.T) {
	req := &AllocServiceRegistrationsRequest{}
	require.True(t, req.IsRead())
}
