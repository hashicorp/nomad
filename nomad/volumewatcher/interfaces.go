package volumewatcher

import (
	"github.com/hashicorp/nomad/nomad/structs"
)

// VolumeRaftEndpoints exposes the volume watcher to a set of functions
// to apply data transforms via Raft.
type VolumeRaftEndpoints interface {

	// UpsertVolumeClaims applys a batch of claims to raft
	UpsertVolumeClaims(*structs.CSIVolumeClaimBatchRequest) (uint64, error)
}

// CSIVolumeRPC is a minimal interface of the Server, intended as an aid
// for testing logic surrounding server-to-server or server-to-client
// RPC calls and to avoid circular references between the nomad
// package and the volumewatcher
type CSIVolumeRPC interface {
	Unpublish(args *structs.CSIVolumeUnpublishRequest, reply *structs.CSIVolumeUnpublishResponse) error
}

// claimUpdater is the function used to update claims on behalf of a volume
// (used to wrap batch updates so that we can test
// volumeWatcher methods synchronously without batching)
type updateClaimsFn func(claims []structs.CSIVolumeClaimRequest) (uint64, error)
