// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package csimanager

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/dynamicplugins"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/csi"
	"github.com/hashicorp/nomad/plugins/csi/fake"
	"github.com/stretchr/testify/require"
)

func setupTestNodeInstanceManager(t *testing.T) (*fake.Client, *instanceManager) {
	tp := &fake.Client{}

	logger := testlog.HCLogger(t)
	pinfo := &dynamicplugins.PluginInfo{
		Name: "test-plugin",
	}

	return tp, &instanceManager{
		logger: logger,
		info:   pinfo,
		client: tp,
		fp: &pluginFingerprinter{
			logger:                          logger.Named("fingerprinter"),
			info:                            pinfo,
			client:                          tp,
			fingerprintNode:                 true,
			hadFirstSuccessfulFingerprintCh: make(chan struct{}),
		},
	}
}

func TestInstanceManager_Shutdown(t *testing.T) {

	var pluginHealth bool
	var lock sync.Mutex
	ctx, cancelFn := context.WithCancel(context.Background())
	client, im := setupTestNodeInstanceManager(t)
	im.shutdownCtx = ctx
	im.shutdownCtxCancelFn = cancelFn
	im.shutdownCh = make(chan struct{})
	im.updater = func(_ string, info *structs.CSIInfo) {
		lock.Lock()
		defer lock.Unlock()
		pluginHealth = info.Healthy
	}

	// set up a mock successful fingerprint so that we can get
	// a healthy plugin before shutting down
	client.NextPluginGetCapabilitiesResponse = &csi.PluginCapabilitySet{}
	client.NextPluginGetCapabilitiesErr = nil
	client.NextNodeGetInfoResponse = &csi.NodeGetInfoResponse{NodeID: "foo"}
	client.NextNodeGetInfoErr = nil
	client.NextNodeGetCapabilitiesResponse = &csi.NodeCapabilitySet{}
	client.NextNodeGetCapabilitiesErr = nil
	client.NextPluginProbeResponse = true

	go im.runLoop()

	require.Eventually(t, func() bool {
		lock.Lock()
		defer lock.Unlock()
		return pluginHealth
	}, 1*time.Second, 10*time.Millisecond)

	cancelFn() // fires im.shutdown()

	require.Eventually(t, func() bool {
		lock.Lock()
		defer lock.Unlock()
		return !pluginHealth
	}, 1*time.Second, 10*time.Millisecond)

}
