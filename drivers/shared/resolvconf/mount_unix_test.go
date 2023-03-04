//go:build !windows
// +build !windows

package resolvconf

import (
	"os"
	"path/filepath"
	"testing"

	dresolvconf "github.com/docker/libnetwork/resolvconf"
	"github.com/stretchr/testify/require"
)

func Test_copySystemDNS(t *testing.T) {
	require := require.New(t)
	data, err := os.ReadFile(dresolvconf.Path())
	require.NoError(err)

	dest := filepath.Join(t.TempDir(), "resolv.conf")

	require.NoError(copySystemDNS(dest))
	require.FileExists(dest)

	tmpResolv, err := os.ReadFile(dest)
	require.NoError(err)
	require.Equal(data, tmpResolv)
}
