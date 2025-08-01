// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package winsvc

import (
	"errors"
	"fmt"
	"io/fs"
	"reflect"
	"slices"
	"time"

	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

// Base registry path for eventlog registrations
const EVENTLOG_REGISTRY_PATH = `SYSTEM\CurrentControlSet\Services\EventLog\Application`

// Registry value name for supported event types
const EVENTLOG_SUPPORTED_EVENTS_KEY = "TypesSupported"

// Event types registered as supported
const EVENTLOG_SUPPORTED_EVENTS uint32 = eventlog.Error | eventlog.Warning | eventlog.Info

// NewWindowsServiceManager creates a new instance of the wrapper
// to interact with the Windows service manager.
func NewWindowsServiceManager() (WindowsServiceManager, error) {
	m, err := mgr.Connect()
	if err != nil {
		return nil, err
	}

	return &windowsServiceManager{manager: m}, nil
}

type windowsServiceManager struct {
	manager *mgr.Mgr
}

func (m *windowsServiceManager) IsServiceRegistered(name string) (bool, error) {
	list, err := m.manager.ListServices()
	if err != nil {
		return false, err
	}

	if slices.Contains(list, name) {
		return true, nil
	}

	return false, nil
}

func (m *windowsServiceManager) GetService(name string) (WindowsService, error) {
	service, err := m.manager.OpenService(name)
	if err != nil {
		return nil, err
	}

	return &windowsService{service: service}, nil
}

func (m *windowsServiceManager) CreateService(name, bin string, config WindowsServiceConfiguration) (WindowsService, error) {
	wsvc, err := m.manager.CreateService(name, bin, mgr.Config{})
	if err != nil {
		return nil, err
	}

	service := &windowsService{service: wsvc}

	// Only apply configuration if configuration is provided
	if !reflect.ValueOf(config).IsZero() {
		if err := service.Configure(config); err != nil {
			return nil, err
		}
	}

	return service, nil
}

func (m *windowsServiceManager) Close() error {
	return m.manager.Disconnect()
}

type windowsService struct {
	service *mgr.Service
}

func (s *windowsService) Name() string {
	return s.service.Name
}

func (s *windowsService) Configure(config WindowsServiceConfiguration) error {
	serviceCfg, err := s.service.Config()
	if err != nil {
		return err
	}

	serviceCfg.StartType = uint32(config.StartType)
	serviceCfg.DisplayName = config.DisplayName
	serviceCfg.Description = config.Description
	serviceCfg.BinaryPathName = config.BinaryPathName

	if err := s.service.UpdateConfig(serviceCfg); err != nil {
		return err
	}

	return nil
}

func (s *windowsService) Start() error {
	if running, _ := s.IsRunning(); running {
		return nil
	}

	if err := s.service.Start(); err != nil {
		return err
	}

	if err := s.waitFor(s.IsRunning); err != nil {
		return err
	}

	return nil
}

func (s *windowsService) Stop() error {
	if stopped, _ := s.IsStopped(); stopped {
		return nil
	}

	if _, err := s.service.Control(svc.Stop); err != nil {
		return err
	}

	if err := s.waitFor(s.IsStopped); err != nil {
		return err
	}

	return nil
}

func (s *windowsService) Close() error {
	return s.service.Close()
}

func (s *windowsService) Delete() error {
	return s.service.Delete()
}

func (s *windowsService) IsRunning() (bool, error) {
	return s.isService(svc.Running)
}

func (s *windowsService) IsStopped() (bool, error) {
	return s.isService(svc.Stopped)
}

func (s *windowsService) EnableEventlog() error {
	// Check if the service is already setup in the eventlog
	key, err := registry.OpenKey(registry.LOCAL_MACHINE,
		EVENTLOG_REGISTRY_PATH+`\`+s.Name(),
		registry.ALL_ACCESS,
	)
	defer key.Close()

	// If it could not be opened, assume error is caused
	// due to nonexistence. If it was for some other reason
	// the error will be encountered again when attempting to
	// create.
	if err != nil {
		if err := eventlog.InstallAsEventCreate(s.Name(), EVENTLOG_SUPPORTED_EVENTS); err != nil {
			return err
		}
	} else {
		// Since the service is already registered, just
		// ensure it is properly configured. Currently
		// that just means the supported events.
		val, _, err := key.GetIntegerValue(EVENTLOG_SUPPORTED_EVENTS_KEY)
		if err != nil || uint32(val) != EVENTLOG_SUPPORTED_EVENTS {
			if err := key.SetDWordValue(EVENTLOG_SUPPORTED_EVENTS_KEY, EVENTLOG_SUPPORTED_EVENTS); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *windowsService) DisableEventlog() error {
	// Check if the service is currently enabled in the eventlog
	key, err := registry.OpenKey(registry.LOCAL_MACHINE,
		EVENTLOG_REGISTRY_PATH+`\`+s.Name(),
		registry.READ,
	)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	defer key.Close()

	return eventlog.Remove(s.Name())
}

func (s *windowsService) isService(state svc.State) (bool, error) {
	status, err := s.service.Query()
	if err != nil {
		return false, err
	}

	return status.State == state, nil
}

func (s *windowsService) waitFor(condition func() (bool, error)) error {
	for range WINDOWS_SERVICE_STATE_TIMEOUT * 4 {
		complete, err := condition()
		if err != nil {
			return err
		}
		if complete {
			return nil
		}

		time.Sleep(time.Second / 4)
	}

	return fmt.Errorf("timeout exceeded waiting for process")
}
