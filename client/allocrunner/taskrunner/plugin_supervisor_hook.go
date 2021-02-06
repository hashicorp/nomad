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
	"github.com/hashicorp/nomad/client/dynamicplugins"
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
// * When the task has started, it starts a loop of attempting to connect to the
//   plugin, to perform initial fingerprinting of the plugins capabilities before
//   notifying the plugin manager of the plugin.
type csiPluginSupervisorHook struct {
	logger     hclog.Logger
	alloc      *structs.Allocation
	task       *structs.Task
	runner     *TaskRunner
	mountPoint string

	// eventEmitter is used to emit events to the task
	eventEmitter ti.EventEmitter

	shutdownCtx      context.Context
	shutdownCancelFn context.CancelFunc

	running     bool
	runningLock sync.Mutex

	// previousHealthstate is used by the supervisor goroutine to track historic
	// health states for gating task events.
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

func newCSIPluginSupervisorHook(csiRootDir string, eventEmitter ti.EventEmitter, runner *TaskRunner, logger hclog.Logger) *csiPluginSupervisorHook {
	task := runner.Task()

	// The Plugin directory will look something like this:
	// .
	// ..
	// csi.sock - A unix domain socket used to communicate with the CSI Plugin
	// staging/
	//  {volume-id}/{usage-mode-hash}/ - Intermediary mount point that will be used by plugins that support NODE_STAGE_UNSTAGE capabilities.
	// per-alloc/
	//  {alloc-id}/{volume-id}/{usage-mode-hash}/ - Mount Point that will be bind-mounted into tasks that utilise the volume
	pluginRoot := filepath.Join(csiRootDir, string(task.CSIPluginConfig.Type), task.CSIPluginConfig.ID)

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
	// Create the mount directory that the container will access if it doesn't
	// already exist. Default to only nomad user access.
	if err := os.MkdirAll(h.mountPoint, 0700); err != nil && !os.IsExist(err) {
		return fmt.Errorf("failed to create mount point: %v", err)
	}

	configMount := &drivers.MountConfig{
		TaskPath:        h.task.CSIPluginConfig.MountDir,
		HostPath:        h.mountPoint,
		Readonly:        false,
		PropagationMode: "bidirectional",
	}
	devMount := &drivers.MountConfig{
		TaskPath: "/dev",
		HostPath: "/dev",
		Readonly: false,
	}

	mounts := ensureMountpointInserted(h.runner.hookResources.getMounts(), configMount)
	mounts = ensureMountpointInserted(mounts, devMount)

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
	if h.running {
		h.runningLock.Unlock()
		return nil
	}
	h.runningLock.Unlock()

	go h.ensureSupervisorLoop(h.shutdownCtx)
	return nil
}

// ensureSupervisorLoop should be called in a goroutine. It will terminate when
// the passed in context is terminated.
//
// The supervisor works by:
// - Initially waiting for the plugin to become available. This loop is expensive
//   and may do things like create new gRPC Clients on every iteration.
// - After receiving an initial healthy status, it will inform the plugin catalog
//   of the plugin, registering it with the plugins fingerprinted capabilities.
// - We then perform a more lightweight check, simply probing the plugin on a less
//   frequent interval to ensure it is still alive, emitting task events when this
//   status changes.
//
// Deeper fingerprinting of the plugin is implemented by the csimanager.
func (h *csiPluginSupervisorHook) ensureSupervisorLoop(ctx context.Context) {
	h.runningLock.Lock()
	if h.running {
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
				h.logger.Debug("CSI Plugin not ready", "error", err)

				// Plugin is not yet returning healthy, because we want to optimise for
				// quickly bringing a plugin online, we use a short timeout here.
				// TODO(dani): Test with more plugins and adjust.
				t.Reset(5 * time.Second)
				continue
			}

			// Mark the plugin as healthy in a task event
			h.previousHealthState = pluginHealthy
			event := structs.NewTaskEvent(structs.TaskPluginHealthy)
			event.SetMessage(fmt.Sprintf("plugin: %s", h.task.CSIPluginConfig.ID))
			h.eventEmitter.EmitEvent(event)

			break WAITFORREADY
		}
	}

	// Step 2: Register the plugin with the catalog.
	deregisterPluginFn, err := h.registerPlugin(socketPath)
	if err != nil {
		h.logger.Error("CSI Plugin registration failed", "error", err)
		event := structs.NewTaskEvent(structs.TaskPluginUnhealthy)
		event.SetMessage(fmt.Sprintf("failed to register plugin: %s, reason: %v", h.task.CSIPluginConfig.ID, err))
		h.eventEmitter.EmitEvent(event)
	}

	// Step 3: Start the lightweight supervisor loop.
	t.Reset(0)
	for {
		select {
		case <-ctx.Done():
			// De-register plugins on task shutdown
			deregisterPluginFn()
			return
		case <-t.C:
			pluginHealthy, err := h.supervisorLoopOnce(ctx, socketPath)
			if err != nil {
				h.logger.Error("CSI Plugin fingerprinting failed", "error", err)
			}

			// The plugin has transitioned to a healthy state. Emit an event.
			if !h.previousHealthState && pluginHealthy {
				event := structs.NewTaskEvent(structs.TaskPluginHealthy)
				event.SetMessage(fmt.Sprintf("plugin: %s", h.task.CSIPluginConfig.ID))
				h.eventEmitter.EmitEvent(event)
			}

			// The plugin has transitioned to an unhealthy state. Emit an event.
			if h.previousHealthState && !pluginHealthy {
				event := structs.NewTaskEvent(structs.TaskPluginUnhealthy)
				if err != nil {
					event.SetMessage(fmt.Sprintf("error: %v", err))
				} else {
					event.SetMessage("Unknown Reason")
				}
				h.eventEmitter.EmitEvent(event)
			}

			h.previousHealthState = pluginHealthy

			// This loop is informational and in some plugins this may be expensive to
			// validate. We use a longer timeout (30s) to avoid causing undue work.
			t.Reset(30 * time.Second)
		}
	}
}

