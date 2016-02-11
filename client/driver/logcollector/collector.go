package logcollector

import (
	"fmt"
	"io"
	"log"
	s1 "log/syslog"
	"net"
	"path/filepath"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/driver/executor"
	"github.com/hashicorp/nomad/client/driver/logrotator"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mcuadros/go-syslog"
)

// LogCollectorContext holds context to configure the syslog server
type LogCollectorContext struct {
	// TaskName is the name of the Task
	TaskName string

	// AllocDir is the handle to do operations on the alloc dir of
	// the task
	AllocDir *allocdir.AllocDir

	// LogConfig provides configuration related to log rotation
	LogConfig *structs.LogConfig

	// PortUpperBound is the upper bound of the ports that we can use to start
	// the syslog server
	PortUpperBound uint

	// PortLowerBound is the lower bound of the ports that we can use to start
	// the syslog server
	PortLowerBound uint
}

// SyslogCollectorState holds the address and islation information of a launched
// syslog server
type SyslogCollectorState struct {
	IsolationConfig *executor.IsolationConfig
	Addr            string
}

// LogCollector is an interface which allows a driver to launch a log server
// and update log configuration
type LogCollector interface {
	LaunchCollector(ctx *LogCollectorContext) (*SyslogCollectorState, error)
	Exit() error
	UpdateLogConfig(logConfig *structs.LogConfig) error
}

// SyslogCollector is a LogCollector which starts a syslog server and does
// rotation to incoming stream
type SyslogCollector struct {
	addr      net.Addr
	logConfig *structs.LogConfig
	ctx       *LogCollectorContext

	lro     *logrotator.LogRotator
	lre     *logrotator.LogRotator
	server  *syslog.Server
	taskDir string

	logger *log.Logger
}

// NewSyslogCollector returns an implementation of the SyslogCollector
func NewSyslogCollector(logger *log.Logger) *SyslogCollector {
	return &SyslogCollector{logger: logger}
}

// LaunchCollector launches a new syslog server and starts writing log lines to
// files and rotates them
func (s *SyslogCollector) LaunchCollector(ctx *LogCollectorContext) (*SyslogCollectorState, error) {
	addr, err := s.getFreePort(ctx.PortLowerBound, ctx.PortUpperBound)
	if err != nil {
		return nil, err
	}
	s.logger.Printf("[DEBUG] sylog-server: launching syslog server on addr: %v", addr)
	s.ctx = ctx
	// configuring the task dir
	if err := s.configureTaskDir(); err != nil {
		return nil, err
	}

	channel := make(syslog.LogPartsChannel)
	handler := syslog.NewChannelHandler(channel)

	s.server = syslog.NewServer()
	s.server.SetFormat(&CustomParser{logger: s.logger})
	s.server.SetHandler(handler)
	s.server.ListenTCP(addr.String())
	if err := s.server.Boot(); err != nil {
		return nil, err
	}
	logFileSize := int64(ctx.LogConfig.MaxFileSizeMB * 1024 * 1024)

	ro, wo := io.Pipe()
	lro, err := logrotator.NewLogRotator(filepath.Join(s.taskDir, allocdir.TaskLocal),
		fmt.Sprintf("%v.stdout", ctx.TaskName), ctx.LogConfig.MaxFiles,
		logFileSize, s.logger)
	if err != nil {
		return nil, err
	}
	go lro.Start(ro)

	re, we := io.Pipe()
	lre, err := logrotator.NewLogRotator(filepath.Join(s.taskDir, allocdir.TaskLocal),
		fmt.Sprintf("%v.stderr", ctx.TaskName), ctx.LogConfig.MaxFiles,
		logFileSize, s.logger)
	if err != nil {
		return nil, err
	}
	go lre.Start(re)

	go func(channel syslog.LogPartsChannel) {
		for logParts := range channel {
			// If the severity of the log line is err then we write to stderr
			// otherwise all messages go to stdout
			s := logParts["severity"].(Priority)
			if s.Severity == s1.LOG_ERR {
				we.Write(logParts["content"].([]byte))
			} else {
				wo.Write(logParts["content"].([]byte))
			}
			wo.Write([]byte("\n"))
		}
	}(channel)
	go s.server.Wait()
	return &SyslogCollectorState{Addr: addr.String()}, nil
}

// Exit kills the syslog server
func (s *SyslogCollector) Exit() error {
	return s.server.Kill()
}

// UpdateLogConfig updates the log configuration
func (s *SyslogCollector) UpdateLogConfig(logConfig *structs.LogConfig) error {
	s.ctx.LogConfig = logConfig
	if s.lro == nil {
		return fmt.Errorf("log rotator for stdout doesn't exist")
	}
	s.lro.MaxFiles = logConfig.MaxFiles
	s.lro.FileSize = int64(logConfig.MaxFileSizeMB * 1024 * 1024)

	if s.lre == nil {
		return fmt.Errorf("log rotator for stderr doesn't exist")
	}
	s.lre.MaxFiles = logConfig.MaxFiles
	s.lre.FileSize = int64(logConfig.MaxFileSizeMB * 1024 * 1024)
	return nil

}

// configureTaskDir sets the task dir in the SyslogCollector
func (s *SyslogCollector) configureTaskDir() error {
	taskDir, ok := s.ctx.AllocDir.TaskDirs[s.ctx.TaskName]
	if !ok {
		return fmt.Errorf("couldn't find task directory for task %v", s.ctx.TaskName)
	}
	s.taskDir = taskDir
	return nil
}

// getFreePort returns a free port ready to be listened on between upper and
// lower bounds
func (s *SyslogCollector) getFreePort(lowerBound uint, upperBound uint) (net.Addr, error) {
	for i := lowerBound; i <= upperBound; i++ {
		addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("localhost:%v", i))
		if err != nil {
			return nil, err
		}
		l, err := net.ListenTCP("tcp", addr)
		if err != nil {
			continue
		}
		defer l.Close()
		return l.Addr(), nil
	}
	return nil, fmt.Errorf("No free port found")
}
