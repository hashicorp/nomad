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
// It provides a few things to a plugin task running inside Nomad. These are:
// * A mount to the `csi_plugin.mount_dir` where the plugin will create its csi.sock
// * A mount to `local/csi` that node plugins will use to stage volume mounts.
// * When the task has started, it starts a loop of attempting to connect to the
//   plugin, to perform initial fingerprinting of the plugins capabilities before
//   notifying the plugin manager of the plugin.
type csiPluginSupervisorHook struct {
	logger           hclog.Logger
	alloc            *structs.Allocation
	task             *structs.Task
	runner           *TaskRunner
	mountPoint       string
	socketMountPoint string
	socketPath       string

	caps *drivers.Capabilities

	// eventEmitter is used to emit events to the task
	eventEmitter ti.EventEmitter
	lifecycle    ti.TaskLifecycle

	shutdownCtx      context.Context
	shutdownCancelFn context.CancelFunc

	running     bool
	runningLock sync.Mutex

	// previousHealthstate is used by the supervisor goroutine to track historic
	// health states for gating task events.
	previousHealthState bool
}

type csiPluginSupervisorHookConfig struct {
	clientStateDirPath string
	events             ti.EventEmitter
	runner             *TaskRunner
	lifecycle          ti.TaskLifecycle
	capabilities       *drivers.Capabilities
	logger             hclog.Logger
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

// This hook creates a csi/ directory within the client's datadir used to
// manage plugins and mount points volumes. The layout is as follows:

// plugins/
//    {alloc-id}/csi.sock
//       Per-allocation directories of unix domain sockets used to communicate
//       with the CSI plugin. Nomad creates the directory and the plugin creates
//       the socket file. This directory is bind-mounted to the
//       csi_plugin.mount_config dir in the plugin task.
//
// {plugin-type}/{plugin-id}/
//    staging/
//       {volume-id}/{usage-mode}/
//          Intermediate mount point used by node plugins that support
//          NODE_STAGE_UNSTAGE capability.
//
//    per-alloc/
//       {alloc-id}/{volume-id}/{usage-mode}/
//          Mount point bound from the staging directory into tasks that use
//          the mounted volumes

func newCSIPluginSupervisorHook(config *csiPluginSupervisorHookConfig) *csiPluginSupervisorHook {
	task := config.runner.Task()

	pluginRoot := filepath.Join(config.clientStateDirPath, "csi",
		string(task.CSIPluginConfig.Type), task.CSIPluginConfig.ID)

	socketMountPoint := filepath.Join(config.clientStateDirPath, "csi",
		"plugins", config.runner.Alloc().ID)

	shutdownCtx, cancelFn := context.WithCancel(context.Background())

	hook := &csiPluginSupervisorHook{
		alloc:            config.runner.Alloc(),
		runner:           config.runner,
		lifecycle:        config.lifecycle,
		logger:           config.logger,
		task:             task,
		mountPoint:       pluginRoot,
		socketMountPoint: socketMountPoint,
		caps:             config.capabilities,
		shutdownCtx:      shutdownCtx,
		shutdownCancelFn: cancelFn,
		eventEmitter:     config.events,
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

	if err := os.MkdirAll(h.socketMountPoint, 0700); err != nil && !os.IsExist(err) {
		return fmt.Errorf("failed to create socket mount point: %v", err)
	}

	// where the socket will be mounted
	configMount := &drivers.MountConfig{
		TaskPath:        h.task.CSIPluginConfig.MountDir,
		HostPath:        h.socketMountPoint,
		Readonly:        false,
		PropagationMode: "bidirectional",
	}
	// where the staging and per-alloc directories will be mounted
	volumeStagingMounts := &drivers.MountConfig{
		// TODO(tgross): add this TaskPath to the CSIPluginConfig as well
		TaskPath:        "/local/csi",
		HostPath:        h.mountPoint,
		Readonly:        false,
		PropagationMode: "bidirectional",
	}
	// devices from the host
	devMount := &drivers.MountConfig{
		TaskPath: "/dev",
		HostPath: "/dev",
		Readonly: false,
	}

	// TODO(tgross): https://github.com/hashicorp/nomad/issues/11786
	// If we're already registered, we should be able to update the
	// definition in the update hook

	// For backwards compatibility, ensure that we don't overwrite the
	// socketPath on client restart with existing plugin allocations.
	pluginInfo, _ := h.runner.dynamicRegistry.PluginForAlloc(
		string(h.task.CSIPluginConfig.Type), h.task.CSIPluginConfig.ID, h.alloc.ID)
	if pluginInfo != nil {
		h.socketPath = pluginInfo.ConnectionInfo.SocketPath
	} else {
		h.socketPath = filepath.Join(h.socketMountPoint, structs.CSISocketName)
	}

	switch h.caps.FSIsolation {
	case drivers.FSIsolationNone:
		// Plugin tasks with no filesystem isolation won't have the
		// plugin dir bind-mounted to their alloc dir, but we can
		// provide them the path to the socket. These Nomad-only
		// plugins will need to be aware of the csi directory layout
		// in the client data dir
		resp.Env = map[string]string{
			"CSI_ENDPOINT": h.socketPath}
	default:
		resp.Env = map[string]string{
			"CSI_ENDPOINT": filepath.Join(
				h.task.CSIPluginConfig.MountDir, structs.CSISocketName)}
	}

	mounts := ensureMountpointInserted(h.runner.hookResources.getMounts(), configMount)
	mounts = ensureMountpointInserted(mounts, volumeStagingMounts)
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

	client := csi.NewClient(h.socketPath, h.logger.Named("csi_client").With(
		"plugin.name", h.task.CSIPluginConfig.ID,
		"plugin.type", h.task.CSIPluginConfig.Type))
	defer client.Close()

	t := time.NewTimer(0)

	// We're in Poststart at this point, so if we can't connect within
	// this deadline, assume it's broken so we can restart the task
	startCtx, startCancelFn := context.WithTimeout(ctx, 30*time.Second)
	defer startCancelFn()

	var err error
	var pluginHealthy bool

	// Step 1: Wait for the plugin to initially become available.
WAITFORREADY:
	for {
		select {
		case <-startCtx.Done():
			h.kill(ctx, fmt.Errorf("CSI plugin failed probe: %v", err))
			return
		case <-t.C:
			pluginHealthy, err = h.supervisorLoopOnce(startCtx, client)
			if err != nil || !pluginHealthy {
				h.logger.Debug("CSI plugin not ready", "error", err)
				// Use only a short delay here to optimize for quickly
				// bringing up a plugin
				t.Reset(5 * time.Second)
				continue
			}

			// Mark the plugin as healthy in a task event
			h.logger.Debug("CSI plugin is ready")
			h.previousHealthState = pluginHealthy
			event := structs.NewTaskEvent(structs.TaskPluginHealthy)
			event.SetMessage(fmt.Sprintf("plugin: %s", h.task.CSIPluginConfig.ID))
			h.eventEmitter.EmitEvent(event)

			break WAITFORREADY
		}
	}

	// Step 2: Register the plugin with the catalog.
	deregisterPluginFn, err := h.registerPlugin(client, h.socketPath)
	if err != nil {
		h.kill(ctx, fmt.Errorf("CSI plugin failed to register: %v", err))
		return
	}

	// Step 3: Start the lightweight supervisor loop. At this point, failures
	// don't cause the task to restart
	t.Reset(0)
	for {
		select {
		case <-ctx.Done():
			// De-register plugins on task shutdown
			deregisterPluginFn()
			return
		case <-t.C:
			pluginHealthy, err := h.supervisorLoopOnce(ctx, client)
			if err != nil {
				h.logger.Error("CSI plugin fingerprinting failed", "error", err)
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
					event.SetMessage(fmt.Sprintf("Error: %v", err))
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

func (h *csiPluginSupervisorHook) registerPlugin(client csi.CSIPlugin, socketPath string) (func(), error) {
	// At this point we know the plugin is ready and we can fingerprint it
	// to get its vendor name and version
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
				"ContainerMountPoint": "/local/csi",
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
		allocID := reg.AllocID
		deregistrationFns = append(deregistrationFns, func() {
			err := h.runner.dynamicRegistry.DeregisterPlugin(rtype, rname, allocID)
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

func (h *csiPluginSupervisorHook) supervisorLoopOnce(ctx context.Context, client csi.CSIPlugin) (bool, error) {
	probeCtx, probeCancelFn := context.WithTimeout(ctx, 5*time.Second)
	defer probeCancelFn()

	healthy, err := client.PluginProbe(probeCtx)
	if err != nil {
		return false, err
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
	err := os.RemoveAll(h.socketMountPoint)
	if err != nil {
		h.logger.Error("could not remove plugin socket directory", "dir", h.socketMountPoint, "error", err)
	}
	h.shutdownCancelFn()
	return nil
}

func (h *csiPluginSupervisorHook) kill(ctx context.Context, reason error) {
	h.logger.Error("killing task because plugin failed", "error", reason)
	event := structs.NewTaskEvent(structs.TaskPluginUnhealthy)
	event.SetMessage(fmt.Sprintf("Error: %v", reason.Error()))
	h.eventEmitter.EmitEvent(event)

	if err := h.lifecycle.Kill(ctx,
		structs.NewTaskEvent(structs.TaskKilling).
			SetFailsTask().
			SetDisplayMessage("CSI plugin did not become healthy before timeout"),
	); err != nil {
		h.logger.Error("failed to kill task", "kill_reason", reason, "error", err)
	}
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
