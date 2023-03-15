//go:build !windows
// +build !windows

package resolvconf

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	dresolvconf "github.com/docker/libnetwork/resolvconf"
	"github.com/stretchr/testify/require"
)

func Test_copySystemDNS(t *testing.T) {
	require := require.New(t)
	data, err := ioutil.ReadFile(dresolvconf.Path())
	require.NoError(err)

	dest := filepath.Join(t.TempDir(), "resolv.conf")

	require.NoError(copySystemDNS(dest))
	require.FileExists(dest)

	tmpResolv, err := ioutil.ReadFile(dest)
	require.NoError(err)
	require.Equal(data, tmpResolv)
}
