package taskrunner

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"

	hclog "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunnerv2/interfaces"
	"github.com/hashicorp/nomad/client/logmon"
	"github.com/hashicorp/nomad/helper/uuid"
)

// logmonHook launches logmon and manages task logging
type logmonHook struct {
	// logmon is the handle to the log monitor process for the task.
	logmon             logmon.LogMon
	logmonPluginClient *plugin.Client

	stdoutFifo string
	stderrFifo string

	taskDir *allocdir.TaskDir

	logger hclog.Logger
}

func newLogMonHook(tr *TaskRunner, logger hclog.Logger) *logmonHook {
	hook := &logmonHook{
		taskDir: tr.taskDir,
		logger:  logger,
	}

	tr.taskLoggingFifoGetter = hook.getFifos
	return hook
}

func (h *logmonHook) getFifos() (stdout, stderr string) {
	return h.stdoutFifo, h.stderrFifo
}

func (*logmonHook) Name() string {
	return "logmon"
}

func (h *logmonHook) launchLogMon() error {
	l, c, err := logmon.LaunchLogMon(h.logger)
	if err != nil {
		return err
	}

	h.logmon = l
	h.logmonPluginClient = c
	return nil
}

func (h *logmonHook) Prestart(ctx context.Context,
	req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {

	// Launch logmon instance for the task.
	err := h.launchLogMon()
	if err != nil {
		h.logger.Error("failed to launch logmon process", "error", err)
		return err
	}

	if runtime.GOOS == "windows" {
		id := uuid.Generate()[:8]
		h.stdoutFifo = fmt.Sprintf("//./pipe/%s.stdout.%s", id, req.Task.Name)
		h.stderrFifo = fmt.Sprintf("//./pipe/%s.stderr.%s", id, req.Task.Name)
	} else {
		h.stdoutFifo = filepath.Join(h.taskDir.LogDir, fmt.Sprintf("%s.stdout", req.Task.Name))
		h.stderrFifo = filepath.Join(h.taskDir.LogDir, fmt.Sprintf("%s.stderr", req.Task.Name))
	}

	err = h.logmon.Start(&logmon.LogConfig{
		LogDir:        h.taskDir.LogDir,
		StdoutLogFile: fmt.Sprintf("%s.stdout", req.Task.Name),
		StderrLogFile: fmt.Sprintf("%s.stderr", req.Task.Name),
		StdoutFifo:    h.stdoutFifo,
		StderrFifo:    h.stderrFifo,
		MaxFiles:      req.Task.LogConfig.MaxFiles,
		MaxFileSizeMB: req.Task.LogConfig.MaxFileSizeMB,
	})
	if err != nil {
		h.logger.Error("failed to start logmon", "error", err)
		return err
	}
	return nil
}

func (h *logmonHook) Stop(context.Context, *interfaces.TaskStopRequest, *interfaces.TaskStopResponse) error {

	h.logmon.Stop()
	h.logmonPluginClient.Kill()

	return nil
}
