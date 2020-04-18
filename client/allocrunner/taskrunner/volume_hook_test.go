package taskrunner

import (
	"testing"

	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/pluginmanager/csimanager"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
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
				"foo": {
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

func TestVolumeHook_Interpolation(t *testing.T) {

	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	taskEnv := taskenv.NewBuilder(mock.Node(), alloc, task, "global").SetHookEnv("volume",
		map[string]string{
			"PROPAGATION_MODE": "private",
			"VOLUME_ID":        "my-other-volume",
		},
	).Build()

	mounts := []*structs.VolumeMount{
		{
			Volume:          "foo",
			Destination:     "/tmp",
			ReadOnly:        false,
			PropagationMode: "bidirectional",
		},
		{
			Volume:          "foo",
			Destination:     "/bar-${NOMAD_JOB_NAME}",
			ReadOnly:        false,
			PropagationMode: "bidirectional",
		},
		{
			Volume:          "${VOLUME_ID}",
			Destination:     "/baz",
			ReadOnly:        false,
			PropagationMode: "bidirectional",
		},
		{
			Volume:          "foo",
			Destination:     "/quux",
			ReadOnly:        false,
			PropagationMode: "${PROPAGATION_MODE}",
		},
	}

	expected := []*structs.VolumeMount{
		{
			Volume:          "foo",
			Destination:     "/tmp",
			ReadOnly:        false,
			PropagationMode: "bidirectional",
		},
		{
			Volume:          "foo",
			Destination:     "/bar-my-job",
			ReadOnly:        false,
			PropagationMode: "bidirectional",
		},
		{
			Volume:          "my-other-volume",
			Destination:     "/baz",
			ReadOnly:        false,
			PropagationMode: "bidirectional",
		},
		{
			Volume:          "foo",
			Destination:     "/quux",
			ReadOnly:        false,
			PropagationMode: "private",
		},
	}

	interpolateVolumeMounts(mounts, taskEnv)
	require.Equal(t, expected, mounts)
}
