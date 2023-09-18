// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package csimanager

import (
	"context"

	"github.com/hashicorp/nomad/client/pluginmanager"
	nstructs "github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/csi"
)

var _ Manager = &MockCSIManager{}

type MockCSIManager struct {
	VM *MockVolumeManager

	NextWaitForPluginErr    error
	NextManagerForPluginErr error
}

func (m *MockCSIManager) PluginManager() pluginmanager.PluginManager {
	panic("implement me")
}

func (m *MockCSIManager) WaitForPlugin(_ context.Context, pluginType, pluginID string) error {
	return m.NextWaitForPluginErr
}

func (m *MockCSIManager) ManagerForPlugin(_ context.Context, pluginID string) (VolumeManager, error) {
	return m.VM, m.NextManagerForPluginErr
}

func (m *MockCSIManager) Shutdown() {
	panic("implement me")
}

var _ VolumeManager = &MockVolumeManager{}

type MockVolumeManager struct {
	NextExpandVolumeErr  error
	LastExpandVolumeCall *MockExpandVolumeCall
}

func (m *MockVolumeManager) MountVolume(_ context.Context, vol *nstructs.CSIVolume, alloc *nstructs.Allocation, usageOpts *UsageOptions, publishContext map[string]string) (*MountInfo, error) {
	panic("implement me")
}

func (m *MockVolumeManager) UnmountVolume(_ context.Context, volID, remoteID, allocID string, usageOpts *UsageOptions) error {
	panic("implement me")
}

func (m *MockVolumeManager) HasMount(_ context.Context, mountInfo *MountInfo) (bool, error) {
	panic("implement me")
}

func (m *MockVolumeManager) ExpandVolume(_ context.Context, volID, remoteID, allocID string, usageOpts *UsageOptions, capacity *csi.CapacityRange) (int64, error) {
	m.LastExpandVolumeCall = &MockExpandVolumeCall{
		volID, remoteID, allocID, usageOpts, capacity,
	}
	return capacity.RequiredBytes, m.NextExpandVolumeErr
}

type MockExpandVolumeCall struct {
	VolID, RemoteID, AllocID string
	UsageOpts                *UsageOptions
	Capacity                 *csi.CapacityRange
}

func (m *MockVolumeManager) ExternalID() string {
	return "mock-volume-manager"
}
