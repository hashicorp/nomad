// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package docker

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/armon/circbuf"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/consul-template/signals"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"

	"github.com/hashicorp/nomad/drivers/docker/docklog"
	"github.com/hashicorp/nomad/plugins/drivers"
	pstructs "github.com/hashicorp/nomad/plugins/shared/structs"
)

type taskHandle struct {
	// dockerClient is useful for normal docker API calls. It should be used
	// for all calls that aren't Wait() or Stop() (and their variations).
	dockerClient *docker.Client

	// infinityClient is useful for
	// - the Wait docker API call(s) (no limit on container lifetime)
	// - the Stop docker API call(s) (context with task kill_timeout required)
	// Do not use this client for any other docker API calls, instead use the
	// normal dockerClient which includes a default timeout.
	infinityClient *docker.Client

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
	exec, err := h.dockerClient.CreateExec(createExecOpts)
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
	if err := h.dockerClient.StartExec(exec.ID, startOpts); err != nil {
		return nil, err
	}
	execResult.Stdout = stdout.Bytes()
	execResult.Stderr = stderr.Bytes()
	res, err := h.dockerClient.InspectExec(exec.ID)
	if err != nil {
		return execResult, err
	}

	execResult.ExitResult.ExitCode = res.ExitCode
	return execResult, nil
}

func (h *taskHandle) Signal(ctx context.Context, s os.Signal) error {
	// Convert types
	sysSig, ok := s.(syscall.Signal)
	if !ok {
		return fmt.Errorf("Failed to determine signal number")
	}

	// TODO When we expose signals we will need a mapping layer that converts
	// MacOS signals to the correct signal number for docker. Or we change the
	// interface to take a signal string and leave it up to driver to map?

	opts := docker.KillContainerOptions{
		ID:      h.containerID,
		Signal:  docker.Signal(sysSig),
		Context: ctx,
	}

	// remember Kill just means send a signal; this is not the complex StopContainer case
	return h.dockerClient.KillContainer(opts)
}

// parseSignal interprets the signal name into an os.Signal. If no name is
// provided, the docker driver defaults to SIGTERM. If the OS is Windows and
// SIGINT is provided, the signal is converted to SIGTERM.
func parseSignal(os, signal string) (os.Signal, error) {
	// Unlike other drivers, docker defaults to SIGTERM, aiming for consistency
	// with the 'docker stop' command.
	// https://docs.docker.com/engine/reference/commandline/stop/#extended-description
	if signal == "" {
		signal = "SIGTERM"
	}

	// Windows Docker daemon does not support SIGINT, SIGTERM is the semantic equivalent that
	// allows for graceful shutdown before being followed up by a SIGKILL.
	// Supported signals:
	//   https://github.com/moby/moby/blob/0111ee70874a4947d93f64b672f66a2a35071ee2/pkg/signal/signal_windows.go#L17-L26
	if os == "windows" && signal == "SIGINT" {
		signal = "SIGTERM"
	}

	return signals.Parse(signal)
}

// Kill is used to terminate the task.
func (h *taskHandle) Kill(killTimeout time.Duration, signal string) error {
	var err error
	// Calling StopContainer lets docker handle the stop signal (specified
	// in the Dockerfile or defaulting to SIGTERM). If kill_signal is specified,
	// Signal is used to kill the container with the desired signal before
	// calling StopContainer
	if signal == "" {
		// give the context timeout some wiggle room beyond the kill timeout
		// docker will use, so we can happy path even in the force kill case
		graciousTimeout := killTimeout + dockerTimeout
		ctx, cancel := context.WithTimeout(context.Background(), graciousTimeout)
		defer cancel()
		apiTimeout := uint(killTimeout.Seconds())
		err = h.infinityClient.StopContainerWithContext(h.containerID, apiTimeout, ctx)
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), killTimeout)
		defer cancel()

		sig, parseErr := parseSignal(runtime.GOOS, signal)
		if parseErr != nil {
			return fmt.Errorf("failed to parse signal: %v", parseErr)
		}

		if err := h.Signal(ctx, sig); err != nil {
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
		case <-ctx.Done():
		}

		// Stop the container forcefully.
		err = h.dockerClient.StopContainer(h.containerID, 0)
	}

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

	exitCode, werr := h.infinityClient.WaitContainer(h.containerID)
	if werr != nil {
		h.logger.Error("failed to wait for container; already terminated")
	}

	if exitCode != 0 {
		werr = fmt.Errorf("Docker container exited with non-zero exit code: %d", exitCode)
	}

	container, ierr := h.dockerClient.InspectContainerWithOptions(docker.InspectContainerOptions{
		ID: h.containerID,
	})
	oom := false
	if ierr != nil {
		h.logger.Error("failed to inspect container", "error", ierr)
	} else if container.State.OOMKilled {
		h.logger.Error("OOM Killed",
			"container_id", h.containerID,
			"container_image", h.containerImage,
			"nomad_job_name", h.task.JobName,
			"nomad_task_name", h.task.Name,
			"nomad_alloc_id", h.task.AllocID)

		// Note that with cgroups.v2 the cgroup OOM killer is not
		// observed by docker container status. But we can't test the
		// exit code, as 137 is used for any SIGKILL
		oom = true
		werr = fmt.Errorf("OOM Killed")
	}

	// Shutdown stats collection
	close(h.doneCh)

	// Stop the container just incase the docker daemon's wait returned
	// incorrectly.
	if err := h.dockerClient.StopContainer(h.containerID, 0); err != nil {
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
