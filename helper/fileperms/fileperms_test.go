package fileperms

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Check(t *testing.T) {
	f, err := ioutil.TempFile("", "fileperms")
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, f.Close())
		require.NoError(t, os.RemoveAll(f.Name()))
	})

	t.Run("matches", func(t *testing.T) {
		err := Check(f, Oct600)
		require.NoError(t, err)
	})

	t.Run("mismatches", func(t *testing.T) {
		err := Check(f, Oct777)
		require.EqualError(t, err, "file mode expected 777, got 600")
	})

	t.Run("chmod", func(t *testing.T) {
		err := os.Chmod(f.Name(), Oct655)
		require.NoError(t, err)
		err = Check(f, Oct655)
		require.NoError(t, err)
	})
}
