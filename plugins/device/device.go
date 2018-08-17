package device

import (
	"context"
	"time"

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

	// Stats returns a stream of statistics per device.
	Stats(ctx context.Context) (<-chan *StatsResponse, error)
}

// FingerprintResponse includes a set of detected devices or an error in the
// process of fingerprinting.
type FingerprintResponse struct {
	// Devices is a set of devices that have been detected.
	Devices []*DeviceGroup

	// Error is populated when fingerprinting has failed.
	Error error
}

// NewFingerprint takes a set of device groups and returns a fingerprint
// response
func NewFingerprint(devices ...*DeviceGroup) *FingerprintResponse {
	return &FingerprintResponse{
		Devices: devices,
	}
}

// NewFingerprintError takes an error and returns a fingerprint response
func NewFingerprintError(err error) *FingerprintResponse {
	return &FingerprintResponse{
		Error: err,
	}
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

// StatsResponse returns statistics for each device group.
type StatsResponse struct {
	// Groups contains statistics for each device group.
	Groups []*DeviceGroupStats

	// Error is populated when collecting statistics has failed.
	Error error
}

// DeviceGroupStats contains statistics for each device of a particular
// device group, identified by the vendor, type and name of the device.
type DeviceGroupStats struct {
	Vendor string
	Type   string
	Name   string

	// InstanceStats is a mapping of each device ID to its statistics.
	InstanceStats map[string]*DeviceStats
}

// DeviceStats is the statistics for an individual device
type DeviceStats struct {
	// Summary exposes a single summary metric that should be the most
	// informative to users.
	Summary *StatValue

	// Stats contains the verbose statistics for the device.
	Stats *StatObject

	// Timestamp is the time the statistics were collected.
	Timestamp time.Time
}

// StatObject is a collection of statistics either exposed at the top
// level or via nested StatObjects.
type StatObject struct {
	// Nested is a mapping of object name to a nested stats object.
	Nested map[string]*StatObject

	// Attributes is a mapping of statistic name to its value.
	Attributes map[string]*StatValue
}

// StatValue exposes the values of a particular statistic. The value may be of
// type float, integer, string or boolean. Numeric types can be exposed as a
// single value or as a fraction.
type StatValue struct {
	// FloatNumeratorVal exposes a floating point value. If denominator is set
	// it is assumed to be a fractional value, otherwise it is a scalar.
	FloatNumeratorVal   float64
	FloatDenominatorVal float64

	// IntNumeratorVal exposes a int value. If denominator is set it is assumed
	// to be a fractional value, otherwise it is a scalar.
	IntNumeratorVal   int64
	IntDenominatorVal int64

	// StringVal exposes a string value. These are likely annotations.
	StringVal string

	// BoolVal exposes a boolean statistic.
	BoolVal bool

	// Unit gives the unit type: °F, %, MHz, MB, etc.
	Unit string

	// Desc provides a human readable description of the statistic.
	Desc string
}
