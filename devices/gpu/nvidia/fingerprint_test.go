package nvidia

import (
	"context"
	"errors"
	"sort"
	"testing"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/devices/gpu/nvidia/nvml"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/plugins/device"
	"github.com/hashicorp/nomad/plugins/shared/structs"
	"github.com/stretchr/testify/require"
)

func TestIgnoreFingerprintedDevices(t *testing.T) {
	for _, testCase := range []struct {
		Name           string
		DeviceData     []*nvml.FingerprintDeviceData
		IgnoredGPUIds  map[string]struct{}
		ExpectedResult []*nvml.FingerprintDeviceData
	}{
		{
			Name: "Odd ignored",
			DeviceData: []*nvml.FingerprintDeviceData{
				{
					DeviceData: &nvml.DeviceData{
						DeviceName: helper.StringToPtr("DeviceName1"),
						UUID:       "UUID1",
						MemoryMiB:  helper.Uint64ToPtr(1000),
					},
				},
				{
					DeviceData: &nvml.DeviceData{
						DeviceName: helper.StringToPtr("DeviceName2"),
						UUID:       "UUID2",
						MemoryMiB:  helper.Uint64ToPtr(1000),
					},
				},
				{
					DeviceData: &nvml.DeviceData{
						DeviceName: helper.StringToPtr("DeviceName3"),
						UUID:       "UUID3",
						MemoryMiB:  helper.Uint64ToPtr(1000),
					},
				},
			},
			IgnoredGPUIds: map[string]struct{}{
				"UUID2": {},
			},
			ExpectedResult: []*nvml.FingerprintDeviceData{
				{
					DeviceData: &nvml.DeviceData{
						DeviceName: helper.StringToPtr("DeviceName1"),
						UUID:       "UUID1",
						MemoryMiB:  helper.Uint64ToPtr(1000),
					},
				},
				{
					DeviceData: &nvml.DeviceData{
						DeviceName: helper.StringToPtr("DeviceName3"),
						UUID:       "UUID3",
						MemoryMiB:  helper.Uint64ToPtr(1000),
					},
				},
			},
		},
		{
			Name: "Even ignored",
			DeviceData: []*nvml.FingerprintDeviceData{
				{
					DeviceData: &nvml.DeviceData{
						DeviceName: helper.StringToPtr("DeviceName1"),
						UUID:       "UUID1",
						MemoryMiB:  helper.Uint64ToPtr(1000),
					},
				},
				{
					DeviceData: &nvml.DeviceData{
						DeviceName: helper.StringToPtr("DeviceName2"),
						UUID:       "UUID2",
						MemoryMiB:  helper.Uint64ToPtr(1000),
					},
				},
				{
					DeviceData: &nvml.DeviceData{
						DeviceName: helper.StringToPtr("DeviceName3"),
						UUID:       "UUID3",
						MemoryMiB:  helper.Uint64ToPtr(1000),
					},
				},
			},
			IgnoredGPUIds: map[string]struct{}{
				"UUID1": {},
				"UUID3": {},
			},
			ExpectedResult: []*nvml.FingerprintDeviceData{
				{
					DeviceData: &nvml.DeviceData{
						DeviceName: helper.StringToPtr("DeviceName2"),
						UUID:       "UUID2",
						MemoryMiB:  helper.Uint64ToPtr(1000),
					},
				},
			},
		},
		{
			Name: "All ignored",
			DeviceData: []*nvml.FingerprintDeviceData{
				{
					DeviceData: &nvml.DeviceData{
						DeviceName: helper.StringToPtr("DeviceName1"),
						UUID:       "UUID1",
						MemoryMiB:  helper.Uint64ToPtr(1000),
					},
				},
				{
					DeviceData: &nvml.DeviceData{
						DeviceName: helper.StringToPtr("DeviceName2"),
						UUID:       "UUID2",
						MemoryMiB:  helper.Uint64ToPtr(1000),
					},
				},
				{
					DeviceData: &nvml.DeviceData{
						DeviceName: helper.StringToPtr("DeviceName3"),
						UUID:       "UUID3",
						MemoryMiB:  helper.Uint64ToPtr(1000),
					},
				},
			},
			IgnoredGPUIds: map[string]struct{}{
				"UUID1": {},
				"UUID2": {},
				"UUID3": {},
			},
			ExpectedResult: nil,
		},
		{
			Name: "No ignored",
			DeviceData: []*nvml.FingerprintDeviceData{
				{
					DeviceData: &nvml.DeviceData{
						DeviceName: helper.StringToPtr("DeviceName1"),
						UUID:       "UUID1",
						MemoryMiB:  helper.Uint64ToPtr(1000),
					},
				},
				{
					DeviceData: &nvml.DeviceData{
						DeviceName: helper.StringToPtr("DeviceName2"),
						UUID:       "UUID2",
						MemoryMiB:  helper.Uint64ToPtr(1000),
					},
				},
				{
					DeviceData: &nvml.DeviceData{
						DeviceName: helper.StringToPtr("DeviceName3"),
						UUID:       "UUID3",
						MemoryMiB:  helper.Uint64ToPtr(1000),
					},
				},
			},
			IgnoredGPUIds: map[string]struct{}{},
			ExpectedResult: []*nvml.FingerprintDeviceData{
				{
					DeviceData: &nvml.DeviceData{
						DeviceName: helper.StringToPtr("DeviceName1"),
						UUID:       "UUID1",
						MemoryMiB:  helper.Uint64ToPtr(1000),
					},
				},
				{
					DeviceData: &nvml.DeviceData{
						DeviceName: helper.StringToPtr("DeviceName2"),
						UUID:       "UUID2",
						MemoryMiB:  helper.Uint64ToPtr(1000),
					},
				},
				{
					DeviceData: &nvml.DeviceData{
						DeviceName: helper.StringToPtr("DeviceName3"),
						UUID:       "UUID3",
						MemoryMiB:  helper.Uint64ToPtr(1000),
					},
				},
			},
		},
		{
			Name:       "No DeviceData provided",
			DeviceData: nil,
			IgnoredGPUIds: map[string]struct{}{
				"UUID1": {},
				"UUID2": {},
				"UUID3": {},
			},
			ExpectedResult: nil,
		},
	} {
		t.Run(testCase.Name, func(t *testing.T) {
			actualResult := ignoreFingerprintedDevices(testCase.DeviceData, testCase.IgnoredGPUIds)
			require.New(t).Equal(testCase.ExpectedResult, actualResult)
		})
	}
}

