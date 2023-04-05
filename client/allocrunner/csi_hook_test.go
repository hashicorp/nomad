package allocrunner

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/pluginmanager"
	"github.com/hashicorp/nomad/client/pluginmanager/csimanager"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/stretchr/testify/require"
)

var _ interfaces.RunnerPrerunHook = (*csiHook)(nil)
var _ interfaces.RunnerPostrunHook = (*csiHook)(nil)

func TestCSIHook(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.Alloc()
	logger := testlog.HCLogger(t)

	testcases := []struct {
		name                   string
		volumeRequests         map[string]*structs.VolumeRequest
		startsUnschedulable    bool
		startsWithClaims       bool
		expectedClaimErr       error
		expectedMounts         map[string]*csimanager.MountInfo
		expectedMountCalls     int
		expectedUnmountCalls   int
		expectedClaimCalls     int
		expectedUnpublishCalls int
	}{

		{
			name: "simple case",
			volumeRequests: map[string]*structs.VolumeRequest{
				"vol0": {
					Name:           "vol0",
					Type:           structs.VolumeTypeCSI,
					Source:         "testvolume0",
					ReadOnly:       true,
					AccessMode:     structs.CSIVolumeAccessModeSingleNodeReader,
					AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
					MountOptions:   &structs.CSIMountOptions{},
					PerAlloc:       false,
				},
			},
			expectedMounts: map[string]*csimanager.MountInfo{
				"vol0": &csimanager.MountInfo{Source: fmt.Sprintf(
					"test-alloc-dir/%s/testvolume0/ro-file-system-single-node-reader-only", alloc.ID)},
			},
			expectedMountCalls:     1,
			expectedUnmountCalls:   1,
			expectedClaimCalls:     1,
			expectedUnpublishCalls: 1,
		},

		{
			name: "per-alloc case",
			volumeRequests: map[string]*structs.VolumeRequest{
				"vol0": {
					Name:           "vol0",
					Type:           structs.VolumeTypeCSI,
					Source:         "testvolume0",
					ReadOnly:       true,
					AccessMode:     structs.CSIVolumeAccessModeSingleNodeReader,
					AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
					MountOptions:   &structs.CSIMountOptions{},
					PerAlloc:       true,
				},
			},
			expectedMounts: map[string]*csimanager.MountInfo{
				"vol0": &csimanager.MountInfo{Source: fmt.Sprintf(
					"test-alloc-dir/%s/testvolume0/ro-file-system-single-node-reader-only", alloc.ID)},
			},
			expectedMountCalls:     1,
			expectedUnmountCalls:   1,
			expectedClaimCalls:     1,
			expectedUnpublishCalls: 1,
		},

		{
			name: "fatal error on claim",
			volumeRequests: map[string]*structs.VolumeRequest{
				"vol0": {
					Name:           "vol0",
					Type:           structs.VolumeTypeCSI,
					Source:         "testvolume0",
					ReadOnly:       true,
					AccessMode:     structs.CSIVolumeAccessModeSingleNodeReader,
					AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
					MountOptions:   &structs.CSIMountOptions{},
					PerAlloc:       false,
				},
			},
			startsUnschedulable: true,
			expectedMounts: map[string]*csimanager.MountInfo{
				"vol0": &csimanager.MountInfo{Source: fmt.Sprintf(
					"test-alloc-dir/%s/testvolume0/ro-file-system-single-node-reader-only", alloc.ID)},
			},
			expectedMountCalls:     0,
			expectedUnmountCalls:   0,
			expectedClaimCalls:     1,
			expectedUnpublishCalls: 0,
			expectedClaimErr: errors.New(
				"claim volumes: could not claim volume testvolume0: volume is currently unschedulable"),
		},

		{
			name: "retryable error on claim",
			volumeRequests: map[string]*structs.VolumeRequest{
				"vol0": {
					Name:           "vol0",
					Type:           structs.VolumeTypeCSI,
					Source:         "testvolume0",
					ReadOnly:       true,
					AccessMode:     structs.CSIVolumeAccessModeSingleNodeReader,
					AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
					MountOptions:   &structs.CSIMountOptions{},
					PerAlloc:       false,
				},
			},
			startsWithClaims: true,
			expectedMounts: map[string]*csimanager.MountInfo{
				"vol0": &csimanager.MountInfo{Source: fmt.Sprintf(
					"test-alloc-dir/%s/testvolume0/ro-file-system-single-node-reader-only", alloc.ID)},
			},
			expectedMountCalls:     1,
			expectedUnmountCalls:   1,
			expectedClaimCalls:     2,
			expectedUnpublishCalls: 1,
		},

		// TODO: this won't actually work on the client.
		// https://github.com/hashicorp/nomad/issues/11798
		//
		// {
		// 	name: "one source volume mounted read-only twice",
		// 	volumeRequests: map[string]*structs.VolumeRequest{
		// 		"vol0": {
		// 			Name:           "vol0",
		// 			Type:           structs.VolumeTypeCSI,
		// 			Source:         "testvolume0",
		// 			ReadOnly:       true,
		// 			AccessMode:     structs.CSIVolumeAccessModeMultiNodeReader,
		// 			AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
		// 			MountOptions:   &structs.CSIMountOptions{},
		// 			PerAlloc:       false,
		// 		},
		// 		"vol1": {
		// 			Name:           "vol1",
		// 			Type:           structs.VolumeTypeCSI,
		// 			Source:         "testvolume0",
		// 			ReadOnly:       false,
		// 			AccessMode:     structs.CSIVolumeAccessModeMultiNodeReader,
		// 			AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
		// 			MountOptions:   &structs.CSIMountOptions{},
		// 			PerAlloc:       false,
		// 		},
		// 	},
		// 	expectedMounts: map[string]*csimanager.MountInfo{
		// 		"vol0": &csimanager.MountInfo{Source: fmt.Sprintf(
		// 			"test-alloc-dir/%s/testvolume0/ro-file-system-multi-node-reader-only", alloc.ID)},
		// 		"vol1": &csimanager.MountInfo{Source: fmt.Sprintf(
		// 			"test-alloc-dir/%s/testvolume0/ro-file-system-multi-node-reader-only", alloc.ID)},
		// 	},
		// 	expectedMountCalls:     1,
		// 	expectedUnmountCalls:   1,
		// 	expectedClaimCalls:     1,
		// 	expectedUnpublishCalls: 1,
		// },
	}

	for i := range testcases {
		tc := testcases[i]
		t.Run(tc.name, func(t *testing.T) {
			alloc.Job.TaskGroups[0].Volumes = tc.volumeRequests

			callCounts := map[string]int{}
			mgr := mockPluginManager{mounter: mockVolumeMounter{callCounts: callCounts}}
			rpcer := mockRPCer{
				alloc:            alloc,
				callCounts:       callCounts,
				hasExistingClaim: pointer.Of(tc.startsWithClaims),
				schedulable:      pointer.Of(!tc.startsUnschedulable),
			}
			ar := mockAllocRunner{
				res: &cstructs.AllocHookResources{},
				caps: &drivers.Capabilities{
					FSIsolation:  drivers.FSIsolationChroot,
					MountConfigs: drivers.MountConfigSupportAll,
				},
			}
			hook := newCSIHook(alloc, logger, mgr, rpcer, ar, ar.res, "secret")
			hook.minBackoffInterval = 1 * time.Millisecond
			hook.maxBackoffInterval = 10 * time.Millisecond
			hook.maxBackoffDuration = 500 * time.Millisecond

			require.NotNil(t, hook)

			if tc.expectedClaimErr != nil {
				require.EqualError(t, hook.Prerun(), tc.expectedClaimErr.Error())
				mounts := ar.res.GetCSIMounts()
				require.Nil(t, mounts)
			} else {
				require.NoError(t, hook.Prerun())
				mounts := ar.res.GetCSIMounts()
				require.NotNil(t, mounts)
				require.Equal(t, tc.expectedMounts, mounts)
				require.NoError(t, hook.Postrun())
			}

			require.Equal(t, tc.expectedMountCalls, callCounts["mount"])
			require.Equal(t, tc.expectedUnmountCalls, callCounts["unmount"])
			require.Equal(t, tc.expectedClaimCalls, callCounts["claim"])
			require.Equal(t, tc.expectedUnpublishCalls, callCounts["unpublish"])

		})
	}

}

