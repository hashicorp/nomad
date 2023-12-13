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
)

// TestPrevAlloc_StreamAllocDir_Ok asserts that streaming a tar to an alloc dir
// works.
func TestPrevAlloc_StreamAllocDir_Ok(t *testing.T) {
	ci.Parallel(t)
	ctestutil.RequireRoot(t)

	dir := t.TempDir()

	// Create foo/
	fooDir := filepath.Join(dir, "foo")
	if err := os.Mkdir(fooDir, 0777); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Change ownership of foo/ to test #3702 (any non-root user is fine)
	const uid, gid = 1, 1
	if err := os.Chown(fooDir, uid, gid); err != nil {
		t.Fatalf("err : %v", err)
	}

	dirInfo, err := os.Stat(fooDir)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create foo/bar
	f, err := os.Create(filepath.Join(fooDir, "bar"))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, err := f.WriteString("123"); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := f.Chmod(0644); err != nil {
		t.Fatalf("err: %v", err)
	}
	fInfo, err := f.Stat()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	f.Close()

	// Create foo/baz -> bar symlink
	if err := os.Symlink("bar", filepath.Join(dir, "foo", "baz")); err != nil {
		t.Fatalf("err: %v", err)
	}
	linkInfo, err := os.Lstat(filepath.Join(dir, "foo", "baz"))
	if err != nil {
		t.Fatalf("err: %v", err)
	}

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
		t.Fatalf("err: %v", err)
	}
	tw.Close()

	dir1 := t.TempDir()

	rc := io.NopCloser(buf)
	prevAlloc := &remotePrevAlloc{logger: testlog.HCLogger(t)}
	if err := prevAlloc.streamAllocDir(context.Background(), rc, dir1); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure foo is present
	fi, err := os.Stat(filepath.Join(dir1, "foo"))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if fi.Mode() != dirInfo.Mode() {
		t.Fatalf("mode: %v", fi.Mode())
	}
	stat := fi.Sys().(*syscall.Stat_t)
	if stat.Uid != uid || stat.Gid != gid {
		t.Fatalf("foo/ has incorrect ownership: expected %d:%d found %d:%d",
			uid, gid, stat.Uid, stat.Gid)
	}

	fi1, err := os.Stat(filepath.Join(dir1, "bar"))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if fi1.Mode() != fInfo.Mode() {
		t.Fatalf("mode: %v", fi1.Mode())
	}

	fi2, err := os.Lstat(filepath.Join(dir1, "baz"))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if fi2.Mode() != linkInfo.Mode() {
		t.Fatalf("mode: %v", fi2.Mode())
	}
}