func (h *csiPluginSupervisorHook) registerPlugin(socketPath string) (func(), error) {

	// At this point we know the plugin is ready and we can fingerprint it
	// to get its vendor name and version
	client, err := csi.NewClient(socketPath, h.logger.Named("csi_client").With("plugin.name", h.task.CSIPluginConfig.ID, "plugin.type", h.task.CSIPluginConfig.Type))
	if err != nil {
		return nil, fmt.Errorf("failed to create csi client: %v", err)
	}
	defer client.Close()

	info, err := client.PluginInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to probe plugin: %v", err)
	}

	mkInfoFn := func(pluginType string) *dynamicplugins.PluginInfo {
		return &dynamicplugins.PluginInfo{
			Type:    pluginType,
			Name:    h.task.CSIPluginConfig.ID,
			Version: info.PluginVersion,
			ConnectionInfo: &dynamicplugins.PluginConnectionInfo{
				SocketPath: socketPath,
			},
			AllocID: h.alloc.ID,
			Options: map[string]string{
				"Provider":            info.Name, // vendor name
				"MountPoint":          h.mountPoint,
				"ContainerMountPoint": h.task.CSIPluginConfig.MountDir,
			},
		}
	}

	registrations := []*dynamicplugins.PluginInfo{}

	switch h.task.CSIPluginConfig.Type {
	case structs.CSIPluginTypeController:
		registrations = append(registrations, mkInfoFn(dynamicplugins.PluginTypeCSIController))
	case structs.CSIPluginTypeNode:
		registrations = append(registrations, mkInfoFn(dynamicplugins.PluginTypeCSINode))
	case structs.CSIPluginTypeMonolith:
		registrations = append(registrations, mkInfoFn(dynamicplugins.PluginTypeCSIController))
		registrations = append(registrations, mkInfoFn(dynamicplugins.PluginTypeCSINode))
	}

	deregistrationFns := []func(){}

	for _, reg := range registrations {
		if err := h.runner.dynamicRegistry.RegisterPlugin(reg); err != nil {
			for _, fn := range deregistrationFns {
				fn()
			}
			return nil, err
		}

		// need to rebind these so that each deregistration function
		// closes over its own registration
		rname := reg.Name
		rtype := reg.Type
		deregistrationFns = append(deregistrationFns, func() {
			err := h.runner.dynamicRegistry.DeregisterPlugin(rtype, rname)
			if err != nil {
				h.logger.Error("failed to deregister csi plugin", "name", rname, "type", rtype, "error", err)
			}
		})
	}

	return func() {
		for _, fn := range deregistrationFns {
			fn()
		}
	}, nil
}

func (h *csiPluginSupervisorHook) supervisorLoopOnce(ctx context.Context, socketPath string) (bool, error) {
	_, err := os.Stat(socketPath)
	if err != nil {
		return false, fmt.Errorf("failed to stat socket: %v", err)
	}

	client, err := csi.NewClient(socketPath, h.logger.Named("csi_client").With("plugin.name", h.task.CSIPluginConfig.ID, "plugin.type", h.task.CSIPluginConfig.Type))
	if err != nil {
		return false, fmt.Errorf("failed to create csi client: %v", err)
	}
	defer client.Close()

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
// Stop hooks must be idempotent. The context is cancelled prematurely if the
// task is killed.
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
