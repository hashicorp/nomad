package taskrunner

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"

	hclog "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/logmon"
	"github.com/hashicorp/nomad/helper/uuid"
)

// logmonHook launches logmon and manages task logging
type logmonHook struct {
	// logmon is the handle to the log monitor process for the task.
	logmon             logmon.LogMon
	logmonPluginClient *plugin.Client

	config *logmonHookConfig

	logger hclog.Logger
}

type logmonHookConfig struct {
	logDir     string
	stdoutFifo string
	stderrFifo string
}

func newLogMonHook(cfg *logmonHookConfig, logger hclog.Logger) *logmonHook {
	hook := &logmonHook{
		config: cfg,
		logger: logger,
	}

	return hook
}

func newLogMonHookConfig(taskName, logDir string) *logmonHookConfig {
	cfg := &logmonHookConfig{
		logDir: logDir,
	}
	if runtime.GOOS == "windows" {
		id := uuid.Generate()[:8]
		cfg.stdoutFifo = fmt.Sprintf("//./pipe/%s-%s.stdout", taskName, id)
		cfg.stderrFifo = fmt.Sprintf("//./pipe/%s-%s.stderr", taskName, id)
	} else {
		cfg.stdoutFifo = filepath.Join(logDir, fmt.Sprintf(".%s.stdout.fifo", taskName))
		cfg.stderrFifo = filepath.Join(logDir, fmt.Sprintf(".%s.stderr.fifo", taskName))
	}
	return cfg
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
	if err := h.launchLogMon(); err != nil {
		h.logger.Error("failed to launch logmon process", "error", err)
		return err
	}

	err := h.logmon.Start(&logmon.LogConfig{
		LogDir:        h.config.logDir,
		StdoutLogFile: fmt.Sprintf("%s.stdout", req.Task.Name),
		StderrLogFile: fmt.Sprintf("%s.stderr", req.Task.Name),
		StdoutFifo:    h.config.stdoutFifo,
		StderrFifo:    h.config.stderrFifo,
		MaxFiles:      req.Task.LogConfig.MaxFiles,
		MaxFileSizeMB: req.Task.LogConfig.MaxFileSizeMB,
	})
	if err != nil {
		h.logger.Error("failed to start logmon", "error", err)
		return err
	}

	resp.Done = true
	return nil
}

func (h *logmonHook) Stop(context.Context, *interfaces.TaskStopRequest, *interfaces.TaskStopResponse) error {

	if h.logmon != nil {
		h.logmon.Stop()
	}
	if h.logmonPluginClient != nil {
		h.logmonPluginClient.Kill()
	}

	return nil
}
