// Copyright (c) 2019, NVIDIA CORPORATION. All rights reserved.

package nvml

import (
	"unsafe"
)

// #include "nvml.h"
import "C"

// Enable or disable MIG mode
const (
	DEVICE_MIG_DISABLE = C.NVML_DEVICE_MIG_DISABLE
	DEVICE_MIG_ENABLE  = C.NVML_DEVICE_MIG_ENABLE
)

// GPU Instance Profiles
const (
	GPU_INSTANCE_PROFILE_1_SLICE = C.NVML_GPU_INSTANCE_PROFILE_1_SLICE
	GPU_INSTANCE_PROFILE_2_SLICE = C.NVML_GPU_INSTANCE_PROFILE_2_SLICE
	GPU_INSTANCE_PROFILE_3_SLICE = C.NVML_GPU_INSTANCE_PROFILE_3_SLICE
	GPU_INSTANCE_PROFILE_4_SLICE = C.NVML_GPU_INSTANCE_PROFILE_4_SLICE
	GPU_INSTANCE_PROFILE_7_SLICE = C.NVML_GPU_INSTANCE_PROFILE_7_SLICE
	GPU_INSTANCE_PROFILE_COUNT   = C.NVML_GPU_INSTANCE_PROFILE_COUNT
)

// Compute Instance Profiles
const (
	COMPUTE_INSTANCE_PROFILE_1_SLICE = C.NVML_COMPUTE_INSTANCE_PROFILE_1_SLICE
	COMPUTE_INSTANCE_PROFILE_2_SLICE = C.NVML_COMPUTE_INSTANCE_PROFILE_2_SLICE
	COMPUTE_INSTANCE_PROFILE_3_SLICE = C.NVML_COMPUTE_INSTANCE_PROFILE_3_SLICE
	COMPUTE_INSTANCE_PROFILE_4_SLICE = C.NVML_COMPUTE_INSTANCE_PROFILE_4_SLICE
	COMPUTE_INSTANCE_PROFILE_7_SLICE = C.NVML_COMPUTE_INSTANCE_PROFILE_7_SLICE
	COMPUTE_INSTANCE_PROFILE_COUNT   = C.NVML_COMPUTE_INSTANCE_PROFILE_COUNT
)

// Compute Instance Engine Profiles
const (
	COMPUTE_INSTANCE_ENGINE_PROFILE_SHARED = C.NVML_COMPUTE_INSTANCE_ENGINE_PROFILE_SHARED
	COMPUTE_INSTANCE_ENGINE_PROFILE_COUNT  = C.NVML_COMPUTE_INSTANCE_ENGINE_PROFILE_COUNT
)

// Opaque GPUInstance type
type GPUInstance struct {
	handle C.nvmlGpuInstance_t
	device *Device
}

// type GPUInstancePlacement C.nvmlGpuInstancePlacement_t
// Generated using `go tool cgo -godefs mig.go`
type GPUInstancePlacement struct {
	Start uint32
	Size  uint32
}

// type GPUInstanceProfileInfo C.nvmlGpuInstanceProfileInfo_t
// Generated using `go tool cgo -godefs mig.go`
type GPUInstanceProfileInfo struct {
	ID                  uint32
	IsP2pSupported      uint32
	SliceCount          uint32
	InstanceCount       uint32
	MultiprocessorCount uint32
	CopyEngineCount     uint32
	DecoderCount        uint32
	EncoderCount        uint32
	JpegCount           uint32
	OfaCount            uint32
	MemorySizeMB        uint64
}

// type GPUInstanceInfo_t C.nvmlGpuInstanceInfo_t
// Generated using `go tool cgo -godefs mig.go`
type GPUInstanceInfo struct {
	Device    *Device
	ID        uint32
	ProfileID uint32
	Placement GPUInstancePlacement
}

// Opaque ComputeInstance type
type ComputeInstance struct {
	handle      C.nvmlComputeInstance_t
	gpuInstance GPUInstance
}

// type ComputeInstanceProfileInfo C.nvmlComputeInstanceProfileInfo_t
// Generated using `go tool cgo -godefs mig.go`
type ComputeInstanceProfileInfo struct {
	ID                    uint32
	SliceCount            uint32
	InstanceCount         uint32
	MultiprocessorCount   uint32
	SharedCopyEngineCount uint32
	SharedDecoderCount    uint32
	SharedEncoderCount    uint32
	SharedJpegCount       uint32
	SharedOfaCount        uint32
}

// type ComputeInstanceInfo C.nvmlComputeInstanceInfo_t
// Generated using `go tool cgo -godefs mig.go`
type ComputeInstanceInfo struct {
	Device      *Device
	GPUInstance GPUInstance
	ID          uint32
	ProfileID   uint32
}

