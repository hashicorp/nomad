package logmon

import (
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/nomad/client/lib/fifo"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func TestLogmon_Start_rotate(t *testing.T) {
	require := require.New(t)
	dir, err := ioutil.TempDir("", "nomadtest")
	require.NoError(err)
	defer os.RemoveAll(dir)
	stdoutLog := "stdout"
	stdoutFifoPath := filepath.Join(dir, "stdout.fifo")
	stderrLog := "stderr"
	stderrFifoPath := filepath.Join(dir, "stderr.fifo")

	cfg := &LogConfig{
		LogDir:        dir,
		StdoutLogFile: stdoutLog,
		StdoutFifo:    stdoutFifoPath,
		StderrLogFile: stderrLog,
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

// asserts that calling Start twice restarts the log rotator
func TestLogmon_Start_restart(t *testing.T) {
	require := require.New(t)
	dir, err := ioutil.TempDir("", "nomadtest")
	require.NoError(err)
	defer os.RemoveAll(dir)
	stdoutLog := "stdout"
	stdoutFifoPath := filepath.Join(dir, "stdout.fifo")
	stderrLog := "stderr"
	stderrFifoPath := filepath.Join(dir, "stderr.fifo")

	cfg := &LogConfig{
		LogDir:        dir,
		StdoutLogFile: stdoutLog,
		StdoutFifo:    stdoutFifoPath,
		StderrLogFile: stderrLog,
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
		raw, err := ioutil.ReadFile(filepath.Join(dir, "stdout.0"))
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
		raw, err := ioutil.ReadFile(filepath.Join(dir, "stdout.0"))
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
		raw, err := ioutil.ReadFile(filepath.Join(dir, "stdout.0"))
		if err != nil {
			return false, err
		}

		expected := "test\ntest\n" == string(raw)
		return expected, fmt.Errorf("unexpected stdout %q", string(raw))
	}, func(err error) {
		require.NoError(err)
	})
}
