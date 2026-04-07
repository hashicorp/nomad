// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"github.com/hashicorp/nomad/nomad/batchtimeout"
	"github.com/hashicorp/nomad/nomad/structs"
)

type batchTimeoutRaftShim struct {
	s *Server
}

var _ batchtimeout.RaftApplier = batchTimeoutRaftShim{}

func (b batchTimeoutRaftShim) UpdateAllocDesiredTransition(req *structs.AllocUpdateDesiredTransitionRequest) (uint64, error) {
	_, index, err := b.s.raftApply(structs.AllocUpdateDesiredTransitionRequestType, req)
	return index, err
}
