package volumewatcher

import (
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

	watcher := NewVolumesWatcher(testlog.HCLogger(t), srv, "")
	watcher.SetEnabled(true, srv.State())

	plugin := mock.CSIPlugin()
	node := testNode(plugin, srv.State())
	alloc := mock.Alloc()
	alloc.ClientStatus = structs.AllocClientStatusComplete
	vol := testVolume(plugin, alloc, node.ID)

	index++
	err := srv.State().CSIVolumeRegister(index, []*structs.CSIVolume{vol})
	require.NoError(err)

	claim := &structs.CSIVolumeClaim{
		Mode:  structs.CSIVolumeClaimGC,
		State: structs.CSIVolumeClaimStateNodeDetached,
	}
	index++
	err = srv.State().CSIVolumeClaim(index, vol.Namespace, vol.ID, claim)
	require.NoError(err)
	require.Eventually(func() bool {
		watcher.wlock.RLock()
		defer watcher.wlock.RUnlock()
		return 1 == len(watcher.watchers)
	}, time.Second, 10*time.Millisecond)

	watcher.SetEnabled(false, nil)
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

	watcher := NewVolumesWatcher(testlog.HCLogger(t), srv, "")

	plugin := mock.CSIPlugin()
	node := testNode(plugin, srv.State())
	alloc := mock.Alloc()
	alloc.ClientStatus = structs.AllocClientStatusComplete
	vol := testVolume(plugin, alloc, node.ID)

	watcher.SetEnabled(true, srv.State())

	index++
	err := srv.State().CSIVolumeRegister(index, []*structs.CSIVolume{vol})
	require.NoError(err)

	// we should get or start up a watcher when we get an update for
	// the volume from the state store
	require.Eventually(func() bool {
		watcher.wlock.RLock()
		defer watcher.wlock.RUnlock()
		return 1 == len(watcher.watchers)
	}, time.Second, 10*time.Millisecond)

	// step-down (this is sync, but step-up is async)
	watcher.SetEnabled(false, nil)
	require.Equal(0, len(watcher.watchers))

	// step-up again
	watcher.SetEnabled(true, srv.State())
	require.Eventually(func() bool {
		watcher.wlock.RLock()
		defer watcher.wlock.RUnlock()
		return 1 == len(watcher.watchers) &&
			!watcher.watchers[vol.ID+vol.Namespace].isRunning()
	}, time.Second, 10*time.Millisecond)
}

// TestVolumeWatch_StartStop tests the start and stop of the watcher when
// it receives notifcations and has completed its work
func TestVolumeWatch_StartStop(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	srv := &MockStatefulRPCServer{}
	srv.state = state.TestStateStore(t)
	index := uint64(100)
	watcher := NewVolumesWatcher(testlog.HCLogger(t), srv, "")

	watcher.SetEnabled(true, srv.State())
	require.Equal(0, len(watcher.watchers))

	plugin := mock.CSIPlugin()
	node := testNode(plugin, srv.State())
	alloc1 := mock.Alloc()
	alloc1.ClientStatus = structs.AllocClientStatusRunning
	alloc2 := mock.Alloc()
	alloc2.Job = alloc1.Job
	alloc2.ClientStatus = structs.AllocClientStatusRunning
	index++
	err := srv.State().UpsertJob(structs.MsgTypeTestSetup, index, alloc1.Job)
	require.NoError(err)
	index++
	err = srv.State().UpsertAllocs(structs.MsgTypeTestSetup, index, []*structs.Allocation{alloc1, alloc2})
	require.NoError(err)

	// register a volume
	vol := testVolume(plugin, alloc1, node.ID)
	index++
	err = srv.State().CSIVolumeRegister(index, []*structs.CSIVolume{vol})
	require.NoError(err)

	// assert we get a watcher; there are no claims so it should immediately stop
	require.Eventually(func() bool {
		watcher.wlock.RLock()
		defer watcher.wlock.RUnlock()
		return 1 == len(watcher.watchers) &&
			!watcher.watchers[vol.ID+vol.Namespace].isRunning()
	}, time.Second*2, 10*time.Millisecond)

	// claim the volume for both allocs
	claim := &structs.CSIVolumeClaim{
		AllocationID: alloc1.ID,
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
		AllocationID: alloc1.ID,
		NodeID:       node.ID,
	}
	index++
	err = srv.State().CSIVolumeClaim(index, vol.Namespace, vol.ID, claim)
	require.NoError(err)

	ws := memdb.NewWatchSet()
	vol, _ = srv.State().CSIVolumeByID(ws, vol.Namespace, vol.ID)
	require.Equal(2, len(vol.ReadAllocs))

	// alloc becomes terminal
	alloc1.ClientStatus = structs.AllocClientStatusComplete
	index++
	err = srv.State().UpsertAllocs(structs.MsgTypeTestSetup, index, []*structs.Allocation{alloc1})
	require.NoError(err)
	index++
	claim.State = structs.CSIVolumeClaimStateReadyToFree
	err = srv.State().CSIVolumeClaim(index, vol.Namespace, vol.ID, claim)
	require.NoError(err)

	// 1 claim has been released and watcher stops
	require.Eventually(func() bool {
		ws := memdb.NewWatchSet()
		vol, _ := srv.State().CSIVolumeByID(ws, vol.Namespace, vol.ID)
		return len(vol.ReadAllocs) == 1 && len(vol.PastClaims) == 0
	}, time.Second*2, 10*time.Millisecond)

	require.Eventually(func() bool {
		watcher.wlock.RLock()
		defer watcher.wlock.RUnlock()
		return !watcher.watchers[vol.ID+vol.Namespace].isRunning()
	}, time.Second*5, 10*time.Millisecond)
}

// TestVolumeWatch_RegisterDeregister tests the start and stop of
// watchers around registration
func TestVolumeWatch_RegisterDeregister(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	srv := &MockStatefulRPCServer{}
	srv.state = state.TestStateStore(t)

	index := uint64(100)

	watcher := NewVolumesWatcher(testlog.HCLogger(t), srv, "")

	watcher.SetEnabled(true, srv.State())
	require.Equal(0, len(watcher.watchers))

	plugin := mock.CSIPlugin()
	alloc := mock.Alloc()
	alloc.ClientStatus = structs.AllocClientStatusComplete

	// register a volume without claims
	vol := mock.CSIVolume(plugin)
	index++
	err := srv.State().CSIVolumeRegister(index, []*structs.CSIVolume{vol})
	require.NoError(err)

	// watcher should be started but immediately stopped
	require.Eventually(func() bool {
		watcher.wlock.RLock()
		defer watcher.wlock.RUnlock()
		return 1 == len(watcher.watchers)
	}, time.Second, 10*time.Millisecond)

	require.False(watcher.watchers[vol.ID+vol.Namespace].isRunning())
}