func TestCheckFingerprintUpdates(t *testing.T) {
	for _, testCase := range []struct {
		Name                     string
		Device                   *NvidiaDevice
		AllDevices               []*nvml.FingerprintDeviceData
		DeviceMapAfterMethodCall map[string]struct{}
		ExpectedResult           bool
	}{
		{
			Name: "No updates",
			Device: &NvidiaDevice{devices: map[string]struct{}{
				"1": {},
				"2": {},
				"3": {},
			}},
			AllDevices: []*nvml.FingerprintDeviceData{
				{
					DeviceData: &nvml.DeviceData{
						UUID: "1",
					},
				},
				{
					DeviceData: &nvml.DeviceData{
						UUID: "2",
					},
				},
				{
					DeviceData: &nvml.DeviceData{
						UUID: "3",
					},
				},
			},
			ExpectedResult: false,
			DeviceMapAfterMethodCall: map[string]struct{}{
				"1": {},
				"2": {},
				"3": {},
			},
		},
		{
			Name: "New Device Appeared",
			Device: &NvidiaDevice{devices: map[string]struct{}{
				"1": {},
				"2": {},
				"3": {},
			}},
			AllDevices: []*nvml.FingerprintDeviceData{
				{
					DeviceData: &nvml.DeviceData{
						UUID: "1",
					},
				},
				{
					DeviceData: &nvml.DeviceData{
						UUID: "2",
					},
				},
				{
					DeviceData: &nvml.DeviceData{
						UUID: "3",
					},
				},
				{
					DeviceData: &nvml.DeviceData{
						UUID: "I am new",
					},
				},
			},
			ExpectedResult: true,
			DeviceMapAfterMethodCall: map[string]struct{}{
				"1":        {},
				"2":        {},
				"3":        {},
				"I am new": {},
			},
		},
		{
			Name: "Device disappeared",
			Device: &NvidiaDevice{devices: map[string]struct{}{
				"1": {},
				"2": {},
				"3": {},
			}},
			AllDevices: []*nvml.FingerprintDeviceData{
				{
					DeviceData: &nvml.DeviceData{
						UUID: "1",
					},
				},
				{
					DeviceData: &nvml.DeviceData{
						UUID: "2",
					},
				},
			},
			ExpectedResult: true,
			DeviceMapAfterMethodCall: map[string]struct{}{
				"1": {},
				"2": {},
			},
		},
		{
			Name:   "No devices in NvidiaDevice map",
			Device: &NvidiaDevice{},
			AllDevices: []*nvml.FingerprintDeviceData{
				{
					DeviceData: &nvml.DeviceData{
						UUID: "1",
					},
				},
				{
					DeviceData: &nvml.DeviceData{
						UUID: "2",
					},
				},
				{
					DeviceData: &nvml.DeviceData{
						UUID: "3",
					},
				},
			},
			ExpectedResult: true,
			DeviceMapAfterMethodCall: map[string]struct{}{
				"1": {},
				"2": {},
				"3": {},
			},
		},
		{
			Name: "No devices detected",
			Device: &NvidiaDevice{devices: map[string]struct{}{
				"1": {},
				"2": {},
				"3": {},
			}},
			AllDevices:               nil,
			ExpectedResult:           true,
			DeviceMapAfterMethodCall: map[string]struct{}{},
		},
	} {
		t.Run(testCase.Name, func(t *testing.T) {
			actualResult := testCase.Device.fingerprintChanged(testCase.AllDevices)
			req := require.New(t)
			// check that function returns valid "updated / not updated" state
			req.Equal(testCase.ExpectedResult, actualResult)
			// check that function propely updates devices map
			req.Equal(testCase.Device.devices, testCase.DeviceMapAfterMethodCall)
		})
	}
}

