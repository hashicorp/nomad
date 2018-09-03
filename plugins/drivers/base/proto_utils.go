package base

import (
	"strings"

	nstructs "github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers/base/proto"
	"github.com/ugorji/go/codec"
)

var protoTaskStateMap = map[TaskState]proto.TaskState{
	TaskStateUnknown: proto.TaskState_UNKNOWN,
	TaskStateRunning: proto.TaskState_RUNNING,
	TaskStateExited:  proto.TaskState_EXITED,
}

func taskConfigFromProto(pb *proto.TaskConfig) *TaskConfig {
	if pb == nil {
		return &TaskConfig{}
	}
	var driverConfig map[string]interface{}
	codec.NewDecoderBytes(pb.MsgpackDriverConfig, nstructs.MsgpackHandle).Decode(&driverConfig)
	return &TaskConfig{
		ID:           pb.Id,
		Name:         pb.Name,
		Env:          pb.Env,
		DriverConfig: driverConfig,
		Resources:    Resources{},      //TODO
		Devices:      []DeviceConfig{}, //TODO
		Mounts:       []MountConfig{},  //TODO
		User:         pb.User,
		AllocDir:     pb.AllocDir,
	}
}

func taskConfigToProto(cfg *TaskConfig) *proto.TaskConfig {
	pb := &proto.TaskConfig{
		Id:        cfg.ID,
		Name:      cfg.Name,
		Env:       cfg.Env,
		Resources: &proto.Resources{},
		Mounts:    []*proto.Mount{},
		Devices:   []*proto.Device{},
		User:      cfg.User,
		AllocDir:  cfg.AllocDir,
	}
	codec.NewEncoderBytes(&pb.MsgpackDriverConfig, nstructs.MsgpackHandle).Encode(cfg.DriverConfig)
	return pb
}

func taskHandleFromProto(pb *proto.TaskHandle) *TaskHandle {
	if pb == nil {
		return &TaskHandle{}
	}
	return &TaskHandle{
		Config:      taskConfigFromProto(pb.Config),
		State:       TaskState(strings.ToLower(pb.State.String())),
		driverState: pb.DriverState,
	}
}

func taskHandleToProto(handle *TaskHandle) *proto.TaskHandle {
	return &proto.TaskHandle{
		Config:      taskConfigToProto(handle.Config),
		State:       protoTaskStateMap[handle.State],
		DriverState: handle.driverState,
	}
}
