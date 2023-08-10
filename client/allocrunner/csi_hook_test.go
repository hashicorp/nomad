// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/allocrunner/state"
	"github.com/hashicorp/nomad/client/pluginmanager"
	"github.com/hashicorp/nomad/client/pluginmanager/csimanager"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"
)

var _ interfaces.RunnerPrerunHook = (*csiHook)(nil)
var _ interfaces.RunnerPostrunHook = (*csiHook)(nil)

func TestCSIHook(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.Alloc()
	testMountSrc := fmt.Sprintf(
		"test-alloc-dir/%s/testvolume0/ro-file-system-single-node-reader-only", alloc.ID)
	logger := testlog.HCLogger(t)

	testcases := []struct {
		name                  string
		volumeRequests        map[string]*structs.VolumeRequest
		startsUnschedulable   bool
		startsWithClaims      bool
		startsWithStubs       map[string]*state.CSIVolumeStub
		startsWithValidMounts bool
		failsFirstUnmount     bool
		expectedClaimErr      error
		expectedMounts        map[string]*csimanager.MountInfo
		expectedCalls         map[string]int
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
				"vol0": &csimanager.MountInfo{Source: testMountSrc},
			},
			expectedCalls: map[string]int{
				"claim": 1, "mount": 1, "unmount": 1, "unpublish": 1},
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
				"vol0": &csimanager.MountInfo{Source: testMountSrc},
			},
			expectedCalls: map[string]int{
				"claim": 1, "mount": 1, "unmount": 1, "unpublish": 1},
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
				"vol0": &csimanager.MountInfo{Source: testMountSrc},
			},
			expectedCalls: map[string]int{"claim": 1},
			expectedClaimErr: errors.New(
				"claiming volumes: could not claim volume testvolume0: volume is currently unschedulable"),
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
				"vol0": &csimanager.MountInfo{Source: testMountSrc},
			},
			expectedCalls: map[string]int{
				"claim": 2, "mount": 1, "unmount": 1, "unpublish": 1},
		},
		{
			name: "already mounted",
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
			startsWithStubs: map[string]*state.CSIVolumeStub{"vol0": {
				VolumeID:       "vol0",
				PluginID:       "vol0-plugin",
				ExternalNodeID: "i-example",
				MountInfo:      &csimanager.MountInfo{Source: testMountSrc},
			}},
			startsWithValidMounts: true,
			expectedMounts: map[string]*csimanager.MountInfo{
				"vol0": &csimanager.MountInfo{Source: testMountSrc},
			},
			expectedCalls: map[string]int{"hasMount": 1, "unmount": 1, "unpublish": 1},
		},
		{
			name: "existing but invalid mounts",
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
			startsWithStubs: map[string]*state.CSIVolumeStub{"vol0": {
				VolumeID:       "testvolume0",
				PluginID:       "vol0-plugin",
				ExternalNodeID: "i-example",
				MountInfo:      &csimanager.MountInfo{Source: testMountSrc},
			}},
			startsWithValidMounts: false,
			expectedMounts: map[string]*csimanager.MountInfo{
				"vol0": &csimanager.MountInfo{Source: testMountSrc},
			},
			expectedCalls: map[string]int{
				"hasMount": 1, "claim": 1, "mount": 1, "unmount": 1, "unpublish": 1},
		},

		{
			name: "retry on failed unmount",
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
			failsFirstUnmount: true,
			expectedMounts: map[string]*csimanager.MountInfo{
				"vol0": &csimanager.MountInfo{Source: testMountSrc},
			},
			expectedCalls: map[string]int{
				"claim": 1, "mount": 1, "unmount": 2, "unpublish": 2},
		},

		{
			name:           "should not run",
			volumeRequests: map[string]*structs.VolumeRequest{},
		},
	}

	for i := range testcases {
		tc := testcases[i]
		t.Run(tc.name, func(t *testing.T) {

			alloc.Job.TaskGroups[0].Volumes = tc.volumeRequests

			callCounts := &callCounter{counts: map[string]int{}}
			mgr := mockPluginManager{mounter: mockVolumeMounter{
				hasMounts:         tc.startsWithValidMounts,
				callCounts:        callCounts,
				failsFirstUnmount: pointer.Of(tc.failsFirstUnmount),
			}}
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
				stubs: tc.startsWithStubs,
			}

			hook := newCSIHook(alloc, logger, mgr, rpcer, ar, ar.res, "secret")
			hook.minBackoffInterval = 1 * time.Millisecond
			hook.maxBackoffInterval = 10 * time.Millisecond
			hook.maxBackoffDuration = 500 * time.Millisecond

			must.NotNil(t, hook)

			if tc.expectedClaimErr != nil {
				must.EqError(t, hook.Prerun(), tc.expectedClaimErr.Error())
				mounts := ar.res.GetCSIMounts()
				must.Nil(t, mounts)
			} else {
				must.NoError(t, hook.Prerun())
				mounts := ar.res.GetCSIMounts()
				must.MapEq(t, tc.expectedMounts, mounts,
					must.Sprintf("got mounts: %v", mounts))
				must.NoError(t, hook.Postrun())
			}

			if tc.failsFirstUnmount {
				// retrying the unmount doesn't block Postrun, so give it time
				// to run once more before checking the call counts to ensure
				// this doesn't flake between 1 and 2 unmount/unpublish calls
				time.Sleep(100 * time.Millisecond)
			}

			counts := callCounts.get()
			must.MapEq(t, tc.expectedCalls, counts,
				must.Sprintf("got calls: %v", counts))

		})
	}

}

