package nomad

import (
	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs"
)

// csiBatchRelease is a helper for any time we need to release a bunch
// of volume claims at once. It de-duplicates the volumes and batches
// the raft messages into manageable chunks. Intended for use by RPCs
// that have already been forwarded to the leader.
type csiBatchRelease struct {
	srv    *Server
	logger log.Logger

	maxBatchSize int
	seen         map[string]struct{}
	batches      []*structs.CSIVolumeClaimBatchRequest
}

func newCSIBatchRelease(srv *Server, logger log.Logger, max int) *csiBatchRelease {
	return &csiBatchRelease{
		srv:          srv,
		logger:       logger,
		maxBatchSize: max,
		seen:         map[string]struct{}{},
		batches:      []*structs.CSIVolumeClaimBatchRequest{{}},
	}
}

// add the volume ID + namespace to the deduplicated batches
func (c *csiBatchRelease) add(vol, namespace string) {
	id := vol + namespace

	// ignore duplicates
	_, seen := c.seen[id]
	if seen {
		return
	}

	req := structs.CSIVolumeClaimRequest{
		VolumeID: vol,
		Claim:    structs.CSIVolumeClaimRelease,
	}
	req.Namespace = namespace
	req.Region = c.srv.config.Region

	for _, batch := range c.batches {
		// otherwise append to the first non-full batch
		if len(batch.Claims) < c.maxBatchSize {
			batch.Claims = append(batch.Claims, req)
			return
		}
	}
	// no non-full batch found, make a new one
	newBatch := &structs.CSIVolumeClaimBatchRequest{
		Claims: []structs.CSIVolumeClaimRequest{req}}
	c.batches = append(c.batches, newBatch)
}

// apply flushes the batches to raft
func (c *csiBatchRelease) apply() error {
	for _, batch := range c.batches {
		if len(batch.Claims) > 0 {
			_, _, err := c.srv.raftApply(structs.CSIVolumeClaimBatchRequestType, batch)
			if err != nil {
				c.logger.Error("csi raft apply failed", "error", err, "method", "claim")
				return err
			}
		}
	}
	return nil
}
