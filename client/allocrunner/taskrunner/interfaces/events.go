// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package interfaces

import "github.com/hashicorp/nomad/nomad/structs"

type EventEmitter interface {
	EmitEvent(event *structs.TaskEvent)
}
