package structs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCSIVolumeClaim(t *testing.T) {
	vol := NewCSIVolume("", 0)
	vol.AccessMode = CSIVolumeAccessModeMultiNodeSingleWriter
	vol.Schedulable = true

	alloc := &Allocation{ID: "al"}

	vol.ClaimRead(alloc)
	require.True(t, vol.CanReadOnly())
	require.True(t, vol.CanWrite())
	require.True(t, vol.ClaimRead(alloc))

	vol.ClaimWrite(alloc)
	require.True(t, vol.CanReadOnly())
	require.False(t, vol.CanWrite())
	require.False(t, vol.ClaimWrite(alloc))

	vol.ClaimRelease(alloc)
	require.True(t, vol.CanReadOnly())
	require.True(t, vol.CanWrite())
}
