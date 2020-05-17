package nomad

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCSI_Batcher(t *testing.T) {
	srv, shutdown := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()

	batcher := newCSIBatchRelease(srv, nil, 5)

	batcher.add("vol0", "global")
	batcher.add("vol", "0global")
	batcher.add("vol1", "global")
	batcher.add("vol1", "global")
	batcher.add("vol2", "global")
	batcher.add("vol2", "other")
	batcher.add("vol3", "global")
	batcher.add("vol4", "global")
	batcher.add("vol5", "global")
	batcher.add("vol6", "global")

	require.Len(t, batcher.batches, 2)
	require.Len(t, batcher.batches[0].Claims, 5, "first batch")
	require.Equal(t, batcher.batches[0].Claims[4].VolumeID, "vol2")
	require.Equal(t, batcher.batches[0].Claims[4].Namespace, "other")
	require.Len(t, batcher.batches[1].Claims, 4, "second batch")
}
