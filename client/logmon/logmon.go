// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package logmon

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/lib/fifo"
	"github.com/hashicorp/nomad/client/logmon/logging"
)

const (
	// processOutputCloseTolerance is the length of time we will wait for the
	// launched process to close its stdout/stderr before we force close it. If
	// data is written after this tolerance, we will not capture it.
	processOutputCloseTolerance = 2 * time.Second
)

type LogConfig struct {
	// LogDir is the host path where logs are to be written to
	LogDir string

	// AllocDir is the sandbox directory that contains LogDir
	AllocDir string

	// StdoutLogFile is the path relative to LogDir for stdout logging
	StdoutLogFile string

	// StderrLogFile is the path relative to LogDir for stderr logging
	StderrLogFile string

	// StdoutFifo is the path on the host to the stdout pipe
	StdoutFifo string

	// StderrFifo is the path on the host to the stderr pipe
	StderrFifo string

	// MaxFiles is the max rotated files allowed
	MaxFiles int

	// MaxFileSizeMB is the max log file size in MB allowed before rotation occures
	MaxFileSizeMB int
}

type LogMon interface {
	Start(*LogConfig) error
	Stop() error
}

func NewLogMon(logger hclog.Logger) LogMon {
	return &logmonImpl{
		logger: logger,
	}
}

type logmonImpl struct {
	logger hclog.Logger
	tl     *TaskLogger
	lock   sync.Mutex
}

func (l *logmonImpl) Start(cfg *LogConfig) error {
	l.lock.Lock()
	defer l.lock.Unlock()

	// first time Start has been called
	if l.tl == nil {
		return l.start(cfg)
	}

	// stdout and stderr have been closed, this happens during task restarts
	// restart the TaskLogger
	if !l.tl.IsRunning() {
		l.tl.Close()
		return l.start(cfg)
	}

	// if the TaskLogger has been created and is currently running, noop
	return nil
}

func (l *logmonImpl) start(cfg *LogConfig) error {
	tl, err := NewTaskLogger(cfg, l.logger)
	if err != nil {
		return err
	}
	l.tl = tl
	return nil
}

func (l *logmonImpl) Stop() error {
	l.lock.Lock()
	defer l.lock.Unlock()
	if l.tl != nil {
		l.tl.Close()
	}
	return nil
}

type TaskLogger struct {
	config *LogConfig

	// rotator for stdout
	lro *logRotatorWrapper

	// rotator for stderr
	lre *logRotatorWrapper
}

// IsRunning will return true as long as one rotator wrapper is still running
func (tl *TaskLogger) IsRunning() bool {
	lroRunning := tl.lro != nil && tl.lro.isRunning()
	lreRunning := tl.lre != nil && tl.lre.isRunning()

	return lroRunning && lreRunning
}

func (tl *TaskLogger) Close() {
	var wg sync.WaitGroup
	if tl.lro != nil {
		wg.Go(func() {
			tl.lro.Close()
		})
	}
	if tl.lre != nil {
		wg.Go(func() {
			tl.lre.Close()
		})
	}
	wg.Wait()
}

func NewTaskLogger(cfg *LogConfig, logger hclog.Logger) (*TaskLogger, error) {
	tl := &TaskLogger{config: cfg}

	logFileSize := int64(cfg.MaxFileSizeMB * 1024 * 1024)
	lro, err := logging.NewFileRotator(cfg.LogDir, cfg.StdoutLogFile,
		cfg.MaxFiles, logFileSize, logger, cfg.AllocDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout logfile for %q: %v", cfg.StdoutLogFile, err)
	}

	wrapperOut, err := newLogRotatorWrapper(cfg.StdoutFifo, logger, lro)
	if err != nil {
		return nil, err
	}

	tl.lro = wrapperOut

	lre, err := logging.NewFileRotator(cfg.LogDir, cfg.StderrLogFile,
		cfg.MaxFiles, logFileSize, logger, cfg.AllocDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr logfile for %q: %v", cfg.StderrLogFile, err)
	}

	wrapperErr, err := newLogRotatorWrapper(cfg.StderrFifo, logger, lre)
	if err != nil {
		return nil, err
	}

	tl.lre = wrapperErr

	return tl, nil

}

// logRotatorWrapper wraps our log rotator and exposes a pipe that can feed the
// log rotator data. The processOutWriter should be attached to the process and
// data will be copied from the reader to the rotator.
type logRotatorWrapper struct {
	fifoPath          string
	rotatorWriter     io.WriteCloser
	hasFinishedCopied chan struct{}
	logger            hclog.Logger

	// readerLock guards processOutReader, which is replaced each time a
	// new writer generation opens the fifo.
	readerLock       sync.Mutex
	processOutReader io.ReadCloser
	openCompleted    chan struct{}

	// closing is closed by Close to stop the copy loop from opening the
	// fifo again once the current writer generation is drained.
	closing   chan struct{}
	closeOnce sync.Once
}

// isRunning will return true until the reader is closed
func (l *logRotatorWrapper) isRunning() bool {
	select {
	case <-l.hasFinishedCopied:
		return false
	default:
		return true
	}
}

