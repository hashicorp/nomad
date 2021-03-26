package drivers

import (
	"time"

	"github.com/golang/protobuf/ptypes"
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
		ID:               pb.Id,
		JobName:          pb.JobName,
		TaskGroupName:    pb.TaskGroupName,
		Name:             pb.Name,
		Env:              pb.Env,
		DeviceEnv:        pb.DeviceEnv,
		rawDriverConfig:  pb.MsgpackDriverConfig,
		Resources:        ResourcesFromProto(pb.Resources),
		Devices:          DevicesFromProto(pb.Devices),
		Mounts:           MountsFromProto(pb.Mounts),
		User:             pb.User,
		AllocDir:         pb.AllocDir,
		StdoutPath:       pb.StdoutPath,
		StderrPath:       pb.StderrPath,
		AllocID:          pb.AllocId,
		NetworkIsolation: NetworkIsolationSpecFromProto(pb.NetworkIsolationSpec),
		DNS:              dnsConfigFromProto(pb.Dns),
	}
}

func taskConfigToProto(cfg *TaskConfig) *proto.TaskConfig {
	if cfg == nil {
		return &proto.TaskConfig{}
	}
	pb := &proto.TaskConfig{
		Id:                   cfg.ID,
		JobName:              cfg.JobName,
		TaskGroupName:        cfg.TaskGroupName,
		Name:                 cfg.Name,
		Env:                  cfg.Env,
		DeviceEnv:            cfg.DeviceEnv,
		Resources:            ResourcesToProto(cfg.Resources),
		Devices:              DevicesToProto(cfg.Devices),
		Mounts:               MountsToProto(cfg.Mounts),
		User:                 cfg.User,
		AllocDir:             cfg.AllocDir,
		MsgpackDriverConfig:  cfg.rawDriverConfig,
		StdoutPath:           cfg.StdoutPath,
		StderrPath:           cfg.StderrPath,
		AllocId:              cfg.AllocID,
		NetworkIsolationSpec: NetworkIsolationSpecToProto(cfg.NetworkIsolation),
		Dns:                  dnsConfigToProto(cfg.DNS),
	}
	return pb
}

func ResourcesFromProto(pb *proto.Resources) *Resources {
	var r Resources
	if pb == nil {
		return &r
	}

	if pb.AllocatedResources != nil {
		r.NomadResources = &structs.AllocatedTaskResources{}

		if pb.AllocatedResources.Cpu != nil {
			r.NomadResources.Cpu.CpuShares = pb.AllocatedResources.Cpu.CpuShares
		}

		if pb.AllocatedResources.Memory != nil {
			r.NomadResources.Memory.MemoryMB = pb.AllocatedResources.Memory.MemoryMb
			r.NomadResources.Memory.MemoryMaxMB = pb.AllocatedResources.Memory.MemoryMaxMb
		}

		for _, network := range pb.AllocatedResources.Networks {
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
			PercentTicks:     pb.LinuxResources.PercentTicks,
		}
	}

	if pb.Ports != nil {
		ports := structs.AllocatedPorts(make([]structs.AllocatedPortMapping, len(pb.Ports)))
		for i, port := range pb.Ports {
			ports[i] = structs.AllocatedPortMapping{
				Label:  port.Label,
				Value:  int(port.Value),
				To:     int(port.To),
				HostIP: port.HostIp,
			}
		}
		r.Ports = &ports
	}

	return &r
}

func ResourcesToProto(r *Resources) *proto.Resources {
	if r == nil {
		return nil
	}

	var pb proto.Resources
	if r.NomadResources != nil {
		pb.AllocatedResources = &proto.AllocatedTaskResources{
			Cpu: &proto.AllocatedCpuResources{
				CpuShares: r.NomadResources.Cpu.CpuShares,
			},
			Memory: &proto.AllocatedMemoryResources{
				MemoryMb:    r.NomadResources.Memory.MemoryMB,
				MemoryMaxMb: r.NomadResources.Memory.MemoryMaxMB,
			},
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
			pb.AllocatedResources.Networks[i] = &n
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
			PercentTicks:     r.LinuxResources.PercentTicks,
		}
	}

	if r.Ports != nil {
		ports := make([]*proto.PortMapping, len(*r.Ports))
		for i, port := range *r.Ports {
			ports[i] = &proto.PortMapping{
				Label:  port.Label,
				Value:  int32(port.Value),
				To:     int32(port.To),
				HostIp: port.HostIP,
			}
		}

		pb.Ports = ports
	}

	return &pb
}

