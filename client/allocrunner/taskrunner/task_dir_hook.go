// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"strings"

	log "github.com/hashicorp/go-hclog"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	cconfig "github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
)

const (
	// TaskDirHookIsDoneDataKey is used to mark whether the hook is done. We
	// do not use the Done response value because we still need to set the
	// environment variables every time a task starts.
	// TODO(0.9.1): Use the resp.Env map and switch to resp.Done. We need to
	// remove usage of the envBuilder
	TaskDirHookIsDoneDataKey = "is_done"
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
	fsi := h.runner.driverCapabilities.FSIsolation
	if v, ok := req.PreviousState[TaskDirHookIsDoneDataKey]; ok && v == "true" {
		setEnvvars(h.runner.envBuilder, fsi, h.runner.taskDir, h.runner.clientConfig)
		resp.State = map[string]string{
			TaskDirHookIsDoneDataKey: "true",
		}
		return nil
	}

	cc := h.runner.clientConfig
	chroot := cconfig.DefaultChrootEnv
	if len(cc.ChrootEnv) > 0 {
		chroot = cc.ChrootEnv
	}

	// Emit the event that we are going to be building the task directory
	h.runner.EmitEvent(structs.NewTaskEvent(structs.TaskSetup).SetMessage(structs.TaskBuildingTaskDir))

	// Build the task directory structure
	err := h.runner.taskDir.Build(fsi == drivers.FSIsolationChroot, chroot)
	if err != nil {
		return err
	}

	// Update the environment variables based on the built task directory
	setEnvvars(h.runner.envBuilder, fsi, h.runner.taskDir, h.runner.clientConfig)
	resp.State = map[string]string{
		TaskDirHookIsDoneDataKey: "true",
	}
	return nil
}

// setEnvvars sets path and host env vars depending on the FS isolation used.
func setEnvvars(envBuilder *taskenv.Builder, fsi drivers.FSIsolation, taskDir *allocdir.TaskDir, conf *cconfig.Config) {

	envBuilder.SetClientTaskRoot(taskDir.Dir)
	envBuilder.SetClientSharedAllocDir(taskDir.SharedAllocDir)
	envBuilder.SetClientTaskLocalDir(taskDir.LocalDir)
	envBuilder.SetClientTaskSecretsDir(taskDir.SecretsDir)

	// Set driver-specific environment variables
	switch fsi {
	case drivers.FSIsolationNone:
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
	if fsi != drivers.FSIsolationImage {
		// COMPAT(1.0) using inclusive language, blacklist is kept for backward compatibility.
		filter := strings.Split(conf.ReadAlternativeDefault(
			[]string{"env.denylist", "env.blacklist"},
			cconfig.DefaultEnvDenylist,
		), ",")
		envBuilder.SetHostEnvvars(filter)
	}
}
