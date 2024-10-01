// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package docker

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/armon/circbuf"
	containerapi "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/hashicorp/consul-template/signals"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/lib/cgroupslib"
	"github.com/hashicorp/nomad/drivers/docker/docklog"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/plugins/drivers"
	pstructs "github.com/hashicorp/nomad/plugins/shared/structs"
)

type taskHandle struct {
	// dockerClient is useful for normal docker API calls. It should be used
	// for all calls that aren't Wait() or Stop() (and their variations).
	dockerClient *client.Client

	dockerCGroupDriver string

	// infinityClient is useful for
	// - the Wait docker API call(s) (no limit on container lifetime)
	// - the Stop docker API call(s) (context with task kill_timeout required)
	// Do not use this client for any other docker API calls, instead use the
	// normal dockerClient which includes a default timeout.
	infinityClient *client.Client

	logger                  hclog.Logger
	dlogger                 docklog.DockerLogger
	dloggerPluginClient     *plugin.Client
	task                    *drivers.TaskConfig
	containerID             string
	containerCgroup         string
	containerImage          string
	doneCh                  chan bool
	waitCh                  chan struct{}
	removeContainerOnExit   bool
	net                     *drivers.DriverNetwork
	disableCpusetManagement bool

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
	createExecOpts := containerapi.ExecOptions{
		AttachStdin:  false,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          false,
		Cmd:          fullCmd,
	}
	exec, err := h.dockerClient.ContainerExecCreate(ctx, h.containerID, createExecOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create exec object: %v", err)
	}

	execResult := &drivers.ExecTaskResult{ExitResult: &drivers.ExitResult{}}
	stdout, _ := circbuf.NewBuffer(int64(drivers.CheckBufSize))
	stderr, _ := circbuf.NewBuffer(int64(drivers.CheckBufSize))
	startOpts := containerapi.ExecStartOptions{
		Detach: false,
		Tty:    false,
	}

	// hijack exec output streams
	hijacked, err := h.dockerClient.ContainerExecAttach(ctx, exec.ID, startOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to attach to exec object: %w", err)
	}

	_, err = stdcopy.StdCopy(stdout, stderr, hijacked.Reader)
	if err != nil {
		return nil, err
	}
	defer hijacked.Close()

	execResult.Stdout = stdout.Bytes()
	execResult.Stderr = stderr.Bytes()
	res, err := h.dockerClient.ContainerExecInspect(ctx, exec.ID)
	if err != nil {
		return execResult, fmt.Errorf("failed to inspect exit code of exec object: %w", err)
	}

	execResult.ExitResult.ExitCode = res.ExitCode
	return execResult, nil
}

func (h *taskHandle) Signal(ctx context.Context, s string) error {
	_, err := signals.Parse(s)
	if err != nil {
		return fmt.Errorf("failed to parse signal: %v", err)
	}

	return h.dockerClient.ContainerKill(ctx, h.containerID, s)
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
		apiTimeout := int(killTimeout.Seconds())
		err = h.infinityClient.ContainerStop(ctx, h.containerID, containerapi.StopOptions{Timeout: pointer.Of(apiTimeout)})
	} else {
		_, parseErr := parseSignal(runtime.GOOS, signal)
		if parseErr != nil {
			return fmt.Errorf("failed to parse signal: %v", parseErr)
		}

		ctx, cancel := context.WithTimeout(context.Background(), killTimeout)
		defer cancel()

		if err := h.Signal(ctx, signal); err != nil {
			// Container has already been removed.
			if errdefs.IsNotFound(err) {
				h.logger.Debug("attempted to signal nonexistent container")
				return nil
			}
			// Container has already been stopped.
			if errdefs.IsNotModified(err) {
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
		err = h.dockerClient.ContainerStop(context.Background(), h.containerID, containerapi.StopOptions{Timeout: pointer.Of(0)})
	}

	if err != nil {
		// Container has already been removed.
		if errdefs.IsNotFound(err) {
			h.logger.Debug("attempted to stop nonexistent container")
			return nil
		}
		// Container has already been stopped.
		if errdefs.IsNotModified(err) {
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

func (h *taskHandle) startCpusetFixer() {
	if cgroupslib.GetMode() == cgroupslib.OFF || h.disableCpusetManagement {
		return
	}

	if h.task.Resources.LinuxResources.CpusetCpus != "" {
		// nothing to fixup if the task is given static cores
		return
	}

	go (&cpuset{
		doneCh:      h.doneCh,
		source:      h.task.Resources.LinuxResources.CpusetCgroupPath,
		destination: h.dockerCgroup(),
	}).watch()
}

// dockerCgroup returns the path to the cgroup docker will use for the container.
//
// The api does not provide this value, so we are left to compute it ourselves.
//
// https://docs.docker.com/config/containers/runmetrics/#find-the-cgroup-for-a-given-container
func (h *taskHandle) dockerCgroup() string {
	cgroup := h.containerCgroup
	if cgroup == "" {
		mode := cgroupslib.GetMode()
		usingCgroupfs := h.dockerCGroupDriver == "cgroupfs"
		switch {
		case mode == cgroupslib.CG1:
			cgroup = "/sys/fs/cgroup/cpuset/docker/" + h.containerID
		case mode == cgroupslib.CG2 && usingCgroupfs:
			cgroup = "/sys/fs/cgroup/docker/" + h.containerID
		default:
			cgroup = "/sys/fs/cgroup/system.slice/docker-" + h.containerID + ".scope"
		}
	}
	return cgroup
}

func (h *taskHandle) run() {
	defer h.shutdownLogger()

	h.startCpusetFixer()

	var werr error
	var exitCode containerapi.WaitResponse
	// this needs to use the background context because the container can
	// outlive Nomad itself
	exitCodeC, errC := h.infinityClient.ContainerWait(
		context.Background(), h.containerID, containerapi.WaitConditionNotRunning)

	select {
	case exitCode = <-exitCodeC:
		if exitCode.StatusCode != 0 {
			werr = fmt.Errorf("Docker container exited with non-zero exit code: %d", exitCode.StatusCode)
		}
	case werr = <-errC:
		h.logger.Error("failed to wait for container; already terminated")
	}

	ctx, inspectCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer inspectCancel()

	container, ierr := h.dockerClient.ContainerInspect(ctx, h.containerID)
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
	// incorrectly. Container should have exited by now so kill_timeout can be
	// ignored.
	ctx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer stopCancel()
	if err := h.dockerClient.ContainerStop(ctx, h.containerID, containerapi.StopOptions{
		Timeout: pointer.Of(0),
	}); err != nil {
		if !errdefs.IsNotModified(err) && !errdefs.IsNotFound(err) {
			h.logger.Error("error stopping container", "error", err)
		}
	}

	// Set the result
	h.exitResultLock.Lock()
	h.exitResult = &drivers.ExitResult{
		ExitCode:  int(exitCode.StatusCode),
		Signal:    0,
		OOMKilled: oom,
		Err:       werr,
	}
	h.exitResultLock.Unlock()
	close(h.waitCh)
}
