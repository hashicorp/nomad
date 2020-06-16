package host

import (
	"testing"

	"github.com/kr/pretty"
	"github.com/stretchr/testify/require"
)

func TestHostUtils(t *testing.T) {
	mounts := mountedPaths()
	require.NotEmpty(t, mounts)

	du, err := diskUsage("/")
	require.NoError(t, err)
	require.NotZero(t, du.DiskMB)
	require.NotZero(t, du.UsedMB)

	out := call("echo", "1")
	require.Equal(t, "1\n", out)
}

func TestMakeHostData(t *testing.T) {
	pretty.Log(MakeHostData())
}
