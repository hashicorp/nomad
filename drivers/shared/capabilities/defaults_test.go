package capabilities

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSet_NomadDefaults(t *testing.T) {
	result := NomadDefaults()
	require.Len(t, result.Slice(false), 13)
	defaults := strings.ToLower(HCLSpecLiteral)
	for _, c := range result.Slice(false) {
		require.Contains(t, defaults, c)
	}
}

func TestSet_DockerDefaults(t *testing.T) {
	result := DockerDefaults()
	require.Len(t, result.Slice(false), 14)
	require.Contains(t, result.String(), "net_raw")
}

func TestCaps_Calculate(t *testing.T) {
	for _, tc := range []struct {
		name string

		// input
		allowCaps []string // driver config
		capAdd    []string // task config
		capDrop   []string // task config

		// output
		exp []string
		err error
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
			name:      "allow all",
			allowCaps: []string{"all"},
			capAdd:    nil,
			capDrop:   nil,
			exp:       Supported().Slice(true),
			err:       nil,
		},
		{
			name:      "allow selection",
			allowCaps: []string{"cap_net_raw", "chown", "SYS_TIME"},
			capAdd:    nil,
			capDrop:   nil,
			exp:       []string{"CAP_CHOWN", "CAP_NET_RAW", "CAP_SYS_TIME"},
			err:       nil,
		},
		{
			name:      "add allowed",
			allowCaps: NomadDefaults().Slice(false),
			capAdd:    []string{"chown", "KILL"},
			capDrop:   nil,
			exp:       []string{"CAP_CHOWN", "CAP_KILL"},
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
	} {
		t.Run(tc.name, func(t *testing.T) {
			caps, err := Calculate(tc.allowCaps, tc.capAdd, tc.capDrop)
			require.Equal(t, tc.err, err)
			require.Equal(t, tc.exp, caps)
		})
	}
}
