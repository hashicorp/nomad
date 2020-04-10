package volumewatcher

import (
	"context"
	"fmt"
	"sync"
	"time"

	log "github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"golang.org/x/time/rate"
)

const (
	// LimitStateQueriesPerSecond is the number of state queries allowed per
	// second
	LimitStateQueriesPerSecond = 100.0

	// CrossVolumeUpdateBatchDuration is the duration in which volume
	// claim updates are batched across all volume watchers before
	// being committed to Raft.
	CrossVolumeUpdateBatchDuration = 250 * time.Millisecond
)

var (
	// notEnabled is the error returned when the volume watcher is not
	// enabled
	notEnabled = fmt.Errorf("volume watcher not enabled")
)

// Watcher is used to watch volumes and their allocations created
// by the scheduler and trigger the scheduler when allocation health
// transitions.
type Watcher struct {
	enabled bool
	logger  log.Logger

	// queryLimiter is used to limit the rate of blocking queries
	queryLimiter *rate.Limiter

	// updateBatchDuration is the duration in which volume
	// claim updates are batched across all volume watchers
	// before being committed to Raft.
	updateBatchDuration time.Duration

	// raft contains the set of Raft endpoints that can be used by the
	// volumes watcher
	raft VolumeRaftEndpoints

	// rpc contains the set of Server methods that can be used by
	// the volumes watcher for RPC
	rpc ClientRPC

	// state is the state that is watched for state changes.
	state *state.StateStore

	// watchers is the set of active watchers, one per volume
	watchers map[string]*volumeWatcher

	// volumeUpdateBatcher is used to batch volume claim updates
	volumeUpdateBatcher *VolumeUpdateBatcher

	// ctx and exitFn are used to cancel the watcher
	ctx    context.Context
	exitFn context.CancelFunc

	wlock sync.RWMutex
}

// NewVolumesWatcher returns a volumes watcher that is used to watch
// volumes and trigger the scheduler as needed.
func NewVolumesWatcher(logger log.Logger,
	raft VolumeRaftEndpoints, rpc ClientRPC, stateQueriesPerSecond float64,
	updateBatchDuration time.Duration) *Watcher {

	// the leader step-down calls SetEnabled(false) which is what
	// cancels this context, rather than passing in its own shutdown
	// context
	ctx, exitFn := context.WithCancel(context.Background())

	return &Watcher{
		raft:                raft,
		rpc:                 rpc,
		queryLimiter:        rate.NewLimiter(rate.Limit(stateQueriesPerSecond), 100),
		updateBatchDuration: updateBatchDuration,
		logger:              logger.Named("volumes_watcher"),
		ctx:                 ctx,
		exitFn:              exitFn,
	}
}

// ReapVolume starts a volume watcher for a volume
func (w *Watcher) Reap(req *structs.CSIVolumeClaimRequest) (uint64, error) {
	index := uint64(0) // TODO(tgross): not sure we can do anything useful with this
	watcher, err := w.getOrCreateWatcher(req.VolumeID, req.RequestNamespace())
	if err != nil {
		return 0, err
	}
	go watcher.Notify(watcher.v)
	return index, nil
}

// SetEnabled is used to control if the watcher is enabled. The
// watcher should only be enabled on the active leader. When being
// enabled the state is passed in as it is no longer valid once a
// leader election has taken place.
func (w *Watcher) SetEnabled(enabled bool, state *state.StateStore) {
	w.wlock.Lock()
	defer w.wlock.Unlock()

	wasEnabled := w.enabled
	w.enabled = enabled

	if state != nil {
		w.state = state
	}

	// Flush the state to create the necessary objects
	w.flush()

	// If we are starting now, launch the watch daemon
	if enabled && !wasEnabled {
		go w.watchVolumes(w.ctx)
	}
}

// flush is used to clear the state of the watcher
func (w *Watcher) flush() {
	// Stop all the watchers and clear it
	for _, watcher := range w.watchers {
		watcher.Stop()
	}

	// Kill everything associated with the watcher
	if w.exitFn != nil {
		w.exitFn()
	}

	w.watchers = make(map[string]*volumeWatcher, 32)
	w.ctx, w.exitFn = context.WithCancel(context.Background())
	w.volumeUpdateBatcher = NewVolumeUpdateBatcher(w.updateBatchDuration, w.raft, w.ctx)
}

