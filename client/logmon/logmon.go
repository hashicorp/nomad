package logmon

import (
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/driver/logging"
	"github.com/hashicorp/nomad/client/lib/fifo"
	"github.com/hashicorp/nomad/helper/discover"
)

const (
	// processOutputCloseTolerance is the length of time we will wait for the
	// launched process to close its stdout/stderr before we force close it. If
	// data is written after this tolerance, we will not capture it.
	processOutputCloseTolerance = 2 * time.Second
)

var Handshake = plugin.HandshakeConfig{
	// This isn't required when using VersionedPlugins
	ProtocolVersion:  1,
	MagicCookieKey:   "LOGMON_PLUGIN",
	MagicCookieValue: "logmon",
}

type LogConfig struct {
	// LogDir is the host path where logs are to be written to
	LogDir string

	// FifoDir is the host path where the fifo created
	FifoDir string

	// StdoutLogFile is the path relative to LogDir for stdout logging
	StdoutLogFile string

	// StderrLogFile is the path relative to LogDir for stderr logging
	StderrLogFile string

	// MaxFiles is the max rotated files allowed
	MaxFiles int

	// MaxFileSizeMB is the max log file size in MB allowed before rotation occures
	MaxFileSizeMB int

	// UID is id for the desired user to write log files as
	UID int

	// GID is id for the desried group to write log files as
	GID int
}

type LogMon interface {
	Start(*LogConfig) error
	Stop() error
}

func LaunchLogMon(logger hclog.Logger) (LogMon, *plugin.Client, error) {
	bin, err := discover.NomadExecutable()
	if err != nil {
		return nil, nil, err
	}

	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: Handshake,
		Plugins: map[string]plugin.Plugin{
			"logmon": NewPlugin(NewLogMon(hclog.L().Named("logmon"))),
		},
		Cmd: exec.Command(bin, "logmon"),
		AllowedProtocols: []plugin.Protocol{
			plugin.ProtocolGRPC,
		},
	})

	rpcClient, err := client.Client()
	if err != nil {
		return nil, nil, err
	}

	raw, err := rpcClient.Dispense("logmon")
	if err != nil {
		return nil, nil, err
	}

	l := raw.(LogMon)
	return l, client, nil

}

func NewLogMon(logger hclog.Logger) LogMon {
	return &logmonImpl{
		logger: logger,
	}
}

type logmonImpl struct {
	logger hclog.Logger
	tl     *TaskLogger
}

func (l *logmonImpl) Start(cfg *LogConfig) error {
	tl, err := NewTaskLogger(cfg, l.logger)
	if err != nil {
		return err
	}
	l.tl = tl
	return nil
}

func (l *logmonImpl) Stop() error {
	l.tl.Close()
	return nil
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

func (tl *TaskLogger) StdoutFifo() string {
	return tl.lro.fifoPath
}

func NewTaskLogger(cfg *LogConfig, logger hclog.Logger) (*TaskLogger, error) {
	tl := &TaskLogger{config: cfg}

	logFileSize := int64(cfg.MaxFileSizeMB * 1024 * 1024)
	lro, err := logging.NewFileRotator(cfg.LogDir, cfg.StdoutLogFile,
		cfg.MaxFiles, logFileSize, cfg.UID, cfg.GID, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout logfile for %q: %v", cfg.StdoutLogFile, err)
	}

	wrapperOut, err := newLogRotatorWrapper(filepath.Join(cfg.FifoDir, cfg.StdoutLogFile), logger, lro)
	if err != nil {
		return nil, err
	}

	tl.lro = wrapperOut

	lre, err := logging.NewFileRotator(cfg.LogDir, cfg.StderrLogFile,
		cfg.MaxFiles, logFileSize, cfg.UID, cfg.GID, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr logfile for %q: %v", cfg.StderrLogFile, err)
	}

	wrapperErr, err := newLogRotatorWrapper(filepath.Join(cfg.FifoDir, cfg.StderrLogFile), logger, lre)
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
	processOutReader  io.ReadCloser
	rotatorWriter     *logging.FileRotator
	hasFinishedCopied chan struct{}
	logger            hclog.Logger
}

// newLogRotatorWrapper takes a rotator and returns a wrapper that has the
// processOutWriter to attach to the processes stdout or stderr.
func newLogRotatorWrapper(path string, logger hclog.Logger, rotator *logging.FileRotator) (*logRotatorWrapper, error) {
	logger.Info("opening fifo", "path", path)
	f, err := fifo.New(path)
	if err != nil {
		return nil, fmt.Errorf("failed to create fifo for extracting logs: %v", err)
	}

	wrap := &logRotatorWrapper{
		fifoPath:          path,
		processOutReader:  f,
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
