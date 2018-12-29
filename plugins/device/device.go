package device

import (
	"context"

	"github.com/hashicorp/nomad/plugins/base"
)

const (
	// DeviceTypeGPU is a canonical device type for a GPU.
	DeviceTypeGPU = "gpu"
)

// DevicePlugin is the interface for a plugin that can expose detected devices
// to Nomad and inform it how to mount them.
type DevicePlugin interface {
	base.BasePlugin

	// Fingerprint returns a stream of devices that are detected.
	Fingerprint(ctx context.Context) (<-chan *FingerprintResponse, error)

	// Reserve is used to reserve a set of devices and retrieve mount
	// instructions.
	Reserve(deviceIDs []string) (*ContainerReservation, error)
}

// FingerprintResponse includes a set of detected devices or an error in the
// process of fingerprinting.
type FingerprintResponse struct {
	// Devices is a set of devices that have been detected.
	Devices []*DeviceGroup

	// Error is populated when fingerprinting has failed.
	Error error
}

// DeviceGroup is a grouping of devices that share a common vendor, device type
// and name.
type DeviceGroup struct {
	// Vendor is the vendor providing the device (nvidia, intel, etc).
	Vendor string

	// Type is the type of the device (gpu, fpga, etc).
	Type string

	// Name is the devices model name.
	Name string

	// Devices is the set of device instances.
	Devices []*Device

	// Attributes are a set of attributes shared for all the devices.
	Attributes map[string]string
}

// Device is an instance of a particular device.
type Device struct {
	// ID is the identifier for the device.
	ID string

	// Healthy marks whether the device is healthy and can be used for
	// scheduling.
	Healthy bool

	// HealthDesc describes why the device may be unhealthy.
	HealthDesc string

	// HwLocality captures hardware locality information for the device.
	HwLocality *DeviceLocality
}

// DeviceLocality captures hardware locality information for a device.
type DeviceLocality struct {
	// PciBusID is the PCI bus ID of the device.
	PciBusID string
}

// ContainerReservation describes how to mount a device into a container. A
// container is an isolated environment that shares the host's OS.
type ContainerReservation struct {
	// Envs are a set of environment variables to set for the task.
	Envs map[string]string

	// Mounts are used to mount host volumes into a container that may include
	// libraries, etc.
	Mounts []*Mount

	// Devices are the set of devices to mount into the container.
	Devices []*DeviceSpec
}

// Mount is used to mount a host directory into a container.
type Mount struct {
	// TaskPath is the location in the task's file system to mount.
	TaskPath string

	// HostPath is the host directory path to mount.
	HostPath string

	// ReadOnly defines whether the mount should be read only to the task.
	ReadOnly bool
}

// DeviceSpec captures how to mount a device into a container.
type DeviceSpec struct {
	// TaskPath is the location to mount the device in the task's file system.
	TaskPath string

	// HostPath is the host location of the device.
	HostPath string

	// CgroupPerms defines the permissions to use when mounting the device.
	CgroupPerms string
}
