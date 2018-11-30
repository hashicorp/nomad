package nvidia

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCliConfigureArgs(t *testing.T) {
	cases := []struct {
		cfg      Config
		expected []string
	}{
		{
			cfg:      Config{},
			expected: []string{"--load-kmods", "configure", "--devices=none"},
		},
		{
			cfg: Config{
				Devices:      []string{"0", "UUID"},
				Capabilities: []string{"utility", "compute"},
				Requirements: []string{"cuda>=8.0"},
			},
			expected: []string{"--load-kmods", "configure", "--devices=0,UUID", "--utility", "--compute", "--require=cuda>=8.0"},
		},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("case '%s'", strings.Join(c.expected, " ")), func(t *testing.T) {
			expected := append([]string{}, c.expected...)
			expected = append(expected, "/tmp/rootfs")

			found, err := cliConfigureArgs("/tmp/rootfs", c.cfg)
			require.NoError(t, err)
			require.EqualValues(t, expected, found)
		})
	}
}
