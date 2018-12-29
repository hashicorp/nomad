package nvml

import (
	"errors"
	"testing"

	"github.com/hashicorp/nomad/helper"
	"github.com/stretchr/testify/require"
)

type MockNVMLDriver struct {
	systemDriverCallSuccessful               bool
	deviceCountCallSuccessful                bool
	deviceInfoByIndexCallSuccessful          bool
	deviceInfoAndStatusByIndexCallSuccessful bool
	driverVersion                            string
	devices                                  []*DeviceInfo
	deviceStatus                             []*DeviceStatus
}

func (m *MockNVMLDriver) Initialize() error {
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

func (m *MockNVMLDriver) DeviceInfoByIndex(index uint) (*DeviceInfo, error) {
	if index >= uint(len(m.devices)) {
		return nil, errors.New("index is out of range")
	}
	if !m.deviceInfoByIndexCallSuccessful {
		return nil, errors.New("failed to get device info by index")
	}
	return m.devices[index], nil
}

func (m *MockNVMLDriver) DeviceInfoAndStatusByIndex(index uint) (*DeviceInfo, *DeviceStatus, error) {
	if index >= uint(len(m.devices)) || index >= uint(len(m.deviceStatus)) {
		return nil, nil, errors.New("index is out of range")
	}
	if !m.deviceInfoAndStatusByIndexCallSuccessful {
		return nil, nil, errors.New("failed to get device info and status by index")
	}
	return m.devices[index], m.deviceStatus[index], nil
}

func TestGetFingerprintDataFromNVML(t *testing.T) {
	for _, testCase := range []struct {
		Name                string
		DriverConfiguration *MockNVMLDriver
		ExpectedError       bool
		ExpectedResult      *FingerprintData
	}{
		{
			Name:           "fail on systemDriverCallSuccessful",
			ExpectedError:  true,
			ExpectedResult: nil,
			DriverConfiguration: &MockNVMLDriver{
				systemDriverCallSuccessful:      false,
				deviceCountCallSuccessful:       true,
				deviceInfoByIndexCallSuccessful: true,
			},
		},
		{
			Name:           "fail on deviceCountCallSuccessful",
			ExpectedError:  true,
			ExpectedResult: nil,
			DriverConfiguration: &MockNVMLDriver{
				systemDriverCallSuccessful:      true,
				deviceCountCallSuccessful:       false,
				deviceInfoByIndexCallSuccessful: true,
			},
		},
		{
			Name:           "fail on deviceInfoByIndexCall",
			ExpectedError:  true,
			ExpectedResult: nil,
			DriverConfiguration: &MockNVMLDriver{
				systemDriverCallSuccessful:      true,
				deviceCountCallSuccessful:       true,
				deviceInfoByIndexCallSuccessful: false,
				devices: []*DeviceInfo{
					{
						UUID:               "UUID1",
						Name:               helper.StringToPtr("ModelName1"),
						MemoryMiB:          helper.Uint64ToPtr(16),
						PCIBusID:           "busId",
						PowerW:             helper.UintToPtr(100),
						BAR1MiB:            helper.Uint64ToPtr(100),
						PCIBandwidthMBPerS: helper.UintToPtr(100),
						CoresClockMHz:      helper.UintToPtr(100),
						MemoryClockMHz:     helper.UintToPtr(100),
					}, {
						UUID:               "UUID2",
						Name:               helper.StringToPtr("ModelName2"),
						MemoryMiB:          helper.Uint64ToPtr(8),
						PCIBusID:           "busId",
						PowerW:             helper.UintToPtr(100),
						BAR1MiB:            helper.Uint64ToPtr(100),
						PCIBandwidthMBPerS: helper.UintToPtr(100),
						CoresClockMHz:      helper.UintToPtr(100),
						MemoryClockMHz:     helper.UintToPtr(100),
					},
				},
			},
		},
		{
			Name:          "successful outcome",
			ExpectedError: false,
			ExpectedResult: &FingerprintData{
				DriverVersion: "driverVersion",
				Devices: []*FingerprintDeviceData{
					{
						DeviceData: &DeviceData{
							DeviceName: helper.StringToPtr("ModelName1"),
							UUID:       "UUID1",
							MemoryMiB:  helper.Uint64ToPtr(16),
							PowerW:     helper.UintToPtr(100),
							BAR1MiB:    helper.Uint64ToPtr(100),
						},
						PCIBusID:           "busId1",
						PCIBandwidthMBPerS: helper.UintToPtr(100),
						CoresClockMHz:      helper.UintToPtr(100),
						MemoryClockMHz:     helper.UintToPtr(100),
						DisplayState:       "Enabled",
						PersistenceMode:    "Enabled",
					}, {
						DeviceData: &DeviceData{
							DeviceName: helper.StringToPtr("ModelName2"),
							UUID:       "UUID2",
							MemoryMiB:  helper.Uint64ToPtr(8),
							PowerW:     helper.UintToPtr(200),
							BAR1MiB:    helper.Uint64ToPtr(200),
						},
						PCIBusID:           "busId2",
						PCIBandwidthMBPerS: helper.UintToPtr(200),
						CoresClockMHz:      helper.UintToPtr(200),
						MemoryClockMHz:     helper.UintToPtr(200),
						DisplayState:       "Enabled",
						PersistenceMode:    "Enabled",
					},
				},
			},
			DriverConfiguration: &MockNVMLDriver{
				systemDriverCallSuccessful:      true,
				deviceCountCallSuccessful:       true,
				deviceInfoByIndexCallSuccessful: true,
				driverVersion:                   "driverVersion",
				devices: []*DeviceInfo{
					{
						UUID:               "UUID1",
						Name:               helper.StringToPtr("ModelName1"),
						MemoryMiB:          helper.Uint64ToPtr(16),
						PCIBusID:           "busId1",
						PowerW:             helper.UintToPtr(100),
						BAR1MiB:            helper.Uint64ToPtr(100),
						PCIBandwidthMBPerS: helper.UintToPtr(100),
						CoresClockMHz:      helper.UintToPtr(100),
						MemoryClockMHz:     helper.UintToPtr(100),
						DisplayState:       "Enabled",
						PersistenceMode:    "Enabled",
					}, {
						UUID:               "UUID2",
						Name:               helper.StringToPtr("ModelName2"),
						MemoryMiB:          helper.Uint64ToPtr(8),
						PCIBusID:           "busId2",
						PowerW:             helper.UintToPtr(200),
						BAR1MiB:            helper.Uint64ToPtr(200),
						PCIBandwidthMBPerS: helper.UintToPtr(200),
						CoresClockMHz:      helper.UintToPtr(200),
						MemoryClockMHz:     helper.UintToPtr(200),
						DisplayState:       "Enabled",
						PersistenceMode:    "Enabled",
					},
				},
			},
		},
	} {
		cli := nvmlClient{driver: testCase.DriverConfiguration}
		fingerprintData, err := cli.GetFingerprintData()
		if testCase.ExpectedError && err == nil {
			t.Errorf("case '%s' : expected Error, but didn't get one", testCase.Name)
		}
		if !testCase.ExpectedError && err != nil {
			t.Errorf("case '%s' : unexpected Error '%v'", testCase.Name, err)
		}
		require.New(t).Equal(testCase.ExpectedResult, fingerprintData)
	}
}

func TestGetStatsDataFromNVML(t *testing.T) {
	for _, testCase := range []struct {
		Name                string
		DriverConfiguration *MockNVMLDriver
		ExpectedError       bool
		ExpectedResult      []*StatsData
	}{
		{
			Name:           "fail on deviceCountCallSuccessful",
			ExpectedError:  true,
			ExpectedResult: nil,
			DriverConfiguration: &MockNVMLDriver{
				systemDriverCallSuccessful:               true,
				deviceCountCallSuccessful:                false,
				deviceInfoByIndexCallSuccessful:          true,
				deviceInfoAndStatusByIndexCallSuccessful: true,
			},
		},
		{
			Name:           "fail on DeviceInfoAndStatusByIndex call",
			ExpectedError:  true,
			ExpectedResult: nil,
			DriverConfiguration: &MockNVMLDriver{
				systemDriverCallSuccessful:               true,
				deviceCountCallSuccessful:                true,
				deviceInfoAndStatusByIndexCallSuccessful: false,
				devices: []*DeviceInfo{
					{
						UUID:               "UUID1",
						Name:               helper.StringToPtr("ModelName1"),
						MemoryMiB:          helper.Uint64ToPtr(16),
						PCIBusID:           "busId1",
						PowerW:             helper.UintToPtr(100),
						BAR1MiB:            helper.Uint64ToPtr(100),
						PCIBandwidthMBPerS: helper.UintToPtr(100),
						CoresClockMHz:      helper.UintToPtr(100),
						MemoryClockMHz:     helper.UintToPtr(100),
					}, {
						UUID:               "UUID2",
						Name:               helper.StringToPtr("ModelName2"),
						MemoryMiB:          helper.Uint64ToPtr(8),
						PCIBusID:           "busId2",
						PowerW:             helper.UintToPtr(200),
						BAR1MiB:            helper.Uint64ToPtr(200),
						PCIBandwidthMBPerS: helper.UintToPtr(200),
						CoresClockMHz:      helper.UintToPtr(200),
						MemoryClockMHz:     helper.UintToPtr(200),
					},
				},
				deviceStatus: []*DeviceStatus{
					{
						TemperatureC:       helper.UintToPtr(1),
						GPUUtilization:     helper.UintToPtr(1),
						MemoryUtilization:  helper.UintToPtr(1),
						EncoderUtilization: helper.UintToPtr(1),
						DecoderUtilization: helper.UintToPtr(1),
						UsedMemoryMiB:      helper.Uint64ToPtr(1),
						ECCErrorsL1Cache:   helper.Uint64ToPtr(1),
						ECCErrorsL2Cache:   helper.Uint64ToPtr(1),
						ECCErrorsDevice:    helper.Uint64ToPtr(1),
						PowerUsageW:        helper.UintToPtr(1),
						BAR1UsedMiB:        helper.Uint64ToPtr(1),
					},
					{
						TemperatureC:       helper.UintToPtr(2),
						GPUUtilization:     helper.UintToPtr(2),
						MemoryUtilization:  helper.UintToPtr(2),
						EncoderUtilization: helper.UintToPtr(2),
						DecoderUtilization: helper.UintToPtr(2),
						UsedMemoryMiB:      helper.Uint64ToPtr(2),
						ECCErrorsL1Cache:   helper.Uint64ToPtr(2),
						ECCErrorsL2Cache:   helper.Uint64ToPtr(2),
						ECCErrorsDevice:    helper.Uint64ToPtr(2),
						PowerUsageW:        helper.UintToPtr(2),
						BAR1UsedMiB:        helper.Uint64ToPtr(2),
					},
				},
			},
		},
		{
			Name:          "successful outcome",
			ExpectedError: false,
			ExpectedResult: []*StatsData{
				{
					DeviceData: &DeviceData{
						DeviceName: helper.StringToPtr("ModelName1"),
						UUID:       "UUID1",
						MemoryMiB:  helper.Uint64ToPtr(16),
						PowerW:     helper.UintToPtr(100),
						BAR1MiB:    helper.Uint64ToPtr(100),
					},
					TemperatureC:       helper.UintToPtr(1),
					GPUUtilization:     helper.UintToPtr(1),
					MemoryUtilization:  helper.UintToPtr(1),
					EncoderUtilization: helper.UintToPtr(1),
					DecoderUtilization: helper.UintToPtr(1),
					UsedMemoryMiB:      helper.Uint64ToPtr(1),
					ECCErrorsL1Cache:   helper.Uint64ToPtr(1),
					ECCErrorsL2Cache:   helper.Uint64ToPtr(1),
					ECCErrorsDevice:    helper.Uint64ToPtr(1),
					PowerUsageW:        helper.UintToPtr(1),
					BAR1UsedMiB:        helper.Uint64ToPtr(1),
				},
				{
					DeviceData: &DeviceData{
						DeviceName: helper.StringToPtr("ModelName2"),
						UUID:       "UUID2",
						MemoryMiB:  helper.Uint64ToPtr(8),
						PowerW:     helper.UintToPtr(200),
						BAR1MiB:    helper.Uint64ToPtr(200),
					},
					TemperatureC:       helper.UintToPtr(2),
					GPUUtilization:     helper.UintToPtr(2),
					MemoryUtilization:  helper.UintToPtr(2),
					EncoderUtilization: helper.UintToPtr(2),
					DecoderUtilization: helper.UintToPtr(2),
					UsedMemoryMiB:      helper.Uint64ToPtr(2),
					ECCErrorsL1Cache:   helper.Uint64ToPtr(2),
					ECCErrorsL2Cache:   helper.Uint64ToPtr(2),
					ECCErrorsDevice:    helper.Uint64ToPtr(2),
					PowerUsageW:        helper.UintToPtr(2),
					BAR1UsedMiB:        helper.Uint64ToPtr(2),
				},
			},
			DriverConfiguration: &MockNVMLDriver{
				deviceCountCallSuccessful:                true,
				deviceInfoByIndexCallSuccessful:          true,
				deviceInfoAndStatusByIndexCallSuccessful: true,
				devices: []*DeviceInfo{
					{
						UUID:               "UUID1",
						Name:               helper.StringToPtr("ModelName1"),
						MemoryMiB:          helper.Uint64ToPtr(16),
						PCIBusID:           "busId1",
						PowerW:             helper.UintToPtr(100),
						BAR1MiB:            helper.Uint64ToPtr(100),
						PCIBandwidthMBPerS: helper.UintToPtr(100),
						CoresClockMHz:      helper.UintToPtr(100),
						MemoryClockMHz:     helper.UintToPtr(100),
					}, {
						UUID:               "UUID2",
						Name:               helper.StringToPtr("ModelName2"),
						MemoryMiB:          helper.Uint64ToPtr(8),
						PCIBusID:           "busId2",
						PowerW:             helper.UintToPtr(200),
						BAR1MiB:            helper.Uint64ToPtr(200),
						PCIBandwidthMBPerS: helper.UintToPtr(200),
						CoresClockMHz:      helper.UintToPtr(200),
						MemoryClockMHz:     helper.UintToPtr(200),
					},
				},
				deviceStatus: []*DeviceStatus{
					{
						TemperatureC:       helper.UintToPtr(1),
						GPUUtilization:     helper.UintToPtr(1),
						MemoryUtilization:  helper.UintToPtr(1),
						EncoderUtilization: helper.UintToPtr(1),
						DecoderUtilization: helper.UintToPtr(1),
						UsedMemoryMiB:      helper.Uint64ToPtr(1),
						ECCErrorsL1Cache:   helper.Uint64ToPtr(1),
						ECCErrorsL2Cache:   helper.Uint64ToPtr(1),
						ECCErrorsDevice:    helper.Uint64ToPtr(1),
						PowerUsageW:        helper.UintToPtr(1),
						BAR1UsedMiB:        helper.Uint64ToPtr(1),
					},
					{
						TemperatureC:       helper.UintToPtr(2),
						GPUUtilization:     helper.UintToPtr(2),
						MemoryUtilization:  helper.UintToPtr(2),
						EncoderUtilization: helper.UintToPtr(2),
						DecoderUtilization: helper.UintToPtr(2),
						UsedMemoryMiB:      helper.Uint64ToPtr(2),
						ECCErrorsL1Cache:   helper.Uint64ToPtr(2),
						ECCErrorsL2Cache:   helper.Uint64ToPtr(2),
						ECCErrorsDevice:    helper.Uint64ToPtr(2),
						PowerUsageW:        helper.UintToPtr(2),
						BAR1UsedMiB:        helper.Uint64ToPtr(2),
					},
				},
			},
		},
	} {
		cli := nvmlClient{driver: testCase.DriverConfiguration}
		statsData, err := cli.GetStatsData()
		if testCase.ExpectedError && err == nil {
			t.Errorf("case '%s' : expected Error, but didn't get one", testCase.Name)
		}
		if !testCase.ExpectedError && err != nil {
			t.Errorf("case '%s' : unexpected Error '%v'", testCase.Name, err)
		}
		require.New(t).Equal(testCase.ExpectedResult, statsData)
	}
}
