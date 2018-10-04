package interfaces

import "github.com/hashicorp/nomad/nomad/structs"

type EventEmitter interface {
	EmitEvent(event *structs.TaskEvent)
}
