package base

import (
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/ugorji/go/codec"
)

// TaskHandle is the state shared between a driver and the client.
// It is returned to the client after starting the task and used
// for recovery of tasks during a driver restart.
type TaskHandle struct {
	Driver      string
	Config      *TaskConfig
	State       TaskState
	driverState []byte
}

func NewTaskHandle(driver string) *TaskHandle {
	return &TaskHandle{Driver: driver}
}

func (h *TaskHandle) SetDriverState(v interface{}) error {
	h.driverState = []byte{}
	return codec.NewEncoderBytes(&h.driverState, structs.MsgpackHandle).Encode(v)
}

func (h *TaskHandle) GetDriverState(v interface{}) error {
	return codec.NewDecoderBytes(h.driverState, structs.MsgpackHandle).Decode(v)

}
