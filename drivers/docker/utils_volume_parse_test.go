// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package docker

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestParseVolumeSpecWindows_ForwardSlashDestination covers #28760: a
// Windows volume spec whose destination uses forward slashes, e.g.
// "local:c:/app/", must split at the first colon. The rxName pattern's
// greedy tail used to swallow the destination's drive letter, producing
// source "local:c" and destination "/app/".
func TestParseVolumeSpecWindows_ForwardSlashDestination(t *testing.T) {
	cases := []struct {
		name          string
		bindSpec      string
		hostPath      string
		containerPath string
		mode          string
	}{
		{
			name:          "relative source with forward slash destination",
			bindSpec:      `local:c:/app/`,
			hostPath:      `local`,
			containerPath: `c:/app/`,
		},
		{
			name:          "named source with forward slash destination",
			bindSpec:      `myvol:c:/data`,
			hostPath:      `myvol`,
			containerPath: `c:/data`,
		},
		{
			name:          "relative source with backslash destination",
			bindSpec:      `local:c:\app\`,
			hostPath:      `local`,
			containerPath: `c:\app\`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hp, cp, m, err := parseVolumeSpec(c.bindSpec, "windows")
			require.NoError(t, err)
			require.Equal(t, c.hostPath, hp)
			require.Equal(t, c.containerPath, cp)
			require.Equal(t, c.mode, m)
		})
	}
}
