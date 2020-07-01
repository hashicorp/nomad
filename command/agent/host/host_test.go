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
}

func TestMakeHostData(t *testing.T) {
	host, err := MakeHostData()
	require.NoError(t, err)
	require.NotEmpty(t, host.OS)
	require.Empty(t, host.Network)
	require.NotEmpty(t, host.ResolvConf)
	require.NotEmpty(t, host.Hosts)
	require.NotEmpty(t, host.Disk)
}