func TestAttributesFromFingerprintDeviceData(t *testing.T) {
	for _, testCase := range []struct {
		Name                  string
		FingerprintDeviceData *nvml.FingerprintDeviceData
		ExpectedResult        map[string]*structs.Attribute
	}{
		{
			Name: "All attributes are not nil",
			FingerprintDeviceData: &nvml.FingerprintDeviceData{
				DeviceData: &nvml.DeviceData{
					UUID:       "1",
					DeviceName: helper.StringToPtr("Type1"),
					MemoryMiB:  helper.Uint64ToPtr(256),
					PowerW:     helper.UintToPtr(2),
					BAR1MiB:    helper.Uint64ToPtr(256),
				},
				PCIBusID:           "pciBusID1",
				PCIBandwidthMBPerS: helper.UintToPtr(1),
				CoresClockMHz:      helper.UintToPtr(1),
				MemoryClockMHz:     helper.UintToPtr(1),
				DisplayState:       "Enabled",
				PersistenceMode:    "Enabled",
			},
			ExpectedResult: map[string]*structs.Attribute{
				MemoryAttr: {
					Int:  helper.Int64ToPtr(256),
					Unit: structs.UnitMiB,
				},
				PowerAttr: {
					Int:  helper.Int64ToPtr(2),
					Unit: structs.UnitW,
				},
				BAR1Attr: {
					Int:  helper.Int64ToPtr(256),
					Unit: structs.UnitMiB,
				},
				PCIBandwidthAttr: {
					Int:  helper.Int64ToPtr(1),
					Unit: structs.UnitMBPerS,
				},
				CoresClockAttr: {
					Int:  helper.Int64ToPtr(1),
					Unit: structs.UnitMHz,
				},
				MemoryClockAttr: {
					Int:  helper.Int64ToPtr(1),
					Unit: structs.UnitMHz,
				},
				DisplayStateAttr: {
					String: helper.StringToPtr("Enabled"),
				},
				PersistenceModeAttr: {
					String: helper.StringToPtr("Enabled"),
				},
			},
		},
		{
			Name: "nil values are omitted",
			FingerprintDeviceData: &nvml.FingerprintDeviceData{
				DeviceData: &nvml.DeviceData{
					UUID:       "1",
					DeviceName: helper.StringToPtr("Type1"),
					MemoryMiB:  nil,
					PowerW:     helper.UintToPtr(2),
					BAR1MiB:    helper.Uint64ToPtr(256),
				},
				PCIBusID:        "pciBusID1",
				DisplayState:    "Enabled",
				PersistenceMode: "Enabled",
			},
			ExpectedResult: map[string]*structs.Attribute{
				PowerAttr: {
					Int:  helper.Int64ToPtr(2),
					Unit: structs.UnitW,
				},
				BAR1Attr: {
					Int:  helper.Int64ToPtr(256),
					Unit: structs.UnitMiB,
				},
				DisplayStateAttr: {
					String: helper.StringToPtr("Enabled"),
				},
				PersistenceModeAttr: {
					String: helper.StringToPtr("Enabled"),
				},
			},
		},
	} {
		t.Run(testCase.Name, func(t *testing.T) {
			actualResult := attributesFromFingerprintDeviceData(testCase.FingerprintDeviceData)
			require.Equal(t, testCase.ExpectedResult, actualResult)
		})
	}
}

