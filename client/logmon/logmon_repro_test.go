// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !windows

package logmon

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/lib/fifo"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
)

// TestLogmon_Start_restartWhileWriterOpen covers a task restart where the
// prestart Start call arrives before the previous process has closed the
// write side of its fifos. That Start is a no-op, so logmon must keep
// collecting output from the next writer on its own. Before the fix the copy
// goroutine exited on EOF and left its read side open, so the next process
// could open the fifo, fill the pipe buffer and block on write.
func TestLogmon_Start_restartWhileWriterOpen(t *testing.T) {
	ci.Parallel(t)

	allocDir := t.TempDir()
	dir, _ := os.MkdirTemp(allocDir, "")
	stdoutFifo := filepath.Join(dir, "stdout.fifo")
	stderrFifo := filepath.Join(dir, "stderr.fifo")

	cfg := &LogConfig{
		AllocDir:      allocDir,
		LogDir:        dir,
		StdoutLogFile: "stdout",
		StdoutFifo:    stdoutFifo,
		StderrLogFile: "stderr",
		StderrFifo:    stderrFifo,
		MaxFiles:      2,
		MaxFileSizeMB: 1,
	}

	lm := NewLogMon(testlog.HCLogger(t))
	must.NoError(t, lm.Start(cfg))
	t.Cleanup(func() { must.NoError(t, lm.Stop()) })

	oldOut, err := fifo.OpenWriter(stdoutFifo)
	must.NoError(t, err)
	oldErr, err := fifo.OpenWriter(stderrFifo)
	must.NoError(t, err)
	_, err = oldErr.Write([]byte("first\n"))
	must.NoError(t, err)

	// Restart: Start runs while the old process still holds the writers.
	must.NoError(t, lm.Start(cfg))

	// The old process exits after Start.
	must.NoError(t, oldOut.Close())
	must.NoError(t, oldErr.Close())

	// The new process writes more than one pipe buffer (64 KiB on Linux).
	newOut, err := fifo.OpenWriter(stdoutFifo)
	must.NoError(t, err)
	t.Cleanup(func() { newOut.Close() })
	newErr, err := fifo.OpenWriter(stderrFifo)
	must.NoError(t, err)
	t.Cleanup(func() { newErr.Close() })

	line := make([]byte, 256)
	for i := range line {
		line[i] = 'x'
	}
	line[len(line)-1] = '\n'
	const lines = 1024 // 256 KiB
	for i := 0; i < lines; i++ {
		_, err = newErr.Write(line)
		must.NoError(t, err)
	}

	want := len("first\n") + lines*len(line)
	testutil.WaitForResult(func() (bool, error) {
		fi, err := os.Stat(filepath.Join(dir, "stderr.0"))
		if err != nil {
			return false, err
		}
		return fi.Size() == int64(want), fmt.Errorf("stderr.0 has %d bytes, want %d", fi.Size(), want)
	}, func(err error) {
		must.NoError(t, err)
	})
}
