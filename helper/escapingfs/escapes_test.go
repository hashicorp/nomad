// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package escapingfs

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func write(t *testing.T, file, data string) {
	err := os.WriteFile(file, []byte(data), 0600)
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
		dir := t.TempDir()

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
		dir := t.TempDir()

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
		dir := t.TempDir()

		escape, err := PathEscapesAllocDir(dir, "", "/")
		require.NoError(t, err)
		require.False(t, escape)
	})

	t.Run("no-escape", func(t *testing.T) {
		dir := t.TempDir()

		write(t, filepath.Join(dir, "foo"), "hi")

		escape, err := PathEscapesAllocDir(dir, "", "/foo")
		require.NoError(t, err)
		require.False(t, escape)
	})

	t.Run("no-escape-no-exist", func(t *testing.T) {
		dir := t.TempDir()

		escape, err := PathEscapesAllocDir(dir, "", "/no-exist")
		require.NoError(t, err)
		require.False(t, escape)
	})

	t.Run("symlink-escape", func(t *testing.T) {
		dir := t.TempDir()

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
		dir := t.TempDir()

		escape, err := PathEscapesAllocDir(dir, "", "../../foo")
		require.NoError(t, err)
		require.True(t, escape)
	})

	t.Run("relative-escape-prefix", func(t *testing.T) {
		dir := t.TempDir()

		escape, err := PathEscapesAllocDir(dir, "/foo/bar", "../../../foo")
		require.NoError(t, err)
		require.True(t, escape)
	})
}

func TestPathEscapesSandbox(t *testing.T) {
	cases := []struct {
		name     string
		path     string
		dir      string
		expected bool
	}{
		{
			// this is the ${NOMAD_SECRETS_DIR} case
			name:     "ok joined absolute path inside sandbox",
			path:     filepath.Join("/alloc", "/secrets"),
			dir:      "/alloc",
			expected: false,
		},
		{
			name:     "fail unjoined absolute path outside sandbox",
			path:     "/secrets",
			dir:      "/alloc",
			expected: true,
		},
		{
			name:     "ok joined relative path inside sandbox",
			path:     filepath.Join("/alloc", "./safe"),
			dir:      "/alloc",
			expected: false,
		},
		{
			name:     "fail unjoined relative path outside sandbox",
			path:     "./safe",
			dir:      "/alloc",
			expected: true,
		},
		{
			name:     "ok relative path traversal constrained to sandbox",
			path:     filepath.Join("/alloc", "../../alloc/safe"),
			dir:      "/alloc",
			expected: false,
		},
		{
			name:     "ok unjoined absolute path traversal constrained to sandbox",
			path:     filepath.Join("/alloc", "/../alloc/safe"),
			dir:      "/alloc",
			expected: false,
		},
		{
			name:     "ok unjoined absolute path traversal constrained to sandbox",
			path:     "/../alloc/safe",
			dir:      "/alloc",
			expected: false,
		},
		{
			name:     "fail joined relative path traverses outside sandbox",
			path:     filepath.Join("/alloc", "../../../unsafe"),
			dir:      "/alloc",
			expected: true,
		},
		{
			name:     "fail unjoined relative path traverses outside sandbox",
			path:     "../../../unsafe",
			dir:      "/alloc",
			expected: true,
		},
		{
			name:     "fail joined absolute path tries to transverse outside sandbox",
			path:     filepath.Join("/alloc", "/alloc/../../unsafe"),
			dir:      "/alloc",
			expected: true,
		},
		{
			name:     "fail unjoined absolute path tries to transverse outside sandbox",
			path:     "/alloc/../../unsafe",
			dir:      "/alloc",
			expected: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			caseMsg := fmt.Sprintf("path: %v\ndir: %v", tc.path, tc.dir)
			escapes := PathEscapesSandbox(tc.dir, tc.path)
			require.Equal(t, tc.expected, escapes, caseMsg)
		})
	}
}
