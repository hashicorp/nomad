package csimanager

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/dynamicplugins"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
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

	fingerprintNode       bool
	fingerprintController bool

	client csi.CSIPlugin
}

func newInstanceManager(logger hclog.Logger, updater UpdateNodeCSIInfoFunc, p *dynamicplugins.PluginInfo) *instanceManager {
	ctx, cancelFn := context.WithCancel(context.Background())

	return &instanceManager{
		logger:  logger.Named(p.Name),
		info:    p,
		updater: updater,

		fingerprintNode:       p.Type == dynamicplugins.PluginTypeCSINode,
		fingerprintController: p.Type == dynamicplugins.PluginTypeCSIController,

		shutdownCtx:         ctx,
		shutdownCtxCancelFn: cancelFn,
		shutdownCh:          make(chan struct{}),
	}
}

func (i *instanceManager) run() {
	c, err := csi.NewClient(i.info.ConnectionInfo.SocketPath)
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
	// basicInfo holds a cache of data that should not change within a CSI plugin.
	// This allows us to minimize the number of requests we make to plugins on each
	// run of the fingerprinter, and reduces the chances of performing overly
	// expensive actions repeatedly, and improves stability of data through
	// transient failures.
	var basicInfo *structs.CSIInfo

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

			if basicInfo == nil {
				info, err := i.buildBasicFingerprint(ctx)
				if err != nil {
					// If we receive a fingerprinting error, update the stats with as much
					// info as possible and wait for the next fingerprint interval.
					info.HealthDescription = fmt.Sprintf("failed initial fingerprint with err: %v", err)
					cancelFn()
					i.updater(i.info.Name, basicInfo)
					timer.Reset(managerFingerprintInterval)
					continue
				}

				// If fingerprinting succeeded, we don't need to repopulate the basic
				// info and we can stop here.
				basicInfo = info
			}

			info := basicInfo.Copy()
			var fp *structs.CSIInfo
			var err error

			if i.fingerprintNode {
				fp, err = i.buildNodeFingerprint(ctx, info)
			} else if i.fingerprintController {
				fp, err = i.buildControllerFingerprint(ctx, info)
			}

			if err != nil {
				info.Healthy = false
				info.HealthDescription = fmt.Sprintf("failed fingerprinting with error: %v", err)
			} else {
				info = fp
			}

			cancelFn()
			i.updater(i.info.Name, info)
			timer.Reset(managerFingerprintInterval)
		}
	}
}

func (i *instanceManager) buildControllerFingerprint(ctx context.Context, base *structs.CSIInfo) (*structs.CSIInfo, error) {
	fp := base.Copy()

	healthy, err := i.client.PluginProbe(ctx)
	if err != nil {
		return nil, err
	}
	fp.SetHealthy(healthy)

	return fp, nil
}

func (i *instanceManager) buildNodeFingerprint(ctx context.Context, base *structs.CSIInfo) (*structs.CSIInfo, error) {
	fp := base.Copy()

	healthy, err := i.client.PluginProbe(ctx)
	if err != nil {
		return nil, err
	}
	fp.SetHealthy(healthy)

	return fp, nil
}

func structCSITopologyFromCSITopology(a *csi.Topology) *structs.CSITopology {
	if a == nil {
		return nil
	}

	return &structs.CSITopology{
		Segments: helper.CopyMapStringString(a.Segments),
	}
}

func (i *instanceManager) buildBasicFingerprint(ctx context.Context) (*structs.CSIInfo, error) {
	info := &structs.CSIInfo{
		PluginID:          i.info.Name,
		Healthy:           false,
		HealthDescription: "initial fingerprint not completed",
	}

	if i.fingerprintNode {
		info.NodeInfo = &structs.CSINodeInfo{}
	}
	if i.fingerprintController {
		info.ControllerInfo = &structs.CSIControllerInfo{}
	}

	capabilities, err := i.client.PluginGetCapabilities(ctx)
	if err != nil {
		return info, err
	}

	info.RequiresControllerPlugin = capabilities.HasControllerService()
	info.RequiresTopologies = capabilities.HasToplogies()

	if i.fingerprintNode {
		nodeInfo, err := i.client.NodeGetInfo(ctx)
		if err != nil {
			return info, err
		}

		info.NodeInfo.ID = nodeInfo.NodeID
		info.NodeInfo.MaxVolumes = nodeInfo.MaxVolumes
		info.NodeInfo.AccessibleTopology = structCSITopologyFromCSITopology(nodeInfo.AccessibleTopology)
	}

	return info, nil
}

func (i *instanceManager) shutdown() {
	i.shutdownCtxCancelFn()
	<-i.shutdownCh
}
