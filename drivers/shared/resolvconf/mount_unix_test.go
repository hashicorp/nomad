// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !windows

package resolvconf

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/docker/libnetwork/resolvconf"
	"github.com/shoenig/test/must"
)

func Test_copySystemDNS(t *testing.T) {
	data, err := os.ReadFile(resolvconf.Path())
	must.NoError(t, err)

	resolvConfFile := filepath.Join(t.TempDir(), "resolv.conf")

	must.NoError(t, copySystemDNS(resolvConfFile))
	must.FileExists(t, resolvConfFile)

	tmpResolv, readErr := os.ReadFile(resolvConfFile)
	must.NoError(t, readErr)
	must.Eq(t, data, tmpResolv)
}
