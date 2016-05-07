// +build darwin dragonfly freebsd linux netbsd openbsd solaris

package executor

import (
	"fmt"
	"io"
	"log/syslog"
	"os/exec"
	"time"

	"golang.org/x/sys/unix"

	"github.com/hashicorp/nomad/client/driver/logging"
)

func (e *UniversalExecutor) LaunchSyslogServer(ctx *ExecutorContext) (*SyslogServerState, error) {
	e.ctx = ctx

	// configuring the task dir
	if err := e.configureTaskDir(); err != nil {
		return nil, err
	}

	e.syslogChan = make(chan *logging.SyslogMessage, 2048)
	l, err := e.getListener(e.ctx.PortLowerBound, e.ctx.PortUpperBound)
	if err != nil {
		return nil, err
	}
	e.logger.Printf("[DEBUG] sylog-server: launching syslog server on addr: %v", l.Addr().String())
	if err := e.configureLoggers(); err != nil {
		return nil, err
	}

	e.syslogServer = logging.NewSyslogServer(l, e.syslogChan, e.logger)
	go e.syslogServer.Start()
	go e.collectLogs(e.lre, e.lro)
	syslogAddr := fmt.Sprintf("%s://%s", l.Addr().Network(), l.Addr().String())
	return &SyslogServerState{Addr: syslogAddr}, nil
}

func (e *UniversalExecutor) collectLogs(we io.Writer, wo io.Writer) {
	for logParts := range e.syslogChan {
		// If the severity of the log line is err then we write to stderr
		// otherwise all messages go to stdout
		if logParts.Severity == syslog.LOG_ERR {
			e.lre.Write(logParts.Message)
			e.lre.Write([]byte{'\n'})
		} else {
			e.lro.Write(logParts.Message)
			e.lro.Write([]byte{'\n'})
		}
	}
}

func (e *UniversalExecutor) wait() {
	defer close(e.processExited)
	err := e.cmd.Wait()
	ic := &cstructs.IsolationConfig{Cgroup: e.groups, CgroupPaths: e.cgPaths}
	if err == nil {
		e.exitState = &ProcessState{Pid: 0, ExitCode: 0, IsolationConfig: ic, Time: time.Now()}
		return
	}
	exitCode := 1
	var signal int
	if exitErr, ok := err.(*exec.ExitError); ok {
		if status, ok := exitErr.Sys().(unix.WaitStatus); ok {
			exitCode = status.ExitStatus()
			if status.Signaled() {
				signal = int(status.Signal())
				exitCode = 128 + signal
			}
		}
	} else {
		e.logger.Printf("[DEBUG] executor: unexpected Wait() error type: %v", err)
	}

	e.exitState = &ProcessState{Pid: 0, ExitCode: exitCode, Signal: signal, IsolationConfig: ic, Time: time.Now()}
}
