// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: MPL-2.0

//go:build !windows

package hostnames

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

func TestGenerateEtcHostsMount(t *testing.T) {

	testCases := []struct {
		name        string
		spec        *drivers.NetworkIsolationSpec
		extraHosts  []string
		expected    []string
		expectedErr string
	}{
		{
			name: "no-spec",
		},
		{
			name: "no-hosts-config",
			spec: &drivers.NetworkIsolationSpec{Mode: drivers.NetIsolationModeGroup},
		},
		{
			name: "base-case",
			spec: &drivers.NetworkIsolationSpec{
				Mode: drivers.NetIsolationModeGroup,
				HostsConfig: &drivers.HostsConfig{
					Address:  "192.168.1.1",
					Hostname: "xyzzy",
				},
			},
			expected: []string{
				"192.168.1.1 xyzzy",
			},
		},
		{
			name: "with-valid-extra-hosts",
			spec: &drivers.NetworkIsolationSpec{
				Mode: drivers.NetIsolationModeGroup,
				HostsConfig: &drivers.HostsConfig{
					Address:  "192.168.1.1",
					Hostname: "xyzzy",
				},
			},
			extraHosts: []string{
				"apple:192.168.1.2",
				"banana:2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			},
			expected: []string{
				"192.168.1.1 xyzzy",
				"192.168.1.2 apple",
				"2001:0db8:85a3:0000:0000:8a2e:0370:7334 banana",
			},
		},
		{
			name: "invalid-extra-hosts-syntax",
			spec: &drivers.NetworkIsolationSpec{
				Mode: drivers.NetIsolationModeGroup,
				HostsConfig: &drivers.HostsConfig{
					Address:  "192.168.1.1",
					Hostname: "xyzzy",
				},
			},
			extraHosts:  []string{"apple192.168.1.2"},
			expectedErr: "invalid hosts entry \"apple192.168.1.2\"",
		},
		{
			name: "invalid-extra-hosts-bad-ip",
			spec: &drivers.NetworkIsolationSpec{
				Mode: drivers.NetIsolationModeGroup,
				HostsConfig: &drivers.HostsConfig{
					Address:  "192.168.1.1",
					Hostname: "xyzzy",
				},
			},
			extraHosts:  []string{"apple:192.168.1.256"},
			expectedErr: "invalid IP address \"apple:192.168.1.256\"",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require := require.New(t)

			taskDir := t.TempDir()
			dest := filepath.Join(taskDir, "hosts")

			got, err := GenerateEtcHostsMount(taskDir, tc.spec, tc.extraHosts)

			if tc.expectedErr != "" {
				require.EqualError(err, tc.expectedErr)
			} else {
				require.NoError(err)
			}
			if len(tc.expected) == 0 {
				require.Nil(got)
			} else {
				require.NotNil(got)
				require.FileExists(dest)
				tmpHosts, err := os.ReadFile(dest)
				require.NoError(err)
				for _, line := range tc.expected {
					require.Contains(string(tmpHosts), line)
				}
			}
		})
	}
}

func TestGenerateEtcHostsMount_Symlink(t *testing.T) {
	spec := &drivers.NetworkIsolationSpec{
		Mode: drivers.NetIsolationModeGroup,
		HostsConfig: &drivers.HostsConfig{
			Address:  "192.168.1.1",
			Hostname: "xyzzy",
		},
	}

	t.Run("dangling", func(t *testing.T) {
		taskDir := t.TempDir()
		target := filepath.Join(t.TempDir(), "victim")
		must.NoError(t, os.Symlink(target, filepath.Join(taskDir, "hosts")))

		got, err := GenerateEtcHostsMount(taskDir, spec, nil)

		// the target must not have been created through the link
		_, statErr := os.Lstat(target)
		must.ErrorIs(t, statErr, os.ErrNotExist)

		must.ErrorContains(t, err, "refusing to mount symlinked hosts file")
		must.Nil(t, got)
	})

	t.Run("existing", func(t *testing.T) {
		taskDir := t.TempDir()
		target := filepath.Join(t.TempDir(), "victim")
		must.NoError(t, os.WriteFile(target, []byte("original"), 0600))
		must.NoError(t, os.Symlink(target, filepath.Join(taskDir, "hosts")))

		got, err := GenerateEtcHostsMount(taskDir, spec, nil)
		must.ErrorContains(t, err, "refusing to mount symlinked hosts file")
		must.Nil(t, got)

		b, err := os.ReadFile(target)
		must.NoError(t, err)
		must.Eq(t, "original", string(b))
	})
}
