package taskrunner

import (
	"context"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/nomad/structs"
)

// csiPluginSupervisorHook manages supervising plugins that are running as Nomad
// tasks. These plugins will be fingerprinted and it will manage connecting them
// to their requisite plugin manager.
//
// It provides a couple of things to a task running inside Nomad. These are:
// * A mount to the `csi_socket_path` directory, that will then be used by Nomad
//   to connect to the nested plugin.
// * A mount to the `csi_intermediary_directory` that will be used as an
//   intermediary mount point for Nomad to use for requesting volumes.
//
// When the task has started, it starts a loop of attempting to connect to the
// plugin, to perform initial fingerprinting of the plugins capabilities before
// notifying the plugin manager of the plugin.
type csiPluginSupervisorHook struct {
	logger hclog.Logger
	alloc  *structs.Allocation
	runner *TaskRunner
}

// The plugin supervisor uses the PrestartHook mechanism to setup the requisite
// mount points and configuration for the task that exposes a CSI plugin.
var _ interfaces.TaskPrestartHook = &csiPluginSupervisorHook{}

// The plugin supervisor uses the PoststartHook mechanism to start polling the
// plugin for readiness and supported functionality before registering the
// plugin with the catalog.
var _ interfaces.TaskPoststartHook = &csiPluginSupervisorHook{}

// The plugin supervisor uses the StopHook mechanism to deregister the plugin
// with the catalog and to ensure any mounts are cleaned up.
var _ interfaces.TaskStopHook = &csiPluginSupervisorHook{}

func newCSIPluginSupervisorHook(runner *TaskRunner, logger hclog.Logger) *csiPluginSupervisorHook {
	hook := &csiPluginSupervisorHook{
		alloc:  runner.Alloc(),
		runner: runner,
		logger: logger,
	}

	return hook
}

func (*csiPluginSupervisorHook) Name() string {
	return "csi_plugin_supervisor"
}

// Prestart is called before the task is started including after every
// restart. This requires that the mount paths for a plugin be idempotent,
// despite us not knowing the name of the plugin ahead of time.
// Because of this, we use the allocid_taskname as the unique identifier for a
// plugin on the filesystem.
func (h *csiPluginSupervisorHook) Prestart(ctx context.Context,
	req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {
	return nil
}

// Poststart is called after the task has started. Poststart is not
// called if the allocation is terminal.
//
// The context is cancelled if the task is killed.
func (c *csiPluginSupervisorHook) Poststart(_ context.Context, _ *interfaces.TaskPoststartRequest, _ *interfaces.TaskPoststartResponse) error {
	panic("not implemented")
}

// Stop is called after the task has exited and will not be started
// again. It is the only hook guaranteed to be executed whenever
// TaskRunner.Run is called (and not gracefully shutting down).
// Therefore it may be called even when prestart and the other hooks
// have not.
//
// Stop hooks must be idempotent. The context is cancelled if the task
// is killed.
func (h *csiPluginSupervisorHook) Stop(_ context.Context, req *interfaces.TaskStopRequest, _ *interfaces.TaskStopResponse) error {
	return nil
}