// type DeviceAttributes C.nvmlDeviceAttributes_t
// Generated using `go tool cgo -godefs mig.go`
type DeviceAttributes struct {
	MultiprocessorCount       uint32
	SharedCopyEngineCount     uint32
	SharedDecoderCount        uint32
	SharedEncoderCount        uint32
	SharedJpegCount           uint32
	SharedOfaCount            uint32
	GpuInstanceSliceCount     uint32
	ComputeInstanceSliceCount uint32
	MemorySizeMB              uint64
}

// Device.SetMigMode()
func (d *Device) SetMigMode(mode int) (activationStatus error, err error) {
	ret := dl.lookupSymbol("nvmlDeviceSetMigMode")
	if ret != C.NVML_SUCCESS {
		return nil, errorString(ret)
	}

	var as C.nvmlReturn_t
	ret = C.nvmlDeviceSetMigMode(d.handle.dev, C.uint(mode), &as)
	return errorString(as), errorString(ret)
}

// Device.GetMigMode()
func (d *Device) GetMigMode() (currentMode, pendingMode int, err error) {
	ret := dl.lookupSymbol("nvmlDeviceGetMigMode")
	if ret != C.NVML_SUCCESS {
		return 0, 0, errorString(ret)
	}

	var cm, pm C.uint
	ret = C.nvmlDeviceGetMigMode(d.handle.dev, &cm, &pm)
	return int(cm), int(pm), errorString(ret)
}

// Device.GetGPUInstanceProfileInfo()
func (d *Device) GetGPUInstanceProfileInfo(profile int) (profileInfo GPUInstanceProfileInfo, err error) {
	ret := dl.lookupSymbol("nvmlDeviceGetGpuInstanceProfileInfo")
	if ret != C.NVML_SUCCESS {
		return GPUInstanceProfileInfo{}, errorString(ret)
	}

	var pi C.nvmlGpuInstanceProfileInfo_t
	ret = C.nvmlDeviceGetGpuInstanceProfileInfo(d.handle.dev, C.uint(profile), &pi)
	return *(*GPUInstanceProfileInfo)(unsafe.Pointer(&pi)), errorString(ret)
}

// Device.GetGPUInstancePossiblePlacements()
func (d *Device) GetGPUInstancePossiblePlacements(profileInfo *GPUInstanceProfileInfo) (placement GPUInstancePlacement, count int, err error) {
	ret := dl.lookupSymbol("nvmlDeviceGetGpuInstancePossiblePlacements")
	if ret != C.NVML_SUCCESS {
		return GPUInstancePlacement{}, 0, errorString(ret)
	}

	var pi C.nvmlGpuInstancePlacement_t
	var c C.uint
	ret = C.nvmlDeviceGetGpuInstancePossiblePlacements(d.handle.dev, C.uint(profileInfo.ID), &pi, &c)
	return *(*GPUInstancePlacement)(unsafe.Pointer(&pi)), int(c), errorString(ret)
}

// Device.GPUInstanceRemainingCapacity()
func (d *Device) GPUInstanceRemainingCapacity(profileInfo *GPUInstanceProfileInfo) (count int, err error) {
	ret := dl.lookupSymbol("nvmlDeviceGetGpuInstanceRemainingCapacity")
	if ret != C.NVML_SUCCESS {
		return 0, errorString(ret)
	}

	var c C.uint
	ret = C.nvmlDeviceGetGpuInstanceRemainingCapacity(d.handle.dev, C.uint(profileInfo.ID), &c)
	return int(c), errorString(ret)
}

// Device.CreateGPUInstance()
func (d *Device) CreateGPUInstance(profileInfo *GPUInstanceProfileInfo) (gpuInstance GPUInstance, err error) {
	ret := dl.lookupSymbol("nvmlDeviceCreateGpuInstance")
	if ret != C.NVML_SUCCESS {
		return GPUInstance{}, errorString(ret)
	}

	var gi C.nvmlGpuInstance_t
	ret = C.nvmlDeviceCreateGpuInstance(d.handle.dev, C.uint(profileInfo.ID), &gi)
	return GPUInstance{gi, d}, errorString(ret)
}

// GPUInstance.Destroy()
func (g *GPUInstance) Destroy() (err error) {
	ret := dl.lookupSymbol("nvmlGpuInstanceDestroy")
	if ret != C.NVML_SUCCESS {
		return errorString(ret)
	}

	ret = C.nvmlGpuInstanceDestroy(g.handle)
	return errorString(ret)
}

