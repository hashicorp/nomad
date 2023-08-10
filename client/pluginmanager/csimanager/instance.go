// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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
	info    *dynamicplugins.PluginInfo
	logger  hclog.Logger
	eventer TriggerNodeEvent

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

func newInstanceManager(logger hclog.Logger, eventer TriggerNodeEvent, updater UpdateNodeCSIInfoFunc, p *dynamicplugins.PluginInfo) *instanceManager {
	ctx, cancelFn := context.WithCancel(context.Background())
	logger = logger.Named(p.Name)
	return &instanceManager{
		logger:  logger,
		eventer: eventer,
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
		allocID:             p.AllocID,

		volumeManagerSetupCh: make(chan struct{}),

		shutdownCtx:         ctx,
		shutdownCtxCancelFn: cancelFn,
		shutdownCh:          make(chan struct{}),
	}
}

func (i *instanceManager) run() {
	c := csi.NewClient(i.info.ConnectionInfo.SocketPath, i.logger)
	i.client = c
	i.fp.client = c

	go i.setupVolumeManager()
	go i.runLoop()
}

func (i *instanceManager) setupVolumeManager() {
	if i.info.Type != dynamicplugins.PluginTypeCSINode {
		i.logger.Debug("not a node plugin, skipping volume manager setup", "type", i.info.Type)
		return
	}

	select {
	case <-i.shutdownCtx.Done():
		return
	case <-i.fp.hadFirstSuccessfulFingerprintCh:

		var externalID string
		if i.fp.basicInfo != nil && i.fp.basicInfo.NodeInfo != nil {
			externalID = i.fp.basicInfo.NodeInfo.ID
		}

		i.volumeManager = newVolumeManager(i.logger, i.eventer, i.client, i.mountPoint, i.containerMountPoint, i.fp.requiresStaging, externalID)
		i.logger.Debug("volume manager setup complete")
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

			// run one last fingerprint so that we mark the plugin as unhealthy.
			// the client has been closed so this will return quickly with the
			// plugin's basic info
			ctx, cancelFn := i.requestCtxWithTimeout(time.Second)
			info := i.fp.fingerprint(ctx)
			cancelFn()
			if info != nil {
				i.updater(i.info.Name, info)
			}
			close(i.shutdownCh)
			return

		case <-timer.C:
			ctx, cancelFn := i.requestCtxWithTimeout(managerFingerprintInterval)
			info := i.fp.fingerprint(ctx)
			cancelFn()
			if info != nil {
				i.updater(i.info.Name, info)
			}
			timer.Reset(managerFingerprintInterval)
		}
	}
}

func (i *instanceManager) shutdown() {
	i.shutdownCtxCancelFn()
	<-i.shutdownCh
}
