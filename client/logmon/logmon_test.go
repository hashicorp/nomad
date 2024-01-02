// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package logmon

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/lib/fifo"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func TestLogmon_Start_rotate(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)
	var stdoutFifoPath, stderrFifoPath string

	dir := t.TempDir()

	if runtime.GOOS == "windows" {
		stdoutFifoPath = "//./pipe/test-rotate.stdout"
		stderrFifoPath = "//./pipe/test-rotate.stderr"
	} else {
		stdoutFifoPath = filepath.Join(dir, "stdout.fifo")
		stderrFifoPath = filepath.Join(dir, "stderr.fifo")
	}

	cfg := &LogConfig{
		LogDir:        dir,
		StdoutLogFile: "stdout",
		StdoutFifo:    stdoutFifoPath,
		StderrLogFile: "stderr",
		StderrFifo:    stderrFifoPath,
		MaxFiles:      2,
		MaxFileSizeMB: 1,
	}

	lm := NewLogMon(testlog.HCLogger(t))
	require.NoError(lm.Start(cfg))

	stdout, err := fifo.OpenWriter(stdoutFifoPath)
	require.NoError(err)

	// Write enough bytes such that the log is rotated
	bytes1MB := make([]byte, 1024*1024)
	_, err = rand.Read(bytes1MB)
	require.NoError(err)

	_, err = stdout.Write(bytes1MB)
	require.NoError(err)

	testutil.WaitForResult(func() (bool, error) {
		_, err = os.Stat(filepath.Join(dir, "stdout.0"))
		return err == nil, err
	}, func(err error) {
		require.NoError(err)
	})
	testutil.WaitForResult(func() (bool, error) {
		_, err = os.Stat(filepath.Join(dir, "stdout.1"))
		return err == nil, err
	}, func(err error) {
		require.NoError(err)
	})
	_, err = os.Stat(filepath.Join(dir, "stdout.2"))
	require.Error(err)
	require.NoError(lm.Stop())
	require.NoError(lm.Stop())
}

// asserts that calling Start twice restarts the log rotator and that any logs
// published while the listener was unavailable are received.
func TestLogmon_Start_restart_flusheslogs(t *testing.T) {
	ci.Parallel(t)

	if runtime.GOOS == "windows" {
		t.Skip("windows does not support pushing data to a pipe with no servers")
	}

	require := require.New(t)
	var stdoutFifoPath, stderrFifoPath string

	dir := t.TempDir()

	if runtime.GOOS == "windows" {
		stdoutFifoPath = "//./pipe/test-restart.stdout"
		stderrFifoPath = "//./pipe/test-restart.stderr"
	} else {
		stdoutFifoPath = filepath.Join(dir, "stdout.fifo")
		stderrFifoPath = filepath.Join(dir, "stderr.fifo")
	}

	cfg := &LogConfig{
		LogDir:        dir,
		StdoutLogFile: "stdout",
		StdoutFifo:    stdoutFifoPath,
		StderrLogFile: "stderr",
		StderrFifo:    stderrFifoPath,
		MaxFiles:      2,
		MaxFileSizeMB: 1,
	}

	lm := NewLogMon(testlog.HCLogger(t))
	impl, ok := lm.(*logmonImpl)
	require.True(ok)
	require.NoError(lm.Start(cfg))

	stdout, err := fifo.OpenWriter(stdoutFifoPath)
	require.NoError(err)
	stderr, err := fifo.OpenWriter(stderrFifoPath)
	require.NoError(err)

	// Write a string and assert it was written to the file
	_, err = stdout.Write([]byte("test\n"))
	require.NoError(err)

	testutil.WaitForResult(func() (bool, error) {
		raw, err := os.ReadFile(filepath.Join(dir, "stdout.0"))
		if err != nil {
			return false, err
		}
		return "test\n" == string(raw), fmt.Errorf("unexpected stdout %q", string(raw))
	}, func(err error) {
		require.NoError(err)
	})
	require.True(impl.tl.IsRunning())

	// Close stdout and assert that logmon no longer writes to the file
	require.NoError(stdout.Close())
	require.NoError(stderr.Close())

	testutil.WaitForResult(func() (bool, error) {
		return !impl.tl.IsRunning(), fmt.Errorf("logmon is still running")
	}, func(err error) {
		require.NoError(err)
	})

	stdout, err = fifo.OpenWriter(stdoutFifoPath)
	require.NoError(err)
	stderr, err = fifo.OpenWriter(stderrFifoPath)
	require.NoError(err)

	_, err = stdout.Write([]byte("te"))
	require.NoError(err)

	testutil.WaitForResult(func() (bool, error) {
		raw, err := os.ReadFile(filepath.Join(dir, "stdout.0"))
		if err != nil {
			return false, err
		}
		return "test\n" == string(raw), fmt.Errorf("unexpected stdout %q", string(raw))
	}, func(err error) {
		require.NoError(err)
	})

	// Start logmon again and assert that it appended to the file
	require.NoError(lm.Start(cfg))

	stdout, err = fifo.OpenWriter(stdoutFifoPath)
	require.NoError(err)
	stderr, err = fifo.OpenWriter(stderrFifoPath)
	require.NoError(err)

	_, err = stdout.Write([]byte("st\n"))
	require.NoError(err)
	testutil.WaitForResult(func() (bool, error) {
		raw, err := os.ReadFile(filepath.Join(dir, "stdout.0"))
		if err != nil {
			return false, err
		}

		expected := "test\ntest\n" == string(raw)
		return expected, fmt.Errorf("unexpected stdout %q", string(raw))
	}, func(err error) {
		require.NoError(err)
	})
}