// TestCSIHook_Prerun_Validation tests that the validation of task capabilities
// in Prerun ensures at least one task supports CSI.
func TestCSIHook_Prerun_Validation(t *testing.T) {
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
		name        string
		caps        *drivers.Capabilities
		capFunc     func() (*drivers.Capabilities, error)
		expectedErr string
	}

	testcases := []testCase{
		{
			name: "invalid - driver does not support CSI",
			caps: &drivers.Capabilities{
				MountConfigs: drivers.MountConfigSupportNone,
			},
			capFunc:     nil,
			expectedErr: "no task supports CSI",
		},

		{
			name: "invalid - driver error",
			caps: &drivers.Capabilities{},
			capFunc: func() (*drivers.Capabilities, error) {
				return nil, errors.New("error thrown by driver")
			},
			expectedErr: "could not validate task driver capabilities: error thrown by driver",
		},

		{
			name: "valid - driver supports CSI",
			caps: &drivers.Capabilities{
				MountConfigs: drivers.MountConfigSupportAll,
			},
			capFunc: nil,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			alloc.Job.TaskGroups[0].Volumes = volumeRequests

			callCounts := &callCounter{counts: map[string]int{}}
			mgr := mockPluginManager{mounter: mockVolumeMounter{
				callCounts:        callCounts,
				failsFirstUnmount: pointer.Of(false),
			}}
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

			if tc.expectedErr != "" {
				require.EqualError(t, hook.Prerun(), tc.expectedErr)
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

type callCounter struct {
	lock   sync.Mutex
	counts map[string]int
}

func (c *callCounter) inc(name string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.counts[name]++
}

func (c *callCounter) get() map[string]int {
	c.lock.Lock()
	defer c.lock.Unlock()
	return maps.Clone(c.counts)
}

type mockRPCer struct {
	alloc            *structs.Allocation
	callCounts       *callCounter
	hasExistingClaim *bool
	schedulable      *bool
}

// RPC mocks the server RPCs, acting as though any request succeeds
func (r mockRPCer) RPC(method string, args any, reply any) error {
	switch method {
	case "CSIVolume.Claim":
		r.callCounts.inc("claim")
		req := args.(*structs.CSIVolumeClaimRequest)
		vol := r.testVolume(req.VolumeID)
		err := vol.Claim(req.ToClaim(), r.alloc)

		// after the first claim attempt is made, reset the volume's claims as
		// though it's been released from another node
		*r.hasExistingClaim = false
		*r.schedulable = true

		if err != nil {
			return err
		}

		resp := reply.(*structs.CSIVolumeClaimResponse)
		resp.PublishContext = map[string]string{}
		resp.Volume = vol
		resp.QueryMeta = structs.QueryMeta{}

	case "CSIVolume.Unpublish":
		r.callCounts.inc("unpublish")
		resp := reply.(*structs.CSIVolumeUnpublishResponse)
		resp.QueryMeta = structs.QueryMeta{}

	default:
		return fmt.Errorf("unexpected method")
	}
	return nil
}

// testVolume is a helper that optionally starts as unschedulable / claimed, so
// that we can test retryable vs non-retryable failures
func (r mockRPCer) testVolume(id string) *structs.CSIVolume {
	vol := structs.NewCSIVolume(id, 0)
	vol.Schedulable = *r.schedulable
	vol.PluginID = "plugin-" + id
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

	return vol
}

type mockVolumeMounter struct {
	hasMounts         bool
	failsFirstUnmount *bool
	callCounts        *callCounter
}

func (vm mockVolumeMounter) MountVolume(ctx context.Context, vol *structs.CSIVolume, alloc *structs.Allocation, usageOpts *csimanager.UsageOptions, publishContext map[string]string) (*csimanager.MountInfo, error) {
	vm.callCounts.inc("mount")
	return &csimanager.MountInfo{
		Source: filepath.Join("test-alloc-dir", alloc.ID, vol.ID, usageOpts.ToFS()),
	}, nil
}

func (vm mockVolumeMounter) UnmountVolume(ctx context.Context, volID, remoteID, allocID string, usageOpts *csimanager.UsageOptions) error {
	vm.callCounts.inc("unmount")

	if *vm.failsFirstUnmount {
		*vm.failsFirstUnmount = false
		return fmt.Errorf("could not unmount")
	}

	return nil
}

func (vm mockVolumeMounter) HasMount(_ context.Context, mountInfo *csimanager.MountInfo) (bool, error) {
	vm.callCounts.inc("hasMount")
	return mountInfo != nil && vm.hasMounts, nil
}

func (vm mockVolumeMounter) ExternalID() string {
	return "i-example"
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

	stubs    map[string]*state.CSIVolumeStub
	stubFunc func() (map[string]*state.CSIVolumeStub, error)
}

func (ar mockAllocRunner) GetTaskDriverCapabilities(taskName string) (*drivers.Capabilities, error) {
	if ar.capFunc != nil {
		return ar.capFunc()
	}
	return ar.caps, nil
}

func (ar mockAllocRunner) SetCSIVolumes(stubs map[string]*state.CSIVolumeStub) error {
	ar.stubs = stubs
	return nil
}

func (ar mockAllocRunner) GetCSIVolumes() (map[string]*state.CSIVolumeStub, error) {
	if ar.stubFunc != nil {
		return ar.stubFunc()
	}
	return ar.stubs, nil
}
