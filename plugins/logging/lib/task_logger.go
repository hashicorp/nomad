package loglib

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"

	"github.com/hashicorp/nomad/client/lib/fifo"
)

const (
	// processOutputCloseTolerance is the length of time we will wait for the
	// launched process to close its stdout/stderr before we force close it. If
	// data is written after this tolerance, we will not capture it.
	processOutputCloseTolerance = 2 * time.Second
)

type TaskLogger struct {
	config *LogConfig

	// rotator for stdout
	lro *logRotatorWrapper

	// rotator for stderr
	lre *logRotatorWrapper
}

func (tl *TaskLogger) Wait() {
	tl.lro.isDone()
	tl.lre.isDone()
}

func (tl *TaskLogger) Close() {
	var wg sync.WaitGroup
	if tl.lro != nil {
		wg.Add(1)
		go func() {
			tl.lro.Close()
			wg.Done()
		}()
	}
	if tl.lre != nil {
		wg.Add(1)
		go func() {
			tl.lre.Close()
			wg.Done()
		}()
	}
	wg.Wait()
}

func NewTaskLogger(cfg *LogConfig, logger hclog.Logger) (*TaskLogger, error) {
	tl := &TaskLogger{config: cfg}

	logFileSize := int64(cfg.MaxFileSizeMB * 1024 * 1024)
	lro, err := NewFileRotator(cfg.LogDir, cfg.StdoutLogFile,
		cfg.MaxFiles, logFileSize, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout logfile for %q: %v", cfg.StdoutLogFile, err)
	}

	wrapperOut, err := newLogRotatorWrapper(cfg.StdoutFifo, logger, lro)
	if err != nil {
		return nil, err
	}

	tl.lro = wrapperOut

	lre, err := NewFileRotator(cfg.LogDir, cfg.StderrLogFile,
		cfg.MaxFiles, logFileSize, logger)
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

	processOutReader io.ReadCloser
	openCompleted    chan struct{}
}

// isRunning will block until the reader is closed
func (l *logRotatorWrapper) isDone() {
	<-l.hasFinishedCopied
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
	logger.Info("opening fifo", "path", path)

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
		logger:            logger,
	}

	wrap.start(openFn)
	return wrap, nil
}

// start starts a goroutine that copies from the pipe into the rotator. This is
// called by the constructor and not the user of the wrapper.
func (l *logRotatorWrapper) start(openFn func() (io.ReadCloser, error)) {
	go func() {
		defer close(l.hasFinishedCopied)

		reader, err := openFn()
		if err != nil {
			l.logger.Warn("failed to open fifo", "error", err)
			return
		}
		l.processOutReader = reader
		close(l.openCompleted)

		_, err = io.Copy(l.rotatorWriter, reader)
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
		}
	}()
}

// Close closes the rotator and the process writer to ensure that the Wait
// command exits.
func (l *logRotatorWrapper) Close() {
	// Wait up to the close tolerance before we force close
	select {
	case <-l.hasFinishedCopied:
	case <-time.After(processOutputCloseTolerance):
	}

	// Closing the read side of a pipe may block on Windows if the process
	// is being debugged as in:
	// https://github.com/PowerShell/PowerShell/issues/4254
	// The pipe will be closed and cleaned up when the process exits.
	closeDone := make(chan struct{})
	go func() {
		defer close(closeDone)

		// we must wait until reader is opened before we can close it, and
		// cannot inteerrupt an in-flight open request
		//
		// The Close function uses processOutputCloseTolerance to protect
		// against long running open called and then request will be interrupted
		// and file will be closed on process shutdown
		<-l.openCompleted

		if l.processOutReader != nil {
			err := l.processOutReader.Close()
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
