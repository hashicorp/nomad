package allocrunner

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/pluginmanager"
	"github.com/hashicorp/nomad/client/pluginmanager/csimanager"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
)

var _ interfaces.RunnerPrerunHook = (*csiHook)(nil)
var _ interfaces.RunnerPostrunHook = (*csiHook)(nil)

// TODO https://github.com/hashicorp/nomad/issues/11786
// we should implement Update as well
// var _ interfaces.RunnerUpdateHook = (*csiHook)(nil)

func TestCSIHook(t *testing.T) {

	alloc := mock.Alloc()
	logger := testlog.HCLogger(t)

	testcases := []struct {
		name                   string
		volumeRequests         map[string]*structs.VolumeRequest
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
					Name:         "vol0",
					Type:         structs.VolumeTypeCSI,
					Source:       "testvolume0",
					ReadOnly:     true,
					MountOptions: &structs.CSIMountOptions{},
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
					Name:         "vol0",
					Type:         structs.VolumeTypeCSI,
					Source:       "testvolume0",
					ReadOnly:     true,
					MountOptions: &structs.CSIMountOptions{},
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
		// 			MountOptions:   &structs.CSIMountOptions{},
		// 		},
		// 		"vol1": {
		// 			Name:           "vol1",
		// 			Type:           structs.VolumeTypeCSI,
		// 			Source:         "testvolume0",
		// 			ReadOnly:       false,
		// 			MountOptions:   &structs.CSIMountOptions{},
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
			rpcer := mockRPCer{alloc: alloc, callCounts: callCounts}
			ar := mockAllocRunner{
				res: &cstructs.AllocHookResources{},
				caps: &drivers.Capabilities{
					FSIsolation:  drivers.FSIsolationChroot,
					MountConfigs: drivers.MountConfigSupportAll,
				},
			}
			hook := newCSIHook(alloc, logger, mgr, rpcer, ar, ar, "secret")
			hook.maxBackoffInterval = 100 * time.Millisecond
			hook.maxBackoffDuration = 2 * time.Second

			require.NotNil(t, hook)

			require.NoError(t, hook.Prerun())
			mounts := ar.GetAllocHookResources().GetCSIMounts()
			require.NotNil(t, mounts)
			require.Equal(t, tc.expectedMounts, mounts)

			require.NoError(t, hook.Postrun())
			require.Equal(t, tc.expectedMountCalls, callCounts["mount"])
			require.Equal(t, tc.expectedUnmountCalls, callCounts["unmount"])
			require.Equal(t, tc.expectedClaimCalls, callCounts["claim"])
			require.Equal(t, tc.expectedUnpublishCalls, callCounts["unpublish"])

		})
	}

}

// HELPERS AND MOCKS

func testVolume(id string) *structs.CSIVolume {
	vol := structs.NewCSIVolume(id, 0)
	vol.Schedulable = true
	return vol
}

type mockRPCer struct {
	alloc      *structs.Allocation
	callCounts map[string]int
}

// RPC mocks the server RPCs, acting as though any request succeeds
func (r mockRPCer) RPC(method string, args interface{}, reply interface{}) error {
	switch method {
	case "CSIVolume.Claim":
		r.callCounts["claim"]++
		req := args.(*structs.CSIVolumeClaimRequest)
		vol := testVolume(req.VolumeID)
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

func (mgr mockPluginManager) MounterForPlugin(ctx context.Context, pluginID string) (csimanager.VolumeMounter, error) {
	return mgr.mounter, nil
}

// no-op methods to fulfill the interface
func (mgr mockPluginManager) PluginManager() pluginmanager.PluginManager { return nil }
func (mgr mockPluginManager) Shutdown()                                  {}

type mockAllocRunner struct {
	res  *cstructs.AllocHookResources
	caps *drivers.Capabilities
}

func (ar mockAllocRunner) GetAllocHookResources() *cstructs.AllocHookResources {
	return ar.res
}

func (ar mockAllocRunner) SetAllocHookResources(res *cstructs.AllocHookResources) {
	ar.res = res
}

func (ar mockAllocRunner) GetTaskDriverCapabilities(taskName string) (*drivers.Capabilities, error) {
	return ar.caps, nil
}
