package drivers

import (
	"github.com/hashicorp/nomad/plugins/base"
)

// TaskHandle is the state shared between a driver and the client.
// It is returned to the client after starting the task and used
// for recovery of tasks during a driver restart.
type TaskHandle struct {
	Driver      string
	Config      *TaskConfig
	State       TaskState
	DriverState []byte
}

func NewTaskHandle(driver string) *TaskHandle {
	return &TaskHandle{Driver: driver}
}

func (h *TaskHandle) SetDriverState(v interface{}) error {
	h.DriverState = []byte{}
	return base.MsgPackEncode(&h.DriverState, v)
}

func (h *TaskHandle) GetDriverState(v interface{}) error {
	return base.MsgPackDecode(h.DriverState, v)

}

func (h *TaskHandle) Copy() *TaskHandle {
	if h == nil {
		return nil
	}

	handle := new(TaskHandle)
	handle.Driver = h.Driver
	handle.Config = h.Config.Copy()
	handle.State = h.State
	handle.DriverState = make([]byte, len(h.DriverState))
	copy(handle.DriverState, h.DriverState)
	return handle
}
