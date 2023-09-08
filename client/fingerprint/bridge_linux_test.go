// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/stretchr/testify/require"
)

func TestBridgeFingerprint_detect(t *testing.T) {
	ci.Parallel(t)

	f := &BridgeFingerprint{logger: testlog.HCLogger(t)}
	require.NoError(t, f.detect("kernel")) // kernel should be there.

	err := f.detect("nonexistentmodule")
	require.Error(t, err)
	require.Contains(t, err.Error(), "4 errors occurred")
}

func writeFile(t *testing.T, prefix, content string) string {
	f, err := os.CreateTemp("", "bridge-fp-")
	require.NoError(t, err)

	_, err = io.Copy(f, strings.NewReader(content))
	require.NoError(t, err)

	err = f.Close()
	require.NoError(t, err)

	return f.Name()
}

func cleanupFile(t *testing.T, name string) {
	err := os.Remove(name)
	require.NoError(t, err)
}

const (
	dynamicModuleContent = `
ip_tables 32768 0 - Live 0xffffffffc03ee000
x_tables 40960 1 ip_tables, Live 0xffffffffc03e3000
autofs4 45056 2 - Live 0xffffffffc03d7000
bpfilter 32768 0 - Live 0x0000000000000000
br_netfilter 28672 0 - Live 0x0000000000000000
bridge 176128 1 br_netfilter, Live 0x0000000000000000
btrfs 1253376 0 - Live 0xffffffffc02a4000
`

	builtinModuleContent = `
kernel/drivers/mfd/max14577.ko
kernel/drivers/mfd/max77693.ko
kernel/drivers/mfd/sec-core.ko
kernel/drivers/mfd/sec-irq.ko
kernel/drivers/net/bridge.ko
kernel/drivers/net/tun.ko
kernel/drivers/net/xen-netfront.k
`

	dependsModuleContent = `
kernel/net/bridge/netfilter/ebt_log.ko: kernel/net/netfilter/x_tables.ko
kernel/net/bridge/netfilter/ebt_nflog.ko: kernel/net/netfilter/x_tables.ko
kernel/net/bridge/bridge.ko: kernel/net/802/stp.ko kernel/net/llc/llc.ko
kernel/net/bridge/br_netfilter.ko: kernel/net/bridge/bridge.ko kernel/net/802/stp.ko kernel/net/llc/llc.ko
kernel/net/appletalk/appletalk.ko: kernel/net/802/psnap.ko kernel/net/llc/llc.ko
kernel/net/x25/x25.ko:
# Dummy module to test RHEL modules.dep format
kernel/net/bridge/bridgeRHEL.ko.xz: kernel/net/802/stp.ko.xz kernel/net/llc/llc.ko.xz
`
)

func TestBridgeFingerprint_search(t *testing.T) {
	ci.Parallel(t)

	f := &BridgeFingerprint{logger: testlog.HCLogger(t)}

	t.Run("dynamic loaded module", func(t *testing.T) {
		t.Run("present", func(t *testing.T) {
			file := writeFile(t, "bridge-fp-", dynamicModuleContent)
			defer cleanupFile(t, file)

			err := f.searchFile("bridge", file, f.regexp(dynamicModuleRe, "bridge"))
			require.NoError(t, err)
		})

		t.Run("absent", func(t *testing.T) {
			file := writeFile(t, "bridge-fp-", dynamicModuleContent)
			defer cleanupFile(t, file)

			err := f.searchFile("nonexistent", file, f.regexp(dynamicModuleRe, "nonexistent"))
			require.EqualError(t, err, fmt.Sprintf("module nonexistent not in %s", file))
		})
	})

	t.Run("builtin module", func(t *testing.T) {
		t.Run("present", func(t *testing.T) {
			file := writeFile(t, "bridge-fp-", builtinModuleContent)
			defer cleanupFile(t, file)

			err := f.searchFile("bridge", file, f.regexp(builtinModuleRe, "bridge"))
			require.NoError(t, err)
		})

		t.Run("absent", func(t *testing.T) {
			file := writeFile(t, "bridge-fp-", builtinModuleContent)
			defer cleanupFile(t, file)

			err := f.searchFile("nonexistent", file, f.regexp(builtinModuleRe, "nonexistent"))
			require.EqualError(t, err, fmt.Sprintf("module nonexistent not in %s", file))
		})
	})

	t.Run("dynamic unloaded module", func(t *testing.T) {
		t.Run("present", func(t *testing.T) {
			file := writeFile(t, "bridge-fp-", dependsModuleContent)
			defer cleanupFile(t, file)

			err := f.searchFile("bridge", file, f.regexp(dependsModuleRe, "bridge"))
			require.NoError(t, err)

			err = f.searchFile("bridgeRHEL", file, f.regexp(dependsModuleRe, "bridgeRHEL"))
			require.NoError(t, err)
		})

		t.Run("absent", func(t *testing.T) {
			file := writeFile(t, "bridge-fp-", dependsModuleContent)
			defer cleanupFile(t, file)

			err := f.searchFile("nonexistent", file, f.regexp(dependsModuleRe, "nonexistent"))
			require.EqualError(t, err, fmt.Sprintf("module nonexistent not in %s", file))
		})
	})
}