func TestDeviceGroupFromFingerprintData(t *testing.T) {
	for _, testCase := range []struct {
		Name             string
		GroupName        string
		Devices          []*nvml.FingerprintDeviceData
		CommonAttributes map[string]*structs.Attribute
		ExpectedResult   *device.DeviceGroup
	}{
		{
			Name:      "Devices are provided",
			GroupName: "Type1",
			Devices: []*nvml.FingerprintDeviceData{
				{
					DeviceData: &nvml.DeviceData{
						UUID:       "1",
						DeviceName: helper.StringToPtr("Type1"),
						MemoryMiB:  helper.Uint64ToPtr(100),
						PowerW:     helper.UintToPtr(2),
						BAR1MiB:    helper.Uint64ToPtr(256),
					},
					PCIBusID:           "pciBusID1",
					PCIBandwidthMBPerS: helper.UintToPtr(1),
					CoresClockMHz:      helper.UintToPtr(1),
					MemoryClockMHz:     helper.UintToPtr(1),
					DisplayState:       "Enabled",
					PersistenceMode:    "Enabled",
				},
				{
					DeviceData: &nvml.DeviceData{
						UUID:       "2",
						DeviceName: helper.StringToPtr("Type1"),
						MemoryMiB:  helper.Uint64ToPtr(100),
						PowerW:     helper.UintToPtr(2),
						BAR1MiB:    helper.Uint64ToPtr(256),
					},
					PCIBusID:           "pciBusID2",
					PCIBandwidthMBPerS: helper.UintToPtr(1),
					CoresClockMHz:      helper.UintToPtr(1),
					MemoryClockMHz:     helper.UintToPtr(1),
					DisplayState:       "Enabled",
					PersistenceMode:    "Enabled",
				},
			},
			ExpectedResult: &device.DeviceGroup{
				Vendor: vendor,
				Type:   deviceType,
				Name:   "Type1",
				Devices: []*device.Device{
					{
						ID:      "1",
						Healthy: true,
						HwLocality: &device.DeviceLocality{
							PciBusID: "pciBusID1",
						},
					},
					{
						ID:      "2",
						Healthy: true,
						HwLocality: &device.DeviceLocality{
							PciBusID: "pciBusID2",
						},
					},
				},
				Attributes: map[string]*structs.Attribute{
					MemoryAttr: {
						Int:  helper.Int64ToPtr(100),
						Unit: structs.UnitMiB,
					},
					PowerAttr: {
						Int:  helper.Int64ToPtr(2),
						Unit: structs.UnitW,
					},
					BAR1Attr: {
						Int:  helper.Int64ToPtr(256),
						Unit: structs.UnitMiB,
					},
					PCIBandwidthAttr: {
						Int:  helper.Int64ToPtr(1),
						Unit: structs.UnitMBPerS,
					},
					CoresClockAttr: {
						Int:  helper.Int64ToPtr(1),
						Unit: structs.UnitMHz,
					},
					MemoryClockAttr: {
						Int:  helper.Int64ToPtr(1),
						Unit: structs.UnitMHz,
					},
					DisplayStateAttr: {
						String: helper.StringToPtr("Enabled"),
					},
					PersistenceModeAttr: {
						String: helper.StringToPtr("Enabled"),
					},
				},
			},
		},
		{
			Name:      "Devices and common attributes are provided",
			GroupName: "Type1",
			Devices: []*nvml.FingerprintDeviceData{
				{
					DeviceData: &nvml.DeviceData{
						UUID:       "1",
						DeviceName: helper.StringToPtr("Type1"),
						MemoryMiB:  helper.Uint64ToPtr(100),
						PowerW:     helper.UintToPtr(2),
						BAR1MiB:    helper.Uint64ToPtr(256),
					},
					PCIBusID:           "pciBusID1",
					PCIBandwidthMBPerS: helper.UintToPtr(1),
					CoresClockMHz:      helper.UintToPtr(1),
					MemoryClockMHz:     helper.UintToPtr(1),
					DisplayState:       "Enabled",
					PersistenceMode:    "Enabled",
				},
				{
					DeviceData: &nvml.DeviceData{
						UUID:       "2",
						DeviceName: helper.StringToPtr("Type1"),
						MemoryMiB:  helper.Uint64ToPtr(100),
						PowerW:     helper.UintToPtr(2),
						BAR1MiB:    helper.Uint64ToPtr(256),
					},
					PCIBusID:           "pciBusID2",
					PCIBandwidthMBPerS: helper.UintToPtr(1),
					CoresClockMHz:      helper.UintToPtr(1),
					MemoryClockMHz:     helper.UintToPtr(1),
					DisplayState:       "Enabled",
					PersistenceMode:    "Enabled",
				},
			},
			CommonAttributes: map[string]*structs.Attribute{
				DriverVersionAttr: {
					String: helper.StringToPtr("1"),
				},
			},
			ExpectedResult: &device.DeviceGroup{
				Vendor: vendor,
				Type:   deviceType,
				Name:   "Type1",
				Devices: []*device.Device{
					{
						ID:      "1",
						Healthy: true,
						HwLocality: &device.DeviceLocality{
							PciBusID: "pciBusID1",
						},
					},
					{
						ID:      "2",
						Healthy: true,
						HwLocality: &device.DeviceLocality{
							PciBusID: "pciBusID2",
						},
					},
				},
				Attributes: map[string]*structs.Attribute{
					MemoryAttr: {
						Int:  helper.Int64ToPtr(100),
						Unit: structs.UnitMiB,
					},
					PowerAttr: {
						Int:  helper.Int64ToPtr(2),
						Unit: structs.UnitW,
					},
					BAR1Attr: {
						Int:  helper.Int64ToPtr(256),
						Unit: structs.UnitMiB,
					},
					PCIBandwidthAttr: {
						Int:  helper.Int64ToPtr(1),
						Unit: structs.UnitMBPerS,
					},
					CoresClockAttr: {
						Int:  helper.Int64ToPtr(1),
						Unit: structs.UnitMHz,
					},
					MemoryClockAttr: {
						Int:  helper.Int64ToPtr(1),
						Unit: structs.UnitMHz,
					},
					DisplayStateAttr: {
						String: helper.StringToPtr("Enabled"),
					},
					PersistenceModeAttr: {
						String: helper.StringToPtr("Enabled"),
					},
					DriverVersionAttr: {
						String: helper.StringToPtr("1"),
					},
				},
			},
		},
		{
			Name:      "Devices are not provided",
			GroupName: "Type1",
			CommonAttributes: map[string]*structs.Attribute{
				DriverVersionAttr: {
					String: helper.StringToPtr("1"),
				},
			},
			Devices:        nil,
			ExpectedResult: nil,
		},
	} {
		t.Run(testCase.Name, func(t *testing.T) {
			actualResult := deviceGroupFromFingerprintData(testCase.GroupName, testCase.Devices, testCase.CommonAttributes)
			require.New(t).Equal(testCase.ExpectedResult, actualResult)
		})
	}
}

