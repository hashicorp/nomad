package csimanager

import (
	"context"
	"errors"
	"io/ioutil"
	"os"
	"runtime"
	"testing"

	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	csifake "github.com/hashicorp/nomad/plugins/csi/fake"
	"github.com/stretchr/testify/require"
)

func tmpDir(t testing.TB) string {
	t.Helper()
	dir, err := ioutil.TempDir("", "nomad")
	require.NoError(t, err)
	return dir
}

func TestVolumeManager_ensureStagingDir(t *testing.T) {
	t.Parallel()

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
			tmpPath := tmpDir(t)
			defer os.RemoveAll(tmpPath)

			csiFake := &csifake.Client{}
			manager := newVolumeManager(testlog.HCLogger(t), csiFake, tmpPath, tmpPath, true)
			expectedStagingPath := manager.stagingDirForVolume(tmpPath, tc.Volume, tc.UsageOptions)

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
	t.Parallel()
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
				ID:             "foo",
				AttachmentMode: "nonsense",
			},
			UsageOptions: &UsageOptions{},
			ExpectedErr:  errors.New("Unknown volume attachment mode: nonsense"),
		},
		{
			Name: "Returns an error when an invalid AccessMode is provided",
			Volume: &structs.CSIVolume{
				ID:             "foo",
				AttachmentMode: structs.CSIVolumeAttachmentModeBlockDevice,
				AccessMode:     "nonsense",
			},
			UsageOptions: &UsageOptions{},
			ExpectedErr:  errors.New("Unknown volume access mode: nonsense"),
		},
		{
			Name: "Returns an error when the plugin returns an error",
			Volume: &structs.CSIVolume{
				ID:             "foo",
				AttachmentMode: structs.CSIVolumeAttachmentModeBlockDevice,
				AccessMode:     structs.CSIVolumeAccessModeMultiNodeMultiWriter,
			},
			UsageOptions: &UsageOptions{},
			PluginErr:    errors.New("Some Unknown Error"),
			ExpectedErr:  errors.New("Some Unknown Error"),
		},
		{
			Name: "Happy Path",
			Volume: &structs.CSIVolume{
				ID:             "foo",
				AttachmentMode: structs.CSIVolumeAttachmentModeBlockDevice,
				AccessMode:     structs.CSIVolumeAccessModeMultiNodeMultiWriter,
			},
			UsageOptions: &UsageOptions{},
			PluginErr:    nil,
			ExpectedErr:  nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			tmpPath := tmpDir(t)
			defer os.RemoveAll(tmpPath)

			csiFake := &csifake.Client{}
			csiFake.NextNodeStageVolumeErr = tc.PluginErr

			manager := newVolumeManager(testlog.HCLogger(t), csiFake, tmpPath, tmpPath, true)
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
	t.Parallel()
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
			tmpPath := tmpDir(t)
			defer os.RemoveAll(tmpPath)

			csiFake := &csifake.Client{}
			csiFake.NextNodeUnstageVolumeErr = tc.PluginErr

			manager := newVolumeManager(testlog.HCLogger(t), csiFake, tmpPath, tmpPath, true)
			ctx := context.Background()

			err := manager.unstageVolume(ctx, tc.Volume, tc.UsageOptions)

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
	t.Parallel()
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
				ID:             "foo",
				AttachmentMode: structs.CSIVolumeAttachmentModeBlockDevice,
				AccessMode:     structs.CSIVolumeAccessModeMultiNodeMultiWriter,
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
				ID:             "foo",
				AttachmentMode: structs.CSIVolumeAttachmentModeBlockDevice,
				AccessMode:     structs.CSIVolumeAccessModeMultiNodeMultiWriter,
			},
			UsageOptions:         &UsageOptions{},
			PluginErr:            nil,
			ExpectedErr:          nil,
			ExpectedCSICallCount: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			tmpPath := tmpDir(t)
			defer os.RemoveAll(tmpPath)

			csiFake := &csifake.Client{}
			csiFake.NextNodePublishVolumeErr = tc.PluginErr

			manager := newVolumeManager(testlog.HCLogger(t), csiFake, tmpPath, tmpPath, true)
			ctx := context.Background()

			_, err := manager.publishVolume(ctx, tc.Volume, tc.Allocation, tc.UsageOptions, nil)

			if tc.ExpectedErr != nil {
				require.EqualError(t, err, tc.ExpectedErr.Error())
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tc.ExpectedCSICallCount, csiFake.NodePublishVolumeCallCount)
		})
	}
}

func TestVolumeManager_unpublishVolume(t *testing.T) {
	t.Parallel()
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
			tmpPath := tmpDir(t)
			defer os.RemoveAll(tmpPath)

			csiFake := &csifake.Client{}
			csiFake.NextNodeUnpublishVolumeErr = tc.PluginErr

			manager := newVolumeManager(testlog.HCLogger(t), csiFake, tmpPath, tmpPath, true)
			ctx := context.Background()

			err := manager.unpublishVolume(ctx, tc.Volume, tc.Allocation, tc.UsageOptions)

			if tc.ExpectedErr != nil {
				require.EqualError(t, err, tc.ExpectedErr.Error())
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tc.ExpectedCSICallCount, csiFake.NodeUnpublishVolumeCallCount)
		})
	}
}
