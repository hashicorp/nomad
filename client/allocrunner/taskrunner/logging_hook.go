package taskrunner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/pluginmanager/loggingmanager"
	bstructs "github.com/hashicorp/nomad/plugins/base/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	loglib "github.com/hashicorp/nomad/plugins/logging/lib"
)

// loggingHook launches a logging plugin (if one hasn't already been started),
// and manages the task logging with that plugin
type loggingHook struct {
	runner *TaskRunner

	// plugin is name of the selected log monitoring plugin
	pluginName string

	// pluginManager dispenses plugins appropriate for the task
	pluginManager loggingmanager.Manager

	config *loglib.LogConfig

	logger hclog.Logger
}

func newLoggingHook(tr *TaskRunner, logger hclog.Logger) *loggingHook {
	cfg := &loglib.LogConfig{
		JobID:      tr.alloc.JobID,
		AllocID:    tr.allocID,
		GroupName:  tr.alloc.TaskGroup,
		TaskName:   tr.taskName,
		LogDir:     tr.logmonHookConfig.logDir,
		StdoutFifo: tr.logmonHookConfig.stdoutFifo,
		StderrFifo: tr.logmonHookConfig.stderrFifo,

		// TODO: these fields are being set in the Prestart hook, but they're
		// already available at the time we create this hook. Is there any
		// reason not to configure them here?

		// StdoutLogFile: "",
		// StderrLogFile: "",
		// MaxFiles:      0,
		// MaxFileSizeMB: 0,
	}

	hook := &loggingHook{
		runner:        tr,
		pluginManager: tr.loggingManager,
		config:        cfg,
		logger:        logger,
	}

	return hook
}

func (*loggingHook) Name() string {
	return "logmon"
}

func (h *loggingHook) Prestart(ctx context.Context,
	req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {

	if h.isLoggingDisabled() {
		h.logger.Debug("logging is disabled by driver, creating log files")

		h.config.StdoutLogFile = fmt.Sprintf("%s.stdout.0", req.Task.Name)
		h.config.StderrLogFile = fmt.Sprintf("%s.stderr.0", req.Task.Name)

		// these FIFOs won't be created, so we're just writing to files on disk
		err := os.MkdirAll(h.config.LogDir, 0777) // TODO: yikes!
		if err != nil {
			return err
		}
		h.logger.Debug("made LogDir", "LogDir", h.config.LogDir)

		_, err = os.Create(filepath.Join(h.config.LogDir, h.config.StdoutLogFile))
		if err != nil {
			return err
		}
		_, err = os.Create(filepath.Join(h.config.LogDir, h.config.StderrLogFile))
		if err != nil {
			return err
		}

		h.runner.logmonHookConfig.stdoutFifo = filepath.Join(h.config.LogDir, h.config.StdoutLogFile)
		h.runner.logmonHookConfig.stderrFifo = filepath.Join(h.config.LogDir, h.config.StderrLogFile)

		h.logger.Debug("logging is disabled by driver, created log files")
		return nil
	}

	// TODO: this behavior is lifted from legacy logmon, but I think all this
	// data is available when we initially configure the hook?

	h.config.StdoutLogFile = fmt.Sprintf("%s.stdout", req.Task.Name)
	h.config.StderrLogFile = fmt.Sprintf("%s.stderr", req.Task.Name)

	h.config.MaxFiles = req.Task.LogConfig.MaxFiles
	h.config.MaxFileSizeMB = req.Task.LogConfig.MaxFileSizeMB

	// TODO: figure out how we really want to choose these
	h.pluginName = "logmon"
	if alt := req.Task.Meta["logging_plugin"]; alt != "" {
		h.pluginName = alt
	}

	attempts := 0
	for {
		err := h.prestartOneLoop(ctx, req)
		if err == nil {
			return nil
		}
		if err == bstructs.ErrPluginShutdown || grpc.Code(err) == codes.Unavailable {
			h.logger.Warn("logmon shutdown while making request", "error", err)

			if attempts > 3 {
				h.logger.Warn("logmon shutdown while making request; giving up", "attempts", attempts, "error", err)
				return err
			}

			// retry after sending Stop to restart plugin or let plugin restart
			// this task's log shipper
			attempts++
			h.logger.Warn("logmon shutdown while making request; retrying", "attempts", attempts, "error", err)
			h.pluginManager.Stop(h.pluginName, h.config)
			time.Sleep(1 * time.Second)
			continue
		}
		return err
	}
}

func (h *loggingHook) isLoggingDisabled() bool {
	ic, ok := h.runner.driver.(drivers.InternalCapabilitiesDriver)
	if !ok {
		return false
	}

	caps := ic.InternalCapabilities()
	return caps.DisableLogCollection
}

func (h *loggingHook) prestartOneLoop(ctx context.Context, req *interfaces.TaskPrestartRequest) error {
	err := h.pluginManager.Start(h.pluginName, h.config)
	if err != nil {
		h.logger.Error("failed to start logging plugin", "error", err)
		return err
	}

	return nil
}

func (h *loggingHook) Stop(_ context.Context, req *interfaces.TaskStopRequest, _ *interfaces.TaskStopResponse) error {
	return h.pluginManager.Stop(h.pluginName, h.config)
}
