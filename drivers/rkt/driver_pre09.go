package rkt

import (
	"fmt"
	"time"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/drivers/shared/executor"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/plugins/drivers"
)

func (d *Driver) recoverPre09Task(config *drivers.TaskConfig, reattach *plugin.ReattachConfig) error {
	config.ID = fmt.Sprintf("pre09-%s", uuid.Generate())
	exec, pluginClient, err := executor.ReattachToPre09Executor(reattach,
		d.logger.With("task_name", config.Name, "alloc_id", config.AllocID))
	if err != nil {
		d.logger.Error("failed to reattach to executor", "error", err, "task_name", config.Name)
		return fmt.Errorf("failed to reattach to executor: %v", err)
	}

	h := &taskHandle{
		exec:         exec,
		pid:          reattach.Pid,
		pluginClient: pluginClient,
		taskConfig:   config,
		procState:    drivers.TaskStateRunning,
		startedAt:    time.Now(),
		exitResult:   &drivers.ExitResult{},
	}

	d.tasks.Set(config.ID, h)

	go h.run()
	return nil
}