// TestCSIHook_claimVolumesFromAlloc_Validation tests that the validation of task
// capabilities in claimVolumesFromAlloc ensures at least one task supports CSI.
func TestCSIHook_claimVolumesFromAlloc_Validation(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.Alloc()
	logger := testlog.HCLogger(t)
	volumeRequests := map[string]*structs.VolumeRequest{
		"vol0": {
			Name:           "vol0",
			Type:           structs.VolumeTypeCSI,
			Source:         "testvolume0",
			ReadOnly:       true,
			AccessMode:     structs.CSIVolumeAccessModeSingleNodeReader,
			AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
			MountOptions:   &structs.CSIMountOptions{},
			PerAlloc:       false,
		},
	}

	type testCase struct {
		name             string
		caps             *drivers.Capabilities
		capFunc          func() (*drivers.Capabilities, error)
		expectedClaimErr error
	}

	testcases := []testCase{
		{
			name: "invalid - driver does not support CSI",
			caps: &drivers.Capabilities{
				MountConfigs: drivers.MountConfigSupportNone,
			},
			capFunc:          nil,
			expectedClaimErr: errors.New("claim volumes: no task supports CSI"),
		},

		{
			name: "invalid - driver error",
			caps: &drivers.Capabilities{},
			capFunc: func() (*drivers.Capabilities, error) {
				return nil, errors.New("error thrown by driver")
			},
			expectedClaimErr: errors.New("claim volumes: could not validate task driver capabilities: error thrown by driver"),
		},

		{
			name: "valid - driver supports CSI",
			caps: &drivers.Capabilities{
				MountConfigs: drivers.MountConfigSupportAll,
			},
			capFunc:          nil,
			expectedClaimErr: nil,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			alloc.Job.TaskGroups[0].Volumes = volumeRequests

			callCounts := map[string]int{}
			mgr := mockPluginManager{mounter: mockVolumeMounter{callCounts: callCounts}}

			rpcer := mockRPCer{
				alloc:            alloc,
				callCounts:       callCounts,
				hasExistingClaim: pointer.Of(false),
				schedulable:      pointer.Of(true),
			}

			ar := mockAllocRunner{
				res:     &cstructs.AllocHookResources{},
				caps:    tc.caps,
				capFunc: tc.capFunc,
			}

			hook := newCSIHook(alloc, logger, mgr, rpcer, ar, ar.res, "secret")
			require.NotNil(t, hook)

			if tc.expectedClaimErr != nil {
				require.EqualError(t, hook.Prerun(), tc.expectedClaimErr.Error())
				mounts := ar.res.GetCSIMounts()
				require.Nil(t, mounts)
			} else {
				require.NoError(t, hook.Prerun())
				mounts := ar.res.GetCSIMounts()
				require.NotNil(t, mounts)
				require.NoError(t, hook.Postrun())
			}
		})
	}
}

