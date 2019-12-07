package docker

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/armon/circbuf"
	docker "github.com/fsouza/go-dockerclient"
	hclog "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/drivers/docker/docklog"
	"github.com/hashicorp/nomad/plugins/drivers"
	pstructs "github.com/hashicorp/nomad/plugins/shared/structs"
	"golang.org/x/net/context"
)

type taskHandle struct {
	client                *docker.Client
	waitClient            *docker.Client
	logger                hclog.Logger
	dlogger               docklog.DockerLogger
	dloggerPluginClient   *plugin.Client
	task                  *drivers.TaskConfig
	containerID           string
	containerImage        string
	doneCh                chan bool
	waitCh                chan struct{}
	removeContainerOnExit bool
	net                   *drivers.DriverNetwork

	exitResult     *drivers.ExitResult
	exitResultLock sync.Mutex
}

func (h *taskHandle) ExitResult() *drivers.ExitResult {
	h.exitResultLock.Lock()
	defer h.exitResultLock.Unlock()
	return h.exitResult.Copy()
}

type taskHandleState struct {
	// ReattachConfig for the docker logger plugin
	ReattachConfig *pstructs.ReattachConfig

	ContainerID   string
	DriverNetwork *drivers.DriverNetwork
}

func (h *taskHandle) buildState() *taskHandleState {
	s := &taskHandleState{
		ContainerID:   h.containerID,
		DriverNetwork: h.net,
	}
	if h.dloggerPluginClient != nil {
		s.ReattachConfig = pstructs.ReattachConfigFromGoPlugin(h.dloggerPluginClient.ReattachConfig())
	}
	return s
}

func (h *taskHandle) Exec(ctx context.Context, cmd string, args []string) (*drivers.ExecTaskResult, error) {
	fullCmd := make([]string, len(args)+1)
	fullCmd[0] = cmd
	copy(fullCmd[1:], args)
	createExecOpts := docker.CreateExecOptions{
		AttachStdin:  false,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          false,
		Cmd:          fullCmd,
		Container:    h.containerID,
		Context:      ctx,
	}
	exec, err := h.client.CreateExec(createExecOpts)
	if err != nil {
		return nil, err
	}

	execResult := &drivers.ExecTaskResult{ExitResult: &drivers.ExitResult{}}
	stdout, _ := circbuf.NewBuffer(int64(drivers.CheckBufSize))
	stderr, _ := circbuf.NewBuffer(int64(drivers.CheckBufSize))
	startOpts := docker.StartExecOptions{
		Detach:       false,
		Tty:          false,
		OutputStream: stdout,
		ErrorStream:  stderr,
		Context:      ctx,
	}
	if err := client.StartExec(exec.ID, startOpts); err != nil {
		return nil, err
	}
	execResult.Stdout = stdout.Bytes()
	execResult.Stderr = stderr.Bytes()
	res, err := client.InspectExec(exec.ID)
	if err != nil {
		return execResult, err
	}

	execResult.ExitResult.ExitCode = res.ExitCode
	return execResult, nil
}

func (h *taskHandle) Signal(s os.Signal) error {
	// Convert types
	sysSig, ok := s.(syscall.Signal)
	if !ok {
		return fmt.Errorf("Failed to determine signal number")
	}

	// TODO When we expose signals we will need a mapping layer that converts
	// MacOS signals to the correct signal number for docker. Or we change the
	// interface to take a signal string and leave it up to driver to map?

	dockerSignal := docker.Signal(sysSig)
	opts := docker.KillContainerOptions{
		ID:     h.containerID,
		Signal: dockerSignal,
	}
	return h.client.KillContainer(opts)

}

// Kill is used to terminate the task.
func (h *taskHandle) Kill(killTimeout time.Duration, signal os.Signal) error {
	// Only send signal if killTimeout is set, otherwise stop container
	if killTimeout > 0 {
		if err := h.Signal(signal); err != nil {
			// Container has already been removed.
			if strings.Contains(err.Error(), NoSuchContainerError) {
				h.logger.Debug("attempted to signal nonexistent container")
				return nil
			}
			// Container has already been stopped.
			if strings.Contains(err.Error(), ContainerNotRunningError) {
				h.logger.Debug("attempted to signal a not-running container")
				return nil
			}

			h.logger.Error("failed to signal container while killing", "error", err)
			return fmt.Errorf("Failed to signal container %q while killing: %v", h.containerID, err)
		}

		select {
		case <-h.waitCh:
			return nil
		case <-time.After(killTimeout):
		}
	}

	// Stop the container
	err := h.client.StopContainer(h.containerID, 0)
	if err != nil {

		// Container has already been removed.
		if strings.Contains(err.Error(), NoSuchContainerError) {
			h.logger.Debug("attempted to stop nonexistent container")
			return nil
		}
		// Container has already been stopped.
		if strings.Contains(err.Error(), ContainerNotRunningError) {
			h.logger.Debug("attempted to stop an not-running container")
			return nil
		}

		h.logger.Error("failed to stop container", "error", err)
		return fmt.Errorf("Failed to stop container %s: %s", h.containerID, err)
	}

	h.logger.Info("stopped container")
	return nil
}

func (h *taskHandle) shutdownLogger() {
	if h.dlogger == nil {
		return
	}

	if err := h.dlogger.Stop(); err != nil {
		h.logger.Error("failed to stop docker logger process during StopTask",
			"error", err, "logger_pid", h.dloggerPluginClient.ReattachConfig().Pid)
	}
	h.dloggerPluginClient.Kill()
}

func (h *taskHandle) run() {
	defer h.shutdownLogger()

	exitCode, werr := h.waitClient.WaitContainer(h.containerID)
	if werr != nil {
		h.logger.Error("failed to wait for container; already terminated")
	}

	if exitCode != 0 {
		werr = fmt.Errorf("Docker container exited with non-zero exit code: %d", exitCode)
	}

	container, ierr := h.waitClient.InspectContainer(h.containerID)
	oom := false
	if ierr != nil {
		h.logger.Error("failed to inspect container", "error", ierr)
	} else if container.State.OOMKilled {
		oom = true
		werr = fmt.Errorf("OOM Killed")
	}

	// Shutdown stats collection
	close(h.doneCh)

	// Stop the container just incase the docker daemon's wait returned
	// incorrectly
	if err := h.client.StopContainer(h.containerID, 0); err != nil {
		_, noSuchContainer := err.(*docker.NoSuchContainer)
		_, containerNotRunning := err.(*docker.ContainerNotRunning)
		if !containerNotRunning && !noSuchContainer {
			h.logger.Error("error stopping container", "error", err)
		}
	}

	// Set the result
	h.exitResultLock.Lock()
	h.exitResult = &drivers.ExitResult{
		ExitCode:  exitCode,
		Signal:    0,
		OOMKilled: oom,
		Err:       werr,
	}
	h.exitResultLock.Unlock()
	close(h.waitCh)
}
