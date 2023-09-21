package allocrunner

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/allocrunner/state"
	"github.com/hashicorp/nomad/client/pluginmanager/csimanager"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
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
				"claim": 1, "MountVolume": 1, "UnmountVolume": 1, "unpublish": 1},
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
				"claim": 1, "MountVolume": 1, "UnmountVolume": 1, "unpublish": 1},
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
				"claim": 2, "MountVolume": 1, "UnmountVolume": 1, "unpublish": 1},
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
			expectedCalls: map[string]int{"HasMount": 1, "UnmountVolume": 1, "unpublish": 1},
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
				"HasMount": 1, "claim": 1, "MountVolume": 1, "UnmountVolume": 1, "unpublish": 1},
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
				"claim": 1, "MountVolume": 1, "UnmountVolume": 2, "unpublish": 2},
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

			callCounts := testutil.NewCallCounter()
			vm := &csimanager.MockVolumeManager{
				CallCounter: callCounts,
			}
			mgr := &csimanager.MockCSIManager{VM: vm}
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

			if tc.startsWithValidMounts {
				// TODO: this works, but it requires knowledge of how the mock works.  would rather vm.MountVolume()
				vm.Mounts = map[string]bool{
					tc.expectedMounts["vol0"].Source: true,
				}
			}

			if tc.failsFirstUnmount {
				vm.NextUnmountVolumeErr = errors.New("bad first attempt")
			}

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

			counts := callCounts.Get()
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

			mgr := &csimanager.MockCSIManager{
				VM: &csimanager.MockVolumeManager{},
			}
			rpcer := mockRPCer{
				alloc:            alloc,
				callCounts:       testutil.NewCallCounter(),
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

type mockRPCer struct {
	alloc            *structs.Allocation
	callCounts       *testutil.CallCounter
	hasExistingClaim *bool
	schedulable      *bool
}

// RPC mocks the server RPCs, acting as though any request succeeds
func (r mockRPCer) RPC(method string, args any, reply any) error {
	switch method {
	case "CSIVolume.Claim":
		r.callCounts.Inc("claim")
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
		r.callCounts.Inc("unpublish")
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
