package volumewatcher

import (
	"context"
	"testing"
	"time"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

// TestVolumeWatch_EnableDisable tests the watcher registration logic that needs
// to happen during leader step-up/step-down
func TestVolumeWatch_EnableDisable(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	srv := &MockRPCServer{}
	srv.state = state.TestStateStore(t)
	index := uint64(100)

	watcher := NewVolumesWatcher(testlog.HCLogger(t),
		srv, srv,
		LimitStateQueriesPerSecond,
		CrossVolumeUpdateBatchDuration)

	watcher.SetEnabled(true, srv.State())

	plugin := mock.CSIPlugin()
	node := testNode(nil, plugin, srv.State())
	alloc := mock.Alloc()
	alloc.ClientStatus = structs.AllocClientStatusComplete
	vol := testVolume(nil, plugin, alloc, node.ID)

	index++
	err := srv.State().CSIVolumeRegister(index, []*structs.CSIVolume{vol})
	require.NoError(err)

	claim := &structs.CSIVolumeClaim{Mode: structs.CSIVolumeClaimRelease}
	index++
	err = srv.State().CSIVolumeClaim(index, vol.Namespace, vol.ID, claim)
	require.NoError(err)
	require.Eventually(func() bool {
		return 1 == len(watcher.watchers)
	}, time.Second, 10*time.Millisecond)

	watcher.SetEnabled(false, srv.State())
	require.Equal(0, len(watcher.watchers))
}

// TestVolumeWatch_Checkpoint tests the checkpointing of progress across
// leader leader step-up/step-down
func TestVolumeWatch_Checkpoint(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	srv := &MockRPCServer{}
	srv.state = state.TestStateStore(t)
	index := uint64(100)

	watcher := NewVolumesWatcher(testlog.HCLogger(t),
		srv, srv,
		LimitStateQueriesPerSecond,
		CrossVolumeUpdateBatchDuration)

	plugin := mock.CSIPlugin()
	node := testNode(nil, plugin, srv.State())
	alloc := mock.Alloc()
	alloc.ClientStatus = structs.AllocClientStatusComplete
	vol := testVolume(nil, plugin, alloc, node.ID)

	watcher.SetEnabled(true, srv.State())

	index++
	err := srv.State().CSIVolumeRegister(index, []*structs.CSIVolume{vol})
	require.NoError(err)

	// we should get or start up a watcher when we get an update for
	// the volume from the state store
	require.Eventually(func() bool {
		return 1 == len(watcher.watchers)
	}, time.Second, 10*time.Millisecond)

	// step-down (this is sync, but step-up is async)
	watcher.SetEnabled(false, srv.State())
	require.Equal(0, len(watcher.watchers))

	// step-up again
	watcher.SetEnabled(true, srv.State())
	require.Eventually(func() bool {
		return 1 == len(watcher.watchers)
	}, time.Second, 10*time.Millisecond)

	require.True(watcher.watchers[vol.ID+vol.Namespace].isRunning())
}

// TestVolumeWatch_StartStop tests the start and stop of the watcher when
// it receives notifcations and has completed its work
func TestVolumeWatch_StartStop(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	ctx, exitFn := context.WithCancel(context.Background())
	defer exitFn()

	srv := &MockStatefulRPCServer{}
	srv.state = state.TestStateStore(t)
	index := uint64(100)
	srv.volumeUpdateBatcher = NewVolumeUpdateBatcher(
		CrossVolumeUpdateBatchDuration, srv, ctx)

	watcher := NewVolumesWatcher(testlog.HCLogger(t),
		srv, srv,
		LimitStateQueriesPerSecond,
		CrossVolumeUpdateBatchDuration)

	watcher.SetEnabled(true, srv.State())
	require.Equal(0, len(watcher.watchers))

	plugin := mock.CSIPlugin()
	node := testNode(nil, plugin, srv.State())
	alloc := mock.Alloc()
	alloc.ClientStatus = structs.AllocClientStatusRunning
	alloc2 := mock.Alloc()
	alloc2.Job = alloc.Job
	alloc2.ClientStatus = structs.AllocClientStatusRunning
	index++
	err := srv.State().UpsertJob(index, alloc.Job)
	require.NoError(err)
	index++
	err = srv.State().UpsertAllocs(index, []*structs.Allocation{alloc, alloc2})
	require.NoError(err)

	// register a volume
	vol := testVolume(nil, plugin, alloc, node.ID)
	index++
	err = srv.State().CSIVolumeRegister(index, []*structs.CSIVolume{vol})
	require.NoError(err)

	// assert we get a running watcher
	require.Eventually(func() bool {
		return 1 == len(watcher.watchers)
	}, time.Second, 10*time.Millisecond)
	require.True(watcher.watchers[vol.ID+vol.Namespace].isRunning())

	// claim the volume for both allocs
	claim := &structs.CSIVolumeClaim{
		AllocationID: alloc.ID,
		NodeID:       node.ID,
		Mode:         structs.CSIVolumeClaimRead,
	}
	index++
	err = srv.State().CSIVolumeClaim(index, vol.Namespace, vol.ID, claim)
	require.NoError(err)
	claim.AllocationID = alloc2.ID
	index++
	err = srv.State().CSIVolumeClaim(index, vol.Namespace, vol.ID, claim)
	require.NoError(err)

	// reap the volume and assert nothing has happened
	claim = &structs.CSIVolumeClaim{
		AllocationID: alloc.ID,
		NodeID:       node.ID,
		Mode:         structs.CSIVolumeClaimRelease,
	}
	index++
	err = srv.State().CSIVolumeClaim(index, vol.Namespace, vol.ID, claim)
	require.NoError(err)
	require.True(watcher.watchers[vol.ID+vol.Namespace].isRunning())

	// alloc becomes terminal
	alloc.ClientStatus = structs.AllocClientStatusComplete
	index++
	err = srv.State().UpsertAllocs(index, []*structs.Allocation{alloc})
	require.NoError(err)
	index++
	claim.State = structs.CSIVolumeClaimStateReadyToFree
	err = srv.State().CSIVolumeClaim(index, vol.Namespace, vol.ID, claim)
	require.NoError(err)

	// 1 claim has been released but watcher is still running
	require.Eventually(func() bool {
		ws := memdb.NewWatchSet()
		vol, _ := srv.State().CSIVolumeByID(ws, vol.Namespace, vol.ID)
		return len(vol.ReadAllocs) == 1 && len(vol.PastClaims) == 0
	}, time.Second*2, 10*time.Millisecond)

	require.True(watcher.watchers[vol.ID+vol.Namespace].isRunning())

	// the watcher will have incremented the index so we need to make sure
	// our inserts will trigger new events
	index, _ = srv.State().LatestIndex()

	// remaining alloc's job is stopped (alloc is not marked terminal)
	alloc2.Job.Stop = true
	index++
	err = srv.State().UpsertJob(index, alloc2.Job)
	require.NoError(err)

	// job deregistration write a claim with no allocations or nodes
	claim = &structs.CSIVolumeClaim{
		Mode: structs.CSIVolumeClaimRelease,
	}
	index++
	err = srv.State().CSIVolumeClaim(index, vol.Namespace, vol.ID, claim)
	require.NoError(err)

	// all claims have been released and watcher is stopped
	require.Eventually(func() bool {
		ws := memdb.NewWatchSet()
		vol, _ := srv.State().CSIVolumeByID(ws, vol.Namespace, vol.ID)
		return len(vol.ReadAllocs) == 1 && len(vol.PastClaims) == 0
	}, time.Second*2, 10*time.Millisecond)

	require.Eventually(func() bool {
		return !watcher.watchers[vol.ID+vol.Namespace].isRunning()
	}, time.Second*1, 10*time.Millisecond)

	// the watcher will have incremented the index so we need to make sure
	// our inserts will trigger new events
	index, _ = srv.State().LatestIndex()

	// create a new claim
	alloc3 := mock.Alloc()
	alloc3.ClientStatus = structs.AllocClientStatusRunning
	index++
	err = srv.State().UpsertAllocs(index, []*structs.Allocation{alloc3})
	require.NoError(err)
	claim3 := &structs.CSIVolumeClaim{
		AllocationID: alloc3.ID,
		NodeID:       node.ID,
		Mode:         structs.CSIVolumeClaimRelease,
	}
	index++
	err = srv.State().CSIVolumeClaim(index, vol.Namespace, vol.ID, claim3)
	require.NoError(err)

	// a stopped watcher should restore itself on notification
	require.Eventually(func() bool {
		return watcher.watchers[vol.ID+vol.Namespace].isRunning()
	}, time.Second*1, 10*time.Millisecond)
}

// TestVolumeWatch_RegisterDeregister tests the start and stop of
// watchers around registration
func TestVolumeWatch_RegisterDeregister(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	ctx, exitFn := context.WithCancel(context.Background())
	defer exitFn()

	srv := &MockStatefulRPCServer{}
	srv.state = state.TestStateStore(t)
	srv.volumeUpdateBatcher = NewVolumeUpdateBatcher(
		CrossVolumeUpdateBatchDuration, srv, ctx)

	index := uint64(100)

	watcher := NewVolumesWatcher(testlog.HCLogger(t),
		srv, srv,
		LimitStateQueriesPerSecond,
		CrossVolumeUpdateBatchDuration)

	watcher.SetEnabled(true, srv.State())
	require.Equal(0, len(watcher.watchers))

	plugin := mock.CSIPlugin()
	node := testNode(nil, plugin, srv.State())
	alloc := mock.Alloc()
	alloc.ClientStatus = structs.AllocClientStatusComplete

	// register a volume
	vol := testVolume(nil, plugin, alloc, node.ID)
	index++
	err := srv.State().CSIVolumeRegister(index, []*structs.CSIVolume{vol})
	require.NoError(err)

	require.Eventually(func() bool {
		return 1 == len(watcher.watchers)
	}, time.Second, 10*time.Millisecond)

	// reap the volume and assert we've cleaned up
	w := watcher.watchers[vol.ID+vol.Namespace]
	w.Notify(vol)

	require.Eventually(func() bool {
		ws := memdb.NewWatchSet()
		vol, _ := srv.State().CSIVolumeByID(ws, vol.Namespace, vol.ID)
		return len(vol.ReadAllocs) == 0 && len(vol.PastClaims) == 0
	}, time.Second*2, 10*time.Millisecond)

	require.Eventually(func() bool {
		return !watcher.watchers[vol.ID+vol.Namespace].isRunning()
	}, time.Second*1, 10*time.Millisecond)

	require.Equal(1, srv.countCSINodeDetachVolume, "node detach RPC count")
	require.Equal(1, srv.countCSIControllerDetachVolume, "controller detach RPC count")
	require.Equal(2, srv.countUpsertVolumeClaims, "upsert claims count")

	// deregistering the volume doesn't cause an update that triggers
	// a watcher; we'll clean up this watcher in a GC later
	err = srv.State().CSIVolumeDeregister(index, vol.Namespace, []string{vol.ID})
	require.NoError(err)
	require.Equal(1, len(watcher.watchers))
	require.False(watcher.watchers[vol.ID+vol.Namespace].isRunning())
}
