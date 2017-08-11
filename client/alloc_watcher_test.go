package client

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/mock"
)

// TestPrevAlloc_LocalPrevAlloc asserts that when a previous alloc runner is
// set a localPrevAlloc will block on it.
func TestPrevAlloc_LocalPrevAlloc(t *testing.T) {
	_, prevAR := testAllocRunner(false)
	prevAR.alloc.Job.TaskGroups[0].Tasks[0].Config["run_for"] = "10s"

	newAlloc := mock.Alloc()
	newAlloc.PreviousAllocation = prevAR.Alloc().ID
	newAlloc.Job.TaskGroups[0].EphemeralDisk.Sticky = false
	task := newAlloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config["run_for"] = "500ms"

	waiter := newAllocWatcher(newAlloc, prevAR, nil, nil, testLogger())

	// Wait in a goroutine with a context to make sure it exits at the right time
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		defer cancel()
		waiter.Wait(ctx)
	}()

	select {
	case <-ctx.Done():
		t.Fatalf("Wait exited too early")
	case <-time.After(33 * time.Millisecond):
		// Good! It's blocking
	}

	// Start the previous allocs to cause it to update but not terminate
	go prevAR.Run()
	defer prevAR.Destroy()

	select {
	case <-ctx.Done():
		t.Fatalf("Wait exited too early")
	case <-time.After(33 * time.Millisecond):
		// Good! It's still blocking
	}

	// Stop the previous alloc
	prevAR.Destroy()

	select {
	case <-ctx.Done():
		// Good! We unblocked when the previous alloc stopped
	case <-time.After(time.Second):
		t.Fatalf("Wait exited too early")
	}
}

// TestPrevAlloc_StreamAllocDir asserts that streaming a tar to an alloc dir
// works.
func TestPrevAlloc_StreamAllocDir(t *testing.T) {
	t.Parallel()
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(dir)

	if err := os.Mkdir(filepath.Join(dir, "foo"), 0777); err != nil {
		t.Fatalf("err: %v", err)
	}
	dirInfo, err := os.Stat(filepath.Join(dir, "foo"))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	f, err := os.Create(filepath.Join(dir, "foo", "bar"))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, err := f.WriteString("foo"); err != nil {
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

	dir1, err := ioutil.TempDir("", "nomadtest-")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(dir1)

	c1 := testClient(t, func(c *config.Config) {
		c.RPCHandler = nil
	})
	defer c1.Shutdown()

	rc := ioutil.NopCloser(buf)

	prevAlloc := &remotePrevAlloc{logger: testLogger()}
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
