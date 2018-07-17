package taskrunner

import (
	"context"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunnerv2/interfaces"
	cconfig "github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver"
	"github.com/hashicorp/nomad/nomad/structs"
)

type taskDirHook struct {
	runner *TaskRunner
	logger log.Logger
}

func newTaskDirHook(runner *TaskRunner, logger log.Logger) *taskDirHook {
	td := &taskDirHook{
		runner: runner,
	}
	td.logger = logger.Named(td.Name())
	return td
}

func (h *taskDirHook) Name() string {
	return "task_dir"
}

func (h *taskDirHook) Prestart(ctx context.Context, req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {
	cc := h.runner.clientConfig
	chroot := cconfig.DefaultChrootEnv
	if len(cc.ChrootEnv) > 0 {
		chroot = cc.ChrootEnv
	}

	// Emit the event that we are going to be building the task directory
	h.runner.EmitEvent(structs.NewTaskEvent(structs.TaskSetup).SetMessage(structs.TaskBuildingTaskDir))

	// Build the task directory structure
	fsi := h.runner.driver.FSIsolation()
	err := h.runner.taskDir.Build(false, chroot, fsi)
	if err != nil {
		return err
	}

	// Update the environment variables based on the built task directory
	driver.SetEnvvars(h.runner.envBuilder, fsi, h.runner.taskDir, h.runner.clientConfig)
	resp.Done = true
	return nil
}
