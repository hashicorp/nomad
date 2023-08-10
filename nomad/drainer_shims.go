// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

// drainerShim implements the drainer.RaftApplier interface required by the
// NodeDrainer.
type drainerShim struct {
	s *Server
}

func (d drainerShim) NodesDrainComplete(nodes []string, event *structs.NodeEvent) (uint64, error) {
	args := &structs.BatchNodeUpdateDrainRequest{
		Updates:      make(map[string]*structs.DrainUpdate, len(nodes)),
		NodeEvents:   make(map[string]*structs.NodeEvent, len(nodes)),
		WriteRequest: structs.WriteRequest{Region: d.s.config.Region},
		UpdatedAt:    time.Now().Unix(),
	}

	update := &structs.DrainUpdate{}
	for _, node := range nodes {
		args.Updates[node] = update
		if event != nil {
			args.NodeEvents[node] = event
		}
	}

	_, index, err := d.s.raftApply(structs.BatchNodeUpdateDrainRequestType, args)
	return index, err
}

func (d drainerShim) AllocUpdateDesiredTransition(allocs map[string]*structs.DesiredTransition, evals []*structs.Evaluation) (uint64, error) {
	args := &structs.AllocUpdateDesiredTransitionRequest{
		Allocs:       allocs,
		Evals:        evals,
		WriteRequest: structs.WriteRequest{Region: d.s.config.Region},
	}
	_, index, err := d.s.raftApply(structs.AllocUpdateDesiredTransitionRequestType, args)
	return index, err
}
