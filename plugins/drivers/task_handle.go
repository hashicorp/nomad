package drivers

import (
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/base"
)

// TaskHandle is the state shared between a driver and the client.
// It is returned to the client after starting the task and used
// for recovery of tasks during a driver restart.
type TaskHandle struct {
	// Version is set by the driver an allows it to handle upgrading from
	// an older DriverState struct. Prior to 0.9 the only state stored for
	// driver was the reattach config for the executor. To allow upgrading to
	// 0.9, Version 0 is handled as if it is the json encoded reattach config.
	Version     int
	Config      *TaskConfig
	State       TaskState
	DriverState []byte
}

func NewTaskHandle(version int) *TaskHandle {
	return &TaskHandle{Version: version}
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
	handle.Version = h.Version
	handle.Config = h.Config.Copy()
	handle.State = h.State
	handle.DriverState = make([]byte, len(h.DriverState))
	copy(handle.DriverState, h.DriverState)
	return handle
}

// Store this TaskHandle on the given TaskState.
func (h *TaskHandle) Store(ts *structs.TaskState) {
	if h == nil || len(h.DriverState) == 0 {
		// No handle or state, clear existing state
		ts.TaskHandle = nil
		return
	}

	ds := make([]byte, len(h.DriverState))
	copy(ds, h.DriverState)
	ts.TaskHandle = &structs.TaskHandle{
		Version:     h.Version,
		DriverState: ds,
	}
}

// NewTaskHandleFromState returns the TaskHandle stored in a TaskState or nil
// if no handle was stored.
func NewTaskHandleFromState(ts *structs.TaskState) *TaskHandle {
	if ts.TaskHandle == nil {
		return nil
	}

	th := TaskHandle{
		Version:     ts.TaskHandle.Version,
		DriverState: make([]byte, len(ts.TaskHandle.DriverState)),
	}
	copy(th.DriverState, ts.TaskHandle.DriverState)
	return &th
}
