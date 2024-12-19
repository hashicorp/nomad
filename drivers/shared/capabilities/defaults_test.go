// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package capabilities

import (
	"errors"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/hashicorp/nomad/ci"
	"github.com/stretchr/testify/require"
)

func TestSet_NomadDefaults(t *testing.T) {
	ci.Parallel(t)

	result := NomadDefaults()
	require.Len(t, result.Slice(false), 13)
	defaults := strings.ToLower(HCLSpecLiteral)
	for _, c := range result.Slice(false) {
		require.Contains(t, defaults, c)
	}
}

func TestSet_DockerDefaults(t *testing.T) {
	ci.Parallel(t)

	result := DockerDefaults(types.Version{})
	require.Len(t, result.Slice(false), 14)
	require.Contains(t, result.String(), "net_raw")
}

func TestCaps_Calculate(t *testing.T) {
	ci.Parallel(t)

	for _, tc := range []struct {
		name string

		// input
		allowCaps []string // driver config
		capAdd    []string // task config
		capDrop   []string // task config

		// output
		exp  []string
		err  error
		skip bool // error message is linux version dependent
	}{
		{
			name:      "the default setting",
			allowCaps: NomadDefaults().Slice(false),
			capAdd:    nil,
			capDrop:   nil,
			exp:       NomadDefaults().Slice(true),
			err:       nil,
		},
		{
			name:      "allow all no mods",
			allowCaps: []string{"all"},
			capAdd:    nil,
			capDrop:   nil,
			exp:       NomadDefaults().Slice(true),
			err:       nil,
		},
		{
			name:      "allow selection no mods",
			allowCaps: []string{"cap_net_raw", "chown", "SYS_TIME"},
			capAdd:    nil,
			capDrop:   nil,
			exp:       []string{"CAP_CHOWN"},
			err:       nil,
		},
		{
			name:      "allow selection and add them",
			allowCaps: []string{"cap_net_raw", "chown", "SYS_TIME"},
			capAdd:    []string{"net_raw", "sys_time"},
			capDrop:   nil,
			exp:       []string{"CAP_CHOWN", "CAP_NET_RAW", "CAP_SYS_TIME"},
			err:       nil,
		},
		{
			name:      "allow defaults and add redundant",
			allowCaps: NomadDefaults().Slice(false),
			capAdd:    []string{"chown", "KILL"},
			capDrop:   nil,
			exp:       NomadDefaults().Slice(true),
			err:       nil,
		},
		{
			skip:      true,
			name:      "allow defaults and add all",
			allowCaps: NomadDefaults().Slice(false),
			capAdd:    []string{"all"},
			capDrop:   nil,
			exp:       nil,
			err:       errors.New("driver does not allow the following capabilities: audit_control, audit_read, block_suspend, bpf, dac_read_search, ipc_lock, ipc_owner, lease, linux_immutable, mac_admin, mac_override, net_admin, net_broadcast, net_raw, perfmon, sys_admin, sys_boot, sys_module, sys_nice, sys_pacct, sys_ptrace, sys_rawio, sys_resource, sys_time, sys_tty_config, syslog, wake_alarm"),
		},
		{
			name:      "allow defaults and drop all",
			allowCaps: NomadDefaults().Slice(false),
			capAdd:    nil,
			capDrop:   []string{"all"},
			exp:       []string{},
			err:       nil,
		},
		{
			name:      "allow defaults and drop all and add back some",
			allowCaps: NomadDefaults().Slice(false),
			capAdd:    []string{"chown", "fowner"},
			capDrop:   []string{"all"},
			exp:       []string{"CAP_CHOWN", "CAP_FOWNER"},
			err:       nil,
		},
		{
			name:      "add disallowed",
			allowCaps: NomadDefaults().Slice(false),
			capAdd:    []string{"chown", "net_raw"},
			capDrop:   nil,
			exp:       nil,
			err:       errors.New("driver does not allow the following capabilities: net_raw"),
		},
		{
			name:      "drop some",
			allowCaps: NomadDefaults().Slice(false),
			capAdd:    nil,
			capDrop:   []string{"chown", "fowner", "CAP_KILL", "SYS_CHROOT", "mknod", "dac_override"},
			exp:       []string{"CAP_AUDIT_WRITE", "CAP_FSETID", "CAP_NET_BIND_SERVICE", "CAP_SETFCAP", "CAP_SETGID", "CAP_SETPCAP", "CAP_SETUID"},
			err:       nil,
		},
		{
			name:      "drop all",
			allowCaps: NomadDefaults().Slice(false),
			capAdd:    nil,
			capDrop:   []string{"all"},
			exp:       []string{},
			err:       nil,
		},
		{
			name:      "drop all and add back",
			allowCaps: NomadDefaults().Slice(false),
			capAdd:    []string{"chown", "mknod"},
			capDrop:   []string{"all"},
			exp:       []string{"CAP_CHOWN", "CAP_MKNOD"},
			err:       nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			caps, err := Calculate(NomadDefaults(), tc.allowCaps, tc.capAdd, tc.capDrop)
			if !tc.skip {
				require.Equal(t, tc.err, err)
				require.Equal(t, tc.exp, caps)
			} else {
				require.Error(t, err)
				require.Equal(t, tc.exp, caps)
			}
		})
	}
}

