// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package winsvc

type ServiceStartType uint32

const (
	StartManual    ServiceStartType = 3
	StartAutomatic ServiceStartType = 2
	StartDisabled  ServiceStartType = 4
)

type WindowsServiceConfiguration struct {
	StartType      ServiceStartType
	DisplayName    string
	Description    string
	BinaryPathName string
}

type WindowsPaths interface {
	// Expand expands the path defined by the template. Supports
	// values for:
	//   - SystemDrive
	//   - SystemRoot
	//   - ProgramData
	//   - ProgramFiles
	Expand(string) (string, error)
}

type WindowsService interface {
	// Name returns the name of the service
	Name() string
	// Configure applies the configuration to the Windows service.
	// NOTE: Full configuration applied so empty values will remove existing values.
	Configure(config WindowsServiceConfiguration) error
	// Start starts the Windows service and waits for the
	// service to be running.
	Start() error
	// Stop requests the service to stop and waits for the
	// service to stop.
	Stop() error
	// Close closes the connection to the Windows service.
	Close() error
	// Delete deletes the Windows service.
	Delete() error
	// IsRunning returns if the service is currently running.
	IsRunning() (bool, error)
	// IsStopped returns if the service is currently stopped.
	IsStopped() (bool, error)
	// EnableEventlog will add or update the Windows Eventlog
	// configuration for the service. It will set supported
	// events as info, warning, and error.
	EnableEventlog() error
	// DisableEventlog will remove the Windows Eventlog configuration
	// for the service.
	DisableEventlog() error
}

type WindowsServiceManager interface {
	// IsServiceRegistered returns if the service is a registered Windows service.
	IsServiceRegistered(name string) (bool, error)
	// GetService opens and returns the named service.
	GetService(name string) (WindowsService, error)
	// CreateService creates a new Windows service.
	CreateService(name, binaryPath string, config WindowsServiceConfiguration) (WindowsService, error)
	// Close closes Windows service manager connection.
	Close() error
}
