package structs

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCSIVolumeClaim(t *testing.T) {
	vol := NewCSIVolume("", 0)
	vol.AccessMode = CSIVolumeAccessModeMultiNodeSingleWriter
	vol.Schedulable = true

	alloc := &Allocation{ID: "a1", Namespace: "n", JobID: "j"}
	claim := &CSIVolumeClaim{
		AllocationID: alloc.ID,
		NodeID:       "foo",
		Mode:         CSIVolumeClaimRead,
	}

	require.NoError(t, vol.ClaimRead(claim, alloc))
	require.True(t, vol.ReadSchedulable())
	require.True(t, vol.WriteSchedulable())
	require.NoError(t, vol.ClaimRead(claim, alloc))

	claim.Mode = CSIVolumeClaimWrite
	require.NoError(t, vol.ClaimWrite(claim, alloc))
	require.True(t, vol.ReadSchedulable())
	require.False(t, vol.WriteFreeClaims())

	vol.ClaimRelease(claim)
	require.True(t, vol.ReadSchedulable())
	require.False(t, vol.WriteFreeClaims())

	claim.State = CSIVolumeClaimStateReadyToFree
	vol.ClaimRelease(claim)
	require.True(t, vol.ReadSchedulable())
	require.True(t, vol.WriteFreeClaims())

	vol.AccessMode = CSIVolumeAccessModeMultiNodeMultiWriter
	require.NoError(t, vol.ClaimWrite(claim, alloc))
	require.NoError(t, vol.ClaimWrite(claim, alloc))
	require.True(t, vol.WriteFreeClaims())
}

func TestVolume_Copy(t *testing.T) {

	a1 := MockAlloc()
	a2 := MockAlloc()
	a3 := MockAlloc()
	c1 := &CSIVolumeClaim{
		AllocationID:   a1.ID,
		NodeID:         a1.NodeID,
		ExternalNodeID: "c1",
		Mode:           CSIVolumeClaimRead,
		State:          CSIVolumeClaimStateTaken,
	}
	c2 := &CSIVolumeClaim{
		AllocationID:   a2.ID,
		NodeID:         a2.NodeID,
		ExternalNodeID: "c2",
		Mode:           CSIVolumeClaimRead,
		State:          CSIVolumeClaimStateNodeDetached,
	}
	c3 := &CSIVolumeClaim{
		AllocationID:   a3.ID,
		NodeID:         a3.NodeID,
		ExternalNodeID: "c3",
		Mode:           CSIVolumeClaimWrite,
		State:          CSIVolumeClaimStateTaken,
	}

	v1 := &CSIVolume{
		ID:             "vol1",
		Name:           "vol1",
		ExternalID:     "vol-abcdef",
		Namespace:      "default",
		Topologies:     []*CSITopology{{Segments: map[string]string{"AZ1": "123"}}},
		AccessMode:     CSIVolumeAccessModeSingleNodeWriter,
		AttachmentMode: CSIVolumeAttachmentModeBlockDevice,
		MountOptions:   &CSIMountOptions{FSType: "ext4", MountFlags: []string{"ro", "noatime"}},
		Secrets:        CSISecrets{"mysecret": "myvalue"},
		Parameters:     map[string]string{"param1": "val1"},
		Context:        map[string]string{"ctx1": "val1"},

		ReadAllocs:  map[string]*Allocation{a1.ID: a1, a2.ID: nil},
		WriteAllocs: map[string]*Allocation{a3.ID: a3},

		ReadClaims:  map[string]*CSIVolumeClaim{a1.ID: c1, a2.ID: c2},
		WriteClaims: map[string]*CSIVolumeClaim{a3.ID: c3},
		PastClaims:  map[string]*CSIVolumeClaim{},

		Schedulable:         true,
		PluginID:            "moosefs",
		Provider:            "n/a",
		ProviderVersion:     "1.0",
		ControllerRequired:  true,
		ControllersHealthy:  2,
		ControllersExpected: 2,
		NodesHealthy:        4,
		NodesExpected:       5,
		ResourceExhausted:   time.Now(),
	}

	v2 := v1.Copy()
	if !reflect.DeepEqual(v1, v2) {
		t.Fatalf("Copy() returned an unequal Volume; got %#v; want %#v", v1, v2)
	}

	v1.ReadClaims[a1.ID].State = CSIVolumeClaimStateReadyToFree
	v1.ReadAllocs[a2.ID] = a2
	v1.WriteAllocs[a3.ID].ClientStatus = AllocClientStatusComplete
	v1.MountOptions.FSType = "zfs"

	if v2.ReadClaims[a1.ID].State == CSIVolumeClaimStateReadyToFree {
		t.Fatalf("Volume.Copy() failed; changes to original ReadClaims seen in copy")
	}
	if v2.ReadAllocs[a2.ID] != nil {
		t.Fatalf("Volume.Copy() failed; changes to original ReadAllocs seen in copy")
	}
	if v2.WriteAllocs[a3.ID].ClientStatus == AllocClientStatusComplete {
		t.Fatalf("Volume.Copy() failed; changes to original WriteAllocs seen in copy")
	}
	if v2.MountOptions.FSType == "zfs" {
		t.Fatalf("Volume.Copy() failed; changes to original MountOptions seen in copy")
	}

}

