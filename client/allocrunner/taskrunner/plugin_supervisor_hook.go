package taskrunner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	ti "github.com/hashicorp/nomad/client/allocrunner/taskrunner/interfaces"
	"github.com/hashicorp/nomad/client/pluginregistry"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/csi"
	"github.com/hashicorp/nomad/plugins/drivers"
)

// csiPluginSupervisorHook manages supervising plugins that are running as Nomad
// tasks. These plugins will be fingerprinted and it will manage connecting them
// to their requisite plugin manager.
//
// It provides a couple of things to a task running inside Nomad. These are:
// * A mount to the `plugin_mount_dir`, that will then be used by Nomad
//   to connect to the nested plugin and handle volume mounts.
//
// When the task has started, it starts a loop of attempting to connect to the
// plugin, to perform initial fingerprinting of the plugins capabilities before
// notifying the plugin manager of the plugin.
type csiPluginSupervisorHook struct {
	logger     hclog.Logger
	alloc      *structs.Allocation
	task       *structs.Task
	runner     *TaskRunner
	mountPoint string

	// eventEmitter is used to emit events to the task
	eventEmitter ti.EventEmitter

	shutdownCtx         context.Context
	shutdownCancelFn    context.CancelFunc
	running             bool
	runningLock         sync.Mutex
	previousHealthState bool
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

// pluginStartTimeout is the amount of time after a plugin task has started that
// nomad will wait for the plugin to start responding correctly before returning
// an error.
const pluginStartTimeout = 2 * time.Minute

func newCSIPluginSupervisorHook(csiRootDir string, eventEmitter ti.EventEmitter, runner *TaskRunner, logger hclog.Logger) *csiPluginSupervisorHook {
	task := runner.Task()
	pluginRoot := filepath.Join(csiRootDir, string(task.CSIPluginConfig.PluginType), task.CSIPluginConfig.PluginID)

	shutdownCtx, cancelFn := context.WithCancel(context.Background())

	hook := &csiPluginSupervisorHook{
		alloc:            runner.Alloc(),
		runner:           runner,
		logger:           logger,
		task:             task,
		mountPoint:       pluginRoot,
		shutdownCtx:      shutdownCtx,
		shutdownCancelFn: cancelFn,
		eventEmitter:     eventEmitter,
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
	if err := os.MkdirAll(h.mountPoint, 0750); err != nil && !os.IsExist(err) {
		return fmt.Errorf("failed to create mount point: %v", err)
	}

	configMount := &drivers.MountConfig{
		TaskPath:        h.task.CSIPluginConfig.PluginMountDir,
		HostPath:        h.mountPoint,
		Readonly:        false,
		PropagationMode: "bidirectional",
	}

	mounts := ensureMountpointInserted(h.runner.hookResources.getMounts(), configMount)
	h.runner.hookResources.setMounts(mounts)

	resp.Done = true
	return nil
}

// Poststart is called after the task has started. Poststart is not
// called if the allocation is terminal.
//
// The context is cancelled if the task is killed.
func (h *csiPluginSupervisorHook) Poststart(_ context.Context, _ *interfaces.TaskPoststartRequest, _ *interfaces.TaskPoststartResponse) error {
	// If we're already running the supervisor routine, then we don't need to try
	// and restart it here as it only terminates on `Stop` hooks.
	h.runningLock.Lock()
	if h.running == true {
		h.runningLock.Unlock()
		return nil
	}
	h.runningLock.Unlock()

	go h.ensureSupervisorLoop(h.shutdownCtx)
	return nil
}

// ensureSupervisorLoop starts a goroutine that will runs on an interval until
// the passed context is cancelled.
//
// The supervisor works by:
// - Initially waiting for the plugin to become available. This loop is expensive
//   and may do things like create new gRPC Clients on every iteration.
// - After recieving an initial healthy status, it will inform the plugin catalog
//   of the plugin, registering it with the plugins fingerprinted capabilities.
// - We then perform a more lightweight check, simply probing the plugin on a less
//   frequent interval to ensure it is still alive, emiting task events when this
//   status changes.
//
// Deeper fingerprinting of the plugin is not yet implemented, but may happen here
// or in the plugin catalog in the future.
func (h *csiPluginSupervisorHook) ensureSupervisorLoop(ctx context.Context) {
	h.runningLock.Lock()
	if h.running == true {
		h.runningLock.Unlock()
		return
	}
	h.running = true
	h.runningLock.Unlock()

	defer func() {
		h.runningLock.Lock()
		h.running = false
		h.runningLock.Unlock()
	}()

	socketPath := filepath.Join(h.mountPoint, structs.CSISocketName)
	t := time.NewTimer(0)

	// Step 1: Wait for the plugin to initially become available.
WAITFORREADY:
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			pluginHealthy, err := h.supervisorLoopOnce(ctx, socketPath)
			if err != nil || !pluginHealthy {
				h.logger.Info("CSI Plugin not ready", "error", err)
				t.Reset(5 * time.Second)
				continue
			}

			// Mark the plugin as healthy in a task event
			h.previousHealthState = pluginHealthy
			event := structs.NewTaskEvent(structs.TaskPluginHealthy)
			event.SetMessage(fmt.Sprintf("plugin: %s", h.task.CSIPluginConfig.PluginID))
			h.eventEmitter.EmitEvent(event)

			break WAITFORREADY
		}
	}

	// Step 2: Register the plugin with the catalog.
	info := &pluginregistry.PluginInfo{
		Type:    "csi",
		Name:    h.task.CSIPluginConfig.PluginID,
		Version: "1.0.0",
		ConnectionInfo: &pluginregistry.PluginConnectionInfo{
			SocketPath: socketPath,
		},
	}
	if err := h.runner.pluginRegistry.RegisterPlugin(info); err != nil {
		h.logger.Error("CSI Plugin registration failed", "error", err)
		event := structs.NewTaskEvent(structs.TaskPluginUnhealthy)
		event.SetMessage(fmt.Sprintf("failed to register plugin: %s, reason: %v", h.task.CSIPluginConfig.PluginID, err))
		h.eventEmitter.EmitEvent(event)
		return
	}

	// Step 3: Start the lightweight supervisor loop.
	t.Reset(0)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			pluginHealthy, err := h.supervisorLoopOnce(ctx, socketPath)
			if err != nil {
				h.logger.Error("CSI Plugin fingerprinting failed", "error", err)
			}

			// The plugin has transitioned to a healthy state. Emit an event and inform
			// the plugin catalog of the state.
			if h.previousHealthState == false && pluginHealthy {
				event := structs.NewTaskEvent(structs.TaskPluginHealthy)
				event.SetMessage(fmt.Sprintf("plugin: %s", h.task.CSIPluginConfig.PluginID))
				h.eventEmitter.EmitEvent(event)
			}

			// The plugin has transitioned to an unhealthy state. Emit an event and inform
			// the plugin catalog of the state.
			if h.previousHealthState == true && !pluginHealthy {
				event := structs.NewTaskEvent(structs.TaskPluginUnhealthy)
				if err != nil {
					event.SetMessage(fmt.Sprintf("error: %v", err))
				} else {
					event.SetMessage("Unknown Reason")
				}
				h.eventEmitter.EmitEvent(event)
			}

			h.previousHealthState = pluginHealthy
			t.Reset(30 * time.Second)
		}
	}
}

func (h *csiPluginSupervisorHook) supervisorLoopOnce(ctx context.Context, socketPath string) (bool, error) {
	_, err := os.Stat(socketPath)
	if err != nil {
		return false, fmt.Errorf("failed to stat socket: %v", err)
	}

	client, err := csi.NewClient(socketPath)
	defer client.Close()
	if err != nil {
		return false, fmt.Errorf("failed to create csi client: %v", err)
	}

	healthy, err := client.PluginProbe(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to probe plugin: %v", err)
	}

	return healthy, nil
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
	h.shutdownCancelFn()
	return nil
}

func ensureMountpointInserted(mounts []*drivers.MountConfig, mount *drivers.MountConfig) []*drivers.MountConfig {
	for _, mnt := range mounts {
		if mnt.IsEqual(mount) {
			return mounts
		}
	}

	mounts = append(mounts, mount)
	return mounts
}
