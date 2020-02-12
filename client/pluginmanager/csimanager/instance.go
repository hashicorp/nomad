package csimanager

import (
	"context"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/dynamicplugins"
	"github.com/hashicorp/nomad/plugins/csi"
)

const managerFingerprintInterval = 30 * time.Second

// instanceManager is used to manage the fingerprinting and supervision of a
// single CSI Plugin.
type instanceManager struct {
	info   *dynamicplugins.PluginInfo
	logger hclog.Logger

	updater UpdateNodeCSIInfoFunc

	shutdownCtx         context.Context
	shutdownCtxCancelFn context.CancelFunc
	shutdownCh          chan struct{}

	// mountPoint is the root of the mount dir where plugin specific data may be
	// stored and where mount points will be created
	mountPoint string

	// containerMountPoint is the location _inside_ the plugin container that the
	// `mountPoint` is bound in to.
	containerMountPoint string

	// AllocID is the allocation id of the task group running the dynamic plugin
	allocID string

	fp *pluginFingerprinter

	volumeManager        *volumeManager
	volumeManagerSetupCh chan struct{}

	client csi.CSIPlugin
}

func newInstanceManager(logger hclog.Logger, updater UpdateNodeCSIInfoFunc, p *dynamicplugins.PluginInfo) *instanceManager {
	ctx, cancelFn := context.WithCancel(context.Background())
	logger = logger.Named(p.Name)
	return &instanceManager{
		logger:  logger,
		info:    p,
		updater: updater,

		fp: &pluginFingerprinter{
			logger:                          logger.Named("fingerprinter"),
			info:                            p,
			fingerprintNode:                 p.Type == dynamicplugins.PluginTypeCSINode,
			fingerprintController:           p.Type == dynamicplugins.PluginTypeCSIController,
			hadFirstSuccessfulFingerprintCh: make(chan struct{}),
		},

		mountPoint:          p.Options["MountPoint"],
		containerMountPoint: p.Options["ContainerMountPoint"],
		allocID:             p.Options["AllocID"],

		volumeManagerSetupCh: make(chan struct{}),

		shutdownCtx:         ctx,
		shutdownCtxCancelFn: cancelFn,
		shutdownCh:          make(chan struct{}),
	}
}

func (i *instanceManager) run() {
	c, err := csi.NewClient(i.info.ConnectionInfo.SocketPath, i.logger)
	if err != nil {
		i.logger.Error("failed to setup instance manager client", "error", err)
		close(i.shutdownCh)
		return
	}
	i.client = c
	i.fp.client = c

	go i.setupVolumeManager()
	go i.runLoop()
}

func (i *instanceManager) setupVolumeManager() {
	if i.info.Type != dynamicplugins.PluginTypeCSINode {
		i.logger.Debug("Skipping volume manager setup - not managing a Node plugin", "type", i.info.Type)
		return
	}

	select {
	case <-i.shutdownCtx.Done():
		return
	case <-i.fp.hadFirstSuccessfulFingerprintCh:
		i.volumeManager = newVolumeManager(i.logger, i.client, i.mountPoint, i.containerMountPoint, i.fp.requiresStaging)
		i.logger.Debug("Setup volume manager")
		close(i.volumeManagerSetupCh)
		return
	}
}

// VolumeMounter returns the volume manager that is configured for the given plugin
// instance. If called before the volume manager has been setup, it will block until
// the volume manager is ready or the context is closed.
func (i *instanceManager) VolumeMounter(ctx context.Context) (VolumeMounter, error) {
	select {
	case <-i.volumeManagerSetupCh:
		return i.volumeManager, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (i *instanceManager) requestCtxWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(i.shutdownCtx, timeout)
}

func (i *instanceManager) runLoop() {
	timer := time.NewTimer(0)
	for {
		select {
		case <-i.shutdownCtx.Done():
			if i.client != nil {
				i.client.Close()
				i.client = nil
			}
			close(i.shutdownCh)
			return
		case <-timer.C:
			ctx, cancelFn := i.requestCtxWithTimeout(managerFingerprintInterval)

			info := i.fp.fingerprint(ctx)
			cancelFn()
			i.updater(i.info.Name, info)

			timer.Reset(managerFingerprintInterval)
		}
	}
}

func (i *instanceManager) shutdown() {
	i.shutdownCtxCancelFn()
	<-i.shutdownCh
}
