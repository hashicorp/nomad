// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queues

import (
	"context"

	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

type PassthroughQueue struct {
	broker Broker
}

func NewPassthroughQueue(b Broker) *PassthroughQueue {
	return &PassthroughQueue{
		broker: b,
	}
}

// Start is a noop for the passthrough implementation
func (p *PassthroughQueue) Start(context.Context) error { return nil }

func (p *PassthroughQueue) Stop() {}

func (p *PassthroughQueue) Enqueue(e *structs.Evaluation) { p.broker.Enqueue(e) }

func (p *PassthroughQueue) SetEnabled(bool, *state.StateStore) {}
