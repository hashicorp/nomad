package interfaces

import "github.com/hashicorp/nomad/sdk/structs"

type EventEmitter interface {
	EmitEvent(event *structs.TaskEvent)
}
