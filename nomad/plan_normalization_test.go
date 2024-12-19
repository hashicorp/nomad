// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"bytes"
	"testing"
	"time"

	"github.com/hashicorp/go-msgpack/v2/codec"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

// This test compares the size of the normalized + OmitEmpty raft plan log entry
// with the earlier denormalized log.
//
// Whenever this test is changed, care should be taken to ensure the older msgpack size
// is recalculated when new fields are introduced in ApplyPlanResultsRequest
//
// If you make an unrelated change that unexpectedly fails this test,
// consider adding omitempty to the struct you are modifying, e.g.:
//
//	 type NetworkResource struct {
//		// msgpack omit empty fields during serialization
//		_struct bool `codec:",omitempty"` // nolint: structcheck
func TestPlanNormalize(t *testing.T) {
	ci.Parallel(t)

	// This size was calculated using the older ApplyPlanResultsRequest format, in which allocations
	// didn't use OmitEmpty and only the job was normalized in the stopped and preempted allocs.
	// The newer format uses OmitEmpty and uses a minimal set of fields for the diff of the
	// stopped and preempted allocs. The file for the older format hasn't been checked in, because
	// it's not a good idea to check-in a 20mb file to the git repo.
	unoptimizedLogSize := 19460168

	numUpdatedAllocs := 10000
	numStoppedAllocs := 8000
	numPreemptedAllocs := 2000
	mockAlloc := mock.Alloc()
	mockAlloc.Job = nil

	mockUpdatedAllocSlice := make([]*structs.Allocation, numUpdatedAllocs)
	for i := 0; i < numUpdatedAllocs; i++ {
		mockUpdatedAllocSlice = append(mockUpdatedAllocSlice, mockAlloc)
	}

	now := time.Now().UTC().UnixNano()
	mockStoppedAllocSlice := make([]*structs.AllocationDiff, numStoppedAllocs)
	for i := 0; i < numStoppedAllocs; i++ {
		mockStoppedAllocSlice = append(mockStoppedAllocSlice, normalizeStoppedAlloc(mockAlloc, now))
	}

	mockPreemptionAllocSlice := make([]*structs.AllocationDiff, numPreemptedAllocs)
	for i := 0; i < numPreemptedAllocs; i++ {
		mockPreemptionAllocSlice = append(mockPreemptionAllocSlice, normalizePreemptedAlloc(mockAlloc, now))
	}

	// Create a plan result
	applyPlanLogEntry := structs.ApplyPlanResultsRequest{
		AllocUpdateRequest: structs.AllocUpdateRequest{
			AllocsUpdated: mockUpdatedAllocSlice,
			AllocsStopped: mockStoppedAllocSlice,
		},
		AllocsPreempted: mockPreemptionAllocSlice,
	}

	handle := structs.MsgpackHandle
	var buf bytes.Buffer
	if err := codec.NewEncoder(&buf, handle).Encode(applyPlanLogEntry); err != nil {
		t.Fatalf("Encoding failed: %v", err)
	}

	optimizedLogSize := buf.Len()
	ratio := float64(optimizedLogSize) / float64(unoptimizedLogSize)
	must.Less(t, 0.6, ratio)
}
