package csimanager

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/pluginregistry"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/csi"
)

type instanceManager struct {
	info   *pluginregistry.PluginInfo
	logger hclog.Logger

	updater UpdateNodeCSIInfoFunc

	shutdownCtx         context.Context
	shutdownCtxCancelFn context.CancelFunc
	shutdownCh          chan struct{}

	client csi.CSIPlugin
}

func newInstanceManager(logger hclog.Logger, updater UpdateNodeCSIInfoFunc, p *pluginregistry.PluginInfo) *instanceManager {
	ctx, cancelFn := context.WithCancel(context.Background())

	return &instanceManager{
		logger:  logger.Named(p.Name),
		info:    p,
		updater: updater,

		shutdownCtx:         ctx,
		shutdownCtxCancelFn: cancelFn,
		shutdownCh:          make(chan struct{}),
	}
}

func (i *instanceManager) run() {
	c, err := csi.NewClient(i.info.ConnectionInfo.SocketPath)
	if err != nil {
		i.logger.Error("failed to setup instance manager client", "error", err)
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
			ctx, cancelFn := i.requestCtxWithTimeout(30 * time.Second)
			info, err := i.buildFingerprint(ctx)
			if err != nil {
				info = &structs.CSIInfo{
					PluginID:          i.info.Name,
					Healthy:           false,
					HealthDescription: fmt.Sprintf("failed fingerprinting with error: %v", err),
				}
			}
			cancelFn()
			i.updater(i.info.Name, info)

			// TODO: Send update and reset timer
		}
	}
}

func (i *instanceManager) buildFingerprint(ctx context.Context) (*structs.CSIInfo, error) {
	if i.client == nil {
		return nil, fmt.Errorf("No CSI Client")
	}

	healthy, err := i.client.PluginProbe(ctx)
	if err != nil {
		return nil, err
	}
	if !healthy {
		return nil, errors.New("plugin unhealthy")
	}

	nodeInfo, err := i.client.NodeGetInfo(ctx)
	if err != nil {
		return nil, err
	}

	return &structs.CSIInfo{
		PluginID:          i.info.Name,
		Healthy:           false,
		HealthDescription: "healthy",

		NodeID: nodeInfo.NodeID,
	}, nil
}

func (i *instanceManager) shutdown() {
	i.shutdownCtxCancelFn()
	<-i.shutdownCh
}
