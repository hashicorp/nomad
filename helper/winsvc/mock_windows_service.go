// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package winsvc

import (
	"reflect"
	"testing"

	"github.com/shoenig/test/must"
)

func NewMockWindowsServiceManager(t *testing.T) *MockWindowsServiceManager {
	t.Helper()

	m := &MockWindowsServiceManager{t: t}
	return m
}

func NewMockWindowsService(t *testing.T) *MockWindowsService {
	t.Helper()

	m := &MockWindowsService{t: t}
	return m
}

type MockWindowsServiceManager struct {
	services             []*MockWindowsService
	isServiceRegistereds []isServiceRegistered
	getServices          []getService
	createServices       []createService
	t                    *testing.T
}

type isServiceRegistered struct {
	name   string
	result bool
	err    error
}

type getService struct {
	name   string
	result WindowsService
	err    error
}

type createService struct {
	name, binaryPath string
	config           WindowsServiceConfiguration
	result           WindowsService
	err              error
}

func (m *MockWindowsServiceManager) NewMockWindowsService() *MockWindowsService {
	w := NewMockWindowsService(m.t)
	m.services = append(m.services, w)
	return w
}

func (m *MockWindowsServiceManager) ExpectIsServiceRegistered(name string, result bool, err error) {
	m.isServiceRegistereds = append(m.isServiceRegistereds, isServiceRegistered{name, result, err})
}

func (m *MockWindowsServiceManager) ExpectGetService(name string, result WindowsService, err error) {
	m.getServices = append(m.getServices, getService{name, result, err})
}

func (m *MockWindowsServiceManager) ExpectCreateService(name, binaryPath string, config WindowsServiceConfiguration, result WindowsService, err error) {
	m.createServices = append(m.createServices, createService{name, binaryPath, config, result, err})
}

func (m *MockWindowsServiceManager) IsServiceRegistered(name string) (bool, error) {
	m.t.Helper()

	must.SliceNotEmpty(m.t, m.isServiceRegistereds,
		must.Sprint("Unexpected call to IsServiceRegistered"))
	call := m.isServiceRegistereds[0]
	m.isServiceRegistereds = m.isServiceRegistereds[1:]
	must.Eq(m.t, call.name, name,
		must.Sprint("IsServiceRegistered received incorrect argument"))

	return call.result, call.err
}

func (m *MockWindowsServiceManager) GetService(name string) (WindowsService, error) {
	m.t.Helper()

	must.SliceNotEmpty(m.t, m.getServices,
		must.Sprint("Unexpected call to GetService"))
	call := m.getServices[0]
	m.getServices = m.getServices[1:]
	must.Eq(m.t, call.name, name,
		must.Sprint("GetService received incorrect argument"))

	return call.result, call.err
}

func (m *MockWindowsServiceManager) CreateService(name, binaryPath string, config WindowsServiceConfiguration) (WindowsService, error) {
	m.t.Helper()

	must.SliceNotEmpty(m.t, m.createServices,
		must.Sprint("Unexpected call to CreateService"))
	call := m.createServices[0]
	m.createServices = m.createServices[1:]
	must.Eq(m.t, call.name, name,
		must.Sprint("CreateService received incorrect argument"))
	must.StrContains(m.t, binaryPath, call.binaryPath,
		must.Sprint("CreateService received incorrect argument"))

	if !reflect.ValueOf(call.config).IsZero() {
		must.Eq(m.t, call.config, config,
			must.Sprint("CreateService received incorrect argument"))
	}

	return call.result, call.err
}

func (m *MockWindowsServiceManager) Close() error { return nil }

func (m *MockWindowsServiceManager) AssertExpectations() {
	m.t.Helper()
	must.SliceEmpty(m.t, m.isServiceRegistereds,
		must.Sprintf("IsServiceRegistered expecting %d more invocations", len(m.isServiceRegistereds)))
	must.SliceEmpty(m.t, m.getServices,
		must.Sprintf("GetService expecting %d more invocations", len(m.getServices)))
	must.SliceEmpty(m.t, m.createServices,
		must.Sprintf("CreateService expecting %d more invocations", len(m.createServices)))

	for _, srv := range m.services {
		srv.AssertExpectations()
	}
}

type MockWindowsService struct {
	names            []string
	configures       []configure
	starts           []error
	stops            []error
	deletes          []error
	isRunnings       []iscall
	isStoppeds       []iscall
	enableEventlogs  []error
	disableEventlogs []error

	t *testing.T
}

type configure struct {
	config WindowsServiceConfiguration
	err    error
}

type iscall struct {
	result bool
	err    error
}

func (m *MockWindowsService) ExpectName(result string) {
	m.names = append(m.names, result)
}

func (m *MockWindowsService) ExpectConfigure(config WindowsServiceConfiguration, err error) {
	m.configures = append(m.configures, configure{config, err})
}

func (m *MockWindowsService) ExpectStart(err error) {
	m.starts = append(m.starts, err)
}

func (m *MockWindowsService) ExpectStop(err error) {
	m.stops = append(m.stops, err)
}

