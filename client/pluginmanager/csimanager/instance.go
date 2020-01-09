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

	fp *pluginFingerprinter

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
			logger:                logger.Named("fingerprinter"),
			info:                  p,
			fingerprintNode:       p.Type == dynamicplugins.PluginTypeCSINode,
			fingerprintController: p.Type == dynamicplugins.PluginTypeCSIController,
		},

		shutdownCtx:         ctx,
		shutdownCtxCancelFn: cancelFn,
		shutdownCh:          make(chan struct{}),
	}
}

func (i *instanceManager) run() {
	c, err := csi.NewClient(i.info.ConnectionInfo.SocketPath, i.logger.Named("csi_client").With("plugin.name", i.info.Name, "plugin.type", i.info.Type))
	if err != nil {
		i.logger.Error("failed to setup instance manager client", "error", err)
		close(i.shutdownCh)
		return
	}
	i.client = c

	go i.runLoop()
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
