// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import "github.com/hashicorp/nomad/nomad/structs"

// PortCollisionEvent is an event that can happen during scheduling when
// an unexpected port collision is detected.
type PortCollisionEvent struct {
	Reason      string
	Node        *structs.Node
	Allocations []*structs.Allocation

	// TODO: this is a large struct, but may be required to debug unexpected
	// port collisions. Re-evaluate its need in the future if the bug is fixed
	// or not caused by this field.
	NetIndex *structs.NetworkIndex
}

func (ev *PortCollisionEvent) Copy() *PortCollisionEvent {
	if ev == nil {
		return nil
	}
	c := new(PortCollisionEvent)
	*c = *ev
	c.Node = ev.Node.Copy()
	if len(ev.Allocations) > 0 {
		for i, a := range ev.Allocations {
			c.Allocations[i] = a.Copy()
		}

	}
	c.NetIndex = ev.NetIndex.Copy()
	return c
}

func (ev *PortCollisionEvent) Sanitize() *PortCollisionEvent {
	if ev == nil {
		return nil
	}
	clean := ev.Copy()

	clean.Node = ev.Node.Sanitize()
	clean.Node.Meta = make(map[string]string)

	for i, alloc := range ev.Allocations {
		clean.Allocations[i] = alloc.CopySkipJob()
		clean.Allocations[i].Job = nil
	}

	return clean
}
