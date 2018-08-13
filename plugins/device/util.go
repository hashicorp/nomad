package device

import "github.com/hashicorp/nomad/plugins/device/proto"

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
		Vendor:     in.GetVendor(),
		Type:       in.GetDeviceType(),
		Name:       in.GetDeviceName(),
		Devices:    convertProtoDevices(in.GetDevices()),
		Attributes: in.GetAttributes(),
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
		ID:         in.GetID(),
		Healthy:    in.GetHealthy(),
		HealthDesc: in.GetHealthDescription(),
		HwLocality: convertProtoDeviceLocality(in.GetHwLocality()),
	}
}

// convertProtoDeviceLocality converts between a proto and structs DeviceLocality
func convertProtoDeviceLocality(in *proto.DeviceLocality) *DeviceLocality {
	if in == nil {
		return nil
	}

	return &DeviceLocality{
		PciBusID: in.GetPciBusId(),
	}
}

// convertProtoContainerReservation is used to convert between a proto and struct
// ContainerReservation
func convertProtoContainerReservation(in *proto.ContainerReservation) *ContainerReservation {
	if in == nil {
		return nil
	}

	return &ContainerReservation{
		Envs:    in.GetEnvs(),
		Mounts:  convertProtoMounts(in.GetMounts()),
		Devices: convertProtoDeviceSpecs(in.GetDevices()),
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
		TaskPath: in.GetTaskPath(),
		HostPath: in.GetHostPath(),
		ReadOnly: in.GetReadOnly(),
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
		TaskPath:    in.GetTaskPath(),
		HostPath:    in.GetHostPath(),
		CgroupPerms: in.GetPermissions(),
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
		Attributes: in.Attributes,
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
