package structs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCSIVolumeClaim(t *testing.T) {
	vol := NewCSIVolume("", 0)
	vol.AccessMode = CSIVolumeAccessModeMultiNodeSingleWriter
	vol.Schedulable = true

	alloc := &Allocation{ID: "a1"}

	vol.ClaimRead(alloc)
	require.True(t, vol.ReadSchedulable())
	require.True(t, vol.WriteSchedulable())
	require.True(t, vol.ClaimRead(alloc))

	vol.ClaimWrite(alloc)
	require.True(t, vol.ReadSchedulable())
	require.False(t, vol.WriteFreeClaims())

	vol.ClaimRelease(alloc)
	require.True(t, vol.ReadSchedulable())
	require.True(t, vol.WriteFreeClaims())
}