// Device.GetGPUInstances()
func (d *Device) GetGPUInstances(profileInfo *GPUInstanceProfileInfo) (gpuInstances []GPUInstance, err error) {
	ret := dl.lookupSymbol("nvmlDeviceGetGpuInstances")
	if ret != C.NVML_SUCCESS {
		return nil, errorString(ret)
	}

	gis := make([]C.nvmlGpuInstance_t, profileInfo.InstanceCount)
	var c C.uint
	ret = C.nvmlDeviceGetGpuInstances(d.handle.dev, C.uint(profileInfo.ID), &gis[0], &c)
	for i := 0; i < int(c); i++ {
		gpuInstances = append(gpuInstances, GPUInstance{gis[i], d})
	}
	return gpuInstances, errorString(ret)
}

// Device.GetGPUInstanceByID()
func (d *Device) GetGPUInstanceByID(id int) (gpuInstance GPUInstance, err error) {
	ret := dl.lookupSymbol("nvmlDeviceGetGpuInstanceById")
	if ret != C.NVML_SUCCESS {
		return GPUInstance{}, errorString(ret)
	}

	var gi C.nvmlGpuInstance_t
	ret = C.nvmlDeviceGetGpuInstanceById(d.handle.dev, C.uint(id), &gi)
	return GPUInstance{gi, d}, errorString(ret)
}

// GPUInstance.GetInfo()
func (g *GPUInstance) GetInfo() (info GPUInstanceInfo, err error) {
	ret := dl.lookupSymbol("nvmlGpuInstanceGetInfo")
	if ret != C.NVML_SUCCESS {
		return GPUInstanceInfo{}, errorString(ret)
	}

	var gii C.nvmlGpuInstanceInfo_t
	ret = C.nvmlGpuInstanceGetInfo(g.handle, &gii)
	info = *(*GPUInstanceInfo)(unsafe.Pointer(&gii))
	info.Device = g.device
	return info, errorString(ret)
}

// GPUInstance.GetComputeInstanceProfileInfo()
func (g *GPUInstance) GetComputeInstanceProfileInfo(profile int, engProfile int) (profileInfo ComputeInstanceProfileInfo, err error) {
	ret := dl.lookupSymbol("nvmlGpuInstanceGetComputeInstanceProfileInfo")
	if ret != C.NVML_SUCCESS {
		return ComputeInstanceProfileInfo{}, errorString(ret)
	}

	var pi C.nvmlComputeInstanceProfileInfo_t
	ret = C.nvmlGpuInstanceGetComputeInstanceProfileInfo(g.handle, C.uint(profile), C.uint(engProfile), &pi)
	return *(*ComputeInstanceProfileInfo)(unsafe.Pointer(&pi)), errorString(ret)
}

// GPUInstance.ComputeInstanceRemainingCapacity()
func (g *GPUInstance) ComputeInstanceRemainingCapacity(profileInfo *GPUInstanceProfileInfo) (count int, err error) {
	ret := dl.lookupSymbol("nvmlGpuInstanceGetComputeInstanceRemainingCapacity")
	if ret != C.NVML_SUCCESS {
		return 0, errorString(ret)
	}

	var c C.uint
	ret = C.nvmlGpuInstanceGetComputeInstanceRemainingCapacity(g.handle, C.uint(profileInfo.ID), &c)
	return int(c), errorString(ret)
}

// GPUInstance.CreateComputeInstance()
func (g *GPUInstance) CreateComputeInstance(profileInfo *ComputeInstanceProfileInfo) (computeInstance ComputeInstance, err error) {
	ret := dl.lookupSymbol("nvmlGpuInstanceCreateComputeInstance")
	if ret != C.NVML_SUCCESS {
		return ComputeInstance{}, errorString(ret)
	}

	var ci C.nvmlComputeInstance_t
	ret = C.nvmlGpuInstanceCreateComputeInstance(g.handle, C.uint(profileInfo.ID), &ci)
	return ComputeInstance{ci, *g}, errorString(ret)
}

// ComputeInstance.Destroy()
func (c *ComputeInstance) Destroy() (err error) {
	ret := dl.lookupSymbol("nvmlComputeInstanceDestroy")
	if ret != C.NVML_SUCCESS {
		return errorString(ret)
	}

	ret = C.nvmlComputeInstanceDestroy(c.handle)
	return errorString(ret)
}

// GPUInstance.GetComputeInstances()
func (g *GPUInstance) GetComputeInstances(profileInfo *ComputeInstanceProfileInfo) (computeInstances []ComputeInstance, err error) {
	ret := dl.lookupSymbol("nvmlGpuInstanceGetComputeInstances")
	if ret != C.NVML_SUCCESS {
		return nil, errorString(ret)
	}

	cis := make([]C.nvmlComputeInstance_t, profileInfo.InstanceCount)
	var c C.uint
	ret = C.nvmlGpuInstanceGetComputeInstances(g.handle, C.uint(profileInfo.ID), &cis[0], &c)
	for i := 0; i < int(c); i++ {
		computeInstances = append(computeInstances, ComputeInstance{cis[i], *g})
	}
	return computeInstances, errorString(ret)
}

