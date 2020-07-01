package host

import (
	"testing"

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
	host, err := MakeHostData()
	require.NoError(t, err)
	require.NotEmpty(t, host.OS)
	require.NotEmpty(t, host.Network)
	require.NotEmpty(t, host.ResolvConf)
	require.NotEmpty(t, host.Hosts)
	require.NotEmpty(t, host.Disk)
}
