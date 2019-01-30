package exec

import (
	"fmt"
	"time"

	"github.com/hashicorp/nomad/client/state"
	"github.com/hashicorp/nomad/drivers/shared/executor"
	"github.com/hashicorp/nomad/plugins/drivers"
	pstructs "github.com/hashicorp/nomad/plugins/shared/structs"
)

func (d *Driver) recoverPre09Task(h *drivers.TaskHandle) error {
	handle, err := state.UnmarshalPre09HandleID(h.DriverState)
	if err != nil {
		return fmt.Errorf("failed to decode pre09 driver handle: %v", err)
	}

	reattach, err := pstructs.ReattachConfigToGoPlugin(handle.ReattachConfig())
	if err != nil {
		return fmt.Errorf("failed to decode reattach config from pre09 handle: %v", err)
	}

	exec, pluginClient, err := executor.ReattachToPre09Executor(reattach,
		d.logger.With("task_name", h.Config.Name, "alloc_id", h.Config.AllocID))
	if err != nil {
		d.logger.Error("failed to reattach to executor", "error", err, "task_name", h.Config.Name)
		return fmt.Errorf("failed to reattach to executor: %v", err)
	}

	th := &taskHandle{
		exec:         exec,
		pid:          reattach.Pid,
		pluginClient: pluginClient,
		taskConfig:   h.Config,
		procState:    drivers.TaskStateRunning,
		startedAt:    time.Now(),
		exitResult:   &drivers.ExitResult{},
	}

	d.tasks.Set(h.Config.ID, th)

	go th.run()
	return nil
}
