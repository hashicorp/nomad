package device

import (
	"github.com/golang/protobuf/ptypes"
	"github.com/hashicorp/nomad/plugins/device/proto"
	"github.com/hashicorp/nomad/plugins/shared/structs"
)

// convertProtoDeviceGroups converts between a list of proto and structs DeviceGroup
func convertProtoDeviceGroups(in []*proto.DeviceGroup) []*DeviceGroup {
	if in == nil {
		return nil
	}

	out := make([]*DeviceGroup, len(in))
	for i, group := range in {
		out[i] = convertProtoDeviceGroup(group)
	}

	return out
}

// convertProtoDeviceGroup converts between a proto and structs DeviceGroup
func convertProtoDeviceGroup(in *proto.DeviceGroup) *DeviceGroup {
	if in == nil {
		return nil
	}

	return &DeviceGroup{
		Vendor:     in.Vendor,
		Type:       in.DeviceType,
		Name:       in.DeviceName,
		Devices:    convertProtoDevices(in.Devices),
		Attributes: structs.ConvertProtoAttributeMap(in.Attributes),
	}
}

// convertProtoDevices converts between a list of proto and structs Device
func convertProtoDevices(in []*proto.DetectedDevice) []*Device {
	if in == nil {
		return nil
	}

	out := make([]*Device, len(in))
	for i, d := range in {
		out[i] = convertProtoDevice(d)
	}

	return out
}

// convertProtoDevice converts between a proto and structs Device
func convertProtoDevice(in *proto.DetectedDevice) *Device {
	if in == nil {
		return nil
	}

	return &Device{
		ID:         in.ID,
		Healthy:    in.Healthy,
		HealthDesc: in.HealthDescription,
		HwLocality: convertProtoDeviceLocality(in.HwLocality),
	}
}

// convertProtoDeviceLocality converts between a proto and structs DeviceLocality
func convertProtoDeviceLocality(in *proto.DeviceLocality) *DeviceLocality {
	if in == nil {
		return nil
	}

	return &DeviceLocality{
		PciBusID: in.PciBusId,
	}
}

// convertProtoContainerReservation is used to convert between a proto and struct
// ContainerReservation
func convertProtoContainerReservation(in *proto.ContainerReservation) *ContainerReservation {
	if in == nil {
		return nil
	}

	return &ContainerReservation{
		Envs:    in.Envs,
		Mounts:  convertProtoMounts(in.Mounts),
		Devices: convertProtoDeviceSpecs(in.Devices),
	}
}

// convertProtoMount converts between a list of proto and structs Mount
func convertProtoMounts(in []*proto.Mount) []*Mount {
	if in == nil {
		return nil
	}

	out := make([]*Mount, len(in))
	for i, d := range in {
		out[i] = convertProtoMount(d)
	}

	return out
}

// convertProtoMount converts between a proto and structs Mount
func convertProtoMount(in *proto.Mount) *Mount {
	if in == nil {
		return nil
	}

	return &Mount{
		TaskPath: in.TaskPath,
		HostPath: in.HostPath,
		ReadOnly: in.ReadOnly,
	}
}

// convertProtoDeviceSpecs converts between a list of proto and structs DeviceSpecs
func convertProtoDeviceSpecs(in []*proto.DeviceSpec) []*DeviceSpec {
	if in == nil {
		return nil
	}

	out := make([]*DeviceSpec, len(in))
	for i, d := range in {
		out[i] = convertProtoDeviceSpec(d)
	}

	return out
}

// convertProtoDeviceSpec converts between a proto and structs DeviceSpec
func convertProtoDeviceSpec(in *proto.DeviceSpec) *DeviceSpec {
	if in == nil {
		return nil
	}

	return &DeviceSpec{
		TaskPath:    in.TaskPath,
		HostPath:    in.HostPath,
		CgroupPerms: in.Permissions,
	}
}

// convertStructDeviceGroup converts between a list of struct and proto DeviceGroup
func convertStructDeviceGroups(in []*DeviceGroup) []*proto.DeviceGroup {
	if in == nil {
		return nil
	}

	out := make([]*proto.DeviceGroup, len(in))
	for i, g := range in {
		out[i] = convertStructDeviceGroup(g)
	}

	return out
}

// convertStructDeviceGroup converts between a struct and proto DeviceGroup
func convertStructDeviceGroup(in *DeviceGroup) *proto.DeviceGroup {
	if in == nil {
		return nil
	}

	return &proto.DeviceGroup{
		Vendor:     in.Vendor,
		DeviceType: in.Type,
		DeviceName: in.Name,
		Devices:    convertStructDevices(in.Devices),
		Attributes: structs.ConvertStructAttributeMap(in.Attributes),
	}
}

// convertStructDevices converts between a list of struct and proto Device
func convertStructDevices(in []*Device) []*proto.DetectedDevice {
	if in == nil {
		return nil
	}

	out := make([]*proto.DetectedDevice, len(in))
	for i, d := range in {
		out[i] = convertStructDevice(d)
	}

	return out
}

// convertStructDevice converts between a struct and proto Device
func convertStructDevice(in *Device) *proto.DetectedDevice {
	if in == nil {
		return nil
	}

	return &proto.DetectedDevice{
		ID:                in.ID,
		Healthy:           in.Healthy,
		HealthDescription: in.HealthDesc,
		HwLocality:        convertStructDeviceLocality(in.HwLocality),
	}
}

