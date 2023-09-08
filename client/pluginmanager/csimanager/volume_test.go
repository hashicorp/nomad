// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package csimanager

import (
	"context"
	"errors"
	"os"
	"runtime"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/mount"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/csi"
	csifake "github.com/hashicorp/nomad/plugins/csi/fake"
	"github.com/stretchr/testify/require"
)

func checkMountSupport() bool {
	path, err := os.Getwd()
	if err != nil {
		return false
	}

	m := mount.New()
	_, err = m.IsNotAMountPoint(path)
	return err == nil
}

func TestVolumeManager_ensureStagingDir(t *testing.T) {
	if !checkMountSupport() {
		t.Skip("mount point detection not supported for this platform")
	}
	ci.Parallel(t)

	cases := []struct {
		Name                 string
		Volume               *structs.CSIVolume
		UsageOptions         *UsageOptions
		CreateDirAheadOfTime bool
		MountDirAheadOfTime  bool

		ExpectedErr        error
		ExpectedMountState bool
	}{
		{
			Name:         "Creates a directory when one does not exist",
			Volume:       &structs.CSIVolume{ID: "foo"},
			UsageOptions: &UsageOptions{},
		},
		{
			Name:                 "Does not fail because of a pre-existing directory",
			Volume:               &structs.CSIVolume{ID: "foo"},
			UsageOptions:         &UsageOptions{},
			CreateDirAheadOfTime: true,
		},
		{
			Name:         "Returns negative mount info",
			UsageOptions: &UsageOptions{},
			Volume:       &structs.CSIVolume{ID: "foo"},
		},
		{
			Name:                 "Returns positive mount info",
			Volume:               &structs.CSIVolume{ID: "foo"},
			UsageOptions:         &UsageOptions{},
			CreateDirAheadOfTime: true,
			MountDirAheadOfTime:  true,
			ExpectedMountState:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			// Step 1: Validate that the test case makes sense
			if !tc.CreateDirAheadOfTime && tc.MountDirAheadOfTime {
				require.Fail(t, "Cannot Mount without creating a dir")
			}

			if tc.MountDirAheadOfTime {
				// We can enable these tests by either mounting a fake device on linux
				// e.g shipping a small ext4 image file and using that as a loopback
				//     device, but there's no convenient way to implement this.
				t.Skip("TODO: Skipped because we don't detect bind mounts")
			}

			// Step 2: Test Setup
			tmpPath := t.TempDir()

			csiFake := &csifake.Client{}
			eventer := func(e *structs.NodeEvent) {}
			manager := newVolumeManager(testlog.HCLogger(t), eventer, csiFake,
				tmpPath, tmpPath, true, "i-example")
			expectedStagingPath := manager.stagingDirForVolume(tmpPath, tc.Volume.ID, tc.UsageOptions)

			if tc.CreateDirAheadOfTime {
				err := os.MkdirAll(expectedStagingPath, 0700)
				require.NoError(t, err)
			}

			// Step 3: Now we can do some testing

			path, detectedMount, testErr := manager.ensureStagingDir(tc.Volume, tc.UsageOptions)
			if tc.ExpectedErr != nil {
				require.EqualError(t, testErr, tc.ExpectedErr.Error())
				return // We don't perform extra validation if an error was detected.
			}

			require.NoError(t, testErr)
			require.Equal(t, tc.ExpectedMountState, detectedMount)

			// If the ensureStagingDir call had to create a directory itself, then here
			// we validate that the directory exists and its permissions
			if !tc.CreateDirAheadOfTime {
				file, err := os.Lstat(path)
				require.NoError(t, err)
				require.True(t, file.IsDir())

				// TODO: Figure out a windows equivalent of this test
				if runtime.GOOS != "windows" {
					require.Equal(t, os.FileMode(0700), file.Mode().Perm())
				}
			}
		})
	}
}

