package capabilities

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSet_NomadDefaults(t *testing.T) {
	result := NomadDefaults()
	require.Len(t, result.Slice(false), 13)
	defaults := strings.ToLower(HCLSpecLiteral)
	for _, c := range result.Slice(false) {
		require.Contains(t, defaults, c)
	}
}

func TestSet_DockerDefaults(t *testing.T) {
	result := DockerDefaults()
	require.Len(t, result.Slice(false), 14)
	require.Contains(t, result.String(), "net_raw")
}
