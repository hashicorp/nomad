package consul

import (
	"errors"
	"testing"

	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestCSI_DeriveTokens(t *testing.T) {
	logger := testlog.HCLogger(t)
	dFunc := func(alloc *structs.Allocation, taskNames []string) (map[string]string, error) {
		return map[string]string{"a": "b"}, nil
	}
	tc := NewIdentitiesClient(logger, dFunc)
	tokens, err := tc.DeriveSITokens(nil, nil)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"a": "b"}, tokens)
}

func TestCSI_DeriveTokens_error(t *testing.T) {
	logger := testlog.HCLogger(t)
	dFunc := func(alloc *structs.Allocation, taskNames []string) (map[string]string, error) {
		return nil, errors.New("some failure")
	}
	tc := NewIdentitiesClient(logger, dFunc)
	_, err := tc.DeriveSITokens(&structs.Allocation{ID: "a1"}, nil)
	require.Error(t, err)
}