// HELPERS AND MOCKS

type mockRPCer struct {
	alloc            *structs.Allocation
	callCounts       map[string]int
	hasExistingClaim *bool
	schedulable      *bool
}

// RPC mocks the server RPCs, acting as though any request succeeds
func (r mockRPCer) RPC(method string, args interface{}, reply interface{}) error {
	switch method {
	case "CSIVolume.Claim":
		r.callCounts["claim"]++
		req := args.(*structs.CSIVolumeClaimRequest)
		vol := r.testVolume(req.VolumeID)
		err := vol.Claim(req.ToClaim(), r.alloc)
		if err != nil {
			return err
		}

		resp := reply.(*structs.CSIVolumeClaimResponse)
		resp.PublishContext = map[string]string{}
		resp.Volume = vol
		resp.QueryMeta = structs.QueryMeta{}
	case "CSIVolume.Unpublish":
		r.callCounts["unpublish"]++
		resp := reply.(*structs.CSIVolumeUnpublishResponse)
		resp.QueryMeta = structs.QueryMeta{}
	default:
		return fmt.Errorf("unexpected method")
	}
	return nil
}

// testVolume is a helper that optionally starts as unschedulable /
// claimed until after the first claim RPC is made, so that we can
// test retryable vs non-retryable failures
func (r mockRPCer) testVolume(id string) *structs.CSIVolume {
	vol := structs.NewCSIVolume(id, 0)
	vol.Schedulable = *r.schedulable
	vol.RequestedCapabilities = []*structs.CSIVolumeCapability{
		{
			AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
			AccessMode:     structs.CSIVolumeAccessModeSingleNodeReader,
		},
		{
			AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
			AccessMode:     structs.CSIVolumeAccessModeSingleNodeWriter,
		},
	}

	if *r.hasExistingClaim {
		vol.AccessMode = structs.CSIVolumeAccessModeSingleNodeReader
		vol.AttachmentMode = structs.CSIVolumeAttachmentModeFilesystem
		vol.ReadClaims["another-alloc-id"] = &structs.CSIVolumeClaim{
			AllocationID:   "another-alloc-id",
			NodeID:         "another-node-id",
			Mode:           structs.CSIVolumeClaimRead,
			AccessMode:     structs.CSIVolumeAccessModeSingleNodeReader,
			AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
			State:          structs.CSIVolumeClaimStateTaken,
		}
	}

	if r.callCounts["claim"] >= 0 {
		*r.hasExistingClaim = false
		*r.schedulable = true
	}

	return vol
}

