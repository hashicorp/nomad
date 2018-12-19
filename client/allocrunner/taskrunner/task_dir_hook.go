package taskrunner

import (
	"context"
	"strings"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	cconfig "github.com/hashicorp/nomad/client/config"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/client/taskenv"
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
	// Copied in client/state when upgrading from <0.9 schemas, so if you
	// change it here you also must change it there.
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
	fsi := h.runner.driverCapabilities.FSIsolation
	err := h.runner.taskDir.Build(false, chroot, fsi)
	if err != nil {
		return err
	}

	// Update the environment variables based on the built task directory
	setEnvvars(h.runner.envBuilder, fsi, h.runner.taskDir, h.runner.clientConfig)
	resp.Done = true
	return nil
}

// setEnvvars sets path and host env vars depending on the FS isolation used.
func setEnvvars(envBuilder *taskenv.Builder, fsi cstructs.FSIsolation, taskDir *allocdir.TaskDir, conf *cconfig.Config) {
	// Set driver-specific environment variables
	switch fsi {
	case cstructs.FSIsolationNone:
		// Use host paths
		envBuilder.SetAllocDir(taskDir.SharedAllocDir)
		envBuilder.SetTaskLocalDir(taskDir.LocalDir)
		envBuilder.SetSecretsDir(taskDir.SecretsDir)
	default:
		// filesystem isolation; use container paths
		envBuilder.SetAllocDir(allocdir.SharedAllocContainerPath)
		envBuilder.SetTaskLocalDir(allocdir.TaskLocalContainerPath)
		envBuilder.SetSecretsDir(allocdir.TaskSecretsContainerPath)
	}

	// Set the host environment variables for non-image based drivers
	if fsi != cstructs.FSIsolationImage {
		filter := strings.Split(conf.ReadDefault("env.blacklist", cconfig.DefaultEnvBlacklist), ",")
		envBuilder.SetHostEnvvars(filter)
	}
}
