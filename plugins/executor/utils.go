package executor

import (
	"github.com/golang/protobuf/ptypes"
	"github.com/hashicorp/nomad/drivers/shared/executor"
	"github.com/hashicorp/nomad/plugins/executor/proto"
)

func processStateToProto(ps *executor.ProcessState) (*proto.ProcessState, error) {
	timestamp, err := ptypes.TimestampProto(ps.Time)
	if err != nil {
		return nil, err
	}
	pb := &proto.ProcessState{
		Pid:      int32(ps.Pid),
		ExitCode: int32(ps.ExitCode),
		Signal:   int32(ps.Signal),
		Time:     timestamp,
	}

	return pb, nil
}

func processStateFromProto(pb *proto.ProcessState) (*executor.ProcessState, error) {
	timestamp, err := ptypes.Timestamp(pb.Time)
	if err != nil {
		return nil, err
	}

	return &executor.ProcessState{
		Pid:      int(pb.Pid),
		ExitCode: int(pb.ExitCode),
		Signal:   int(pb.Signal),
		Time:     timestamp,
	}, nil
}

func resourcesToProto(r *executor.Resources) *proto.Resources {
	if r == nil {
		return &proto.Resources{}
	}

	return &proto.Resources{
		Cpu:      int32(r.CPU),
		MemoryMB: int32(r.MemoryMB),
		DiskMB:   int32(r.DiskMB),
		Iops:     int32(r.IOPS),
	}
}

func resourcesFromProto(pb *proto.Resources) *executor.Resources {
	if pb == nil {
		return &executor.Resources{}
	}

	return &executor.Resources{
		CPU:      int(pb.Cpu),
		MemoryMB: int(pb.MemoryMB),
		DiskMB:   int(pb.DiskMB),
		IOPS:     int(pb.Iops),
	}
}