// asserts that calling Start twice restarts the log rotator
func TestLogmon_Start_restart(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)
	var stdoutFifoPath, stderrFifoPath string

	dir := t.TempDir()

	if runtime.GOOS == "windows" {
		stdoutFifoPath = "//./pipe/test-restart.stdout"
		stderrFifoPath = "//./pipe/test-restart.stderr"
	} else {
		stdoutFifoPath = filepath.Join(dir, "stdout.fifo")
		stderrFifoPath = filepath.Join(dir, "stderr.fifo")
	}

	cfg := &LogConfig{
		LogDir:        dir,
		StdoutLogFile: "stdout",
		StdoutFifo:    stdoutFifoPath,
		StderrLogFile: "stderr",
		StderrFifo:    stderrFifoPath,
		MaxFiles:      2,
		MaxFileSizeMB: 1,
	}

	lm := NewLogMon(testlog.HCLogger(t))
	impl, ok := lm.(*logmonImpl)
	require.True(ok)
	require.NoError(lm.Start(cfg))
	t.Cleanup(func() {
		require.NoError(lm.Stop())
	})

	stdout, err := fifo.OpenWriter(stdoutFifoPath)
	require.NoError(err)
	stderr, err := fifo.OpenWriter(stderrFifoPath)
	require.NoError(err)

	// Write a string and assert it was written to the file
	_, err = stdout.Write([]byte("test\n"))
	require.NoError(err)

	testutil.WaitForResult(func() (bool, error) {
		raw, err := os.ReadFile(filepath.Join(dir, "stdout.0"))
		if err != nil {
			return false, err
		}
		return "test\n" == string(raw), fmt.Errorf("unexpected stdout %q", string(raw))
	}, func(err error) {
		require.NoError(err)
	})
	require.True(impl.tl.IsRunning())

	// Close stderr and assert that logmon no longer writes to the file
	// Keep stdout open to ensure that IsRunning requires both
	require.NoError(stderr.Close())

	testutil.WaitForResult(func() (bool, error) {
		return !impl.tl.IsRunning(), fmt.Errorf("logmon is still running")
	}, func(err error) {
		require.NoError(err)
	})

	// Start logmon again and assert that it can receive logs again
	require.NoError(lm.Start(cfg))

	stdout, err = fifo.OpenWriter(stdoutFifoPath)
	require.NoError(err)
	t.Cleanup(func() {
		require.NoError(stdout.Close())
	})

	stderr, err = fifo.OpenWriter(stderrFifoPath)
	require.NoError(err)
	t.Cleanup(func() {
		require.NoError(stderr.Close())
	})

	_, err = stdout.Write([]byte("test\n"))
	require.NoError(err)
	testutil.WaitForResult(func() (bool, error) {
		raw, err := os.ReadFile(filepath.Join(dir, "stdout.0"))
		if err != nil {
			return false, err
		}

		expected := "test\ntest\n" == string(raw)
		return expected, fmt.Errorf("unexpected stdout %q", string(raw))
	}, func(err error) {
		require.NoError(err)
	})
}

// panicWriter panics on use
type panicWriter struct{}

func (panicWriter) Write([]byte) (int, error) {
	panic("should not be called")
}
func (panicWriter) Close() error {
	panic("should not be called")
}

// TestLogmon_NewError asserts that newLogRotatorWrapper will return an error
// if its unable to create the necessray files.
func TestLogmon_NewError(t *testing.T) {
	ci.Parallel(t)

	// Pick a path that does not exist
	path := filepath.Join(uuid.Generate(), uuid.Generate(), uuid.Generate())

	logger := testlog.HCLogger(t)

	// No code that uses the writer should get hit
	rotator := panicWriter{}

	w, err := newLogRotatorWrapper(path, logger, rotator)
	require.Error(t, err)
	require.Nil(t, w)
}