// newLogRotatorWrapper takes a rotator and returns a wrapper that has the
// processOutWriter to attach to the stdout or stderr of a process.
func newLogRotatorWrapper(path string, logger hclog.Logger, rotator io.WriteCloser) (*logRotatorWrapper, error) {
	logger.Debug("opening fifo", "path", path)

	var openFn func() (io.ReadCloser, error)
	var err error

	_, serr := os.Stat(path)
	if os.IsNotExist(serr) {
		openFn, err = fifo.CreateAndRead(path)
	} else {
		openFn = func() (io.ReadCloser, error) {
			return fifo.OpenReader(path)
		}
	}

	if err != nil {
		logger.Error("failed to create FIFO", "stat_error", serr, "create_err", err)
		return nil, fmt.Errorf("failed to create fifo for extracting logs: %v", err)
	}

	wrap := &logRotatorWrapper{
		fifoPath:          path,
		rotatorWriter:     rotator,
		hasFinishedCopied: make(chan struct{}),
		openCompleted:     make(chan struct{}),
		closing:           make(chan struct{}),
		logger:            logger,
	}

	wrap.start(openFn)
	return wrap, nil
}

// start starts a goroutine that copies from the pipe into the rotator. This is
// called by the constructor and not the user of the wrapper.
//
// The copy continues across writer generations. When every writer has closed
// the fifo the reader sees EOF and opens the fifo again before closing the
// previous read side, so the fifo always has a reader. A task restarted while
// the previous process still held the write side keeps its output collected
// instead of filling the pipe buffer. The loop ends on a read error or after
// Close.
func (l *logRotatorWrapper) start(openFn func() (io.ReadCloser, error)) {
	go func() {
		defer close(l.hasFinishedCopied)

		var openOnce sync.Once
		markOpened := func() { openOnce.Do(func() { close(l.openCompleted) }) }
		defer markOpened()

		var prev io.ReadCloser
		defer func() {
			if prev != nil {
				prev.Close()
			}
		}()

		for {
			reader, err := openFn()
			if prev != nil {
				// A writer that connected after the EOF is attached to
				// the same pipe, so closing the old read side is safe
				// only now that a new one is open.
				prev.Close()
				prev = nil
			}
			if err != nil {
				l.logger.Warn("failed to open fifo", "error", err)
				return
			}
			l.readerLock.Lock()
			l.processOutReader = reader
			l.readerLock.Unlock()
			markOpened()

			_, err = io.Copy(l.rotatorWriter, reader)

			select {
			case <-l.closing:
				// Close may close the read side under us; that error is
				// expected.
				reader.Close()
				return
			default:
			}
			if err != nil {
				l.logger.Warn("failed to read from log fifo", "error", err)
				// Close reader to propagate io error across pipe.
				// Note that this may block until the process exits on
				// Windows due to
				// https://github.com/PowerShell/PowerShell/issues/4254
				// or similar issues. Since this is already running in
				// a goroutine its safe to block until the process is
				// force-killed.
				reader.Close()
				return
			}
			prev = reader
		}
	}()
}

// Close closes the rotator and the process writer to ensure that the Wait
// command exits.
func (l *logRotatorWrapper) Close() {
	l.closeOnce.Do(func() { close(l.closing) })

	// Give the process up to the close tolerance to close its side, so its
	// remaining output is copied. If the copy loop is waiting in open for a
	// writer that will not come, open and discard a writer to release it.
	unblock := time.NewTimer(100 * time.Millisecond)
	defer unblock.Stop()
	deadline := time.NewTimer(processOutputCloseTolerance)
	defer deadline.Stop()
wait:
	for {
		select {
		case <-l.hasFinishedCopied:
			break wait
		case <-unblock.C:
			if f, _ := fifo.OpenWriter(l.fifoPath); f != nil {
				f.Close()
			}
		case <-deadline.C:
			break wait
		}
	}

	// Closing the read side of a pipe may block on Windows if the process
	// is being debugged as in:
	// https://github.com/PowerShell/PowerShell/issues/4254
	// The pipe will be closed and cleaned up when the process exits.
	closeDone := make(chan struct{})
	go func() {
		defer close(closeDone)

		// we must wait until reader is opened before we can close it, and cannot inteerrupt an in-flight open request
		// The Close function uses processOutputCloseTolerance to protect against long running open called
		// and then request will be interrupted and file will be closed on process shutdown
		<-l.openCompleted

		l.readerLock.Lock()
		reader := l.processOutReader
		l.readerLock.Unlock()
		if reader != nil {
			err := reader.Close()
			if err != nil && !strings.Contains(err.Error(), "file already closed") {
				l.logger.Warn("error closing read-side of process output pipe", "error", err)
			}
		}

	}()

	select {
	case <-closeDone:
	case <-time.After(processOutputCloseTolerance):
		l.logger.Warn("timed out waiting for read-side of process output pipe to close")
	}

	l.rotatorWriter.Close()
}
