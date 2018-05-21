package fingerprint

import (
	"errors"
	"reflect"
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"
)

// mocking NVML lib

type MockNVMLDevice struct {
	uuidCallSuccessful       bool
	uuid                     string
	nameCallSuccessful       bool
	name                     string
	memoryInfoCallSuccessful bool
	totalMemory              uint64
	allocatedMemory          uint64
}

func (m MockNVMLDevice) UUID() (string, error) {
	if !m.uuidCallSuccessful {
		return "", errors.New("failed to get UUID")
	}
	return m.uuid, nil
}

func (m MockNVMLDevice) Name() (string, error) {
	if !m.nameCallSuccessful {
		return "", errors.New("failed to get Name")
	}
	return m.name, nil
}

func (m MockNVMLDevice) MemoryInfo() (uint64, uint64, error) {
	if !m.memoryInfoCallSuccessful {
		return 0, 0, errors.New("failed to get MemoryInfo")
	}
	return m.totalMemory, m.allocatedMemory, nil
}

type MockNVMLDriver struct {
	initCallSuccessful         bool
	systemDriverCallSuccessful bool
	deviceCountCallSuccessful  bool
	driverVersion              string
	devices                    []MockNVMLDevice
}

func (m *MockNVMLDriver) Initialize() error {
	if !m.initCallSuccessful {
		return errors.New("failed to initialize")
	}
	return nil
}

func (m *MockNVMLDriver) Shutdown() error {
	return nil
}

func (m *MockNVMLDriver) SystemDriverVersion() (string, error) {
	if !m.systemDriverCallSuccessful {
		return "", errors.New("failed to get system driver")
	}
	return m.driverVersion, nil
}

func (m *MockNVMLDriver) DeviceCount() (uint, error) {
	if !m.deviceCountCallSuccessful {
		return 0, errors.New("failed to get device length")
	}
	return uint(len(m.devices)), nil
}

func (m *MockNVMLDriver) DeviceHandleByIndex(index uint) (NVMLDevice, error) {
	if index >= uint(len(m.devices)) {
		return MockNVMLDevice{}, errors.New("index is out of range")
	}
	return m.devices[index], nil
}