func TestCSIPluginJobs(t *testing.T) {
	plug := NewCSIPlugin("foo", 1000)
	controller := &Job{
		ID:   "job",
		Type: "service",
		TaskGroups: []*TaskGroup{{
			Name:  "foo",
			Count: 11,
			Tasks: []*Task{{
				CSIPluginConfig: &TaskCSIPluginConfig{
					ID:   "foo",
					Type: CSIPluginTypeController,
				},
			}},
		}},
	}

	summary := &JobSummary{}

	plug.AddJob(controller, summary)
	require.Equal(t, 11, plug.ControllersExpected)

	// New job id & make it a system node plugin job
	node := controller.Copy()
	node.ID = "bar"
	node.Type = "system"
	node.TaskGroups[0].Tasks[0].CSIPluginConfig.Type = CSIPluginTypeNode

	summary = &JobSummary{
		Summary: map[string]TaskGroupSummary{
			"foo": {
				Queued:   1,
				Running:  1,
				Starting: 1,
			},
		},
	}

	plug.AddJob(node, summary)
	require.Equal(t, 3, plug.NodesExpected)

	plug.DeleteJob(node, summary)
	require.Equal(t, 0, plug.NodesExpected)
	require.Empty(t, plug.NodeJobs[""])

	plug.DeleteJob(controller, nil)
	require.Equal(t, 0, plug.ControllersExpected)
	require.Empty(t, plug.ControllerJobs[""])
}

func TestCSIPluginCleanup(t *testing.T) {
	plug := NewCSIPlugin("foo", 1000)
	plug.AddPlugin("n0", &CSIInfo{
		PluginID:                 "foo",
		AllocID:                  "a0",
		Healthy:                  true,
		Provider:                 "foo-provider",
		RequiresControllerPlugin: true,
		RequiresTopologies:       false,
		ControllerInfo:           &CSIControllerInfo{},
	})

	plug.AddPlugin("n0", &CSIInfo{
		PluginID:                 "foo",
		AllocID:                  "a0",
		Healthy:                  true,
		Provider:                 "foo-provider",
		RequiresControllerPlugin: true,
		RequiresTopologies:       false,
		NodeInfo:                 &CSINodeInfo{},
	})

	require.Equal(t, 1, plug.ControllersHealthy)
	require.Equal(t, 1, plug.NodesHealthy)

	plug.DeleteNode("n0")
	require.Equal(t, 0, plug.ControllersHealthy)
	require.Equal(t, 0, plug.NodesHealthy)

	require.Equal(t, 0, len(plug.Controllers))
	require.Equal(t, 0, len(plug.Nodes))
}