func DevicesFromProto(devices []*proto.Device) []*DeviceConfig {
	if devices == nil {
		return nil
	}

	out := make([]*DeviceConfig, len(devices))
	for i, d := range devices {
		out[i] = DeviceFromProto(d)
	}

	return out
}

func DeviceFromProto(device *proto.Device) *DeviceConfig {
	if device == nil {
		return nil
	}

	return &DeviceConfig{
		TaskPath:    device.TaskPath,
		HostPath:    device.HostPath,
		Permissions: device.CgroupPermissions,
	}
}

func MountsFromProto(mounts []*proto.Mount) []*MountConfig {
	if mounts == nil {
		return nil
	}

	out := make([]*MountConfig, len(mounts))
	for i, m := range mounts {
		out[i] = MountFromProto(m)
	}

	return out
}

func MountFromProto(mount *proto.Mount) *MountConfig {
	if mount == nil {
		return nil
	}

	return &MountConfig{
		TaskPath: mount.TaskPath,
		HostPath: mount.HostPath,
		Readonly: mount.Readonly,
	}
}

func DevicesToProto(devices []*DeviceConfig) []*proto.Device {
	if devices == nil {
		return nil
	}

	out := make([]*proto.Device, len(devices))
	for i, d := range devices {
		out[i] = DeviceToProto(d)
	}

	return out
}

func DeviceToProto(device *DeviceConfig) *proto.Device {
	if device == nil {
		return nil
	}

	return &proto.Device{
		TaskPath:          device.TaskPath,
		HostPath:          device.HostPath,
		CgroupPermissions: device.Permissions,
	}
}

func MountsToProto(mounts []*MountConfig) []*proto.Mount {
	if mounts == nil {
		return nil
	}

	out := make([]*proto.Mount, len(mounts))
	for i, m := range mounts {
		out[i] = MountToProto(m)
	}

	return out
}

func MountToProto(mount *MountConfig) *proto.Mount {
	if mount == nil {
		return nil
	}

	return &proto.Mount{
		TaskPath: mount.TaskPath,
		HostPath: mount.HostPath,
		Readonly: mount.Readonly,
	}
}

func taskHandleFromProto(pb *proto.TaskHandle) *TaskHandle {
	if pb == nil {
		return &TaskHandle{}
	}
	return &TaskHandle{
		Version:     int(pb.Version),
		Config:      taskConfigFromProto(pb.Config),
		State:       taskStateFromProtoMap[pb.State],
		DriverState: pb.DriverState,
	}
}

func taskHandleToProto(handle *TaskHandle) *proto.TaskHandle {
	return &proto.TaskHandle{
		Version:     int32(handle.Version),
		Config:      taskConfigToProto(handle.Config),
		State:       taskStateToProtoMap[handle.State],
		DriverState: handle.DriverState,
	}
}

