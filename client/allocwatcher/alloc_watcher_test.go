// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocwatcher

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocdir"
	cstructs "github.com/hashicorp/nomad/client/structs"
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

	path := t.TempDir()

	return &fakeAllocRunner{
		alloc:       alloc,
		AllocDir:    allocdir.NewAllocDir(logger, path, alloc.ID),
		Broadcaster: cstructs.NewAllocBroadcaster(logger),
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
	alloc.Job.TaskGroups[0].Tasks[0].Driver = "mock_driver"

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
	ci.Parallel(t)

	conf, cleanup := newConfig(t)
	defer cleanup()

	conf.Alloc.PreviousAllocation = ""

	watcher, migrator := NewAllocWatcher(conf)
	require.NotNil(t, watcher)
	_, ok := migrator.(NoopPrevAlloc)
	require.True(t, ok, "expected migrator to be NoopPrevAlloc")

	done := make(chan int, 2)
	go func() {
		watcher.Wait(context.Background())
		done <- 1
		migrator.Migrate(context.Background(), nil)
		done <- 1
	}()
	require.False(t, watcher.IsWaiting())
	require.False(t, migrator.IsMigrating())
	<-done
	<-done
}

// TestPrevAlloc_LocalPrevAlloc_Block asserts that when a previous alloc runner
// is set a localPrevAlloc will block on it.
func TestPrevAlloc_LocalPrevAlloc_Block(t *testing.T) {
	ci.Parallel(t)

	conf, cleanup := newConfig(t)

	defer cleanup()

	conf.Alloc.Job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for": "500ms",
	}

	_, waiter := NewAllocWatcher(conf)

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

// TestPrevAlloc_LocalPrevAlloc_Terminated asserts that when a previous alloc
// runner has already terminated the watcher does not block on the broadcaster.
func TestPrevAlloc_LocalPrevAlloc_Terminated(t *testing.T) {
	ci.Parallel(t)

	conf, cleanup := newConfig(t)
	defer cleanup()

	conf.PreviousRunner.Alloc().ClientStatus = structs.AllocClientStatusComplete

	waiter, _ := NewAllocWatcher(conf)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Since prev alloc is terminal, Wait should exit immediately with no
	// context error
	require.NoError(t, waiter.Wait(ctx))
}

// TestPrevAlloc_StreamAllocDir_Error asserts that errors encountered while
// streaming a tar cause the migration to be cancelled and no files are written
// (migrations are atomic).
func TestPrevAlloc_StreamAllocDir_Error(t *testing.T) {
	ci.Parallel(t)

	dest := t.TempDir()

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
	err := tw.WriteHeader(&fooHdr)
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
	err = prevAlloc.streamAllocDir(context.Background(), io.NopCloser(tarBuf), dest)
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
