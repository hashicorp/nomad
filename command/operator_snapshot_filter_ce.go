// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent

package command

import (
	"errors"

	"github.com/hashicorp/nomad/nomad"
)

func (c *OperatorSnapshotFilterCommand) getEntTypes() ([]nomad.SnapshotType, error) {

	// Because the messages we apply in the FSM aren't framed with a length, the
	// FSM can't decode logs that it doesn't have an definition for in the
	// nomad/structs package. So we can't decode objects in the ENT code base
	// from CE even though we could potentially filter out items with an unknown
	// MessageType byte
	return []nomad.SnapshotType{}, errors.New(
		"-exclude-enterprise requires a Nomad Enterprise binary")
}