// watchVolumes is the long lived go-routine that watches for volumes to
// add and remove watchers on.
func (w *Watcher) watchVolumes(ctx context.Context) {
	vIndex := uint64(1)
	for {
		// Block getting all volumes using the last volume index.
		volumes, idx, err := w.getVolumes(ctx, vIndex)
		if err != nil {
			if err == context.Canceled {
				return
			}

			w.logger.Error("failed to retrieve volumes", "error", err)
		}

		// Update the latest index
		vIndex = idx

		for _, v := range volumes {
			if err := w.add(v); err != nil {
				w.logger.Error("failed to track volume", "volume_id", v.ID, "error", err)
			}
		}
	}
}

// getVolumes retrieves all volumes blocking at the given index.
func (w *Watcher) getVolumes(ctx context.Context, minIndex uint64) ([]*structs.CSIVolume, uint64, error) {
	resp, index, err := w.state.BlockingQuery(w.getVolumesImpl, minIndex, ctx)
	if err != nil {
		return nil, 0, err
	}

	return resp.([]*structs.CSIVolume), index, nil
}

// getVolumesImpl retrieves all volumes from the passed state store.
func (w *Watcher) getVolumesImpl(ws memdb.WatchSet, state *state.StateStore) (interface{}, uint64, error) {

	iter, err := state.CSIVolumes(ws)
	if err != nil {
		return nil, 0, err
	}

	var volumes []*structs.CSIVolume
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		volume := raw.(*structs.CSIVolume)
		volumes = append(volumes, volume)
	}

	// Use the last index that affected the volume table
	index, err := state.Index("csi_volumes")
	if err != nil {
		return nil, 0, err
	}

	return volumes, index, nil
}

// add adds a volume to the watch list
func (w *Watcher) add(d *structs.CSIVolume) error {
	w.wlock.Lock()
	defer w.wlock.Unlock()
	_, err := w.addLocked(d)
	return err
}

// addLocked adds a volume to the watch list and should only be called when
// locked. Creating the volumeWatcher starts a go routine to .watch() it
func (w *Watcher) addLocked(v *structs.CSIVolume) (*volumeWatcher, error) {
	// Not enabled so no-op
	if !w.enabled {
		return nil, nil
	}

	// Already watched so trigger an update for the volume
	if watcher, ok := w.watchers[v.ID+v.Namespace]; ok {
		watcher.Notify(v)
		return nil, nil
	}

	watcher := newVolumeWatcher(w, v)
	w.watchers[v.ID+v.Namespace] = watcher
	return watcher, nil
}

// remove stops watching a volume. This can be because the volume is
// complete or being deleted.
func (w *Watcher) remove(v *structs.CSIVolume) {
	w.wlock.Lock()
	defer w.wlock.Unlock()

	// Not enabled so no-op
	if !w.enabled {
		return
	}

	if watcher, ok := w.watchers[v.ID+v.Namespace]; ok {
		watcher.Stop()
		delete(w.watchers, v.ID+v.Namespace)
	}
}

// forceAdd is used to force a lookup of the given volume object and create
// a watcher. If the volume does not exist or is terminal an error is
// returned.
func (w *Watcher) forceAdd(volID, namespace string) (*volumeWatcher, error) {
	snap, err := w.state.Snapshot()
	if err != nil {
		return nil, err
	}

	ws := memdb.NewWatchSet()
	vol, err := snap.CSIVolumeByID(ws, namespace, volID)
	if err != nil {
		return nil, err
	}
	if vol == nil {
		return nil, fmt.Errorf("unknown volume %q in %q", volID, namespace)
	}

	return w.addLocked(vol)
}

// getOrCreateWatcher returns the volume watcher for the given volume ID.
func (w *Watcher) getOrCreateWatcher(volID, namespace string) (*volumeWatcher, error) {
	w.wlock.Lock()
	defer w.wlock.Unlock()

	// Not enabled so no-op
	if !w.enabled {
		return nil, notEnabled
	}

	watcher, ok := w.watchers[volID+namespace]
	if ok {
		return watcher, nil
	}

	return w.forceAdd(volID, namespace)
}

// updatesClaims sends the claims to the batch updater and waits for
// the results
func (w *Watcher) updateClaims(claims []structs.CSIVolumeClaimRequest) (uint64, error) {
	return w.volumeUpdateBatcher.CreateUpdate(claims).Results()
}
