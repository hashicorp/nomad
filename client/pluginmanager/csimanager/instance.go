package csimanager

import (
	"context"
	"sync"
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

	fp *pluginFingerprinter

	volumeManager        *volumeManager
	volumeManagerMu      sync.RWMutex
	volumeManagerSetupCh chan struct{}
	volumeManagerSetup   bool

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

		mountPoint: p.Options["MountPoint"],

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

	go i.runLoop()
}

// VolumeMounter returns the volume manager that is configured for the given plugin
// instance. If called before the volume manager has been setup, it will block until
// the volume manager is ready or the context is closed.
func (i *instanceManager) VolumeMounter(ctx context.Context) (VolumeMounter, error) {
	var vm VolumeMounter
	i.volumeManagerMu.RLock()
	if i.volumeManagerSetup {
		vm = i.volumeManager
	}
	i.volumeManagerMu.RUnlock()

	if vm != nil {
		return vm, nil
	}

	select {
	case <-i.volumeManagerSetupCh:
		i.volumeManagerMu.RLock()
		vm = i.volumeManager
		i.volumeManagerMu.RUnlock()
		return vm, nil
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

			// TODO: refactor this lock into a faster, goroutine-local check
			i.volumeManagerMu.RLock()
			// When we've had a successful fingerprint, and the volume manager is not yet setup,
			// and one is required (we're running a node plugin), then set one up now.
			if i.fp.hadFirstSuccessfulFingerprint && !i.volumeManagerSetup && i.fp.fingerprintNode {
				i.volumeManagerMu.RUnlock()
				i.volumeManagerMu.Lock()
				i.volumeManager = newVolumeManager(i.logger, i.client, i.mountPoint)
				i.volumeManagerSetup = true
				close(i.volumeManagerSetupCh)
				i.volumeManagerMu.Unlock()
			} else {
				i.volumeManagerMu.RUnlock()
			}

			timer.Reset(managerFingerprintInterval)
		}
	}
}

func (i *instanceManager) shutdown() {
	i.shutdownCtxCancelFn()
	<-i.shutdownCh
}
