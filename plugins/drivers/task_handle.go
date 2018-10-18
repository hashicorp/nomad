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
	driverState []byte
}

func NewTaskHandle(driver string) *TaskHandle {
	return &TaskHandle{Driver: driver}
}

func (h *TaskHandle) SetDriverState(v interface{}) error {
	h.driverState = []byte{}
	return base.MsgPackEncode(&h.driverState, v)
}

func (h *TaskHandle) GetDriverState(v interface{}) error {
	return base.MsgPackDecode(h.driverState, v)

}

func (h *TaskHandle) Copy() *TaskHandle {
	if h == nil {
		return nil
	}

	handle := new(TaskHandle)
	*handle = *h
	handle.Config = h.Config.Copy()
	return handle
}