func TestWriteFingerprintToChannel(t *testing.T) {
	for _, testCase := range []struct {
		Name                   string
		Device                 *NvidiaDevice
		ExpectedWriteToChannel *device.FingerprintResponse
	}{
		{
			Name: "Check that FingerprintError is handled properly",
			Device: &NvidiaDevice{
				nvmlClient: &MockNvmlClient{
					FingerprintError: errors.New(""),
				},
				logger: hclog.NewNullLogger(),
			},
			ExpectedWriteToChannel: &device.FingerprintResponse{
				Error: errors.New(""),
			},
		},
		{
			Name: "Check ignore devices works correctly",
			Device: &NvidiaDevice{
				nvmlClient: &MockNvmlClient{
					FingerprintResponseReturned: &nvml.FingerprintData{
						DriverVersion: "1",
						Devices: []*nvml.FingerprintDeviceData{
							{
								DeviceData: &nvml.DeviceData{
									UUID:       "1",
									DeviceName: helper.StringToPtr("Name"),
									MemoryMiB:  helper.Uint64ToPtr(10),
									PowerW:     helper.UintToPtr(100),
									BAR1MiB:    helper.Uint64ToPtr(256),
								},
								PCIBusID:           "pciBusID1",
								PCIBandwidthMBPerS: helper.UintToPtr(1),
								CoresClockMHz:      helper.UintToPtr(1),
								MemoryClockMHz:     helper.UintToPtr(1),
								DisplayState:       "Enabled",
								PersistenceMode:    "Enabled",
							},
							{
								DeviceData: &nvml.DeviceData{
									UUID:       "2",
									DeviceName: helper.StringToPtr("Name"),
									MemoryMiB:  helper.Uint64ToPtr(10),
									PowerW:     helper.UintToPtr(100),
									BAR1MiB:    helper.Uint64ToPtr(256),
								},
								PCIBusID:           "pciBusID2",
								PCIBandwidthMBPerS: helper.UintToPtr(1),
								CoresClockMHz:      helper.UintToPtr(1),
								MemoryClockMHz:     helper.UintToPtr(1),
								DisplayState:       "Enabled",
								PersistenceMode:    "Enabled",
							},
						},
					},
				},
				ignoredGPUIDs: map[string]struct{}{
					"1": {},
				},
				logger: hclog.NewNullLogger(),
			},
			ExpectedWriteToChannel: &device.FingerprintResponse{
				Devices: []*device.DeviceGroup{
					{
						Vendor: vendor,
						Type:   deviceType,
						Name:   "Name",
						Devices: []*device.Device{
							{
								ID:      "2",
								Healthy: true,
								HwLocality: &device.DeviceLocality{
									PciBusID: "pciBusID2",
								},
							},
						},
						Attributes: map[string]*structs.Attribute{
							MemoryAttr: {
								Int:  helper.Int64ToPtr(10),
								Unit: structs.UnitMiB,
							},
							PowerAttr: {
								Int:  helper.Int64ToPtr(100),
								Unit: structs.UnitW,
							},
							BAR1Attr: {
								Int:  helper.Int64ToPtr(256),
								Unit: structs.UnitMiB,
							},
							PCIBandwidthAttr: {
								Int:  helper.Int64ToPtr(1),
								Unit: structs.UnitMBPerS,
							},
							CoresClockAttr: {
								Int:  helper.Int64ToPtr(1),
								Unit: structs.UnitMHz,
							},
							MemoryClockAttr: {
								Int:  helper.Int64ToPtr(1),
								Unit: structs.UnitMHz,
							},
							DisplayStateAttr: {
								String: helper.StringToPtr("Enabled"),
							},
							PersistenceModeAttr: {
								String: helper.StringToPtr("Enabled"),
							},
							DriverVersionAttr: {
								String: helper.StringToPtr("1"),
							},
						},
					},
				},
			},
		},
		{
			Name: "Check devices are split to multiple device groups 1",
			Device: &NvidiaDevice{
				nvmlClient: &MockNvmlClient{
					FingerprintResponseReturned: &nvml.FingerprintData{
						DriverVersion: "1",
						Devices: []*nvml.FingerprintDeviceData{
							{
								DeviceData: &nvml.DeviceData{
									UUID:       "1",
									DeviceName: helper.StringToPtr("Name1"),
									MemoryMiB:  helper.Uint64ToPtr(10),
									PowerW:     helper.UintToPtr(100),
									BAR1MiB:    helper.Uint64ToPtr(256),
								},
								PCIBusID:           "pciBusID1",
								PCIBandwidthMBPerS: helper.UintToPtr(1),
								CoresClockMHz:      helper.UintToPtr(1),
								MemoryClockMHz:     helper.UintToPtr(1),
								DisplayState:       "Enabled",
								PersistenceMode:    "Enabled",
							},
							{
								DeviceData: &nvml.DeviceData{
									UUID:       "2",
									DeviceName: helper.StringToPtr("Name2"),
									MemoryMiB:  helper.Uint64ToPtr(11),
									PowerW:     helper.UintToPtr(100),
									BAR1MiB:    helper.Uint64ToPtr(256),
								},
								PCIBusID:           "pciBusID2",
								PCIBandwidthMBPerS: helper.UintToPtr(1),
								CoresClockMHz:      helper.UintToPtr(1),
								MemoryClockMHz:     helper.UintToPtr(1),
								DisplayState:       "Enabled",
								PersistenceMode:    "Enabled",
							},
							{
								DeviceData: &nvml.DeviceData{
									UUID:       "3",
									DeviceName: helper.StringToPtr("Name3"),
									MemoryMiB:  helper.Uint64ToPtr(12),
									PowerW:     helper.UintToPtr(100),
									BAR1MiB:    helper.Uint64ToPtr(256),
								},
								PCIBusID:           "pciBusID3",
								PCIBandwidthMBPerS: helper.UintToPtr(1),
								CoresClockMHz:      helper.UintToPtr(1),
								MemoryClockMHz:     helper.UintToPtr(1),
								DisplayState:       "Enabled",
								PersistenceMode:    "Enabled",
							},
						},
					},
				},
				logger: hclog.NewNullLogger(),
			},
			ExpectedWriteToChannel: &device.FingerprintResponse{
				Devices: []*device.DeviceGroup{
					{
						Vendor: vendor,
						Type:   deviceType,
						Name:   "Name1",
						Devices: []*device.Device{
							{
								ID:      "1",
								Healthy: true,
								HwLocality: &device.DeviceLocality{
									PciBusID: "pciBusID1",
								},
							},
						},
						Attributes: map[string]*structs.Attribute{
							MemoryAttr: {
								Int:  helper.Int64ToPtr(10),
								Unit: structs.UnitMiB,
							},
							PowerAttr: {
								Int:  helper.Int64ToPtr(100),
								Unit: structs.UnitW,
							},
							BAR1Attr: {
								Int:  helper.Int64ToPtr(256),
								Unit: structs.UnitMiB,
							},
							PCIBandwidthAttr: {
								Int:  helper.Int64ToPtr(1),
								Unit: structs.UnitMBPerS,
							},
							CoresClockAttr: {
								Int:  helper.Int64ToPtr(1),
								Unit: structs.UnitMHz,
							},
							MemoryClockAttr: {
								Int:  helper.Int64ToPtr(1),
								Unit: structs.UnitMHz,
							},
							DisplayStateAttr: {
								String: helper.StringToPtr("Enabled"),
							},
							PersistenceModeAttr: {
								String: helper.StringToPtr("Enabled"),
							},
							DriverVersionAttr: {
								String: helper.StringToPtr("1"),
							},
						},
					},
					{
						Vendor: vendor,
						Type:   deviceType,
						Name:   "Name2",
						Devices: []*device.Device{
							{
								ID:      "2",
								Healthy: true,
								HwLocality: &device.DeviceLocality{
									PciBusID: "pciBusID2",
								},
							},
						},
						Attributes: map[string]*structs.Attribute{
							MemoryAttr: {
								Int:  helper.Int64ToPtr(11),
								Unit: structs.UnitMiB,
							},
							PowerAttr: {
								Int:  helper.Int64ToPtr(100),
								Unit: structs.UnitW,
							},
							BAR1Attr: {
								Int:  helper.Int64ToPtr(256),
								Unit: structs.UnitMiB,
							},
							PCIBandwidthAttr: {
								Int:  helper.Int64ToPtr(1),
								Unit: structs.UnitMBPerS,
							},
							CoresClockAttr: {
								Int:  helper.Int64ToPtr(1),
								Unit: structs.UnitMHz,
							},
							MemoryClockAttr: {
								Int:  helper.Int64ToPtr(1),
								Unit: structs.UnitMHz,
							},
							DisplayStateAttr: {
								String: helper.StringToPtr("Enabled"),
							},
							PersistenceModeAttr: {
								String: helper.StringToPtr("Enabled"),
							},
							DriverVersionAttr: {
								String: helper.StringToPtr("1"),
							},
						},
					},
					{
						Vendor: vendor,
						Type:   deviceType,
						Name:   "Name3",
						Devices: []*device.Device{
							{
								ID:      "3",
								Healthy: true,
								HwLocality: &device.DeviceLocality{
									PciBusID: "pciBusID3",
								},
							},
						},
						Attributes: map[string]*structs.Attribute{
							MemoryAttr: {
								Int:  helper.Int64ToPtr(12),
								Unit: structs.UnitMiB,
							},
							PowerAttr: {
								Int:  helper.Int64ToPtr(100),
								Unit: structs.UnitW,
							},
							BAR1Attr: {
								Int:  helper.Int64ToPtr(256),
								Unit: structs.UnitMiB,
							},
							PCIBandwidthAttr: {
								Int:  helper.Int64ToPtr(1),
								Unit: structs.UnitMBPerS,
							},
							CoresClockAttr: {
								Int:  helper.Int64ToPtr(1),
								Unit: structs.UnitMHz,
							},
							MemoryClockAttr: {
								Int:  helper.Int64ToPtr(1),
								Unit: structs.UnitMHz,
							},
							DisplayStateAttr: {
								String: helper.StringToPtr("Enabled"),
							},
							PersistenceModeAttr: {
								String: helper.StringToPtr("Enabled"),
							},
							DriverVersionAttr: {
								String: helper.StringToPtr("1"),
							},
						},
					},
				},
			},
		},
		{
			Name: "Check devices are split to multiple device groups 2",
			Device: &NvidiaDevice{
				nvmlClient: &MockNvmlClient{
					FingerprintResponseReturned: &nvml.FingerprintData{
						DriverVersion: "1",
						Devices: []*nvml.FingerprintDeviceData{
							{
								DeviceData: &nvml.DeviceData{
									UUID:       "1",
									DeviceName: helper.StringToPtr("Name1"),
									MemoryMiB:  helper.Uint64ToPtr(10),
									PowerW:     helper.UintToPtr(100),
									BAR1MiB:    helper.Uint64ToPtr(256),
								},
								PCIBusID:           "pciBusID1",
								PCIBandwidthMBPerS: helper.UintToPtr(1),
								CoresClockMHz:      helper.UintToPtr(1),
								MemoryClockMHz:     helper.UintToPtr(1),
								DisplayState:       "Enabled",
								PersistenceMode:    "Enabled",
							},
							{
								DeviceData: &nvml.DeviceData{
									UUID:       "2",
									DeviceName: helper.StringToPtr("Name2"),
									MemoryMiB:  helper.Uint64ToPtr(11),
									PowerW:     helper.UintToPtr(100),
									BAR1MiB:    helper.Uint64ToPtr(256),
								},
								PCIBusID:           "pciBusID2",
								PCIBandwidthMBPerS: helper.UintToPtr(1),
								CoresClockMHz:      helper.UintToPtr(1),
								MemoryClockMHz:     helper.UintToPtr(1),
								DisplayState:       "Enabled",
								PersistenceMode:    "Enabled",
							},
							{
								DeviceData: &nvml.DeviceData{
									UUID:       "3",
									DeviceName: helper.StringToPtr("Name2"),
									MemoryMiB:  helper.Uint64ToPtr(12),
									PowerW:     helper.UintToPtr(100),
									BAR1MiB:    helper.Uint64ToPtr(256),
								},
								PCIBusID:           "pciBusID3",
								PCIBandwidthMBPerS: helper.UintToPtr(1),
								CoresClockMHz:      helper.UintToPtr(1),
								MemoryClockMHz:     helper.UintToPtr(1),
								DisplayState:       "Enabled",
								PersistenceMode:    "Enabled",
							},
						},
					},
				},
				logger: hclog.NewNullLogger(),
			},
			ExpectedWriteToChannel: &device.FingerprintResponse{
				Devices: []*device.DeviceGroup{
					{
						Vendor: vendor,
						Type:   deviceType,
						Name:   "Name1",
						Devices: []*device.Device{
							{
								ID:      "1",
								Healthy: true,
								HwLocality: &device.DeviceLocality{
									PciBusID: "pciBusID1",
								},
							},
						},
						Attributes: map[string]*structs.Attribute{
							MemoryAttr: {
								Int:  helper.Int64ToPtr(10),
								Unit: structs.UnitMiB,
							},
							PowerAttr: {
								Int:  helper.Int64ToPtr(100),
								Unit: structs.UnitW,
							},
							BAR1Attr: {
								Int:  helper.Int64ToPtr(256),
								Unit: structs.UnitMiB,
							},
							PCIBandwidthAttr: {
								Int:  helper.Int64ToPtr(1),
								Unit: structs.UnitMBPerS,
							},
							CoresClockAttr: {
								Int:  helper.Int64ToPtr(1),
								Unit: structs.UnitMHz,
							},
							MemoryClockAttr: {
								Int:  helper.Int64ToPtr(1),
								Unit: structs.UnitMHz,
							},
							DisplayStateAttr: {
								String: helper.StringToPtr("Enabled"),
							},
							PersistenceModeAttr: {
								String: helper.StringToPtr("Enabled"),
							},
							DriverVersionAttr: {
								String: helper.StringToPtr("1"),
							},
						},
					},
					{
						Vendor: vendor,
						Type:   deviceType,
						Name:   "Name2",
						Devices: []*device.Device{
							{
								ID:      "2",
								Healthy: true,
								HwLocality: &device.DeviceLocality{
									PciBusID: "pciBusID2",
								},
							},
							{
								ID:      "3",
								Healthy: true,
								HwLocality: &device.DeviceLocality{
									PciBusID: "pciBusID3",
								},
							},
						},
						Attributes: map[string]*structs.Attribute{
							MemoryAttr: {
								Int:  helper.Int64ToPtr(11),
								Unit: structs.UnitMiB,
							},
							PowerAttr: {
								Int:  helper.Int64ToPtr(100),
								Unit: structs.UnitW,
							},
							BAR1Attr: {
								Int:  helper.Int64ToPtr(256),
								Unit: structs.UnitMiB,
							},
							PCIBandwidthAttr: {
								Int:  helper.Int64ToPtr(1),
								Unit: structs.UnitMBPerS,
							},
							CoresClockAttr: {
								Int:  helper.Int64ToPtr(1),
								Unit: structs.UnitMHz,
							},
							MemoryClockAttr: {
								Int:  helper.Int64ToPtr(1),
								Unit: structs.UnitMHz,
							},
							DisplayStateAttr: {
								String: helper.StringToPtr("Enabled"),
							},
							PersistenceModeAttr: {
								String: helper.StringToPtr("Enabled"),
							},
							DriverVersionAttr: {
								String: helper.StringToPtr("1"),
							},
						},
					},
				},
			},
		},
	} {
		t.Run(testCase.Name, func(t *testing.T) {
			channel := make(chan *device.FingerprintResponse, 1)
			testCase.Device.writeFingerprintToChannel(channel)
			actualResult := <-channel
			// writeFingerprintToChannel iterates over map keys
			// and insterts results to an array, so order of elements in output array
			// may be different
			// actualResult, expectedResult arrays has to be sorted firsted
			sort.Slice(actualResult.Devices, func(i, j int) bool {
				return actualResult.Devices[i].Name < actualResult.Devices[j].Name
			})
			sort.Slice(testCase.ExpectedWriteToChannel.Devices, func(i, j int) bool {
				return testCase.ExpectedWriteToChannel.Devices[i].Name < testCase.ExpectedWriteToChannel.Devices[j].Name
			})
			require.Equal(t, testCase.ExpectedWriteToChannel, actualResult)
		})
	}
}

