package docker

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/armon/circbuf"
	metrics "github.com/armon/go-metrics"
	docker "github.com/fsouza/go-dockerclient"
	hclog "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/drivers/docker/docklog"
	"github.com/hashicorp/nomad/helper/stats"
	"github.com/hashicorp/nomad/plugins/drivers"
	"golang.org/x/net/context"
)

type taskHandle struct {
	client                *docker.Client
	waitClient            *docker.Client
	logger                hclog.Logger
	dlogger               docklog.DockerLogger
	dloggerPluginClient   *plugin.Client
	task                  *drivers.TaskConfig
	container             *docker.Container
	resourceUsageLock     sync.RWMutex
	resourceUsage         *structs.TaskResourceUsage
	doneCh                chan bool
	waitCh                chan struct{}
	removeContainerOnExit bool
	net                   *structs.DriverNetwork
	imageID               string

	exitResult *drivers.ExitResult
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
		Container:    h.container.ID,
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
		ID:     h.container.ID,
		Signal: dockerSignal,
	}
	return h.client.KillContainer(opts)

}

// Kill is used to terminate the task.
func (h *taskHandle) Kill(killTimeout time.Duration, signal os.Signal) error {
	// Only send signal if killTimeout is set, otherwise stop container
	if killTimeout > 0 {
		if err := h.Signal(signal); err != nil {
			return err
		}
		select {
		case <-h.waitCh:
			return nil
		case <-time.After(killTimeout):
		}
	}

	// Stop the container
	err := h.client.StopContainer(h.container.ID, 0)
	if err != nil {

		// Container has already been removed.
		if strings.Contains(err.Error(), NoSuchContainerError) {
			h.logger.Debug("attempted to stop nonexistent container")
			return nil
		}
		h.logger.Error("failed to stop container", "error", err)
		return fmt.Errorf("Failed to stop container %s: %s", h.container.ID, err)
	}
	h.logger.Info("stopped container")
	return nil
}

func (h *taskHandle) Stats() (*structs.TaskResourceUsage, error) {
	h.resourceUsageLock.RLock()
	defer h.resourceUsageLock.RUnlock()
	var err error
	if h.resourceUsage == nil {
		err = fmt.Errorf("stats collection hasn't started yet")
	}
	return h.resourceUsage, err
}

func (h *taskHandle) run() {
	exitCode, werr := h.waitClient.WaitContainer(h.container.ID)
	if werr != nil {
		h.logger.Error("failed to wait for container; already terminated")
	}

	if exitCode != 0 {
		werr = fmt.Errorf("Docker container exited with non-zero exit code: %d", exitCode)
	}

	container, ierr := h.waitClient.InspectContainer(h.container.ID)
	oom := false
	if ierr != nil {
		h.logger.Error("failed to inspect container", "error", ierr)
	} else if container.State.OOMKilled {
		oom = true
		werr = fmt.Errorf("OOM Killed")
		labels := []metrics.Label{
			{
				Name:  "job",
				Value: h.task.JobName,
			},
			{
				Name:  "task_group",
				Value: h.task.TaskGroupName,
			},
			{
				Name:  "task",
				Value: h.task.Name,
			},
		}
		metrics.IncrCounterWithLabels([]string{"driver", "docker", "oom"}, 1, labels)
	}

	close(h.doneCh)

	// Shutdown the syslog collector

	// Stop the container just incase the docker daemon's wait returned
	// incorrectly
	if err := h.client.StopContainer(h.container.ID, 0); err != nil {
		_, noSuchContainer := err.(*docker.NoSuchContainer)
		_, containerNotRunning := err.(*docker.ContainerNotRunning)
		if !containerNotRunning && !noSuchContainer {
			h.logger.Error("error stopping container", "error", err)
		}
	}

	// Remove the container
	if h.removeContainerOnExit == true {
		if err := h.client.RemoveContainer(docker.RemoveContainerOptions{ID: h.container.ID, RemoveVolumes: true, Force: true}); err != nil {
			h.logger.Error("error removing container", "error", err)
		}
	} else {
		h.logger.Debug("not removing container due to config")
	}

	// Set the result
	h.exitResult = &drivers.ExitResult{
		ExitCode:  exitCode,
		Signal:    0,
		OOMKilled: oom,
	}
	close(h.waitCh)
}

// collectStats starts collecting resource usage stats of a docker container
func (h *taskHandle) collectStats() {

	statsCh := make(chan *docker.Stats)
	statsOpts := docker.StatsOptions{ID: h.container.ID, Done: h.doneCh, Stats: statsCh, Stream: true}
	go func() {
		//TODO handle Stats error
		if err := h.waitClient.Stats(statsOpts); err != nil {
			h.logger.Debug("error collecting stats from container", "error", err)
		}
	}()
	numCores := runtime.NumCPU()
	for {
		select {
		case s := <-statsCh:
			if s != nil {
				ms := &structs.MemoryStats{
					RSS:      s.MemoryStats.Stats.Rss,
					Cache:    s.MemoryStats.Stats.Cache,
					Swap:     s.MemoryStats.Stats.Swap,
					MaxUsage: s.MemoryStats.MaxUsage,
					Measured: DockerMeasuredMemStats,
				}

				cs := &structs.CpuStats{
					ThrottledPeriods: s.CPUStats.ThrottlingData.ThrottledPeriods,
					ThrottledTime:    s.CPUStats.ThrottlingData.ThrottledTime,
					Measured:         DockerMeasuredCpuStats,
				}

				// Calculate percentage
				cs.Percent = calculatePercent(
					s.CPUStats.CPUUsage.TotalUsage, s.PreCPUStats.CPUUsage.TotalUsage,
					s.CPUStats.SystemCPUUsage, s.PreCPUStats.SystemCPUUsage, numCores)
				cs.SystemMode = calculatePercent(
					s.CPUStats.CPUUsage.UsageInKernelmode, s.PreCPUStats.CPUUsage.UsageInKernelmode,
					s.CPUStats.CPUUsage.TotalUsage, s.PreCPUStats.CPUUsage.TotalUsage, numCores)
				cs.UserMode = calculatePercent(
					s.CPUStats.CPUUsage.UsageInUsermode, s.PreCPUStats.CPUUsage.UsageInUsermode,
					s.CPUStats.CPUUsage.TotalUsage, s.PreCPUStats.CPUUsage.TotalUsage, numCores)
				cs.TotalTicks = (cs.Percent / 100) * stats.TotalTicksAvailable() / float64(numCores)

				h.resourceUsageLock.Lock()
				h.resourceUsage = &structs.TaskResourceUsage{
					ResourceUsage: &structs.ResourceUsage{
						MemoryStats: ms,
						CpuStats:    cs,
					},
					Timestamp: s.Read.UTC().UnixNano(),
				}
				h.resourceUsageLock.Unlock()
			}
		case <-h.doneCh:
			return
		}
	}
}

func calculatePercent(newSample, oldSample, newTotal, oldTotal uint64, cores int) float64 {
	numerator := newSample - oldSample
	denom := newTotal - oldTotal
	if numerator <= 0 || denom <= 0 {
		return 0.0
	}

	return (float64(numerator) / float64(denom)) * float64(cores) * 100.0
}
