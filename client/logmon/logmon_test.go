package logmon

import (
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/hashicorp/nomad/client/lib/fifo"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func TestLogmon_Start_rotate(t *testing.T) {
	var stdoutFifoPath, stderrFifoPath string

	dir, err := ioutil.TempDir("", "nomadtest")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

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
	require.NoError(t, lm.Start(cfg))

	stdout, err := fifo.OpenWriter(stdoutFifoPath)
	require.NoError(t, err)

	// Write enough bytes such that the log is rotated
	bytes1MB := make([]byte, 1024*1024)
	_, err = rand.Read(bytes1MB)
	require.NoError(t, err)

	_, err = stdout.Write(bytes1MB)
	require.NoError(t, err)

	testutil.WaitForResult(func() (bool, error) {
		_, err = os.Stat(filepath.Join(dir, "stdout.0"))
		return err == nil, err
	}, func(err error) {
		require.NoError(t, err)
	})
	testutil.WaitForResult(func() (bool, error) {
		_, err = os.Stat(filepath.Join(dir, "stdout.1"))
		return err == nil, err
	}, func(err error) {
		require.NoError(t, err)
	})
	_, err = os.Stat(filepath.Join(dir, "stdout.2"))
	require.Error(t, err)
	require.NoError(t, lm.Stop())
	require.NoError(t, lm.Stop())
}

// asserts that calling Start twice restarts the log rotator and that any logs
// published while the listener was unavailable are received.
func TestLogmon_Start_restart_flusheslogs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows does not support pushing data to a pipe with no servers")
	}

	var stdoutFifoPath, stderrFifoPath string

	dir, err := ioutil.TempDir("", "nomadtest")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

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
	require.True(t, ok)
	require.NoError(t, lm.Start(cfg))

	stdout, err := fifo.OpenWriter(stdoutFifoPath)
	require.NoError(t, err)
	stderr, err := fifo.OpenWriter(stderrFifoPath)
	require.NoError(t, err)

	// Write a string and assert it was written to the file
	_, err = stdout.Write([]byte("test\n"))
	require.NoError(t, err)

	testutil.WaitForResult(func() (bool, error) {
		raw, err := ioutil.ReadFile(filepath.Join(dir, "stdout.0"))
		if err != nil {
			return false, err
		}
		return "test\n" == string(raw), fmt.Errorf("unexpected stdout %q", string(raw))
	}, func(err error) {
		require.NoError(t, err)
	})
	require.True(t, impl.tl.IsRunning())

	// Close stdout and assert that logmon no longer writes to the file
	require.NoError(t, stdout.Close())
	require.NoError(t, stderr.Close())

	testutil.WaitForResult(func() (bool, error) {
		return !impl.tl.IsRunning(), fmt.Errorf("logmon is still running")
	}, func(err error) {
		require.NoError(t, err)
	})

	stdout, err = fifo.OpenWriter(stdoutFifoPath)
	require.NoError(t, err)
	stderr, err = fifo.OpenWriter(stderrFifoPath)
	require.NoError(t, err)

	_, err = stdout.Write([]byte("te"))
	require.NoError(t, err)

	testutil.WaitForResult(func() (bool, error) {
		raw, err := ioutil.ReadFile(filepath.Join(dir, "stdout.0"))
		if err != nil {
			return false, err
		}
		return "test\n" == string(raw), fmt.Errorf("unexpected stdout %q", string(raw))
	}, func(err error) {
		require.NoError(t, err)
	})

	// Start logmon again and assert that it appended to the file
	require.NoError(t, lm.Start(cfg))

	stdout, err = fifo.OpenWriter(stdoutFifoPath)
	require.NoError(t, err)
	stderr, err = fifo.OpenWriter(stderrFifoPath)
	require.NoError(t, err)

	_, err = stdout.Write([]byte("st\n"))
	require.NoError(t, err)
	testutil.WaitForResult(func() (bool, error) {
		raw, err := ioutil.ReadFile(filepath.Join(dir, "stdout.0"))
		if err != nil {
			return false, err
		}

		expected := "test\ntest\n" == string(raw)
		return expected, fmt.Errorf("unexpected stdout %q", string(raw))
	}, func(err error) {
		require.NoError(t, err)
	})
}

// asserts that calling Start twice restarts the log rotator
func TestLogmon_Start_restart(t *testing.T) {
	var stdoutFifoPath, stderrFifoPath string

	dir, err := ioutil.TempDir("", "nomadtest")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

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
	require.True(t, ok)
	require.NoError(t, lm.Start(cfg))

	stdout, err := fifo.OpenWriter(stdoutFifoPath)
	require.NoError(t, err)
	stderr, err := fifo.OpenWriter(stderrFifoPath)
	require.NoError(t, err)

	// Write a string and assert it was written to the file
	_, err = stdout.Write([]byte("test\n"))
	require.NoError(t, err)

	testutil.WaitForResult(func() (bool, error) {
		raw, err := ioutil.ReadFile(filepath.Join(dir, "stdout.0"))
		if err != nil {
			return false, err
		}
		return "test\n" == string(raw), fmt.Errorf("unexpected stdout %q", string(raw))
	}, func(err error) {
		require.NoError(t, err)
	})
	require.True(t, impl.tl.IsRunning())

	// Close stderr and assert that logmon no longer writes to the file
	// Keep stdout open to ensure that IsRunning requires both
	require.NoError(t, stderr.Close())

	testutil.WaitForResult(func() (bool, error) {
		return !impl.tl.IsRunning(), fmt.Errorf("logmon is still running")
	}, func(err error) {
		require.NoError(t, err)
	})

	// Start logmon again and assert that it can receive logs again
	require.NoError(t, lm.Start(cfg))

	stdout, err = fifo.OpenWriter(stdoutFifoPath)
	require.NoError(t, err)
	stderr, err = fifo.OpenWriter(stderrFifoPath)
	require.NoError(t, err)

	_, err = stdout.Write([]byte("test\n"))
	require.NoError(t, err)
	testutil.WaitForResult(func() (bool, error) {
		raw, err := ioutil.ReadFile(filepath.Join(dir, "stdout.0"))
		if err != nil {
			return false, err
		}

		expected := "test\ntest\n" == string(raw)
		return expected, fmt.Errorf("unexpected stdout %q", string(raw))
	}, func(err error) {
		require.NoError(t, err)
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

	// Pick a path that does not exist
	path := filepath.Join(uuid.Generate(), uuid.Generate(), uuid.Generate())

	logger := testlog.HCLogger(t)

	// No code that uses the writer should get hit
	rotator := panicWriter{}

	w, err := newLogRotatorWrapper(path, logger, rotator)
	require.Error(t, err)
	require.Nil(t, w)
}