func TestGetDataFromNVML(t *testing.T) {
	for _, testCase := range []struct {
		Name                string
		DriverConfiguration *MockNVMLDriver
		ExpectedError       bool
		TerminationExpected bool
		ExpectedResult      []*structs.NvidiaGPUResource
	}{
		{
			Name:                "fail on initialization",
			ExpectedError:       true,
			ExpectedResult:      nil,
			TerminationExpected: false,
			DriverConfiguration: &MockNVMLDriver{
				initCallSuccessful:         false,
				systemDriverCallSuccessful: true,
				deviceCountCallSuccessful:  true,
			},
		},
		{
			Name:                "fail on systemDriverCallSuccessful",
			ExpectedError:       true,
			ExpectedResult:      nil,
			TerminationExpected: true,
			DriverConfiguration: &MockNVMLDriver{
				initCallSuccessful:         true,
				systemDriverCallSuccessful: false,
				deviceCountCallSuccessful:  true,
			},
		},
		{
			Name:                "fail on deviceCountCallSuccessful",
			ExpectedError:       true,
			ExpectedResult:      nil,
			TerminationExpected: true,
			DriverConfiguration: &MockNVMLDriver{
				initCallSuccessful:         true,
				systemDriverCallSuccessful: true,
				deviceCountCallSuccessful:  false,
			},
		},
		{
			Name:          "successful outcome",
			ExpectedError: false,
			ExpectedResult: []*structs.NvidiaGPUResource{
				{
					DriverVersion: "driverVersion",
					ModelName:     "ModelName1",
					UUID:          "UUID1",
					MemoryMiB:     16,
				}, {
					DriverVersion: "driverVersion",
					ModelName:     "ModelName2",
					UUID:          "UUID2",
					MemoryMiB:     8,
				},
			},
			TerminationExpected: false,
			DriverConfiguration: &MockNVMLDriver{
				initCallSuccessful:         true,
				systemDriverCallSuccessful: true,
				deviceCountCallSuccessful:  true,
				driverVersion:              "driverVersion",
				devices: []MockNVMLDevice{
					{
						uuid:                     "UUID1",
						name:                     "ModelName1",
						totalMemory:              16 * 1024 * 1024,
						allocatedMemory:          5 * 1024 * 1024,
						uuidCallSuccessful:       true,
						nameCallSuccessful:       true,
						memoryInfoCallSuccessful: true,
					}, {
						uuid:                     "UUID2",
						name:                     "ModelName2",
						totalMemory:              8 * 1024 * 1024,
						allocatedMemory:          5 * 1024 * 1024,
						uuidCallSuccessful:       true,
						nameCallSuccessful:       true,
						memoryInfoCallSuccessful: true,
					},
				},
			},
		},
		{
			Name:                "device name query failure",
			ExpectedError:       true,
			ExpectedResult:      nil,
			TerminationExpected: true,
			DriverConfiguration: &MockNVMLDriver{
				initCallSuccessful:         true,
				systemDriverCallSuccessful: true,
				deviceCountCallSuccessful:  true,
				driverVersion:              "driverVersion",
				devices: []MockNVMLDevice{
					{
						uuid:                     "UUID1",
						name:                     "ModelName1",
						totalMemory:              16 * 1024 * 1024,
						allocatedMemory:          5 * 1024 * 1024,
						uuidCallSuccessful:       true,
						nameCallSuccessful:       false,
						memoryInfoCallSuccessful: true,
					},
				},
			},
		},
		{
			Name:                "device uuid query failure",
			ExpectedError:       true,
			ExpectedResult:      nil,
			TerminationExpected: true,
			DriverConfiguration: &MockNVMLDriver{
				initCallSuccessful:         true,
				systemDriverCallSuccessful: true,
				deviceCountCallSuccessful:  true,
				driverVersion:              "driverVersion",
				devices: []MockNVMLDevice{
					{
						uuid:                     "UUID1",
						name:                     "ModelName1",
						totalMemory:              16 * 1024 * 1024,
						allocatedMemory:          5 * 1024 * 1024,
						uuidCallSuccessful:       false,
						nameCallSuccessful:       true,
						memoryInfoCallSuccessful: true,
					},
				},
			},
		},
		{
			Name:                "device MemoryInfo query failure",
			ExpectedError:       true,
			ExpectedResult:      nil,
			TerminationExpected: true,
			DriverConfiguration: &MockNVMLDriver{
				initCallSuccessful:         true,
				systemDriverCallSuccessful: true,
				deviceCountCallSuccessful:  true,
				driverVersion:              "driverVersion",
				devices: []MockNVMLDevice{
					{
						uuid:                     "UUID1",
						name:                     "ModelName1",
						totalMemory:              16 * 1024 * 1024,
						allocatedMemory:          5 * 1024 * 1024,
						uuidCallSuccessful:       true,
						nameCallSuccessful:       true,
						memoryInfoCallSuccessful: false,
					},
				},
			},
		},
	} {
		err, shouldTerminate, allNvidiaGPUResources := getDataFromNVML(testCase.DriverConfiguration)
		if testCase.ExpectedError && err == nil {
			t.Errorf("case '%s' : expected Error, but didn't get one", testCase.Name)
		}
		if !testCase.ExpectedError && err != nil {
			t.Errorf("case '%s' : unexpected Error '%v'", testCase.Name, err)
		}
		if testCase.TerminationExpected != shouldTerminate {
			t.Errorf("case '%s' : incorrect termination behavior", testCase.Name)
		}
		if !reflect.DeepEqual(testCase.ExpectedResult, allNvidiaGPUResources) {
			t.Errorf("case '%s' : expected result does not match actual result", testCase.Name)
		}
	}
}
