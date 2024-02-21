// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package host

import (
	"testing"

	"github.com/shoenig/test/must"
)

func TestHostUtils(t *testing.T) {
	mounts := mountedPaths()
	must.SliceNotEmpty(t, mounts)

	du, err := diskUsage("/")
	must.NoError(t, err)
	must.Positive(t, du.DiskMB)
	must.Positive(t, du.UsedMB)
}

func TestMakeHostData(t *testing.T) {

	t.Setenv("VAULT_TOKEN", "foo")
	t.Setenv("BOGUS_TOKEN", "foo")
	t.Setenv("BOGUS_SECRET", "foo")
	t.Setenv("ryanSECRETS", "foo")

	host, err := MakeHostData()
	must.NoError(t, err)
	must.NotEq(t, "", host.OS)
	must.SliceNotEmpty(t, host.Network)
	must.NotEq(t, "", host.ResolvConf)
	must.NotEq(t, "", host.Hosts)
	must.MapNotEmpty(t, host.Disk)
	must.MapNotEmpty(t, host.Environment)
	must.Eq(t, "<redacted>", host.Environment["VAULT_TOKEN"])
	must.Eq(t, "<redacted>", host.Environment["BOGUS_TOKEN"])
	must.Eq(t, "<redacted>", host.Environment["BOGUS_SECRET"])
	must.Eq(t, "<redacted>", host.Environment["ryanSECRETS"])
}
