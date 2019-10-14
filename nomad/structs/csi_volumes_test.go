package structs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCSIVolumeClaim(t *testing.T) {
	vol := CreateCSIVolume(nil)
	vol.MaxReaders = 1
	vol.MaxWriters = 1

	alloc := &Allocation{ID: "al"}

	vol.ClaimRead(alloc)
	require.False(t, vol.CanReadOnly())
	require.True(t, vol.CanWrite())
	require.False(t, vol.ClaimRead(alloc))

	vol.ClaimWrite(alloc)
	require.True(t, vol.CanReadOnly())
	require.False(t, vol.CanWrite())
	require.False(t, vol.ClaimWrite(alloc))

	vol.ClaimRelease(alloc)
	require.True(t, vol.CanReadOnly())
	require.True(t, vol.CanWrite())
}
