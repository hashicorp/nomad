// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package csimanager

import (
	"context"
	"testing"

	"github.com/hashicorp/nomad/client/pluginmanager"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/plugins/csi"
)

var _ Manager = &MockCSIManager{}

// MockCSIManager implements CSI manager with a single mock plugin already
// registered, without having to run any fingerprinting, etc.
type MockCSIManager struct {
	vm *volumeManager

	NextWaitForPluginErr    error
	NextManagerForPluginErr error
}

type MockPlugin struct {
	ID              string
	PluginRPCs      csi.CSIPlugin
	RequiresStaging bool
}

// NewMockCSIManager implements CSI manager with a single plugin loaded
func NewMockCSIManager(t *testing.T, eventer TriggerNodeEvent, plugin MockPlugin, rootDir, containerRootDir string) *MockCSIManager {

	vm := newVolumeManager(
		testlog.HCLogger(t),
		eventer,
		plugin.PluginRPCs,
		rootDir,
		containerRootDir,
		plugin.RequiresStaging,
		uuid.Generate())

	m := &MockCSIManager{
		vm: vm,
	}
	return m
}

func (m *MockCSIManager) PluginManager() pluginmanager.PluginManager {
	panic("implement me")
}

func (m *MockCSIManager) WaitForPlugin(_ context.Context, pluginType, pluginID string) error {
	return m.NextWaitForPluginErr
}

func (m *MockCSIManager) ManagerForPlugin(_ context.Context, pluginID string) (VolumeManager, error) {
	return m.vm, m.NextManagerForPluginErr
}

func (m *MockCSIManager) Shutdown() {
	panic("implement me")
}