func TestVolumeManager_stageVolume(t *testing.T) {
	if !checkMountSupport() {
		t.Skip("mount point detection not supported for this platform")
	}
	ci.Parallel(t)

	cases := []struct {
		Name         string
		Volume       *structs.CSIVolume
		UsageOptions *UsageOptions
		PluginErr    error
		ExpectedErr  error
	}{
		{
			Name: "Returns an error when an invalid AttachmentMode is provided",
			Volume: &structs.CSIVolume{
				ID: "foo",
			},
			UsageOptions: &UsageOptions{AttachmentMode: "nonsense"},
			ExpectedErr:  errors.New("unknown volume attachment mode: nonsense"),
		},
		{
			Name: "Returns an error when an invalid AccessMode is provided",
			Volume: &structs.CSIVolume{
				ID: "foo",
			},
			UsageOptions: &UsageOptions{
				AttachmentMode: structs.CSIVolumeAttachmentModeBlockDevice,
				AccessMode:     "nonsense",
			},
			ExpectedErr: errors.New("unknown volume access mode: nonsense"),
		},
		{
			Name: "Returns an error when the plugin returns an error",
			Volume: &structs.CSIVolume{
				ID: "foo",
			},
			UsageOptions: &UsageOptions{
				AttachmentMode: structs.CSIVolumeAttachmentModeBlockDevice,
				AccessMode:     structs.CSIVolumeAccessModeMultiNodeMultiWriter,
			},
			PluginErr:   errors.New("Some Unknown Error"),
			ExpectedErr: errors.New("Some Unknown Error"),
		},
		{
			Name: "Happy Path",
			Volume: &structs.CSIVolume{
				ID: "foo",
			},
			UsageOptions: &UsageOptions{
				AttachmentMode: structs.CSIVolumeAttachmentModeBlockDevice,
				AccessMode:     structs.CSIVolumeAccessModeMultiNodeMultiWriter,
			},
			PluginErr:   nil,
			ExpectedErr: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			tmpPath := t.TempDir()

			csiFake := &csifake.Client{}
			csiFake.NextNodeStageVolumeErr = tc.PluginErr

			eventer := func(e *structs.NodeEvent) {}
			manager := newVolumeManager(testlog.HCLogger(t), eventer, csiFake,
				tmpPath, tmpPath, true, "i-example")
			ctx := context.Background()

			err := manager.stageVolume(ctx, tc.Volume, tc.UsageOptions, nil)

			if tc.ExpectedErr != nil {
				require.EqualError(t, err, tc.ExpectedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestVolumeManager_unstageVolume(t *testing.T) {
	if !checkMountSupport() {
		t.Skip("mount point detection not supported for this platform")
	}
	ci.Parallel(t)

	cases := []struct {
		Name                 string
		Volume               *structs.CSIVolume
		UsageOptions         *UsageOptions
		PluginErr            error
		ExpectedErr          error
		ExpectedCSICallCount int64
	}{
		{
			Name: "Returns an error when the plugin returns an error",
			Volume: &structs.CSIVolume{
				ID: "foo",
			},
			UsageOptions:         &UsageOptions{},
			PluginErr:            errors.New("Some Unknown Error"),
			ExpectedErr:          errors.New("Some Unknown Error"),
			ExpectedCSICallCount: 1,
		},
		{
			Name: "Happy Path",
			Volume: &structs.CSIVolume{
				ID: "foo",
			},
			UsageOptions:         &UsageOptions{},
			PluginErr:            nil,
			ExpectedErr:          nil,
			ExpectedCSICallCount: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			tmpPath := t.TempDir()

			csiFake := &csifake.Client{}
			csiFake.NextNodeUnstageVolumeErr = tc.PluginErr

			eventer := func(e *structs.NodeEvent) {}
			manager := newVolumeManager(testlog.HCLogger(t), eventer, csiFake,
				tmpPath, tmpPath, true, "i-example")
			ctx := context.Background()

			err := manager.unstageVolume(ctx,
				tc.Volume.ID, tc.Volume.RemoteID(), tc.UsageOptions)

			if tc.ExpectedErr != nil {
				require.EqualError(t, err, tc.ExpectedErr.Error())
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tc.ExpectedCSICallCount, csiFake.NodeUnstageVolumeCallCount)
		})
	}
}

func TestVolumeManager_publishVolume(t *testing.T) {
	if !checkMountSupport() {
		t.Skip("mount point detection not supported for this platform")
	}

	ci.Parallel(t)

	cases := []struct {
		Name                     string
		Allocation               *structs.Allocation
		Volume                   *structs.CSIVolume
		UsageOptions             *UsageOptions
		PluginErr                error
		ExpectedErr              error
		ExpectedCSICallCount     int64
		ExpectedVolumeCapability *csi.VolumeCapability
	}{
		{
			Name:       "Returns an error when the plugin returns an error",
			Allocation: structs.MockAlloc(),
			Volume: &structs.CSIVolume{
				ID: "foo",
			},
			UsageOptions: &UsageOptions{
				AttachmentMode: structs.CSIVolumeAttachmentModeBlockDevice,
				AccessMode:     structs.CSIVolumeAccessModeMultiNodeMultiWriter,
			},
			PluginErr:            errors.New("Some Unknown Error"),
			ExpectedErr:          errors.New("Some Unknown Error"),
			ExpectedCSICallCount: 1,
		},
		{
			Name:       "Happy Path",
			Allocation: structs.MockAlloc(),
			Volume: &structs.CSIVolume{
				ID: "foo",
			},
			UsageOptions: &UsageOptions{
				AttachmentMode: structs.CSIVolumeAttachmentModeBlockDevice,
				AccessMode:     structs.CSIVolumeAccessModeMultiNodeMultiWriter,
			},
			PluginErr:            nil,
			ExpectedErr:          nil,
			ExpectedCSICallCount: 1,
		},
		{
			Name:       "Mount options in the volume",
			Allocation: structs.MockAlloc(),
			Volume: &structs.CSIVolume{
				ID: "foo",
				MountOptions: &structs.CSIMountOptions{
					MountFlags: []string{"ro"},
				},
			},
			UsageOptions: &UsageOptions{
				AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
				AccessMode:     structs.CSIVolumeAccessModeMultiNodeMultiWriter,
			},
			PluginErr:            nil,
			ExpectedErr:          nil,
			ExpectedCSICallCount: 1,
			ExpectedVolumeCapability: &csi.VolumeCapability{
				AccessType: csi.VolumeAccessTypeMount,
				AccessMode: csi.VolumeAccessModeMultiNodeMultiWriter,
				MountVolume: &structs.CSIMountOptions{
					MountFlags: []string{"ro"},
				},
			},
		},
		{
			Name:       "Mount options override in the request",
			Allocation: structs.MockAlloc(),
			Volume: &structs.CSIVolume{
				ID: "foo",
				MountOptions: &structs.CSIMountOptions{
					MountFlags: []string{"ro"},
				},
			},
			UsageOptions: &UsageOptions{
				AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
				AccessMode:     structs.CSIVolumeAccessModeMultiNodeMultiWriter,
				MountOptions: &structs.CSIMountOptions{
					MountFlags: []string{"rw"},
				},
			},
			PluginErr:            nil,
			ExpectedErr:          nil,
			ExpectedCSICallCount: 1,
			ExpectedVolumeCapability: &csi.VolumeCapability{
				AccessType: csi.VolumeAccessTypeMount,
				AccessMode: csi.VolumeAccessModeMultiNodeMultiWriter,
				MountVolume: &structs.CSIMountOptions{
					MountFlags: []string{"rw"},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			tmpPath := t.TempDir()

			csiFake := &csifake.Client{}
			csiFake.NextNodePublishVolumeErr = tc.PluginErr

			eventer := func(e *structs.NodeEvent) {}
			manager := newVolumeManager(testlog.HCLogger(t), eventer, csiFake,
				tmpPath, tmpPath, true, "i-example")
			ctx := context.Background()

			_, err := manager.publishVolume(ctx, tc.Volume, tc.Allocation, tc.UsageOptions, nil)

			if tc.ExpectedErr != nil {
				require.EqualError(t, err, tc.ExpectedErr.Error())
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tc.ExpectedCSICallCount, csiFake.NodePublishVolumeCallCount)

			if tc.ExpectedVolumeCapability != nil {
				require.Equal(t, tc.ExpectedVolumeCapability, csiFake.PrevVolumeCapability)
			}
		})
	}
}

func TestVolumeManager_unpublishVolume(t *testing.T) {
	if !checkMountSupport() {
		t.Skip("mount point detection not supported for this platform")
	}
	ci.Parallel(t)

	cases := []struct {
		Name                 string
		Allocation           *structs.Allocation
		Volume               *structs.CSIVolume
		UsageOptions         *UsageOptions
		PluginErr            error
		ExpectedErr          error
		ExpectedCSICallCount int64
	}{
		{
			Name:       "Returns an error when the plugin returns an error",
			Allocation: structs.MockAlloc(),
			Volume: &structs.CSIVolume{
				ID: "foo",
			},
			UsageOptions:         &UsageOptions{},
			PluginErr:            errors.New("Some Unknown Error"),
			ExpectedErr:          errors.New("Some Unknown Error"),
			ExpectedCSICallCount: 1,
		},
		{
			Name:       "Happy Path",
			Allocation: structs.MockAlloc(),
			Volume: &structs.CSIVolume{
				ID: "foo",
			},
			UsageOptions:         &UsageOptions{},
			PluginErr:            nil,
			ExpectedErr:          nil,
			ExpectedCSICallCount: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			tmpPath := t.TempDir()

			csiFake := &csifake.Client{}
			csiFake.NextNodeUnpublishVolumeErr = tc.PluginErr

			eventer := func(e *structs.NodeEvent) {}
			manager := newVolumeManager(testlog.HCLogger(t), eventer, csiFake,
				tmpPath, tmpPath, true, "i-example")
			ctx := context.Background()

			err := manager.unpublishVolume(ctx,
				tc.Volume.ID, tc.Volume.RemoteID(), tc.Allocation.ID, tc.UsageOptions)

			if tc.ExpectedErr != nil {
				require.EqualError(t, err, tc.ExpectedErr.Error())
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tc.ExpectedCSICallCount, csiFake.NodeUnpublishVolumeCallCount)
		})
	}
}

func TestVolumeManager_MountVolumeEvents(t *testing.T) {
	if !checkMountSupport() {
		t.Skip("mount point detection not supported for this platform")
	}
	ci.Parallel(t)

	tmpPath := t.TempDir()

	csiFake := &csifake.Client{}

	var events []*structs.NodeEvent
	eventer := func(e *structs.NodeEvent) {
		events = append(events, e)
	}

	manager := newVolumeManager(testlog.HCLogger(t), eventer, csiFake,
		tmpPath, tmpPath, true, "i-example")
	ctx := context.Background()
	vol := &structs.CSIVolume{
		ID:        "vol",
		Namespace: "ns",
	}
	alloc := mock.Alloc()
	usage := &UsageOptions{
		AccessMode: structs.CSIVolumeAccessModeMultiNodeMultiWriter,
	}
	pubCtx := map[string]string{}

	_, err := manager.MountVolume(ctx, vol, alloc, usage, pubCtx)
	require.Error(t, err, "unknown volume attachment mode: ")
	require.Equal(t, 1, len(events))
	e := events[0]
	require.Equal(t, "Mount volume", e.Message)
	require.Equal(t, "Storage", e.Subsystem)
	require.Equal(t, "vol", e.Details["volume_id"])
	require.Equal(t, "false", e.Details["success"])
	require.Equal(t, "unknown volume attachment mode: ", e.Details["error"])
	events = events[1:]

	usage.AttachmentMode = structs.CSIVolumeAttachmentModeFilesystem
	_, err = manager.MountVolume(ctx, vol, alloc, usage, pubCtx)
	require.NoError(t, err)

	require.Equal(t, 1, len(events))
	e = events[0]
	require.Equal(t, "Mount volume", e.Message)
	require.Equal(t, "Storage", e.Subsystem)
	require.Equal(t, "vol", e.Details["volume_id"])
	require.Equal(t, "true", e.Details["success"])
	events = events[1:]

	err = manager.UnmountVolume(ctx, vol.ID, vol.RemoteID(), alloc.ID, usage)
	require.NoError(t, err)

	require.Equal(t, 1, len(events))
	e = events[0]
	require.Equal(t, "Unmount volume", e.Message)
	require.Equal(t, "Storage", e.Subsystem)
	require.Equal(t, "vol", e.Details["volume_id"])
	require.Equal(t, "true", e.Details["success"])
}