func TestCaps_Delta(t *testing.T) {
	ci.Parallel(t)

	for _, tc := range []struct {
		name string

		// input
		allowCaps []string // driver config
		capAdd    []string // task config
		capDrop   []string // task config

		// output
		expAdd  []string
		expDrop []string
		err     error
		skip    bool // error message is linux version dependent
	}{
		{
			name:      "the default setting",
			allowCaps: NomadDefaults().Slice(false),
			capAdd:    nil,
			capDrop:   nil,
			expAdd:    []string{},
			expDrop:   []string{"net_raw"},
			err:       nil,
		},
		{
			name:      "allow all no mods",
			allowCaps: []string{"all"},
			capAdd:    nil,
			capDrop:   nil,
			expAdd:    []string{},
			expDrop:   []string{},
			err:       nil,
		},
		{
			name:      "allow non-default no mods",
			allowCaps: []string{"cap_net_raw", "chown", "SYS_TIME"},
			capAdd:    nil,
			capDrop:   nil,
			expAdd:    []string{},
			expDrop: []string{
				"audit_write", "dac_override", "fowner", "fsetid",
				"kill", "mknod", "net_bind_service", "setfcap",
				"setgid", "setpcap", "setuid", "sys_chroot"},
			err: nil,
		},
		{
			name:      "allow default add from default",
			allowCaps: NomadDefaults().Slice(false),
			capAdd:    []string{"chown", "KILL"},
			capDrop:   nil,
			expAdd:    []string{"chown", "kill"},
			expDrop:   []string{"net_raw"},
			err:       nil,
		},
		{
			name:      "allow default add disallowed",
			allowCaps: NomadDefaults().Slice(false),
			capAdd:    []string{"chown", "net_raw"},
			capDrop:   nil,
			expAdd:    nil,
			expDrop:   nil,
			err:       errors.New("driver does not allow the following capabilities: net_raw"),
		},
		{
			name:      "allow default drop from default",
			allowCaps: NomadDefaults().Slice(false),
			capAdd:    nil,
			capDrop:   []string{"chown", "fowner", "CAP_KILL", "SYS_CHROOT", "mknod", "dac_override"},
			expAdd:    []string{},
			expDrop:   []string{"chown", "dac_override", "fowner", "kill", "mknod", "net_raw", "sys_chroot"},
			err:       nil,
		},
		{
			name:      "allow default drop all",
			allowCaps: NomadDefaults().Slice(false),
			capAdd:    nil,
			capDrop:   []string{"all"},
			expAdd:    []string{},
			expDrop:   []string{"all"},
			err:       nil,
		},
		{
			name:      "task drop all and add back",
			allowCaps: NomadDefaults().Slice(false),
			capAdd:    []string{"chown", "fowner"},
			capDrop:   []string{"all"},
			expAdd:    []string{"chown", "fowner"},
			expDrop:   []string{"all"},
			err:       nil,
		},
		{
			name:      "add atop allow all",
			allowCaps: []string{"all"},
			capAdd:    []string{"chown", "fowner"},
			capDrop:   nil,
			expAdd:    []string{"chown", "fowner"},
			expDrop:   []string{},
			err:       nil,
		},
		{
			name:      "add all atop all",
			allowCaps: []string{"all"},
			capAdd:    []string{"all"},
			capDrop:   nil,
			expAdd:    []string{"all"},
			expDrop:   []string{},
			err:       nil,
		},
		{
			skip:      true,
			name:      "add all atop defaults",
			allowCaps: NomadDefaults().Slice(false),
			capAdd:    []string{"all"},
			capDrop:   nil,
			expAdd:    nil,
			expDrop:   nil,
			err:       errors.New("driver does not allow the following capabilities: audit_control, audit_read, block_suspend, bpf, dac_read_search, ipc_lock, ipc_owner, lease, linux_immutable, mac_admin, mac_override, net_admin, net_broadcast, net_raw, perfmon, sys_admin, sys_boot, sys_module, sys_nice, sys_pacct, sys_ptrace, sys_rawio, sys_resource, sys_time, sys_tty_config, syslog, wake_alarm"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			add, drop, err := Delta(DockerDefaults(types.Version{}), tc.allowCaps, tc.capAdd, tc.capDrop)
			if !tc.skip {
				require.Equal(t, tc.err, err)
				require.Equal(t, tc.expAdd, add)
				require.Equal(t, tc.expDrop, drop)
			} else {
				require.Error(t, err)
				require.Equal(t, tc.expDrop, drop)
			}
		})
	}
}
