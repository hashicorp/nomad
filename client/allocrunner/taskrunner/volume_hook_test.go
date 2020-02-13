package taskrunner

import (
	"testing"

	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/pluginmanager/csimanager"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/stretchr/testify/require"
)

func TestVolumeHook_PartitionMountsByVolume_Works(t *testing.T) {
	mounts := []*structs.VolumeMount{
		{
			Volume:      "foo",
			Destination: "/tmp",
			ReadOnly:    false,
		},
		{
			Volume:      "foo",
			Destination: "/bar",
			ReadOnly:    false,
		},
		{
			Volume:      "baz",
			Destination: "/baz",
			ReadOnly:    false,
		},
	}

	expected := map[string][]*structs.VolumeMount{
		"foo": {
			{
				Volume:      "foo",
				Destination: "/tmp",
				ReadOnly:    false,
			},
			{
				Volume:      "foo",
				Destination: "/bar",
				ReadOnly:    false,
			},
		},
		"baz": {
			{
				Volume:      "baz",
				Destination: "/baz",
				ReadOnly:    false,
			},
		},
	}

	// Test with a real collection

	partitioned := partitionMountsByVolume(mounts)
	require.Equal(t, expected, partitioned)

	// Test with nil/emptylist

	partitioned = partitionMountsByVolume(nil)
	require.Equal(t, map[string][]*structs.VolumeMount{}, partitioned)
}

func TestVolumeHook_prepareCSIVolumes(t *testing.T) {
	req := &interfaces.TaskPrestartRequest{
		Task: &structs.Task{
			VolumeMounts: []*structs.VolumeMount{
				{
					Volume:      "foo",
					Destination: "/bar",
				},
			},
		},
	}

	volumes := map[string]*structs.VolumeRequest{
		"foo": {
			Type:   "csi",
			Source: "my-test-volume",
		},
	}

	tr := &TaskRunner{
		allocHookResources: &cstructs.AllocHookResources{
			CSIMounts: map[string]*csimanager.MountInfo{
				"foo": &csimanager.MountInfo{
					Source: "/mnt/my-test-volume",
				},
			},
		},
	}

	expected := []*drivers.MountConfig{
		{
			HostPath: "/mnt/my-test-volume",
			TaskPath: "/bar",
		},
	}

	hook := &volumeHook{
		logger: testlog.HCLogger(t),
		alloc:  structs.MockAlloc(),
		runner: tr,
	}
	mounts, err := hook.prepareCSIVolumes(req, volumes)
	require.NoError(t, err)
	require.Equal(t, expected, mounts)
}