// GPUInstance.GetComputeInstanceByID()
func (g *GPUInstance) GetComputeInstanceByID(id int) (computeInstance ComputeInstance, err error) {
	ret := dl.lookupSymbol("nvmlGpuInstanceGetComputeInstanceById")
	if ret != C.NVML_SUCCESS {
		return ComputeInstance{}, errorString(ret)
	}

	var ci C.nvmlComputeInstance_t
	ret = C.nvmlGpuInstanceGetComputeInstanceById(g.handle, C.uint(id), &ci)
	return ComputeInstance{ci, *g}, errorString(ret)
}

// ComputeInstance.GetInfo()
func (c *ComputeInstance) GetInfo() (info ComputeInstanceInfo, err error) {
	ret := dl.lookupSymbol("nvmlComputeInstanceGetInfo")
	if ret != C.NVML_SUCCESS {
		return ComputeInstanceInfo{}, errorString(ret)
	}

	var cii C.nvmlComputeInstanceInfo_t
	ret = C.nvmlComputeInstanceGetInfo(c.handle, &cii)
	info = *(*ComputeInstanceInfo)(unsafe.Pointer(&cii))
	info.Device = c.gpuInstance.device
	info.GPUInstance = c.gpuInstance
	return info, errorString(ret)
}

// Device.IsMigDeviceHandle()
func (d *Device) IsMigDeviceHandle() (isMigDevice bool, err error) {
	ret := dl.lookupSymbol("nvmlDeviceIsMigDeviceHandle")
	if ret != C.NVML_SUCCESS {
		return false, errorString(ret)
	}

	var is C.uint
	ret = C.nvmlDeviceIsMigDeviceHandle(d.handle.dev, &is)
	return (is != 0), errorString(ret)
}

// Device.GetGPUInstanceId()
func (d *Device) GetGPUInstanceId() (id int, err error) {
	ret := dl.lookupSymbol("nvmlDeviceGetGpuInstanceId")
	if ret != C.NVML_SUCCESS {
		return 0, errorString(ret)
	}

	var gi C.uint
	ret = C.nvmlDeviceGetGpuInstanceId(d.handle.dev, &gi)
	return int(gi), errorString(ret)
}

// Device.GetComputeInstanceId()
func (d *Device) GetComputeInstanceId() (id int, err error) {
	ret := dl.lookupSymbol("nvmlDeviceGetComputeInstanceId")
	if ret != C.NVML_SUCCESS {
		return 0, errorString(ret)
	}

	var ci C.uint
	ret = C.nvmlDeviceGetComputeInstanceId(d.handle.dev, &ci)
	return int(ci), errorString(ret)
}

// Device.GetMaxMigDeviceCount()
func (d *Device) GetMaxMigDeviceCount() (count int, err error) {
	ret := dl.lookupSymbol("nvmlDeviceGetMaxMigDeviceCount")
	if ret != C.NVML_SUCCESS {
		return 0, errorString(ret)
	}

	var c C.uint
	ret = C.nvmlDeviceGetMaxMigDeviceCount(d.handle.dev, &c)
	return int(c), errorString(ret)
}

// Device.GetMigDeviceHandleByIndex()
func (d *Device) GetMigDeviceHandleByIndex(index int) (migDevice *Device, err error) {
	ret := dl.lookupSymbol("nvmlDeviceGetMigDeviceHandleByIndex")
	if ret != C.NVML_SUCCESS {
		return nil, errorString(ret)
	}

	var m C.nvmlDevice_t
	ret = C.nvmlDeviceGetMigDeviceHandleByIndex(d.handle.dev, C.uint(index), &m)
	return &Device{handle: handle{m}}, errorString(ret)
}

// Device.GetMigDeviceHandleByIndex()
func (d *Device) GetDeviceHandleFromMigDeviceHandle() (device *Device, err error) {
	ret := dl.lookupSymbol("nvmlDeviceGetDeviceHandleFromMigDeviceHandle")
	if ret != C.NVML_SUCCESS {
		return nil, errorString(ret)
	}

	var parent C.nvmlDevice_t
	ret = C.nvmlDeviceGetDeviceHandleFromMigDeviceHandle(d.handle.dev, &parent)
	return &Device{handle: handle{parent}}, errorString(ret)
}

// Device.GetAttributes()
func (d *Device) GetAttributes() (attr DeviceAttributes, err error) {
	ret := dl.lookupSymbol("nvmlDeviceGetAttributes")
	if ret != C.NVML_SUCCESS {
		return DeviceAttributes{}, errorString(ret)
	}

	var a C.nvmlDeviceAttributes_t
	ret = C.nvmlDeviceGetAttributes(d.handle.dev, &a)
	return *(*DeviceAttributes)(unsafe.Pointer(&a)), errorString(ret)
}