func (m *MockWindowsService) ExpectDelete(err error) {
	m.deletes = append(m.deletes, err)
}

func (m *MockWindowsService) ExpectIsRunning(result bool, err error) {
	m.isRunnings = append(m.isRunnings, iscall{result, err})
}

func (m *MockWindowsService) ExpectIsStopped(result bool, err error) {
	m.isStoppeds = append(m.isStoppeds, iscall{result, err})
}

func (m *MockWindowsService) ExpectEnableEventlog(err error) {
	m.enableEventlogs = append(m.enableEventlogs, err)
}

func (m *MockWindowsService) ExpectDisableEventlog(err error) {
	m.disableEventlogs = append(m.disableEventlogs, err)
}

func (m *MockWindowsService) Name() string {
	m.t.Helper()

	must.SliceNotEmpty(m.t, m.names,
		must.Sprint("Unexpected call to Name"))
	name := m.names[0]
	m.names = m.names[1:]

	return name
}

func (m *MockWindowsService) Configure(config WindowsServiceConfiguration) error {
	m.t.Helper()

	must.SliceNotEmpty(m.t, m.configures,
		must.Sprint("Unexpected call to Configure"))
	call := m.configures[0]
	m.configures = m.configures[1:]
	if !reflect.ValueOf(call.config).IsZero() {
		must.Eq(m.t, call.config, config,
			must.Sprint("Configure received incorrect argument"))
	}

	return call.err
}

func (m *MockWindowsService) Start() error {
	m.t.Helper()

	must.SliceNotEmpty(m.t, m.starts,
		must.Sprint("Unexpected call to Start"))
	err := m.starts[0]
	m.starts = m.starts[1:]

	return err
}

func (m *MockWindowsService) Stop() error {
	m.t.Helper()

	must.SliceNotEmpty(m.t, m.stops,
		must.Sprint("Unexpected call to Stop"))
	err := m.stops[0]
	m.stops = m.stops[1:]

	return err
}

func (m *MockWindowsService) Delete() error {
	m.t.Helper()

	must.SliceNotEmpty(m.t, m.deletes,
		must.Sprint("Unexpected call to Delete"))
	err := m.deletes[0]
	m.deletes = m.deletes[1:]

	return err
}

func (m *MockWindowsService) IsRunning() (bool, error) {
	m.t.Helper()

	must.SliceNotEmpty(m.t, m.isRunnings,
		must.Sprint("Unexpected call to IsRunning"))
	call := m.isRunnings[0]
	m.isRunnings = m.isRunnings[1:]

	return call.result, call.err
}

func (m *MockWindowsService) IsStopped() (bool, error) {
	m.t.Helper()

	must.SliceNotEmpty(m.t, m.isStoppeds,
		must.Sprint("Unexpected call to IsStopped"))
	call := m.isStoppeds[0]
	m.isStoppeds = m.isStoppeds[1:]

	return call.result, call.err
}

func (m *MockWindowsService) EnableEventlog() error {
	m.t.Helper()

	must.SliceNotEmpty(m.t, m.enableEventlogs,
		must.Sprint("Unexpected call to EnableEventlog"))
	err := m.enableEventlogs[0]
	m.enableEventlogs = m.enableEventlogs[1:]

	return err
}

func (m *MockWindowsService) DisableEventlog() error {
	m.t.Helper()

	must.SliceNotEmpty(m.t, m.disableEventlogs,
		must.Sprint("Unexpected call to DisableEventlog"))
	err := m.disableEventlogs[0]
	m.disableEventlogs = m.disableEventlogs[1:]

	return err
}

func (m *MockWindowsService) Close() error { return nil }

func (m *MockWindowsService) AssertExpectations() {
	m.t.Helper()

	must.SliceEmpty(m.t, m.names,
		must.Sprintf("Name expecting %d more invocations", len(m.names)))
	must.SliceEmpty(m.t, m.configures,
		must.Sprintf("Configure expecting %d more invocations", len(m.configures)))
	must.SliceEmpty(m.t, m.starts,
		must.Sprintf("Start expecting %d more invocations", len(m.starts)))
	must.SliceEmpty(m.t, m.stops,
		must.Sprintf("Stop expecting %d more invocations", len(m.stops)))
	must.SliceEmpty(m.t, m.deletes,
		must.Sprintf("Delete expecting %d more invocations", len(m.deletes)))
	must.SliceEmpty(m.t, m.isRunnings,
		must.Sprintf("IsRunning expecting %d more invocations", len(m.isRunnings)))
	must.SliceEmpty(m.t, m.isStoppeds,
		must.Sprintf("IsStopped expecting %d more invocations", len(m.isStoppeds)))
	must.SliceEmpty(m.t, m.enableEventlogs,
		must.Sprintf("EnableEventlog expecting %d more invocations", len(m.enableEventlogs)))
	must.SliceEmpty(m.t, m.disableEventlogs,
		must.Sprintf("DisableEventlog expecting %d more invocations", len(m.disableEventlogs)))
}
