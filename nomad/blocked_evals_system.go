// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import "github.com/hashicorp/nomad/nomad/structs"

// systemEvals are handled specially, each job may have a blocked eval on each node
type systemEvals struct {
	// byJob maps a jobID to a nodeID to that job's single blocked evalID on that node
	byJob map[structs.NamespacedID]map[string]string

	// byNode maps a nodeID to a set of evalIDs
	byNode map[string]map[string]bool

	// evals maps evalIDs to an eval and token
	evals map[string]*wrappedEval
}

func newSystemEvals() *systemEvals {
	return &systemEvals{
		evals:  map[string]*wrappedEval{},
		byJob:  map[structs.NamespacedID]map[string]string{},
		byNode: map[string]map[string]bool{},
	}
}

func (s *systemEvals) Add(eval *structs.Evaluation, token string) {
	// store the eval by node id
	if _, ok := s.byNode[eval.NodeID]; !ok {
		s.byNode[eval.NodeID] = make(map[string]bool)
	}

	s.byNode[eval.NodeID][eval.ID] = true
	s.evals[eval.ID] = &wrappedEval{eval: eval, token: token}

	// link the job to the node for cleanup
	jobID := structs.NewNamespacedID(eval.JobID, eval.Namespace)
	if _, ok := s.byJob[jobID]; !ok {
		s.byJob[jobID] = make(map[string]string)
	}

	// if we're displacing the old blocked id for this job+node, delete it first
	if prevID, ok := s.byJob[jobID][eval.NodeID]; ok {
		prev, _ := s.Get(prevID)
		s.Remove(prev.eval)
	}

	// set this eval as the new eval for this job on this node
	s.byJob[jobID][eval.NodeID] = eval.ID
}

func (s *systemEvals) Get(evalID string) (*wrappedEval, bool) {
	w, ok := s.evals[evalID]
	return w, ok
}

func (s *systemEvals) Remove(eval *structs.Evaluation) {
	// delete the job index if this eval is the currently listed blocked eval
	jobID := structs.NewNamespacedID(eval.JobID, eval.Namespace)
	e, ok := s.byJob[jobID][eval.NodeID]
	if ok && e == eval.ID {
		delete(s.byJob[jobID], eval.NodeID)
	}

	// delete this eval from the node index, and then the map for this node if empty
	delete(s.byNode[eval.NodeID], eval.ID)
	if len(s.byNode[eval.NodeID]) == 0 {
		delete(s.byNode, eval.NodeID)
	}

	// delete the eval itself
	delete(s.evals, eval.ID)
}

func (s *systemEvals) NodeEvals(nodeID string) (map[*structs.Evaluation]string, bool) {
	out := map[*structs.Evaluation]string{}
	for eID := range s.byNode[nodeID] {
		if w, ok := s.Get(eID); ok {
			out[w.eval] = w.token
		}
	}

	ok := len(out) > 0
	return out, ok
}

func (s *systemEvals) JobEvals(jobID structs.NamespacedID) ([]*structs.Evaluation, bool) {
	out := []*structs.Evaluation{}
	_, ok := s.byJob[jobID]
	for _, eID := range s.byJob[jobID] {
		if e, ok := s.Get(eID); ok {
			out = append(out, e.eval)
		}
	}
	return out, ok
}
