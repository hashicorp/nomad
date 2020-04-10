package volumewatcher

import (
	"context"
	"fmt"
	"sync"

	log "github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"
	multierror "github.com/hashicorp/go-multierror"
	cstructs "github.com/hashicorp/nomad/client/structs"
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

	// updateClaims is the function used to apply claims to raft
	updateClaims updateClaimsFn

	// server interface for CSI client RPCs
	rpc ClientRPC

	logger log.Logger
	ctx    context.Context
	exitFn context.CancelFunc

	// updateCh is triggered when there is an updated volume
	updateCh chan struct{}

	wLock sync.RWMutex
}

// newVolumeWatcher returns a volume watcher that is used to watch
// volumes
func newVolumeWatcher(parent *Watcher, vol *structs.CSIVolume) *volumeWatcher {
	ctx, exitFn := context.WithCancel(parent.ctx)
	w := &volumeWatcher{
		updateCh:     make(chan struct{}, 1),
		updateClaims: parent.updateClaims,
		v:            vol,
		state:        parent.state,
		rpc:          parent.rpc,
		logger:       parent.logger.With("volume_id", vol.ID, "namespace", vol.Namespace),
		ctx:          ctx,
		exitFn:       exitFn,
	}

	// Start the long lived watcher that scans for allocation updates
	go w.watch()

	return w
}

// updateVolume is used to update the tracked volume.
func (w *volumeWatcher) Notify(v *structs.CSIVolume) {
	w.wLock.Lock()
	defer w.wLock.Unlock()

	// Update and trigger
	w.v = v
	select {
	case w.updateCh <- struct{}{}:
	default:
	}
}

// StopWatch stops watching the volume. This should be called whenever a
// volume's claims are fully reaped or the watcher is no longer needed.
func (vw *volumeWatcher) Stop() {
	vw.exitFn()
}

// getVolume returns the tracked volume, fully populated with the current
// state
func (vw *volumeWatcher) getVolume() *structs.CSIVolume {
	vw.wLock.RLock()
	defer vw.wLock.RUnlock()

	// TODO(tgross): do we need to re-snap on each pass?
	ws := memdb.NewWatchSet()
	vol, err := vw.state.CSIVolumeDenormalizePlugins(ws, vw.v.Copy())
	if err != nil {
		vw.logger.Error("could not query plugins for volume", "error", err)
		return nil
	}

	vol, err = vw.state.CSIVolumeDenormalize(ws, vol)
	if err != nil {
		vw.logger.Error("could not query allocs for volume", "error", err)
		return nil
	}
	return vol
}

// watch is the long-running function that watches for changes to a volume.
// Each pass steps the volume's claims through the various states of reaping
// until the volume has no more claims eligible to be reaped.
func (vw *volumeWatcher) watch() {
	for {
		select {
		case <-vw.ctx.Done():
			// TODO(tgross): currently server->client RPC have no cancellation
			// context, so we can't stop the long-runner RPCs gracefully
			return
		case <-vw.updateCh:
			vol := vw.getVolume()
			if vol == nil {
				return
			}
			vw.volumeReap(vw.v)
		}
	}
}

// volumeReap collects errors for logging but doesn't return them
// to the main loop.
// TODO(tgross): should we set a maximum number of errors before
// killing the main watch loop?
func (vw *volumeWatcher) volumeReap(vol *structs.CSIVolume) {
	err := vw.volumeReapImpl(vol)
	if err != nil {
		vw.logger.Error("error releasing volume claims", "error", err)
	}
}

func (vw *volumeWatcher) volumeReapImpl(vol *structs.CSIVolume) error {
	var result *multierror.Error
	nodeClaims := map[string]int{} // node IDs -> count

	collect := func(allocs map[string]*structs.Allocation, claims map[string]*structs.CSIVolumeClaim) {
		for allocID, alloc := range allocs {
			claim, ok := claims[allocID]
			if !ok {
				err := fmt.Errorf(
					"alloc read claims corrupt: %s missing from read claims", allocID)
				result = multierror.Append(result, err)
				continue // TODO: we should never see this, but what can we do with it?
			}
			nodeClaims[claim.NodeID]++
			if alloc == nil || alloc.Terminated() {
				// only overwrite the PastClaim if this is new,
				// so that we can track state between subsequent
				// calls of VolumeReap
				if _, exists := vol.PastClaims[claim.AllocationID]; !exists {
					claim.State = structs.CSIVolumeClaimStateTaken
					vol.PastClaims[claim.AllocationID] = claim
				}
			}
		}
	}

	collect(vol.ReadAllocs, vol.ReadClaims)
	collect(vol.WriteAllocs, vol.WriteClaims)

	if len(vol.PastClaims) == 0 {
		vw.Stop()
		return nil
	}

	for _, claim := range vol.PastClaims {

	NEXT_STEP:
		switch claim.State {
		case structs.CSIVolumeClaimStateTaken:
			err := vw.nodeDetach(vol, claim)
			if err != nil {
				result = multierror.Append(result, err)
				break
			}
			nodeClaims[claim.NodeID]--
			goto NEXT_STEP
		case structs.CSIVolumeClaimStateNodeDetached:
			err := vw.controllerDetach(vol, claim, nodeClaims)
			if err != nil {
				result = multierror.Append(result, err)
				break
			}
			goto NEXT_STEP
		case structs.CSIVolumeClaimStateReadyToFree:
			err := vw.freeClaim(vol, claim)
			if err != nil {
				result = multierror.Append(result, err)
				break
			}
			goto NEXT_STEP
		case structs.CSIVolumeClaimStateFreed:
		default:
			result = multierror.Append(result, fmt.Errorf("invalid state"))
		}

	}

	return result.ErrorOrNil()

}

