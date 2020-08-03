package host

import (
	"os"
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
	// setenv variables that should be redacted
	prev := os.Getenv("VAULT_TOKEN")
	os.Setenv("VAULT_TOKEN", "foo")
	defer os.Setenv("VAULT_TOKEN", prev)

	os.Setenv("BOGUS_TOKEN", "foo")
	os.Setenv("BOGUS_SECRET", "foo")
	os.Setenv("ryanSECRETS", "foo")

	host, err := MakeHostData()
	require.NoError(t, err)
	require.NotEmpty(t, host.OS)
	require.NotEmpty(t, host.Network)
	require.NotEmpty(t, host.ResolvConf)
	require.NotEmpty(t, host.Hosts)
	require.NotEmpty(t, host.Disk)
	require.NotEmpty(t, host.Environment)
	require.Equal(t, "<redacted>", host.Environment["VAULT_TOKEN"])
	require.Equal(t, "<redacted>", host.Environment["BOGUS_TOKEN"])
	require.Equal(t, "<redacted>", host.Environment["BOGUS_SECRET"])
	require.Equal(t, "<redacted>", host.Environment["ryanSECRETS"])
}
