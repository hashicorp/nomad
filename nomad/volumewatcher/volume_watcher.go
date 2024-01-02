// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package volumewatcher

import (
	"context"
	"sync"
	"time"

	log "github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// volumeWatcher is used to watch a single volume and trigger the
// scheduler when allocation health transitions.
type volumeWatcher struct {
	// v is the volume being watched
	v *structs.CSIVolume

	// state is the state that is watched for state changes.
	state *state.StateStore

	// server interface for CSI client RPCs
	rpc CSIVolumeRPC

	// the ACL needed to send RPCs
	leaderAcl string

	logger      log.Logger
	shutdownCtx context.Context // parent context
	ctx         context.Context // own context
	exitFn      context.CancelFunc
	deleteFn    func()

	// quiescentTimeout is the time we wait until the volume has "settled"
	// before stopping the child watcher goroutines
	quiescentTimeout time.Duration

	// updateCh is triggered when there is an updated volume
	updateCh chan *structs.CSIVolume

	wLock   sync.RWMutex
	running bool
}

// newVolumeWatcher returns a volume watcher that is used to watch
// volumes
func newVolumeWatcher(parent *Watcher, vol *structs.CSIVolume) *volumeWatcher {

	w := &volumeWatcher{
		updateCh:         make(chan *structs.CSIVolume, 1),
		v:                vol,
		state:            parent.state,
		rpc:              parent.rpc,
		leaderAcl:        parent.leaderAcl,
		logger:           parent.logger.With("volume_id", vol.ID, "namespace", vol.Namespace),
		shutdownCtx:      parent.ctx,
		deleteFn:         func() { parent.remove(vol.ID + vol.Namespace) },
		quiescentTimeout: parent.quiescentTimeout,
	}

	// Start the long lived watcher that scans for allocation updates
	w.Start()
	return w
}

// Notify signals an update to the tracked volume.
func (vw *volumeWatcher) Notify(v *structs.CSIVolume) {
	if !vw.isRunning() {
		vw.Start()
	}
	select {
	case vw.updateCh <- v:
	case <-vw.shutdownCtx.Done(): // prevent deadlock if we stopped
	}
}

func (vw *volumeWatcher) Start() {
	vw.logger.Trace("starting watcher")
	vw.wLock.Lock()
	defer vw.wLock.Unlock()
	vw.running = true
	go vw.watch()
}

func (vw *volumeWatcher) Stop() {
	vw.logger.Trace("no more claims")
	vw.wLock.Lock()
	defer vw.wLock.Unlock()
	vw.running = false
}

func (vw *volumeWatcher) isRunning() bool {
	vw.wLock.RLock()
	defer vw.wLock.RUnlock()
	select {
	case <-vw.shutdownCtx.Done():
		return false
	default:
		return vw.running
	}
}

// watch is the long-running function that watches for changes to a volume.
// Each pass steps the volume's claims through the various states of reaping
// until the volume has no more claims eligible to be reaped.
func (vw *volumeWatcher) watch() {
	defer vw.deleteFn()
	defer vw.Stop()

	timer, stop := helper.NewSafeTimer(vw.quiescentTimeout)
	defer stop()

	for {
		select {
		// TODO(tgross): currently server->client RPC have no cancellation
		// context, so we can't stop the long-runner RPCs gracefully
		case <-vw.shutdownCtx.Done():
			return
		case vol := <-vw.updateCh:
			vol = vw.getVolume(vol)
			if vol == nil {
				return
			}
			vw.volumeReap(vol)
			timer.Reset(vw.quiescentTimeout)
		case <-timer.C:
			// Wait until the volume has "settled" before stopping this
			// goroutine so that we can handle the burst of updates around
			// freeing claims without having to spin it back up
			return
		}
	}
}

// getVolume returns the tracked volume, fully populated with the current
// state
func (vw *volumeWatcher) getVolume(vol *structs.CSIVolume) *structs.CSIVolume {
	vw.wLock.RLock()
	defer vw.wLock.RUnlock()

	var err error
	ws := memdb.NewWatchSet()

	vol, err = vw.state.CSIVolumeByID(ws, vol.Namespace, vol.ID)
	if err != nil {
		vw.logger.Error("could not query for volume", "error", err)
		return nil
	}
	if vol == nil {
		return nil
	}

	vol, err = vw.state.CSIVolumeDenormalize(ws, vol)
	if err != nil {
		vw.logger.Error("could not query allocs for volume", "error", err)
		return nil
	}
	vw.v = vol
	return vol
}

// volumeReap collects errors for logging but doesn't return them
// to the main loop.
func (vw *volumeWatcher) volumeReap(vol *structs.CSIVolume) {
	vw.logger.Trace("releasing unused volume claims")
	err := vw.volumeReapImpl(vol)
	if err != nil {
		vw.logger.Error("error releasing volume claims", "error", err)
	}
}

func (vw *volumeWatcher) isUnclaimed(vol *structs.CSIVolume) bool {
	return len(vol.ReadClaims) == 0 && len(vol.WriteClaims) == 0 && len(vol.PastClaims) == 0
}

// volumeReapImpl unpublished all the volume's PastClaims. PastClaims
// will be populated from nil or terminal allocs when we call
// CSIVolumeDenormalize(), so this assumes we've done so in the caller
func (vw *volumeWatcher) volumeReapImpl(vol *structs.CSIVolume) error {
	var result *multierror.Error
	for _, claim := range vol.PastClaims {
		err := vw.unpublish(vol, claim)
		if err != nil {
			result = multierror.Append(result, err)
		}
	}
	return result.ErrorOrNil()
}

func (vw *volumeWatcher) collectPastClaims(vol *structs.CSIVolume) *structs.CSIVolume {

	collect := func(allocs map[string]*structs.Allocation,
		claims map[string]*structs.CSIVolumeClaim) {

		for allocID, alloc := range allocs {
			if alloc == nil {
				_, exists := vol.PastClaims[allocID]
				if !exists {
					vol.PastClaims[allocID] = &structs.CSIVolumeClaim{
						AllocationID: allocID,
						State:        structs.CSIVolumeClaimStateReadyToFree,
					}
				}
			} else if alloc.Terminated() {
				// don't overwrite the PastClaim if we've seen it before,
				// so that we can track state between subsequent calls
				_, exists := vol.PastClaims[allocID]
				if !exists {
					claim, ok := claims[allocID]
					if !ok {
						claim = &structs.CSIVolumeClaim{
							AllocationID: allocID,
							NodeID:       alloc.NodeID,
						}
					}
					claim.State = structs.CSIVolumeClaimStateTaken
					vol.PastClaims[allocID] = claim
				}
			}
		}
	}

	collect(vol.ReadAllocs, vol.ReadClaims)
	collect(vol.WriteAllocs, vol.WriteClaims)
	return vol
}

func (vw *volumeWatcher) unpublish(vol *structs.CSIVolume, claim *structs.CSIVolumeClaim) error {
	vw.logger.Trace("unpublishing volume", "alloc", claim.AllocationID)
	req := &structs.CSIVolumeUnpublishRequest{
		VolumeID: vol.ID,
		Claim:    claim,
		WriteRequest: structs.WriteRequest{
			Namespace: vol.Namespace,
			Region:    vw.state.Config().Region,
			AuthToken: vw.leaderAcl,
		},
	}
	err := vw.rpc.Unpublish(req, &structs.CSIVolumeUnpublishResponse{})
	if err != nil {
		return err
	}
	claim.State = structs.CSIVolumeClaimStateReadyToFree
	return nil
}
