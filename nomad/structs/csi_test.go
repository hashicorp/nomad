package structs

import (
	"testing"

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
	require.True(t, vol.WriteFreeClaims())
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
