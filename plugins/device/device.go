package device

import (
	"context"
	"fmt"
	"time"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/shared/structs"
)

const (
	// DeviceTypeGPU is a canonical device type for a GPU.
	DeviceTypeGPU = "gpu"
)

var (
	// ErrPluginDisabled indicates that the device plugin is disabled
	ErrPluginDisabled = fmt.Errorf("device is not enabled")
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

	// Stats returns a stream of statistics per device collected at the passed
	// interval.
	Stats(ctx context.Context, interval time.Duration) (<-chan *StatsResponse, error)
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
	Attributes map[string]*structs.Attribute
}

// Validate validates that the device group is valid
func (d *DeviceGroup) Validate() error {
	var mErr multierror.Error

	if d.Vendor == "" {
		multierror.Append(&mErr, fmt.Errorf("device vendor must be specified"))
	}
	if d.Type == "" {
		multierror.Append(&mErr, fmt.Errorf("device type must be specified"))
	}
	if d.Name == "" {
		multierror.Append(&mErr, fmt.Errorf("device name must be specified"))
	}

	for i, dev := range d.Devices {
		if dev == nil {
			multierror.Append(&mErr, fmt.Errorf("device %d is nil", i))
			continue
		}

		if err := dev.Validate(); err != nil {
			multierror.Append(&mErr, multierror.Prefix(err, fmt.Sprintf("device %d: ", i)))
		}
	}

	for k, v := range d.Attributes {
		if err := v.Validate(); err != nil {
			multierror.Append(&mErr, fmt.Errorf("device attribute %q invalid: %v", k, err))
		}
	}

	return mErr.ErrorOrNil()

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

// Validate validates that the device is valid
func (d *Device) Validate() error {
	if d.ID == "" {
		return fmt.Errorf("device ID must be specified")
	}

	return nil
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

// NewStatsError takes an error and returns a stats response
func NewStatsError(err error) *StatsResponse {
	return &StatsResponse{
		Error: err,
	}
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
	Summary *structs.StatValue

	// Stats contains the verbose statistics for the device.
	Stats *structs.StatObject

	// Timestamp is the time the statistics were collected.
	Timestamp time.Time
}
