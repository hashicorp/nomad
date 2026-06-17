// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package queues

import (
	"context"

	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

type TestQueue struct {
	queue []*structs.Evaluation
}

// Start is a noop for the passthrough implementation
func (p *TestQueue) Start(context.Context) error { return nil }

func (p *TestQueue) Enqueue(e *structs.Evaluation) {
	p.queue = append(p.queue, e)
}

func (p *TestQueue) SetEnabled(bool, *state.StateStore) {}

func (p *TestQueue) Status(ns map[string]bool) structs.QueueStatusResponse {
	var allow bool
	if ns == nil {
		allow = true
	}
	var resp structs.QueueStatusResponse
	results := []*structs.Evaluation{}
	for _, e := range p.queue {
		if allow || ns[e.Namespace] {
			results = append(results, e)
		}
	}
	resp.Workloads = results
	resp.Type = "test"
	return resp
}