func exitResultToProto(result *ExitResult) *proto.ExitResult {
	if result == nil {
		return &proto.ExitResult{}
	}
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

func TaskStatsToProto(stats *TaskResourceUsage) (*proto.TaskStats, error) {
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

func TaskStatsFromProto(pb *proto.TaskStats) (*TaskResourceUsage, error) {
	timestamp, err := ptypes.Timestamp(pb.Timestamp)
	if err != nil {
		return nil, err
	}

	pids := map[string]*ResourceUsage{}
	for pid, ru := range pb.ResourceUsageByPid {
		pids[pid] = resourceUsageFromProto(ru)
	}

	stats := &TaskResourceUsage{
		Timestamp:     timestamp.UnixNano(),
		ResourceUsage: resourceUsageFromProto(pb.AggResourceUsage),
		Pids:          pids,
	}

	return stats, nil
}

func resourceUsageToProto(ru *ResourceUsage) *proto.TaskResourceUsage {
	cpu := &proto.CPUUsage{
		MeasuredFields:   cpuUsageMeasuredFieldsToProto(ru.CpuStats.Measured),
		SystemMode:       ru.CpuStats.SystemMode,
		UserMode:         ru.CpuStats.UserMode,
		TotalTicks:       ru.CpuStats.TotalTicks,
		ThrottledPeriods: ru.CpuStats.ThrottledPeriods,
		ThrottledTime:    ru.CpuStats.ThrottledTime,
		Percent:          ru.CpuStats.Percent,
	}

	memory := &proto.MemoryUsage{
		MeasuredFields: memoryUsageMeasuredFieldsToProto(ru.MemoryStats.Measured),
		Rss:            ru.MemoryStats.RSS,
		Cache:          ru.MemoryStats.Cache,
		Swap:           ru.MemoryStats.Swap,
		Usage:          ru.MemoryStats.Usage,
		MaxUsage:       ru.MemoryStats.MaxUsage,
		KernelUsage:    ru.MemoryStats.KernelUsage,
		KernelMaxUsage: ru.MemoryStats.KernelMaxUsage,
	}

	return &proto.TaskResourceUsage{
		Cpu:    cpu,
		Memory: memory,
	}
}

func resourceUsageFromProto(pb *proto.TaskResourceUsage) *ResourceUsage {
	cpu := CpuStats{}
	if pb.Cpu != nil {
		cpu = CpuStats{
			Measured:         cpuUsageMeasuredFieldsFromProto(pb.Cpu.MeasuredFields),
			SystemMode:       pb.Cpu.SystemMode,
			UserMode:         pb.Cpu.UserMode,
			TotalTicks:       pb.Cpu.TotalTicks,
			ThrottledPeriods: pb.Cpu.ThrottledPeriods,
			ThrottledTime:    pb.Cpu.ThrottledTime,
			Percent:          pb.Cpu.Percent,
		}
	}

	memory := MemoryStats{}
	if pb.Memory != nil {
		memory = MemoryStats{
			Measured:       memoryUsageMeasuredFieldsFromProto(pb.Memory.MeasuredFields),
			RSS:            pb.Memory.Rss,
			Cache:          pb.Memory.Cache,
			Swap:           pb.Memory.Swap,
			Usage:          pb.Memory.Usage,
			MaxUsage:       pb.Memory.MaxUsage,
			KernelUsage:    pb.Memory.KernelUsage,
			KernelMaxUsage: pb.Memory.KernelMaxUsage,
		}
	}

	return &ResourceUsage{
		CpuStats:    &cpu,
		MemoryStats: &memory,
	}
}

func BytesToMB(bytes int64) int64 {
	return bytes / (1024 * 1024)
}

var cpuUsageMeasuredFieldToProtoMap = map[string]proto.CPUUsage_Fields{
	"System Mode":       proto.CPUUsage_SYSTEM_MODE,
	"User Mode":         proto.CPUUsage_USER_MODE,
	"Total Ticks":       proto.CPUUsage_TOTAL_TICKS,
	"Throttled Periods": proto.CPUUsage_THROTTLED_PERIODS,
	"Throttled Time":    proto.CPUUsage_THROTTLED_TIME,
	"Percent":           proto.CPUUsage_PERCENT,
}

var cpuUsageMeasuredFieldFromProtoMap = map[proto.CPUUsage_Fields]string{
	proto.CPUUsage_SYSTEM_MODE:       "System Mode",
	proto.CPUUsage_USER_MODE:         "User Mode",
	proto.CPUUsage_TOTAL_TICKS:       "Total Ticks",
	proto.CPUUsage_THROTTLED_PERIODS: "Throttled Periods",
	proto.CPUUsage_THROTTLED_TIME:    "Throttled Time",
	proto.CPUUsage_PERCENT:           "Percent",
}

func cpuUsageMeasuredFieldsToProto(fields []string) []proto.CPUUsage_Fields {
	r := make([]proto.CPUUsage_Fields, 0, len(fields))

	for _, f := range fields {
		if v, ok := cpuUsageMeasuredFieldToProtoMap[f]; ok {
			r = append(r, v)
		}
	}

	return r
}

func cpuUsageMeasuredFieldsFromProto(fields []proto.CPUUsage_Fields) []string {
	r := make([]string, 0, len(fields))

	for _, f := range fields {
		if v, ok := cpuUsageMeasuredFieldFromProtoMap[f]; ok {
			r = append(r, v)
		}
	}

	return r
}

var memoryUsageMeasuredFieldToProtoMap = map[string]proto.MemoryUsage_Fields{
	"RSS":              proto.MemoryUsage_RSS,
	"Cache":            proto.MemoryUsage_CACHE,
	"Swap":             proto.MemoryUsage_SWAP,
	"Usage":            proto.MemoryUsage_USAGE,
	"Max Usage":        proto.MemoryUsage_MAX_USAGE,
	"Kernel Usage":     proto.MemoryUsage_KERNEL_USAGE,
	"Kernel Max Usage": proto.MemoryUsage_KERNEL_MAX_USAGE,
}

var memoryUsageMeasuredFieldFromProtoMap = map[proto.MemoryUsage_Fields]string{
	proto.MemoryUsage_RSS:              "RSS",
	proto.MemoryUsage_CACHE:            "Cache",
	proto.MemoryUsage_SWAP:             "Swap",
	proto.MemoryUsage_USAGE:            "Usage",
	proto.MemoryUsage_MAX_USAGE:        "Max Usage",
	proto.MemoryUsage_KERNEL_USAGE:     "Kernel Usage",
	proto.MemoryUsage_KERNEL_MAX_USAGE: "Kernel Max Usage",
}

func memoryUsageMeasuredFieldsToProto(fields []string) []proto.MemoryUsage_Fields {
	r := make([]proto.MemoryUsage_Fields, 0, len(fields))

	for _, f := range fields {
		if v, ok := memoryUsageMeasuredFieldToProtoMap[f]; ok {
			r = append(r, v)
		}
	}

	return r
}

func memoryUsageMeasuredFieldsFromProto(fields []proto.MemoryUsage_Fields) []string {
	r := make([]string, 0, len(fields))

	for _, f := range fields {
		if v, ok := memoryUsageMeasuredFieldFromProtoMap[f]; ok {
			r = append(r, v)
		}
	}

	return r
}

func netIsolationModeToProto(mode NetIsolationMode) proto.NetworkIsolationSpec_NetworkIsolationMode {
	switch mode {
	case NetIsolationModeHost:
		return proto.NetworkIsolationSpec_HOST
	case NetIsolationModeGroup:
		return proto.NetworkIsolationSpec_GROUP
	case NetIsolationModeTask:
		return proto.NetworkIsolationSpec_TASK
	case NetIsolationModeNone:
		return proto.NetworkIsolationSpec_NONE
	default:
		return proto.NetworkIsolationSpec_HOST
	}
}

func netIsolationModeFromProto(pb proto.NetworkIsolationSpec_NetworkIsolationMode) NetIsolationMode {
	switch pb {
	case proto.NetworkIsolationSpec_HOST:
		return NetIsolationModeHost
	case proto.NetworkIsolationSpec_GROUP:
		return NetIsolationModeGroup
	case proto.NetworkIsolationSpec_TASK:
		return NetIsolationModeTask
	case proto.NetworkIsolationSpec_NONE:
		return NetIsolationModeNone
	default:
		return NetIsolationModeHost
	}
}

func NetworkIsolationSpecToProto(spec *NetworkIsolationSpec) *proto.NetworkIsolationSpec {
	if spec == nil {
		return nil
	}
	return &proto.NetworkIsolationSpec{
		Path:   spec.Path,
		Labels: spec.Labels,
		Mode:   netIsolationModeToProto(spec.Mode),
	}
}

func NetworkIsolationSpecFromProto(pb *proto.NetworkIsolationSpec) *NetworkIsolationSpec {
	if pb == nil {
		return nil
	}
	return &NetworkIsolationSpec{
		Path:   pb.Path,
		Labels: pb.Labels,
		Mode:   netIsolationModeFromProto(pb.Mode),
	}
}

func dnsConfigToProto(dns *DNSConfig) *proto.DNSConfig {
	if dns == nil {
		return nil
	}

	return &proto.DNSConfig{
		Servers:  dns.Servers,
		Searches: dns.Searches,
		Options:  dns.Options,
	}
}

func dnsConfigFromProto(pb *proto.DNSConfig) *DNSConfig {
	if pb == nil {
		return nil
	}

	return &DNSConfig{
		Servers:  pb.Servers,
		Searches: pb.Searches,
		Options:  pb.Options,
	}
}
