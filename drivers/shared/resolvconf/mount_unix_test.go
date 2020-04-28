// +build !windows

package resolvconf

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_copySystemDNS(t *testing.T) {
	require := require.New(t)
	data, err := ioutil.ReadFile("/etc/resolv.conf")
	require.NoError(err)

	tmp, err := ioutil.TempDir("", "copySystemDNS_Test")
	require.NoError(err)
	defer os.RemoveAll(tmp)
	dest := filepath.Join(tmp, "resolv.conf")

	require.NoError(copySystemDNS(dest))
	require.FileExists(dest)

	tmpResolv, err := ioutil.ReadFile(dest)
	require.NoError(err)
	require.Equal(data, tmpResolv)
}
