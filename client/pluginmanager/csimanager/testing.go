// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package csimanager

import (
	"context"
	"path/filepath"

	"github.com/hashicorp/nomad/client/pluginmanager"
	nstructs "github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/csi"
	"github.com/hashicorp/nomad/testutil"
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
	if m.VM == nil {
		m.VM = &MockVolumeManager{}
	}
	return m.VM, m.NextManagerForPluginErr
}

func (m *MockCSIManager) Shutdown() {
	panic("implement me")
}

var _ VolumeManager = &MockVolumeManager{}

type MockVolumeManager struct {
	CallCounter *testutil.CallCounter

	Mounts map[string]bool // lazy set

	NextMountVolumeErr   error
	NextUnmountVolumeErr error

	NextExpandVolumeErr  error
	LastExpandVolumeCall *MockExpandVolumeCall
}

func (m *MockVolumeManager) mountName(volID, allocID string, usageOpts *UsageOptions) string {
	return filepath.Join("test-alloc-dir", allocID, volID, usageOpts.ToFS())
}

func (m *MockVolumeManager) MountVolume(_ context.Context, vol *nstructs.CSIVolume, alloc *nstructs.Allocation, usageOpts *UsageOptions, publishContext map[string]string) (*MountInfo, error) {
	if m.CallCounter != nil {
		m.CallCounter.Inc("MountVolume")
	}

	if m.NextMountVolumeErr != nil {
		err := m.NextMountVolumeErr
		m.NextMountVolumeErr = nil // reset it
		return nil, err
	}

	// "mount" it
	if m.Mounts == nil {
		m.Mounts = make(map[string]bool)
	}
	source := m.mountName(vol.ID, alloc.ID, usageOpts)
	m.Mounts[source] = true

	return &MountInfo{
		Source: source,
	}, nil
}

func (m *MockVolumeManager) UnmountVolume(_ context.Context, volID, remoteID, allocID string, usageOpts *UsageOptions) error {
	if m.CallCounter != nil {
		m.CallCounter.Inc("UnmountVolume")
	}

	if m.NextUnmountVolumeErr != nil {
		err := m.NextUnmountVolumeErr
		m.NextUnmountVolumeErr = nil // reset it
		return err
	}

	// "unmount" it
	delete(m.Mounts, m.mountName(volID, allocID, usageOpts))
	return nil
}

func (m *MockVolumeManager) HasMount(_ context.Context, mountInfo *MountInfo) (bool, error) {
	if m.CallCounter != nil {
		m.CallCounter.Inc("HasMount")
	}
	if m.Mounts == nil {
		return false, nil
	}
	return m.Mounts[mountInfo.Source], nil
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
