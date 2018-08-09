package base

import "github.com/hashicorp/nomad/plugins/drivers/base/proto"

func taskConfigFromProto(pb *proto.TaskConfig) *TaskConfig {
	//TODO
	return &TaskConfig{
		ID: pb.Id,
	}
}

func taskConfigToProto(cfg *TaskConfig) *proto.TaskConfig {
	//TODO
	return &proto.TaskConfig{
		Id: cfg.ID,
	}
}

func taskHandleFromProto(pb *proto.TaskHandle) *TaskHandle {
	return &TaskHandle{
		Driver:      pb.Driver,
		Config:      taskConfigFromProto(pb.Config),
		State:       TaskState(pb.State),
		driverState: pb.MsgpackDriverState,
	}
}

func taskHandleToProto(handle *TaskHandle) *proto.TaskHandle {
	return &proto.TaskHandle{
		Driver:             handle.Driver,
		Config:             taskConfigToProto(handle.Config),
		State:              string(handle.State),
		MsgpackDriverState: handle.driverState,
	}
}
