package executor

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"runtime"

	syslog "github.com/RackSec/srslog"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/driver/logging"
)

type SyslogLogger struct {
	syslogChan   chan *logging.SyslogMessage
	syslogServer *logging.SyslogServer
	listener     net.Listener

	tl *TaskLogger

	logger hclog.Logger
}

func (l *SyslogLogger) Addr() string {
	return fmt.Sprintf("%s://%s", l.listener.Addr().Network(), l.listener.Addr().String())
}

func (l *SyslogLogger) Close() {
	l.syslogServer.Shutdown()
	l.tl.Close()
}

func NewSyslogLogger(name string, cfg *LogConfig, portLowerBound, portUpperBound uint, logger hclog.Logger) (*SyslogLogger, error) {
	l := &SyslogLogger{
		syslogChan: make(chan *logging.SyslogMessage, 2048),
		logger:     logger.Named("syslog-server"),
	}

	listener, err := getListener(portLowerBound, portUpperBound)
	if err != nil {
		return nil, err
	}
	l.listener = listener

	tl, err := NewTaskLogger(name, cfg, logger)
	if err != nil {
		return nil, err
	}

	logger.Debug("launching syslog server", "addr", listener.Addr().String())
	l.syslogServer = logging.NewSyslogServer(listener, l.syslogChan, l.logger)
	go l.syslogServer.Start()
	go l.collectLogs(tl.lre.rotatorWriter, tl.lro.rotatorWriter)
	return l, nil
}

// getFreePort returns a free port ready to be listened on between upper and
// lower bounds
func getListener(lowerBound uint, upperBound uint) (net.Listener, error) {
	if runtime.GOOS == "windows" {
		return listenerTCP(lowerBound, upperBound)
	}

	return listenerUnix()
}

// listenerTCP creates a TCP listener using an unused port between an upper and
// lower bound
func listenerTCP(lowerBound uint, upperBound uint) (net.Listener, error) {
	for i := lowerBound; i <= upperBound; i++ {
		addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("localhost:%v", i))
		if err != nil {
			return nil, err
		}
		l, err := net.ListenTCP("tcp", addr)
		if err != nil {
			continue
		}
		return l, nil
	}
	return nil, fmt.Errorf("No free port found")
}

// listenerUnix creates a Unix domain socket
func listenerUnix() (net.Listener, error) {
	f, err := ioutil.TempFile("", "plugin")
	if err != nil {
		return nil, err
	}
	path := f.Name()

	if err := f.Close(); err != nil {
		return nil, err
	}
	if err := os.Remove(path); err != nil {
		return nil, err
	}

	return net.Listen("unix", path)
}

func (l *SyslogLogger) collectLogs(we io.Writer, wo io.Writer) {
	for logParts := range l.syslogChan {
		// If the severity of the log line is err then we write to stderr
		// otherwise all messages go to stdout
		if logParts.Severity == syslog.LOG_ERR {
			we.Write(logParts.Message)
			we.Write([]byte{'\n'})
		} else {
			wo.Write(logParts.Message)
			wo.Write([]byte{'\n'})
		}
	}
}
