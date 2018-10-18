package drivers

import (
	"time"

	"github.com/golang/protobuf/ptypes"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers/proto"
)

var taskStateToProtoMap = map[TaskState]proto.TaskState{
	TaskStateUnknown: proto.TaskState_UNKNOWN,
	TaskStateRunning: proto.TaskState_RUNNING,
	TaskStateExited:  proto.TaskState_EXITED,
}

var taskStateFromProtoMap = map[proto.TaskState]TaskState{
	proto.TaskState_UNKNOWN: TaskStateUnknown,
	proto.TaskState_RUNNING: TaskStateRunning,
	proto.TaskState_EXITED:  TaskStateExited,
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

func healthStateFromProto(pb proto.FingerprintResponse_HealthState) HealthState {
	switch pb {
	case proto.FingerprintResponse_UNDETECTED:
		return HealthStateUndetected
	case proto.FingerprintResponse_UNHEALTHY:
		return HealthStateUnhealthy
	case proto.FingerprintResponse_HEALTHY:
		return HealthStateHealthy
	}
	return HealthStateUndetected
}

func taskConfigFromProto(pb *proto.TaskConfig) *TaskConfig {
	if pb == nil {
		return &TaskConfig{}
	}
	return &TaskConfig{
		ID:              pb.Id,
		Name:            pb.Name,
		Env:             pb.Env,
		rawDriverConfig: pb.MsgpackDriverConfig,
		Resources:       resourcesFromProto(pb.Resources),
		Devices:         []DeviceConfig{}, //TODO
		Mounts:          []MountConfig{},  //TODO
		User:            pb.User,
		AllocDir:        pb.AllocDir,
		StdoutPath:      pb.StdoutPath,
		StderrPath:      pb.StderrPath,
	}
}

func taskConfigToProto(cfg *TaskConfig) *proto.TaskConfig {
	if cfg == nil {
		return &proto.TaskConfig{}
	}
	pb := &proto.TaskConfig{
		Id:                  cfg.ID,
		Name:                cfg.Name,
		Env:                 cfg.Env,
		Resources:           resourcesToProto(cfg.Resources),
		Mounts:              []*proto.Mount{},
		Devices:             []*proto.Device{},
		User:                cfg.User,
		AllocDir:            cfg.AllocDir,
		MsgpackDriverConfig: cfg.rawDriverConfig,
		StdoutPath:          cfg.StdoutPath,
		StderrPath:          cfg.StderrPath,
	}
	return pb
}

func resourcesFromProto(pb *proto.Resources) *Resources {
	var r Resources
	if pb == nil {
		return &r
	}

	if pb.RawResources != nil {
		r.NomadResources = &structs.Resources{
			CPU:      int(pb.RawResources.Cpu),
			MemoryMB: int(pb.RawResources.Memory),
			IOPS:     int(pb.RawResources.Iops),
			DiskMB:   int(pb.RawResources.Disk),
		}

		for _, network := range pb.RawResources.Networks {
			var n structs.NetworkResource
			n.Device = network.Device
			n.IP = network.Ip
			n.CIDR = network.Cidr
			n.MBits = int(network.Mbits)
			for _, port := range network.ReservedPorts {
				n.ReservedPorts = append(n.ReservedPorts, structs.Port{
					Label: port.Label,
					Value: int(port.Value),
				})
			}
			for _, port := range network.DynamicPorts {
				n.DynamicPorts = append(n.DynamicPorts, structs.Port{
					Label: port.Label,
					Value: int(port.Value),
				})
			}
			r.NomadResources.Networks = append(r.NomadResources.Networks, &n)
		}
	}

	if pb.LinuxResources != nil {
		r.LinuxResources = &LinuxResources{
			CPUPeriod:        pb.LinuxResources.CpuPeriod,
			CPUQuota:         pb.LinuxResources.CpuQuota,
			CPUShares:        pb.LinuxResources.CpuShares,
			MemoryLimitBytes: pb.LinuxResources.MemoryLimitBytes,
			OOMScoreAdj:      pb.LinuxResources.OomScoreAdj,
			CpusetCPUs:       pb.LinuxResources.CpusetCpus,
			CpusetMems:       pb.LinuxResources.CpusetMems,
		}
	}

	return &r
}

func resourcesToProto(r *Resources) *proto.Resources {
	if r == nil {
		return nil
	}
	var pb proto.Resources
	if r.NomadResources != nil {
		pb.RawResources = &proto.RawResources{
			Cpu:      int64(r.NomadResources.CPU),
			Memory:   int64(r.NomadResources.MemoryMB),
			Iops:     int64(r.NomadResources.IOPS),
			Disk:     int64(r.NomadResources.DiskMB),
			Networks: make([]*proto.NetworkResource, len(r.NomadResources.Networks)),
		}

		for i, network := range r.NomadResources.Networks {
			var n proto.NetworkResource
			n.Device = network.Device
			n.Ip = network.IP
			n.Cidr = network.CIDR
			n.Mbits = int32(network.MBits)
			n.ReservedPorts = []*proto.NetworkPort{}
			for _, port := range network.ReservedPorts {
				n.ReservedPorts = append(n.ReservedPorts, &proto.NetworkPort{
					Label: port.Label,
					Value: int32(port.Value),
				})
			}
			for _, port := range network.DynamicPorts {
				n.DynamicPorts = append(n.DynamicPorts, &proto.NetworkPort{
					Label: port.Label,
					Value: int32(port.Value),
				})
			}
			pb.RawResources.Networks[i] = &n
		}
	}

	if r.LinuxResources != nil {
		pb.LinuxResources = &proto.LinuxResources{
			CpuPeriod:        r.LinuxResources.CPUPeriod,
			CpuQuota:         r.LinuxResources.CPUQuota,
			CpuShares:        r.LinuxResources.CPUShares,
			MemoryLimitBytes: r.LinuxResources.MemoryLimitBytes,
			OomScoreAdj:      r.LinuxResources.OOMScoreAdj,
			CpusetCpus:       r.LinuxResources.CpusetCPUs,
			CpusetMems:       r.LinuxResources.CpusetMems,
		}
	}

	return &pb
}

func taskHandleFromProto(pb *proto.TaskHandle) *TaskHandle {
	if pb == nil {
		return &TaskHandle{}
	}
	return &TaskHandle{
		Config:      taskConfigFromProto(pb.Config),
		State:       taskStateFromProtoMap[pb.State],
		DriverState: pb.DriverState,
	}
}

func taskHandleToProto(handle *TaskHandle) *proto.TaskHandle {
	return &proto.TaskHandle{
		Config:      taskConfigToProto(handle.Config),
		State:       taskStateToProtoMap[handle.State],
		DriverState: handle.DriverState,
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
		Id:          status.ID,
		Name:        status.Name,
		State:       taskStateToProtoMap[status.State],
		StartedAt:   started,
		CompletedAt: completed,
		Result:      exitResultToProto(status.ExitResult),
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
		ID:          pb.Id,
		Name:        pb.Name,
		State:       taskStateFromProtoMap[pb.State],
		StartedAt:   started,
		CompletedAt: completed,
		ExitResult:  exitResultFromProto(pb.Result),
	}, nil
}

func taskStatsToProto(stats *cstructs.TaskResourceUsage) (*proto.TaskStats, error) {
	timestamp, err := ptypes.TimestampProto(time.Unix(0, stats.Timestamp))
	if err != nil {
		return nil, err
	}

	pids := map[string]*proto.TaskResourceUsage{}
	for pid, ru := range stats.Pids {
		pids[pid] = resourceUsageToProto(ru)
	}

	return &proto.TaskStats{
		Timestamp:          timestamp,
		AggResourceUsage:   resourceUsageToProto(stats.ResourceUsage),
		ResourceUsageByPid: pids,
	}, nil
}

func taskStatsFromProto(pb *proto.TaskStats) (*cstructs.TaskResourceUsage, error) {
	timestamp, err := ptypes.Timestamp(pb.Timestamp)
	if err != nil {
		return nil, err
	}

	pids := map[string]*cstructs.ResourceUsage{}
	for pid, ru := range pb.ResourceUsageByPid {
		pids[pid] = resourceUsageFromProto(ru)
	}

	stats := &cstructs.TaskResourceUsage{
		Timestamp:     timestamp.Unix(),
		ResourceUsage: resourceUsageFromProto(pb.AggResourceUsage),
		Pids:          pids,
	}

	return stats, nil
}

func resourceUsageToProto(ru *cstructs.ResourceUsage) *proto.TaskResourceUsage {
	cpu := &proto.CPUUsage{}
	for _, field := range ru.CpuStats.Measured {
		switch field {
		case "System Mode":
			cpu.SystemMode = ru.CpuStats.SystemMode
			cpu.MeasuredFields = append(cpu.MeasuredFields, proto.CPUUsage_SYSTEM_MODE)
		case "User Mode":
			cpu.UserMode = ru.CpuStats.UserMode
			cpu.MeasuredFields = append(cpu.MeasuredFields, proto.CPUUsage_USER_MODE)
		case "Total Ticks":
			cpu.TotalTicks = ru.CpuStats.TotalTicks
			cpu.MeasuredFields = append(cpu.MeasuredFields, proto.CPUUsage_TOTAL_TICKS)
		case "Throttled Periods":
			cpu.ThrottledPeriods = ru.CpuStats.ThrottledPeriods
			cpu.MeasuredFields = append(cpu.MeasuredFields, proto.CPUUsage_THROTTLED_PERIODS)
		case "Throttled Time":
			cpu.ThrottledTime = ru.CpuStats.ThrottledTime
			cpu.MeasuredFields = append(cpu.MeasuredFields, proto.CPUUsage_THROTTLED_TIME)
		case "Percent":
			cpu.Percent = ru.CpuStats.Percent
			cpu.MeasuredFields = append(cpu.MeasuredFields, proto.CPUUsage_PERCENT)
		}
	}

	memory := &proto.MemoryUsage{}
	for _, field := range ru.MemoryStats.Measured {
		switch field {
		case "RSS":
			memory.Rss = ru.MemoryStats.RSS
			memory.MeasuredFields = append(memory.MeasuredFields, proto.MemoryUsage_RSS)
		case "Cache":
			memory.Cache = ru.MemoryStats.Cache
			memory.MeasuredFields = append(memory.MeasuredFields, proto.MemoryUsage_CACHE)
		case "Max Usage":
			memory.MaxUsage = ru.MemoryStats.MaxUsage
			memory.MeasuredFields = append(memory.MeasuredFields, proto.MemoryUsage_MAX_USAGE)
		case "Kernel Usage":
			memory.KernelUsage = ru.MemoryStats.KernelUsage
			memory.MeasuredFields = append(memory.MeasuredFields, proto.MemoryUsage_KERNEL_USAGE)
		case "Kernel Max Usage":
			memory.KernelMaxUsage = ru.MemoryStats.KernelMaxUsage
			memory.MeasuredFields = append(memory.MeasuredFields, proto.MemoryUsage_KERNEL_MAX_USAGE)
		}
	}

	return &proto.TaskResourceUsage{
		Cpu:    cpu,
		Memory: memory,
	}
}

func resourceUsageFromProto(pb *proto.TaskResourceUsage) *cstructs.ResourceUsage {
	cpu := cstructs.CpuStats{}
	if pb.Cpu != nil {
		for _, field := range pb.Cpu.MeasuredFields {
			switch field {
			case proto.CPUUsage_SYSTEM_MODE:
				cpu.SystemMode = pb.Cpu.SystemMode
				cpu.Measured = append(cpu.Measured, "System Mode")
			case proto.CPUUsage_USER_MODE:
				cpu.UserMode = pb.Cpu.UserMode
				cpu.Measured = append(cpu.Measured, "User Mode")
			case proto.CPUUsage_TOTAL_TICKS:
				cpu.TotalTicks = pb.Cpu.TotalTicks
				cpu.Measured = append(cpu.Measured, "Total Ticks")
			case proto.CPUUsage_THROTTLED_PERIODS:
				cpu.ThrottledPeriods = pb.Cpu.ThrottledPeriods
				cpu.Measured = append(cpu.Measured, "Throttled Periods")
			case proto.CPUUsage_THROTTLED_TIME:
				cpu.ThrottledTime = pb.Cpu.ThrottledTime
				cpu.Measured = append(cpu.Measured, "Throttled Time")
			case proto.CPUUsage_PERCENT:
				cpu.Percent = pb.Cpu.Percent
				cpu.Measured = append(cpu.Measured, "Percent")
			}
		}
	}

	memory := cstructs.MemoryStats{}
	if pb.Memory != nil {
		for _, field := range pb.Memory.MeasuredFields {
			switch field {
			case proto.MemoryUsage_RSS:
				memory.RSS = pb.Memory.Rss
				memory.Measured = append(memory.Measured, "RSS")
			case proto.MemoryUsage_CACHE:
				memory.Cache = pb.Memory.Cache
				memory.Measured = append(memory.Measured, "Cache")
			case proto.MemoryUsage_MAX_USAGE:
				memory.MaxUsage = pb.Memory.MaxUsage
				memory.Measured = append(memory.Measured, "Max Usage")
			case proto.MemoryUsage_KERNEL_USAGE:
				memory.KernelUsage = pb.Memory.KernelUsage
				memory.Measured = append(memory.Measured, "Kernel Usage")
			case proto.MemoryUsage_KERNEL_MAX_USAGE:
				memory.KernelMaxUsage = pb.Memory.KernelMaxUsage
				memory.Measured = append(memory.Measured, "Kernel Max Usage")
			}
		}
	}

	return &cstructs.ResourceUsage{
		CpuStats:    &cpu,
		MemoryStats: &memory,
	}
}

func BytesToMB(bytes int64) int64 {
	return bytes / (1024 * 1024)
}
