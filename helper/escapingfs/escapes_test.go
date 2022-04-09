package escapingfs

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func setup(t *testing.T) string {
	p, err := ioutil.TempDir("", "escapist")
	require.NoError(t, err)
	return p
}

func cleanup(t *testing.T, root string) {
	err := os.RemoveAll(root)
	require.NoError(t, err)
}

func write(t *testing.T, file, data string) {
	err := ioutil.WriteFile(file, []byte(data), 0600)
	require.NoError(t, err)
}

func Test_PathEscapesAllocViaRelative(t *testing.T) {
	for _, test := range []struct {
		prefix string
		path   string
		exp    bool
	}{
		// directly under alloc-dir/alloc-id/
		{prefix: "", path: "", exp: false},
		{prefix: "", path: "/foo", exp: false},
		{prefix: "", path: "./", exp: false},
		{prefix: "", path: "../", exp: true}, // at alloc-id/

		// under alloc-dir/alloc-id/<foo>/
		{prefix: "foo", path: "", exp: false},
		{prefix: "foo", path: "/foo", exp: false},
		{prefix: "foo", path: "../", exp: false},   // at foo/
		{prefix: "foo", path: "../../", exp: true}, // at alloc-id/

		// under alloc-dir/alloc-id/foo/bar/
		{prefix: "foo/bar", path: "", exp: false},
		{prefix: "foo/bar", path: "/foo", exp: false},
		{prefix: "foo/bar", path: "../", exp: false},      // at bar/
		{prefix: "foo/bar", path: "../../", exp: false},   // at foo/
		{prefix: "foo/bar", path: "../../../", exp: true}, // at alloc-id/
	} {
		result, err := PathEscapesAllocViaRelative(test.prefix, test.path)
		require.NoError(t, err)
		require.Equal(t, test.exp, result)
	}
}

func Test_pathEscapesBaseViaSymlink(t *testing.T) {
	t.Run("symlink-escape", func(t *testing.T) {
		dir := setup(t)
		defer cleanup(t, dir)

		// link from dir/link
		link := filepath.Join(dir, "link")

		// link to /tmp
		target := filepath.Clean("/tmp")
		err := os.Symlink(target, link)
		require.NoError(t, err)

		escape, err := pathEscapesBaseViaSymlink(dir, link)
		require.NoError(t, err)
		require.True(t, escape)
	})

	t.Run("symlink-noescape", func(t *testing.T) {
		dir := setup(t)
		defer cleanup(t, dir)

		// create a file within dir
		target := filepath.Join(dir, "foo")
		write(t, target, "hi")

		// link to file within dir
		link := filepath.Join(dir, "link")
		err := os.Symlink(target, link)
		require.NoError(t, err)

		// link to file within dir does not escape dir
		escape, err := pathEscapesBaseViaSymlink(dir, link)
		require.NoError(t, err)
		require.False(t, escape)
	})
}

func Test_PathEscapesAllocDir(t *testing.T) {

	t.Run("no-escape-root", func(t *testing.T) {
		dir := setup(t)
		defer cleanup(t, dir)

		escape, err := PathEscapesAllocDir(dir, "", "/")
		require.NoError(t, err)
		require.False(t, escape)
	})

	t.Run("no-escape", func(t *testing.T) {
		dir := setup(t)
		defer cleanup(t, dir)

		write(t, filepath.Join(dir, "foo"), "hi")

		escape, err := PathEscapesAllocDir(dir, "", "/foo")
		require.NoError(t, err)
		require.False(t, escape)
	})

	t.Run("no-escape-no-exist", func(t *testing.T) {
		dir := setup(t)
		defer cleanup(t, dir)

		escape, err := PathEscapesAllocDir(dir, "", "/no-exist")
		require.NoError(t, err)
		require.False(t, escape)
	})

	t.Run("symlink-escape", func(t *testing.T) {
		dir := setup(t)
		defer cleanup(t, dir)

		// link from dir/link
		link := filepath.Join(dir, "link")

		// link to /tmp
		target := filepath.Clean("/tmp")
		err := os.Symlink(target, link)
		require.NoError(t, err)

		escape, err := PathEscapesAllocDir(dir, "", "/link")
		require.NoError(t, err)
		require.True(t, escape)
	})

	t.Run("relative-escape", func(t *testing.T) {
		dir := setup(t)
		defer cleanup(t, dir)

		escape, err := PathEscapesAllocDir(dir, "", "../../foo")
		require.NoError(t, err)
		require.True(t, escape)
	})

	t.Run("relative-escape-prefix", func(t *testing.T) {
		dir := setup(t)
		defer cleanup(t, dir)

		escape, err := PathEscapesAllocDir(dir, "/foo/bar", "../../../foo")
		require.NoError(t, err)
		require.True(t, escape)
	})
}
