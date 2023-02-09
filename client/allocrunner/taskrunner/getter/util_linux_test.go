//go:build linux

package getter

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shoenig/go-landlock"
	"github.com/shoenig/test/must"
)

func TestUtil_loadVersionControlGlobalConfigs(t *testing.T) {
	const filePerm = 0o644
	const dirPerm = 0o755
	fakeEtc := t.TempDir()

	var (
		gitFile = filepath.Join(fakeEtc, "gitconfig")
		hgFile  = filepath.Join(fakeEtc, "hgrc")
		hgDir   = filepath.Join(fakeEtc, "hgrc.d")
	)

	err := os.WriteFile(gitFile, []byte("git"), filePerm)
	must.NoError(t, err)

	err = os.WriteFile(hgFile, []byte("hg"), filePerm)
	must.NoError(t, err)

	err = os.Mkdir(hgDir, dirPerm)
	must.NoError(t, err)

	paths := loadVersionControlGlobalConfigs(gitFile, hgFile, hgDir)
	must.SliceEqual(t, []*landlock.Path{
		landlock.File(gitFile, "r"),
		landlock.File(hgFile, "r"),
		landlock.Dir(hgDir, "r"),
	}, paths)
}
