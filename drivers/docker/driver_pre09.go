package docker

import (
	"fmt"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/nomad/client/state"
	"github.com/hashicorp/nomad/drivers/docker/docklog"
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

	client, _, err := d.dockerClients()
	if err != nil {
		return fmt.Errorf("failed to get docker client: %v", err)
	}

	container, err := client.InspectContainerWithOptions(docker.InspectContainerOptions{
		ID: handle.ContainerID,
	})
	if err != nil {
		return fmt.Errorf("failed to inspect container for id %q: %v", handle.ContainerID, err)
	}

	th := &taskHandle{
		client:                client,
		waitClient:            waitClient,
		dlogger:               &executorDockerLoggerShim{exec: exec},
		dloggerPluginClient:   pluginClient,
		logger:                d.logger.With("container_id", container.ID),
		task:                  h.Config,
		containerID:           container.ID,
		containerImage:        container.Image,
		doneCh:                make(chan bool),
		waitCh:                make(chan struct{}),
		removeContainerOnExit: d.config.GC.Container,
	}

	d.tasks.Set(h.Config.ID, th)
	go th.run()
	return nil
}

// executorDockerLoggerShim is used by upgraded tasks as the docker logger. When
// the task exits, the Stop() func of the docker logger is called, this shim
// will proxy that call to the executor Shutdown() func which will stop the
// syslog server started by the pre09 executor
type executorDockerLoggerShim struct {
	exec executor.Executor
}

func (e *executorDockerLoggerShim) Start(*docklog.StartOpts) error { return nil }
func (e *executorDockerLoggerShim) Stop() error {
	if err := e.exec.Shutdown("docker", 0); err != nil {
		return err
	}

	return nil
}