// nodeDetach makes the client NodePublish / NodeUnstage RPCs, which
// must be completed before controller operations or releasing the claim.
func (vw *volumeWatcher) nodeDetach(vol *structs.CSIVolume, claim *structs.CSIVolumeClaim) error {
	nReq := &cstructs.ClientCSINodeDetachVolumeRequest{
		PluginID:       vol.PluginID,
		VolumeID:       vol.ID,
		ExternalID:     vol.RemoteID(),
		AllocID:        claim.AllocationID,
		NodeID:         claim.NodeID,
		AttachmentMode: vol.AttachmentMode,
		AccessMode:     vol.AccessMode,
		ReadOnly:       claim.Mode == structs.CSIVolumeClaimRead,
	}

	err := vw.rpc.NodeDetachVolume(nReq,
		&cstructs.ClientCSINodeDetachVolumeResponse{})
	if err != nil {
		return err
	}
	claim.State = structs.CSIVolumeClaimStateNodeDetached
	return vw.syncClaim(vol, claim)
}

// controllerDetach makes the client RPC to the controller to
// unpublish the volume if a controller is required and no other
// allocs on the node need it
func (vw *volumeWatcher) controllerDetach(vol *structs.CSIVolume, claim *structs.CSIVolumeClaim, nodeClaims map[string]int) error {

	if !vol.ControllerRequired || nodeClaims[claim.NodeID] > 1 {
		claim.State = structs.CSIVolumeClaimStateReadyToFree
		return vw.syncClaim(vol, claim)
	}

	// note: we need to get the CSI Node ID, which is not the same as
	// the Nomad Node ID
	ws := memdb.NewWatchSet()
	targetNode, err := vw.state.NodeByID(ws, claim.NodeID)
	if err != nil {
		return err
	}
	if targetNode == nil {
		return fmt.Errorf("%s: %s", structs.ErrUnknownNodePrefix, claim.NodeID)
	}
	targetCSIInfo, ok := targetNode.CSINodePlugins[vol.PluginID]
	if !ok {
		return fmt.Errorf("failed to find NodeInfo for node: %s", targetNode.ID)
	}

	plug, err := vw.state.CSIPluginByID(ws, vol.PluginID)
	if err != nil {
		return fmt.Errorf("plugin lookup error: %s %v", vol.PluginID, err)
	}
	if plug == nil {
		return fmt.Errorf("plugin lookup error: %s missing plugin", vol.PluginID)
	}

	cReq := &cstructs.ClientCSIControllerDetachVolumeRequest{
		VolumeID:        vol.RemoteID(),
		ClientCSINodeID: targetCSIInfo.NodeInfo.ID,
	}
	cReq.PluginID = plug.ID
	err = vw.rpc.ControllerDetachVolume(cReq,
		&cstructs.ClientCSIControllerDetachVolumeResponse{})
	if err != nil {
		return err
	}
	claim.State = structs.CSIVolumeClaimStateReadyToFree
	return vw.syncClaim(vol, claim)
}

// freeClaim releases the claim from the state store, allowing it to be rescheduled
func (vw *volumeWatcher) freeClaim(vol *structs.CSIVolume, claim *structs.CSIVolumeClaim) error {
	claim.State = structs.CSIVolumeClaimStateFreed
	return vw.syncClaim(vol, claim)
}

func (vw *volumeWatcher) syncClaim(vol *structs.CSIVolume, claim *structs.CSIVolumeClaim) error {
	req := structs.CSIVolumeClaimRequest{
		VolumeID:     vol.ID,
		AllocationID: claim.AllocationID,
		NodeID:       claim.NodeID,
		Claim:        claim.Mode,
		State:        claim.State,
		WriteRequest: structs.WriteRequest{
			Namespace: vol.Namespace,
			// Region:    vol.Region, // TODO(tgross) should volumes have regions?
		},
	}
	_, err := vw.updateClaims([]structs.CSIVolumeClaimRequest{req})
	return err
}
