// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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

	t.Setenv("VAULT_TOKEN", "foo")
	t.Setenv("BOGUS_TOKEN", "foo")
	t.Setenv("BOGUS_SECRET", "foo")
	t.Setenv("ryanSECRETS", "foo")

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
