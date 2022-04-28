// +build !windows

package hostnames

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/nomad/plugins/drivers"
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
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			require := require.New(t)

			taskDir, err := ioutil.TempDir("",
				fmt.Sprintf("generateEtcHosts_Test-%s", tc.name))
			defer os.RemoveAll(taskDir)
			require.NoError(err)
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
				tmpHosts, err := ioutil.ReadFile(dest)
				require.NoError(err)
				for _, line := range tc.expected {
					require.Contains(string(tmpHosts), line)
				}
			}
		})
	}
}