// Test if nonworking driver returns empty fingerprint data
func TestFingerprint(t *testing.T) {
	for _, testCase := range []struct {
		Name                   string
		Device                 *NvidiaDevice
		ExpectedWriteToChannel *device.FingerprintResponse
	}{
		{
			Name: "Check that working driver returns valid fingeprint data",
			Device: &NvidiaDevice{
				initErr: nil,
				nvmlClient: &MockNvmlClient{
					FingerprintResponseReturned: &nvml.FingerprintData{
						DriverVersion: "1",
						Devices: []*nvml.FingerprintDeviceData{
							{
								DeviceData: &nvml.DeviceData{
									UUID:       "1",
									DeviceName: helper.StringToPtr("Name1"),
									MemoryMiB:  helper.Uint64ToPtr(10),
									PowerW:     helper.UintToPtr(100),
									BAR1MiB:    helper.Uint64ToPtr(256),
								},
								PCIBusID:           "pciBusID1",
								PCIBandwidthMBPerS: helper.UintToPtr(1),
								CoresClockMHz:      helper.UintToPtr(1),
								MemoryClockMHz:     helper.UintToPtr(1),
								DisplayState:       "Enabled",
								PersistenceMode:    "Enabled",
							},
							{
								DeviceData: &nvml.DeviceData{
									UUID:       "2",
									DeviceName: helper.StringToPtr("Name1"),
									MemoryMiB:  helper.Uint64ToPtr(10),
									PowerW:     helper.UintToPtr(100),
									BAR1MiB:    helper.Uint64ToPtr(256),
								},
								PCIBusID:           "pciBusID2",
								PCIBandwidthMBPerS: helper.UintToPtr(1),
								CoresClockMHz:      helper.UintToPtr(1),
								MemoryClockMHz:     helper.UintToPtr(1),
								DisplayState:       "Enabled",
								PersistenceMode:    "Enabled",
							},
							{
								DeviceData: &nvml.DeviceData{
									UUID:       "3",
									DeviceName: helper.StringToPtr("Name1"),
									MemoryMiB:  helper.Uint64ToPtr(10),
									PowerW:     helper.UintToPtr(100),
									BAR1MiB:    helper.Uint64ToPtr(256),
								},
								PCIBusID:           "pciBusID3",
								PCIBandwidthMBPerS: helper.UintToPtr(1),
								CoresClockMHz:      helper.UintToPtr(1),
								MemoryClockMHz:     helper.UintToPtr(1),
								DisplayState:       "Enabled",
								PersistenceMode:    "Enabled",
							},
						},
					},
				},
				logger: hclog.NewNullLogger(),
			},
			ExpectedWriteToChannel: &device.FingerprintResponse{
				Devices: []*device.DeviceGroup{
					{
						Vendor: vendor,
						Type:   deviceType,
						Name:   "Name1",
						Devices: []*device.Device{
							{
								ID:      "1",
								Healthy: true,
								HwLocality: &device.DeviceLocality{
									PciBusID: "pciBusID1",
								},
							},
							{
								ID:      "2",
								Healthy: true,
								HwLocality: &device.DeviceLocality{
									PciBusID: "pciBusID2",
								},
							},
							{
								ID:      "3",
								Healthy: true,
								HwLocality: &device.DeviceLocality{
									PciBusID: "pciBusID3",
								},
							},
						},
						Attributes: map[string]*structs.Attribute{
							MemoryAttr: {
								Int:  helper.Int64ToPtr(10),
								Unit: structs.UnitMiB,
							},
							PowerAttr: {
								Int:  helper.Int64ToPtr(100),
								Unit: structs.UnitW,
							},
							BAR1Attr: {
								Int:  helper.Int64ToPtr(256),
								Unit: structs.UnitMiB,
							},
							PCIBandwidthAttr: {
								Int:  helper.Int64ToPtr(1),
								Unit: structs.UnitMBPerS,
							},
							CoresClockAttr: {
								Int:  helper.Int64ToPtr(1),
								Unit: structs.UnitMHz,
							},
							MemoryClockAttr: {
								Int:  helper.Int64ToPtr(1),
								Unit: structs.UnitMHz,
							},
							DisplayStateAttr: {
								String: helper.StringToPtr("Enabled"),
							},
							PersistenceModeAttr: {
								String: helper.StringToPtr("Enabled"),
							},
							DriverVersionAttr: {
								String: helper.StringToPtr("1"),
							},
						},
					},
				},
			},
		},
		{
			Name: "Check that not working driver returns error fingeprint data",
			Device: &NvidiaDevice{
				initErr: errors.New("foo"),
				nvmlClient: &MockNvmlClient{
					FingerprintResponseReturned: &nvml.FingerprintData{
						DriverVersion: "1",
						Devices: []*nvml.FingerprintDeviceData{
							{
								DeviceData: &nvml.DeviceData{
									UUID:       "1",
									DeviceName: helper.StringToPtr("Name1"),
									MemoryMiB:  helper.Uint64ToPtr(10),
								},
							},
							{
								DeviceData: &nvml.DeviceData{
									UUID:       "2",
									DeviceName: helper.StringToPtr("Name1"),
									MemoryMiB:  helper.Uint64ToPtr(10),
								},
							},
							{
								DeviceData: &nvml.DeviceData{
									UUID:       "3",
									DeviceName: helper.StringToPtr("Name1"),
									MemoryMiB:  helper.Uint64ToPtr(10),
								},
							},
						},
					},
				},
				logger: hclog.NewNullLogger(),
			},
			ExpectedWriteToChannel: &device.FingerprintResponse{
				Error: errors.New("foo"),
			},
		},
	} {
		t.Run(testCase.Name, func(t *testing.T) {
			outCh := make(chan *device.FingerprintResponse)
			ctx, cancel := context.WithCancel(context.Background())
			go testCase.Device.fingerprint(ctx, outCh)
			result := <-outCh
			cancel()
			require.New(t).Equal(result, testCase.ExpectedWriteToChannel)
		})
	}
}
