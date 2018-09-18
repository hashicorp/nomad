package base

import (
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes"
	nstructs "github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers/base/proto"
	"github.com/ugorji/go/codec"
)

var protoTaskStateMap = map[TaskState]proto.TaskState{
	TaskStateUnknown: proto.TaskState_UNKNOWN,
	TaskStateRunning: proto.TaskState_RUNNING,
	TaskStateExited:  proto.TaskState_EXITED,
}

func healthStateToProto(health HealthState) proto.FingerprintResponse_HealthState {
	switch health {
	case HealthStateUndetected:
		return proto.FingerprintResponse_UNDETECTED
	case HealthStateUnhealthy:
		return proto.FingerprintResponse_UNHEALTHY
	case HealthStateHealthy:
		return proto.FingerprintResponse_HEALTHY
	}
	return proto.FingerprintResponse_UNDETECTED
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
	if cfg == nil {
		return &proto.TaskConfig{}
	}
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

func exitResultToProto(result *ExitResult) *proto.ExitResult {
	return &proto.ExitResult{
		ExitCode:  int32(result.ExitCode),
		Signal:    int32(result.Signal),
		OomKilled: result.OOMKilled,
	}
}

func exitResultFromProto(pb *proto.ExitResult) *ExitResult {
	return &ExitResult{
		ExitCode:  int(pb.ExitCode),
		Signal:    int(pb.Signal),
		OOMKilled: pb.OomKilled,
	}
}

func taskStatusToProto(status *TaskStatus) (*proto.TaskStatus, error) {
	started, err := ptypes.TimestampProto(status.StartedAt)
	if err != nil {
		return nil, err
	}
	completed, err := ptypes.TimestampProto(status.CompletedAt)
	if err != nil {
		return nil, err
	}
	return &proto.TaskStatus{
		Id:           status.ID,
		Name:         status.Name,
		SizeOnDiskMb: status.SizeOnDiskMB,
		StartedAt:    started,
		CompletedAt:  completed,
		Result:       exitResultToProto(status.ExitResult),
	}, nil
}

func taskStatusFromProto(pb *proto.TaskStatus) (*TaskStatus, error) {
	started, err := ptypes.Timestamp(pb.StartedAt)
	if err != nil {
		return nil, err
	}

	completed, err := ptypes.Timestamp(pb.CompletedAt)
	if err != nil {
		return nil, err
	}

	return &TaskStatus{
		ID:           pb.Id,
		Name:         pb.Name,
		SizeOnDiskMB: pb.SizeOnDiskMb,
		StartedAt:    started,
		CompletedAt:  completed,
		ExitResult:   exitResultFromProto(pb.Result),
	}, nil
}

func taskStatsToProto(stats *TaskStats) (*proto.TaskStats, error) {
	timestamp, err := ptypes.TimestampProto(time.Unix(stats.Timestamp, 0))
	if err != nil {
		return nil, err
	}

	pids := map[string]*proto.TaskResourceUsage{}
	for pid, ru := range stats.ResourceUsageByPid {
		pids[pid] = resourceUsageToProto(ru)
	}

	return &proto.TaskStats{
		Id:                 stats.ID,
		Timestamp:          timestamp,
		AggResourceUsage:   resourceUsageToProto(&stats.AggResourceUsage),
		ResourceUsageByPid: pids,
	}, nil
}

func taskStatsFromProto(pb *proto.TaskStats) (*TaskStats, error) {
	return nil, nil
}

func resourceUsageToProto(ru *ResourceUsage) *proto.TaskResourceUsage {
	cpu := &proto.CPUUsage{}
	for _, field := range ru.CPU.Measured {
		switch field {
		case "System Mode":
			cpu.SystemMode = ru.CPU.SystemMode
			cpu.MeasuredFields = append(cpu.MeasuredFields, proto.CPUUsage_SYSTEM_MODE)
		case "User Mode":
			cpu.UserMode = ru.CPU.UserMode
			cpu.MeasuredFields = append(cpu.MeasuredFields, proto.CPUUsage_USER_MODE)
		case "Total Ticks":
			cpu.TotalTicks = ru.CPU.TotalTicks
			cpu.MeasuredFields = append(cpu.MeasuredFields, proto.CPUUsage_TOTAL_TICKS)
		case "Throttled Periods":
			cpu.ThrottledPeriods = ru.CPU.ThrottledPeriods
			cpu.MeasuredFields = append(cpu.MeasuredFields, proto.CPUUsage_THROTTLED_PERIODS)
		case "Throttled Time":
			cpu.ThrottledTime = ru.CPU.ThrottledTime
			cpu.MeasuredFields = append(cpu.MeasuredFields, proto.CPUUsage_THROTTLED_TIME)
		case "Percent":
			cpu.Percent = ru.CPU.Percent
			cpu.MeasuredFields = append(cpu.MeasuredFields, proto.CPUUsage_PERCENT)
		}
	}

	memory := &proto.MemoryUsage{}
	for _, field := range ru.Memory.Measured {
		switch field {
		case "RSS":
			memory.Rss = ru.Memory.RSS
			memory.MeasuredFields = append(memory.MeasuredFields, proto.MemoryUsage_RSS)
		case "Cache":
			memory.Cache = ru.Memory.Cache
			memory.MeasuredFields = append(memory.MeasuredFields, proto.MemoryUsage_CACHE)
		case "Max Usage":
			memory.MaxUsage = ru.Memory.MaxUsage
			memory.MeasuredFields = append(memory.MeasuredFields, proto.MemoryUsage_MAX_USAGE)
		case "Kernel Usage":
			memory.KernelUsage = ru.Memory.KernelUsage
			memory.MeasuredFields = append(memory.MeasuredFields, proto.MemoryUsage_KERNEL_USAGE)
		case "Kernel Max Usage":
			memory.KernelMaxUsage = ru.Memory.KernelMaxUsage
			memory.MeasuredFields = append(memory.MeasuredFields, proto.MemoryUsage_KERNEL_MAX_USAGE)
		}
	}

	return &proto.TaskResourceUsage{
		Cpu:    cpu,
		Memory: memory,
	}
}

func resourceUsageFromProto(pb *proto.TaskResourceUsage) *ResourceUsage {
	return nil
}
