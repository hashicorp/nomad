package allocwatcher

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocdir"
	cstructs "github.com/hashicorp/nomad/client/structs"
	ctestutil "github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

// fakeAllocRunner implements AllocRunnerMeta
type fakeAllocRunner struct {
	alloc       *structs.Allocation
	AllocDir    *allocdir.AllocDir
	Broadcaster *cstructs.AllocBroadcaster
}

// newFakeAllocRunner creates a new AllocRunnerMeta. Callers must call
// AllocDir.Destroy() when finished.
func newFakeAllocRunner(t *testing.T, logger hclog.Logger) *fakeAllocRunner {
	alloc := mock.Alloc()
	alloc.Job.TaskGroups[0].EphemeralDisk.Sticky = true
	alloc.Job.TaskGroups[0].EphemeralDisk.Migrate = true

	path, err := ioutil.TempDir("", "nomad_test_wathcer")
	require.NoError(t, err)

	return &fakeAllocRunner{
		alloc:       alloc,
		AllocDir:    allocdir.NewAllocDir(logger, path),
		Broadcaster: cstructs.NewAllocBroadcaster(alloc),
	}
}

func (f *fakeAllocRunner) GetAllocDir() *allocdir.AllocDir {
	return f.AllocDir
}

func (f *fakeAllocRunner) Listener() *cstructs.AllocListener {
	return f.Broadcaster.Listen()
}

func (f *fakeAllocRunner) Alloc() *structs.Allocation {
	return f.alloc
}

// newConfig returns a new Config and cleanup func
func newConfig(t *testing.T) (Config, func()) {
	logger := testlog.HCLogger(t)

	prevAR := newFakeAllocRunner(t, logger)

	alloc := mock.Alloc()
	alloc.PreviousAllocation = prevAR.Alloc().ID
	alloc.Job.TaskGroups[0].EphemeralDisk.Sticky = true
	alloc.Job.TaskGroups[0].EphemeralDisk.Migrate = true

	config := Config{
		Alloc:          alloc,
		PreviousRunner: prevAR,
		RPC:            nil,
		Config:         nil,
		MigrateToken:   "fake_token",
		Logger:         logger,
	}

	cleanup := func() {
		prevAR.AllocDir.Destroy()
	}

	return config, cleanup
}

// TestPrevAlloc_Noop asserts that when no previous allocation is set the noop
// implementation is returned that does not block or perform migrations.
func TestPrevAlloc_Noop(t *testing.T) {
	conf, cleanup := newConfig(t)
	defer cleanup()

	conf.Alloc.PreviousAllocation = ""

	watcher := NewAllocWatcher(conf)
	require.NotNil(t, watcher)
	_, ok := watcher.(NoopPrevAlloc)
	require.True(t, ok, "expected watcher to be NoopPrevAlloc")

	done := make(chan int, 2)
	go func() {
		watcher.Wait(context.Background())
		done <- 1
		watcher.Migrate(context.Background(), nil)
		done <- 1
	}()
	require.False(t, watcher.IsWaiting())
	require.False(t, watcher.IsMigrating())
	<-done
	<-done
}