type mockVolumeMounter struct {
	callCounts map[string]int
}

func (vm mockVolumeMounter) MountVolume(ctx context.Context, vol *structs.CSIVolume, alloc *structs.Allocation, usageOpts *csimanager.UsageOptions, publishContext map[string]string) (*csimanager.MountInfo, error) {
	vm.callCounts["mount"]++
	return &csimanager.MountInfo{
		Source: filepath.Join("test-alloc-dir", alloc.ID, vol.ID, usageOpts.ToFS()),
	}, nil
}
func (vm mockVolumeMounter) UnmountVolume(ctx context.Context, volID, remoteID, allocID string, usageOpts *csimanager.UsageOptions) error {
	vm.callCounts["unmount"]++
	return nil
}

type mockPluginManager struct {
	mounter mockVolumeMounter
}

func (mgr mockPluginManager) WaitForPlugin(ctx context.Context, pluginType, pluginID string) error {
	return nil
}

func (mgr mockPluginManager) MounterForPlugin(ctx context.Context, pluginID string) (csimanager.VolumeMounter, error) {
	return mgr.mounter, nil
}

// no-op methods to fulfill the interface
func (mgr mockPluginManager) PluginManager() pluginmanager.PluginManager { return nil }
func (mgr mockPluginManager) Shutdown()                                  {}

type mockAllocRunner struct {
	res     *cstructs.AllocHookResources
	caps    *drivers.Capabilities
	capFunc func() (*drivers.Capabilities, error)
}

func (ar mockAllocRunner) GetTaskDriverCapabilities(taskName string) (*drivers.Capabilities, error) {
	if ar.capFunc != nil {
		return ar.capFunc()
	}
	return ar.caps, nil
}
