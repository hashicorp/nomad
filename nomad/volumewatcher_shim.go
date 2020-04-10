package nomad

import (
	"github.com/hashicorp/nomad/nomad/structs"
)

// volumeWatcherRaftShim is the shim that provides the state watching
// methods. These should be set by the server and passed to the volume
// watcher.
type volumeWatcherRaftShim struct {
	// apply is used to apply a message to Raft
	apply raftApplyFn
}

// convertApplyErrors parses the results of a raftApply and returns the index at
// which it was applied and any error that occurred. Raft Apply returns two
// separate errors, Raft library errors and user returned errors from the FSM.
// This helper, joins the errors by inspecting the applyResponse for an error.
func (shim *volumeWatcherRaftShim) convertApplyErrors(applyResp interface{}, index uint64, err error) (uint64, error) {
	if applyResp != nil {
		if fsmErr, ok := applyResp.(error); ok && fsmErr != nil {
			return index, fsmErr
		}
	}
	return index, err
}

func (shim *volumeWatcherRaftShim) UpsertVolumeClaims(req *structs.CSIVolumeClaimBatchRequest) (uint64, error) {
	fsmErrIntf, index, raftErr := shim.apply(structs.CSIVolumeClaimRequestType, req)
	return shim.convertApplyErrors(fsmErrIntf, index, raftErr)
}