// TestPrevAlloc_LocalPrevAlloc asserts that when a previous alloc runner is
// set a localPrevAlloc will block on it.
func TestPrevAlloc_LocalPrevAlloc(t *testing.T) {
	t.Parallel()
	conf, cleanup := newConfig(t)

	defer cleanup()

	conf.Alloc.Job.TaskGroups[0].Tasks[0].Driver = "mock_driver"
	conf.Alloc.Job.TaskGroups[0].Tasks[0].Config["run_for"] = "500ms"

	waiter := NewAllocWatcher(conf)

	// Wait in a goroutine with a context to make sure it exits at the right time
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		defer cancel()
		waiter.Wait(ctx)
	}()

	// Assert watcher is waiting
	testutil.WaitForResult(func() (bool, error) {
		return waiter.IsWaiting(), fmt.Errorf("expected watcher to be waiting")
	}, func(err error) {
		t.Fatalf("error: %v", err)
	})

	// Broadcast a non-terminal alloc update to assert only terminal
	// updates break out of waiting.
	update := conf.PreviousRunner.Alloc().Copy()
	update.DesiredStatus = structs.AllocDesiredStatusStop
	update.ModifyIndex++
	update.AllocModifyIndex++

	broadcaster := conf.PreviousRunner.(*fakeAllocRunner).Broadcaster
	err := broadcaster.Send(update)
	require.NoError(t, err)

	// Assert watcher is still waiting because alloc isn't terminal
	testutil.WaitForResult(func() (bool, error) {
		return waiter.IsWaiting(), fmt.Errorf("expected watcher to be waiting")
	}, func(err error) {
		t.Fatalf("error: %v", err)
	})

	// Stop the previous alloc and assert watcher stops blocking
	update = update.Copy()
	update.DesiredStatus = structs.AllocDesiredStatusStop
	update.ClientStatus = structs.AllocClientStatusComplete
	update.ModifyIndex++
	update.AllocModifyIndex++

	err = broadcaster.Send(update)
	require.NoError(t, err)

	testutil.WaitForResult(func() (bool, error) {
		if waiter.IsWaiting() {
			return false, fmt.Errorf("did not expect watcher to be waiting")
		}
		return !waiter.IsMigrating(), fmt.Errorf("did not expect watcher to be migrating")
	}, func(err error) {
		t.Fatalf("error: %v", err)
	})
}

// TestPrevAlloc_StreamAllocDir_Ok asserts that streaming a tar to an alloc dir
// works.
func TestPrevAlloc_StreamAllocDir_Ok(t *testing.T) {
	ctestutil.RequireRoot(t)
	t.Parallel()
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(dir)

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

	rc := ioutil.NopCloser(buf)
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

// TestPrevAlloc_StreamAllocDir_Error asserts that errors encountered while
// streaming a tar cause the migration to be cancelled and no files are written
// (migrations are atomic).
func TestPrevAlloc_StreamAllocDir_Error(t *testing.T) {
	t.Parallel()
	dest, err := ioutil.TempDir("", "nomadtest-")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(dest)

	// This test only unit tests streamAllocDir so we only need a partially
	// complete remotePrevAlloc
	prevAlloc := &remotePrevAlloc{
		logger:      testlog.HCLogger(t),
		allocID:     "123",
		prevAllocID: "abc",
		migrate:     true,
	}

	tarBuf := bytes.NewBuffer(nil)
	tw := tar.NewWriter(tarBuf)
	fooHdr := tar.Header{
		Name:     "foo.txt",
		Mode:     0666,
		Size:     1,
		ModTime:  time.Now(),
		Typeflag: tar.TypeReg,
	}
	err = tw.WriteHeader(&fooHdr)
	if err != nil {
		t.Fatalf("error writing file header: %v", err)
	}
	if _, err := tw.Write([]byte{'a'}); err != nil {
		t.Fatalf("error writing file: %v", err)
	}

	// Now write the error file
	contents := []byte("SENTINEL ERROR")
	err = tw.WriteHeader(&tar.Header{
		Name:       allocdir.SnapshotErrorFilename(prevAlloc.prevAllocID),
		Mode:       0666,
		Size:       int64(len(contents)),
		AccessTime: allocdir.SnapshotErrorTime,
		ChangeTime: allocdir.SnapshotErrorTime,
		ModTime:    allocdir.SnapshotErrorTime,
		Typeflag:   tar.TypeReg,
	})
	if err != nil {
		t.Fatalf("error writing sentinel file header: %v", err)
	}
	if _, err := tw.Write(contents); err != nil {
		t.Fatalf("error writing sentinel file: %v", err)
	}

	// Assert streamAllocDir fails
	err = prevAlloc.streamAllocDir(context.Background(), ioutil.NopCloser(tarBuf), dest)
	if err == nil {
		t.Fatalf("expected an error from streamAllocDir")
	}
	if !strings.HasSuffix(err.Error(), string(contents)) {
		t.Fatalf("expected error to end with %q but found: %v", string(contents), err)
	}

	// streamAllocDir leaves cleanup to the caller on error, so assert
	// "foo.txt" was written
	fi, err := os.Stat(filepath.Join(dest, "foo.txt"))
	if err != nil {
		t.Fatalf("error reading foo.txt: %v", err)
	}
	if fi.Size() != fooHdr.Size {
		t.Fatalf("expected foo.txt to be size 1 but found %d", fi.Size())
	}
}
