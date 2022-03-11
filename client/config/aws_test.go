package config

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAWS_GetFoo(t *testing.T) {
	err := GetFoo()
	require.NoError(t, err)
}
