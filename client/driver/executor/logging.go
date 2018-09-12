package executor

import (
	"fmt"
	"io"
	"os"
	"os/user"
	"strconv"
	"strings"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/driver/logging"
)

const (
	// processOutputCloseTolerance is the length of time we will wait for the
	// launched process to close its stdout/stderr before we force close it. If
	// data is written after this tolerance, we will not capture it.
	processOutputCloseTolerance = 2 * time.Second
)

type LogConfig struct {
	// LogDir is the host path where logs should be written
	LogDir string

	// StdoutLogFile is the path relative to LogDir for stdout logging
	StdoutLogFile string

	// StderrLogFile is the path relative to LogDir for stderr logging
	StderrLogFile string

	// MaxFiles is the max rotated files allowed
	MaxFiles int

	// MaxFileSizeMB is the max log file size in MB allowed before rotation occures
	MaxFileSizeMB int

	// UID is id for the desired user to write log files as
	// If unset will default to nobody if available or else root
	UID *int

	// GID is id for the desried group to write log files as
	// If unset will default to nobody if available or else root
	GID *int
}

type TaskLogger struct {
	config *LogConfig

	// rotator for stdout
	lro *logRotatorWrapper

	// rotator for stderr
	lre *logRotatorWrapper
}

func (tl *TaskLogger) Close() {
	tl.lro.Close()
	tl.lre.Close()
}

func (tl *TaskLogger) Stdout() io.Writer {
	return tl.lro.processOutWriter
}

func (tl *TaskLogger) StdoutFD() uintptr {
	return tl.lro.processOutWriter.Fd()
}

func (tl *TaskLogger) Stderr() io.Writer {
	return tl.lre.processOutWriter
}

func (tl *TaskLogger) StderrFD() uintptr {
	return tl.lre.processOutWriter.Fd()
}

func NewTaskLogger(name string, cfg *LogConfig, logger hclog.Logger) (*TaskLogger, error) {
	tl := &TaskLogger{config: cfg}

	var uid, gid int
	u, err := user.Lookup("nobody")
	if err == nil && u != nil {
		uid, _ = strconv.Atoi(u.Uid)
		gid, _ = strconv.Atoi(u.Gid)
	} else {
		uid = -1
		gid = -1
	}

	if cfg.UID != nil {
		uid = *cfg.UID
	}
	if cfg.GID != nil {
		gid = *cfg.GID
	}

	stdoutFileName := fmt.Sprintf("%v.stdout", name)
	if cfg.StdoutLogFile != "" {
		stdoutFileName = cfg.StdoutLogFile
	}
	stderrFileName := fmt.Sprintf("%v.stderr", name)
	if cfg.StderrLogFile != "" {
		stderrFileName = cfg.StderrLogFile
	}

	logFileSize := int64(cfg.MaxFileSizeMB * 1024 * 1024)
	lro, err := logging.NewFileRotator(cfg.LogDir, stdoutFileName,
		cfg.MaxFiles, logFileSize, uid, gid, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout logfile for %q: %v", name, err)
	}

	wrapperOut, err := newLogRotatorWrapper(logger, lro)
	if err != nil {
		return nil, err
	}

	tl.lro = wrapperOut

	lre, err := logging.NewFileRotator(cfg.LogDir, stderrFileName,
		cfg.MaxFiles, logFileSize, uid, gid, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr logfile for %q: %v", name, err)
	}

	wrapperErr, err := newLogRotatorWrapper(logger, lre)
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
	processOutWriter  *os.File
	processOutReader  *os.File
	rotatorWriter     *logging.FileRotator
	hasFinishedCopied chan struct{}
	logger            hclog.Logger
}

// newLogRotatorWrapper takes a rotator and returns a wrapper that has the
// processOutWriter to attach to the processes stdout or stderr.
func newLogRotatorWrapper(logger hclog.Logger, rotator *logging.FileRotator) (*logRotatorWrapper, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create os.Pipe for extracting logs: %v", err)
	}

	wrap := &logRotatorWrapper{
		processOutWriter:  w,
		processOutReader:  r,
		rotatorWriter:     rotator,
		hasFinishedCopied: make(chan struct{}),
		logger:            logger,
	}
	wrap.start()
	return wrap, nil
}

// start starts a go-routine that copies from the pipe into the rotator. This is
// called by the constructor and not the user of the wrapper.
func (l *logRotatorWrapper) start() {
	go func() {
		defer close(l.hasFinishedCopied)
		_, err := io.Copy(l.rotatorWriter, l.processOutReader)
		if err != nil {
			// Close reader to propagate io error across pipe.
			// Note that this may block until the process exits on
			// Windows due to
			// https://github.com/PowerShell/PowerShell/issues/4254
			// or similar issues. Since this is already running in
			// a goroutine its safe to block until the process is
			// force-killed.
			l.processOutReader.Close()
		}
	}()
	return
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
		err := l.processOutReader.Close()
		if err != nil && !strings.Contains(err.Error(), "file already closed") {
			l.logger.Warn("error closing read-side of process output pipe", "err", err)
		}

	}()

	select {
	case <-closeDone:
	case <-time.After(processOutputCloseTolerance):
		l.logger.Warn("timed out waiting for read-side of process output pipe to close")
	}

	l.rotatorWriter.Close()
	return
}
