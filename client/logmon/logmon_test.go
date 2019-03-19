package logmon

import (
	"bytes"
	"crypto/rand"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/lib/fifo"
	"github.com/hashicorp/nomad/helper/testlog"
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

	stdout, err := fifo.Open(stdoutFifoPath)
	require.NoError(err)

	// Write enough bytes such that the log is rotated
	bytes1MB := make([]byte, 1024*1024)
	_, err = rand.Read(bytes1MB)
	require.NoError(err)

	io.Copy(stdout, bytes.NewBuffer(bytes1MB))

	time.Sleep(200 * time.Millisecond)
	_, err = os.Stat(filepath.Join(dir, "stdout.0"))
	require.NoError(err)
	_, err = os.Stat(filepath.Join(dir, "stdout.1"))
	require.NoError(err)

	require.NoError(lm.Stop())
}

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

	stdout, err := fifo.Open(stdoutFifoPath)
	require.NoError(err)
	stderr, err := fifo.Open(stderrFifoPath)
	require.NoError(err)

	// Write a string and assert it was written to the file
	io.Copy(stdout, bytes.NewBufferString("test\n"))
	time.Sleep(200 * time.Millisecond)
	raw, err := ioutil.ReadFile(filepath.Join(dir, "stdout.0"))
	require.NoError(err)
	require.Equal("test\n", string(raw))
	require.True(impl.tl.IsRunning())

	// Close stdout and assert that logmon no longer writes to the file
	require.NoError(stdout.Close())
	require.NoError(stderr.Close())

	stdout, err = fifo.Open(stdoutFifoPath)
	require.NoError(err)
	stderr, err = fifo.Open(stderrFifoPath)
	require.NoError(err)
	require.False(impl.tl.IsRunning())
	io.Copy(stdout, bytes.NewBufferString("te"))
	time.Sleep(200 * time.Millisecond)
	raw, err = ioutil.ReadFile(filepath.Join(dir, "stdout.0"))
	require.NoError(err)
	require.Equal("test\n", string(raw))

	// Start logmon again and assert that it appended to the file
	require.NoError(lm.Start(cfg))
	io.Copy(stdout, bytes.NewBufferString("st\n"))
	time.Sleep(200 * time.Millisecond)
	raw, err = ioutil.ReadFile(filepath.Join(dir, "stdout.0"))
	require.NoError(err)
	require.Equal("test\ntest\n", string(raw))
}