// convertStructDeviceLocality converts between a struct and proto DeviceLocality
func convertStructDeviceLocality(in *DeviceLocality) *proto.DeviceLocality {
	if in == nil {
		return nil
	}

	return &proto.DeviceLocality{
		PciBusId: in.PciBusID,
	}
}

// convertStructContainerReservation is used to convert between a struct and
// proto ContainerReservation
func convertStructContainerReservation(in *ContainerReservation) *proto.ContainerReservation {
	if in == nil {
		return nil
	}

	return &proto.ContainerReservation{
		Envs:    in.Envs,
		Mounts:  convertStructMounts(in.Mounts),
		Devices: convertStructDeviceSpecs(in.Devices),
	}
}

// convertStructMount converts between a list of structs and proto Mount
func convertStructMounts(in []*Mount) []*proto.Mount {
	if in == nil {
		return nil
	}

	out := make([]*proto.Mount, len(in))
	for i, m := range in {
		out[i] = convertStructMount(m)
	}

	return out
}

// convertStructMount converts between a struct and proto Mount
func convertStructMount(in *Mount) *proto.Mount {
	if in == nil {
		return nil
	}

	return &proto.Mount{
		TaskPath: in.TaskPath,
		HostPath: in.HostPath,
		ReadOnly: in.ReadOnly,
	}
}

// convertStructDeviceSpecs converts between a list of struct and proto DeviceSpecs
func convertStructDeviceSpecs(in []*DeviceSpec) []*proto.DeviceSpec {
	if in == nil {
		return nil
	}

	out := make([]*proto.DeviceSpec, len(in))
	for i, d := range in {
		out[i] = convertStructDeviceSpec(d)
	}

	return out
}

// convertStructDeviceSpec converts between a struct and proto DeviceSpec
func convertStructDeviceSpec(in *DeviceSpec) *proto.DeviceSpec {
	if in == nil {
		return nil
	}

	return &proto.DeviceSpec{
		TaskPath:    in.TaskPath,
		HostPath:    in.HostPath,
		Permissions: in.CgroupPerms,
	}
}

// convertProtoDeviceGroupsStats converts between a list of struct and proto
// DeviceGroupStats
func convertProtoDeviceGroupsStats(in []*proto.DeviceGroupStats) []*DeviceGroupStats {
	if in == nil {
		return nil
	}

	out := make([]*DeviceGroupStats, len(in))
	for i, m := range in {
		out[i] = convertProtoDeviceGroupStats(m)
	}

	return out
}

// convertProtoDeviceGroupStats converts between a proto and struct
// DeviceGroupStats
func convertProtoDeviceGroupStats(in *proto.DeviceGroupStats) *DeviceGroupStats {
	if in == nil {
		return nil
	}

	out := &DeviceGroupStats{
		Vendor:        in.Vendor,
		Type:          in.Type,
		Name:          in.Name,
		InstanceStats: make(map[string]*DeviceStats, len(in.InstanceStats)),
	}

	for k, v := range in.InstanceStats {
		out.InstanceStats[k] = convertProtoDeviceStats(v)
	}

	return out
}

// convertProtoDeviceStats converts between a proto and struct DeviceStats
func convertProtoDeviceStats(in *proto.DeviceStats) *DeviceStats {
	if in == nil {
		return nil
	}

	ts, err := ptypes.Timestamp(in.Timestamp)
	if err != nil {
		return nil
	}

	return &DeviceStats{
		Summary:   structs.ConvertProtoStatValue(in.Summary),
		Stats:     structs.ConvertProtoStatObject(in.Stats),
		Timestamp: ts,
	}
}

// convertStructDeviceGroupsStats converts between a list of struct and proto
// DeviceGroupStats
func convertStructDeviceGroupsStats(in []*DeviceGroupStats) []*proto.DeviceGroupStats {
	if in == nil {
		return nil
	}

	out := make([]*proto.DeviceGroupStats, len(in))
	for i, m := range in {
		out[i] = convertStructDeviceGroupStats(m)
	}

	return out
}

// convertStructDeviceGroupStats converts between a struct and proto
// DeviceGroupStats
func convertStructDeviceGroupStats(in *DeviceGroupStats) *proto.DeviceGroupStats {
	if in == nil {
		return nil
	}

	out := &proto.DeviceGroupStats{
		Vendor:        in.Vendor,
		Type:          in.Type,
		Name:          in.Name,
		InstanceStats: make(map[string]*proto.DeviceStats, len(in.InstanceStats)),
	}

	for k, v := range in.InstanceStats {
		out.InstanceStats[k] = convertStructDeviceStats(v)
	}

	return out
}

// convertStructDeviceStats converts between a struct and proto DeviceStats
func convertStructDeviceStats(in *DeviceStats) *proto.DeviceStats {
	if in == nil {
		return nil
	}

	ts, err := ptypes.TimestampProto(in.Timestamp)
	if err != nil {
		return nil
	}

	return &proto.DeviceStats{
		Summary:   structs.ConvertStructStatValue(in.Summary),
		Stats:     structs.ConvertStructStatObject(in.Stats),
		Timestamp: ts,
	}
}
