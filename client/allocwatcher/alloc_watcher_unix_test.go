// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !windows
// +build !windows

package allocwatcher

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/hashicorp/nomad/ci"
	ctestutil "github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/shoenig/test/must"
)

// TestPrevAlloc_StreamAllocDir_Ok asserts that streaming a tar to an alloc dir
// works.
func TestPrevAlloc_StreamAllocDir_Ok(t *testing.T) {
	ci.Parallel(t)
	ctestutil.RequireRoot(t)

	dir := t.TempDir()

	// Create foo/
	fooDir := filepath.Join(dir, "foo")
	must.NoError(t, os.Mkdir(fooDir, 0777))

	// Change ownership of foo/ to test #3702 (any non-root user is fine)
	const uid, gid = 1, 1
	must.NoError(t, os.Chown(fooDir, uid, gid))

	dirInfo, err := os.Stat(fooDir)
	must.NoError(t, err)

	// Create foo/bar
	f, err := os.Create(filepath.Join(fooDir, "bar"))
	must.NoError(t, err)

	_, err = f.WriteString("123")
	must.NoError(t, err)

	err = f.Chmod(0644)
	must.NoError(t, err)

	fInfo, err := f.Stat()
	must.NoError(t, err)
	f.Close()

	// Create foo/baz -> bar symlink
	err = os.Symlink("bar", filepath.Join(dir, "foo", "baz"))
	must.NoError(t, err)

	linkInfo, err := os.Lstat(filepath.Join(dir, "foo", "baz"))
	must.NoError(t, err)

	buf, err := testTar(dir)

	dir1 := t.TempDir()

	rc := io.NopCloser(buf)
	prevAlloc := &remotePrevAlloc{logger: testlog.HCLogger(t)}
	err = prevAlloc.streamAllocDir(context.Background(), rc, dir1)
	must.NoError(t, err)

	// Ensure foo is present
	fi, err := os.Stat(filepath.Join(dir1, "foo"))
	must.NoError(t, err)
	must.Eq(t, dirInfo.Mode(), fi.Mode(), must.Sprintf("unexpected file mode"))

	stat := fi.Sys().(*syscall.Stat_t)
	if stat.Uid != uid || stat.Gid != gid {
		t.Fatalf("foo/ has incorrect ownership: expected %d:%d found %d:%d",
			uid, gid, stat.Uid, stat.Gid)
	}

	fi1, err := os.Stat(filepath.Join(dir1, "bar"))
	must.NoError(t, err)
	must.Eq(t, fInfo.Mode(), fi1.Mode(), must.Sprintf("unexpected file mode"))

	fi2, err := os.Lstat(filepath.Join(dir1, "baz"))
	must.NoError(t, err)
	must.Eq(t, linkInfo.Mode(), fi2.Mode(), must.Sprintf("unexpected file mode"))
}

func TestPrevAlloc_StreamAllocDir_BadSymlink(t *testing.T) {
	ci.Parallel(t)

	dir := t.TempDir()
	sensitiveDir := t.TempDir()

	fooDir := filepath.Join(dir, "foo")
	err := os.Mkdir(fooDir, 0777)
	must.NoError(t, err)

	// Create sensitive -> foo/bar symlink
	err = os.Symlink(sensitiveDir, filepath.Join(dir, "foo", "baz"))
	must.NoError(t, err)

	buf, err := testTar(dir)
	rc := io.NopCloser(buf)

	dir1 := t.TempDir()
	prevAlloc := &remotePrevAlloc{logger: testlog.HCLogger(t)}
	err = prevAlloc.streamAllocDir(context.Background(), rc, dir1)
	must.EqError(t, err, "archive contains symlink that escapes alloc dir")
}

func TestPrevAlloc_StreamAllocDir_BadSymlink_Linkname(t *testing.T) {
	ci.Parallel(t)

	// Create a tar archive with a symlink that attempts to escape the allocation directory
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	t.Cleanup(func() { tw.Close() })
	must.NoError(t, tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeSymlink,
		Name:     "symlink",
		Linkname: "../escape_attempt",
		Mode:     0600,
	}))

	newDir := t.TempDir()
	prevAlloc := &remotePrevAlloc{logger: testlog.HCLogger(t)}
	err := prevAlloc.streamAllocDir(context.Background(), io.NopCloser(&buf), newDir)

	must.EqError(t, err, "archive contains symlink that escapes alloc dir")
}

func TestPrevAlloc_StreamAllocDir_SyminkWriteAttack(t *testing.T) {
	ci.Parallel(t)

	tmpDir := t.TempDir()
	outsidePath := filepath.Join(tmpDir, "outside")
	insidePath := "malformed_link"
	content := "HelloWorld from outside"

	// Create a tar archive with a symlink that attempts to escape the allocation directory
	// by including a header that writes to the same path and follows the symlink target
	// outside of the sandboxed environment.
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	t.Cleanup(func() { tw.Close() })
	must.NoError(t, tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeSymlink,
		Name:     insidePath,
		Linkname: outsidePath,
		Mode:     0600,
	}))
	must.NoError(t, tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeReg,
		Name:     insidePath,
		Size:     int64(len(content)),
		Mode:     0600,
	}))
	_, err := tw.Write([]byte(content))
	must.NoError(t, err)

	newDir := t.TempDir()
	prevAlloc := &remotePrevAlloc{logger: testlog.HCLogger(t)}
	err = prevAlloc.streamAllocDir(context.Background(), io.NopCloser(&buf), newDir)

	// No error expected
	must.NoError(t, err)

	// Check if the symlink target outside the alloc dir has not been written
	_, err = os.Stat(outsidePath)
	must.EqError(t, err, "stat "+outsidePath+": no such file or directory")

	// Check if the symlink inside the alloc dir has been written
	_, err = os.Stat(filepath.Join(newDir, insidePath))
	must.NoError(t, err)

	// Check if the content of the file inside the alloc dir is correct
	contentBytes := make([]byte, len(content))
	f, err := os.Open(filepath.Join(newDir, insidePath))
	defer func() {
		must.NoError(t, f.Close())
	}()
	must.NoError(t, err)
	n, err := f.Read(contentBytes)
	must.NoError(t, err)
	must.Eq(t, content, string(contentBytes[:n]))
}

func testTar(dir string) (*bytes.Buffer, error) {

	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)

	walkFn := func(path string, fileInfo os.FileInfo, err error) error {
		// filepath.Walk passes in an error
		if err != nil {
			return fmt.Errorf("error from filepath.Walk(): %s", err)
		}
		// Include the path of the file name relative to the alloc dir
		// so that we can put the files in the right directories
		link := ""
		if fileInfo.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("error reading symlink: %v", err)
			}
			link = target
		}
		hdr, err := tar.FileInfoHeader(fileInfo, link)
		if err != nil {
			return fmt.Errorf("error creating file header: %v", err)
		}
		hdr.Name = fileInfo.Name()
		tw.WriteHeader(hdr)

		// If it's a directory or symlink we just write the header into the tar
		if fileInfo.IsDir() || (fileInfo.Mode()&os.ModeSymlink != 0) {
			return nil
		}

		// Write the file into the archive
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		if _, err := io.Copy(tw, file); err != nil {
			return err
		}

		return nil
	}

	if err := filepath.Walk(dir, walkFn); err != nil {
		return nil, err
	}
	tw.Close()

	return buf, nil
}
