package volumewatcher

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

// TestVolumeWatch_Batcher tests the update batching logic
func TestVolumeWatch_Batcher(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	ctx, exitFn := context.WithCancel(context.Background())
	defer exitFn()

	srv := &MockBatchingRPCServer{}
	srv.state = state.TestStateStore(t)
	srv.volumeUpdateBatcher = NewVolumeUpdateBatcher(CrossVolumeUpdateBatchDuration, srv, ctx)
	srv.nextCSIControllerDetachError = fmt.Errorf("some controller plugin error")

	plugin := mock.CSIPlugin()
	node := testNode(nil, plugin, srv.State())

	// because we wait for the results to return from the batch for each
	// Watcher.updateClaims, we can't test that we're batching except across
	// multiple volume watchers. create 2 volumes and their watchers here.
	alloc0 := mock.Alloc()
	alloc0.ClientStatus = structs.AllocClientStatusComplete
	vol0 := testVolume(nil, plugin, alloc0, node.ID)
	w0 := &volumeWatcher{
		v:            vol0,
		rpc:          srv,
		state:        srv.State(),
		updateClaims: srv.UpdateClaims,
		logger:       testlog.HCLogger(t),
	}

	alloc1 := mock.Alloc()
	alloc1.ClientStatus = structs.AllocClientStatusComplete
	vol1 := testVolume(nil, plugin, alloc1, node.ID)
	w1 := &volumeWatcher{
		v:            vol1,
		rpc:          srv,
		state:        srv.State(),
		updateClaims: srv.UpdateClaims,
		logger:       testlog.HCLogger(t),
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		w0.volumeReapImpl(vol0)
		wg.Done()
	}()
	go func() {
		// send an artificial updateClaim rather than doing a full
		// reap so that we can guarantee we exercise deduplication
		w1.updateClaims([]structs.CSIVolumeClaimRequest{{
			VolumeID: vol1.ID,
			WriteRequest: structs.WriteRequest{
				Namespace: vol1.Namespace,
			},
		},
			{
				VolumeID: vol1.ID,
				WriteRequest: structs.WriteRequest{
					Namespace: vol1.Namespace,
				},
			}})
		wg.Done()
	}()

	wg.Wait()

	require.Equal(structs.CSIVolumeClaimStateNodeDetached, vol0.PastClaims[alloc0.ID].State)
	require.Equal(1, srv.countCSINodeDetachVolume)
	require.Equal(1, srv.countCSIControllerDetachVolume)
	require.Equal(2, srv.countUpdateClaims)

	// note: it's technically possible that the goroutines under test
	// get de-scheduled and we don't write both updates in the same
	// batch. but this seems really unlikely, so we're testing for
	// both cases here so that if we start seeing a flake here in the
	// future we have a clear cause for it.
	require.GreaterOrEqual(srv.countUpsertVolumeClaims, 1)
	require.Equal(1, srv.countUpsertVolumeClaims)
}
