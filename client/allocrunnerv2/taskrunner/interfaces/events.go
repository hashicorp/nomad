package interfaces

import "github.com/hashicorp/nomad/nomad/structs"

type EventEmitter interface {
	SetState(state string, event *structs.TaskEvent)
	EmitEvent(source, message string)
}
