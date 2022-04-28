package nvidia

import (
	"errors"
	"sort"
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/devices/gpu/nvidia/nvml"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/plugins/device"
	"github.com/hashicorp/nomad/plugins/shared/structs"
	"github.com/stretchr/testify/require"
)

func TestFilterStatsByID(t *testing.T) {
	for _, testCase := range []struct {
		Name           string
		ProvidedStats  []*nvml.StatsData
		ProvidedIDs    map[string]struct{}
		ExpectedResult []*nvml.StatsData
	}{
		{
			Name: "All ids are in the map",
			ProvidedStats: []*nvml.StatsData{
				{
					DeviceData: &nvml.DeviceData{
						UUID:       "UUID1",
						DeviceName: helper.StringToPtr("DeviceName1"),
						MemoryMiB:  helper.Uint64ToPtr(1),
						PowerW:     helper.UintToPtr(2),
						BAR1MiB:    helper.Uint64ToPtr(256),
					},
					PowerUsageW:        helper.UintToPtr(1),
					GPUUtilization:     helper.UintToPtr(1),
					MemoryUtilization:  helper.UintToPtr(1),
					EncoderUtilization: helper.UintToPtr(1),
					DecoderUtilization: helper.UintToPtr(1),
					TemperatureC:       helper.UintToPtr(1),
					UsedMemoryMiB:      helper.Uint64ToPtr(1),
					ECCErrorsL1Cache:   helper.Uint64ToPtr(100),
					ECCErrorsL2Cache:   helper.Uint64ToPtr(100),
					ECCErrorsDevice:    helper.Uint64ToPtr(100),
				},
				{
					DeviceData: &nvml.DeviceData{
						UUID:       "UUID2",
						DeviceName: helper.StringToPtr("DeviceName1"),
						MemoryMiB:  helper.Uint64ToPtr(1),
						PowerW:     helper.UintToPtr(2),
						BAR1MiB:    helper.Uint64ToPtr(256),
					},
					PowerUsageW:        helper.UintToPtr(1),
					GPUUtilization:     helper.UintToPtr(1),
					MemoryUtilization:  helper.UintToPtr(1),
					EncoderUtilization: helper.UintToPtr(1),
					DecoderUtilization: helper.UintToPtr(1),
					TemperatureC:       helper.UintToPtr(1),
					UsedMemoryMiB:      helper.Uint64ToPtr(1),
					ECCErrorsL1Cache:   helper.Uint64ToPtr(100),
					ECCErrorsL2Cache:   helper.Uint64ToPtr(100),
					ECCErrorsDevice:    helper.Uint64ToPtr(100),
				},
				{
					DeviceData: &nvml.DeviceData{
						UUID:       "UUID3",
						DeviceName: helper.StringToPtr("DeviceName1"),
						MemoryMiB:  helper.Uint64ToPtr(1),
						PowerW:     helper.UintToPtr(2),
						BAR1MiB:    helper.Uint64ToPtr(256),
					},
					PowerUsageW:        helper.UintToPtr(1),
					GPUUtilization:     helper.UintToPtr(1),
					MemoryUtilization:  helper.UintToPtr(1),
					EncoderUtilization: helper.UintToPtr(1),
					DecoderUtilization: helper.UintToPtr(1),
					TemperatureC:       helper.UintToPtr(1),
					UsedMemoryMiB:      helper.Uint64ToPtr(1),
					ECCErrorsL1Cache:   helper.Uint64ToPtr(100),
					ECCErrorsL2Cache:   helper.Uint64ToPtr(100),
					ECCErrorsDevice:    helper.Uint64ToPtr(100),
				},
			},
			ProvidedIDs: map[string]struct{}{
				"UUID1": {},
				"UUID2": {},
				"UUID3": {},
			},
			ExpectedResult: []*nvml.StatsData{
				{
					DeviceData: &nvml.DeviceData{
						UUID:       "UUID1",
						DeviceName: helper.StringToPtr("DeviceName1"),
						MemoryMiB:  helper.Uint64ToPtr(1),
						PowerW:     helper.UintToPtr(2),
						BAR1MiB:    helper.Uint64ToPtr(256),
					},
					PowerUsageW:        helper.UintToPtr(1),
					GPUUtilization:     helper.UintToPtr(1),
					MemoryUtilization:  helper.UintToPtr(1),
					EncoderUtilization: helper.UintToPtr(1),
					DecoderUtilization: helper.UintToPtr(1),
					TemperatureC:       helper.UintToPtr(1),
					UsedMemoryMiB:      helper.Uint64ToPtr(1),
					ECCErrorsL1Cache:   helper.Uint64ToPtr(100),
					ECCErrorsL2Cache:   helper.Uint64ToPtr(100),
					ECCErrorsDevice:    helper.Uint64ToPtr(100),
				},
				{
					DeviceData: &nvml.DeviceData{
						UUID:       "UUID2",
						DeviceName: helper.StringToPtr("DeviceName1"),
						MemoryMiB:  helper.Uint64ToPtr(1),
						PowerW:     helper.UintToPtr(2),
						BAR1MiB:    helper.Uint64ToPtr(256),
					},
					PowerUsageW:        helper.UintToPtr(1),
					GPUUtilization:     helper.UintToPtr(1),
					MemoryUtilization:  helper.UintToPtr(1),
					EncoderUtilization: helper.UintToPtr(1),
					DecoderUtilization: helper.UintToPtr(1),
					TemperatureC:       helper.UintToPtr(1),
					UsedMemoryMiB:      helper.Uint64ToPtr(1),
					ECCErrorsL1Cache:   helper.Uint64ToPtr(100),
					ECCErrorsL2Cache:   helper.Uint64ToPtr(100),
					ECCErrorsDevice:    helper.Uint64ToPtr(100),
				},
				{
					DeviceData: &nvml.DeviceData{
						UUID:       "UUID3",
						DeviceName: helper.StringToPtr("DeviceName1"),
						MemoryMiB:  helper.Uint64ToPtr(1),
						PowerW:     helper.UintToPtr(2),
						BAR1MiB:    helper.Uint64ToPtr(256),
					},
					PowerUsageW:        helper.UintToPtr(1),
					GPUUtilization:     helper.UintToPtr(1),
					MemoryUtilization:  helper.UintToPtr(1),
					EncoderUtilization: helper.UintToPtr(1),
					DecoderUtilization: helper.UintToPtr(1),
					TemperatureC:       helper.UintToPtr(1),
					UsedMemoryMiB:      helper.Uint64ToPtr(1),
					ECCErrorsL1Cache:   helper.Uint64ToPtr(100),
					ECCErrorsL2Cache:   helper.Uint64ToPtr(100),
					ECCErrorsDevice:    helper.Uint64ToPtr(100),
				},
			},
		},
		{
			Name: "Odd are not provided in the map",
			ProvidedStats: []*nvml.StatsData{
				{
					DeviceData: &nvml.DeviceData{
						UUID:       "UUID1",
						DeviceName: helper.StringToPtr("DeviceName1"),
						MemoryMiB:  helper.Uint64ToPtr(1),
						PowerW:     helper.UintToPtr(2),
						BAR1MiB:    helper.Uint64ToPtr(256),
					},
					PowerUsageW:        helper.UintToPtr(1),
					GPUUtilization:     helper.UintToPtr(1),
					MemoryUtilization:  helper.UintToPtr(1),
					EncoderUtilization: helper.UintToPtr(1),
					DecoderUtilization: helper.UintToPtr(1),
					TemperatureC:       helper.UintToPtr(1),
					UsedMemoryMiB:      helper.Uint64ToPtr(1),
					ECCErrorsL1Cache:   helper.Uint64ToPtr(100),
					ECCErrorsL2Cache:   helper.Uint64ToPtr(100),
					ECCErrorsDevice:    helper.Uint64ToPtr(100),
				},
				{
					DeviceData: &nvml.DeviceData{
						UUID:       "UUID2",
						DeviceName: helper.StringToPtr("DeviceName1"),
						MemoryMiB:  helper.Uint64ToPtr(1),
						PowerW:     helper.UintToPtr(2),
						BAR1MiB:    helper.Uint64ToPtr(256),
					},
					PowerUsageW:        helper.UintToPtr(1),
					GPUUtilization:     helper.UintToPtr(1),
					MemoryUtilization:  helper.UintToPtr(1),
					EncoderUtilization: helper.UintToPtr(1),
					DecoderUtilization: helper.UintToPtr(1),
					TemperatureC:       helper.UintToPtr(1),
					UsedMemoryMiB:      helper.Uint64ToPtr(1),
					ECCErrorsL1Cache:   helper.Uint64ToPtr(100),
					ECCErrorsL2Cache:   helper.Uint64ToPtr(100),
					ECCErrorsDevice:    helper.Uint64ToPtr(100),
				},
				{
					DeviceData: &nvml.DeviceData{
						UUID:       "UUID3",
						DeviceName: helper.StringToPtr("DeviceName1"),
						MemoryMiB:  helper.Uint64ToPtr(1),
						PowerW:     helper.UintToPtr(2),
						BAR1MiB:    helper.Uint64ToPtr(256),
					},
					PowerUsageW:        helper.UintToPtr(1),
					GPUUtilization:     helper.UintToPtr(1),
					MemoryUtilization:  helper.UintToPtr(1),
					EncoderUtilization: helper.UintToPtr(1),
					DecoderUtilization: helper.UintToPtr(1),
					TemperatureC:       helper.UintToPtr(1),
					UsedMemoryMiB:      helper.Uint64ToPtr(1),
					ECCErrorsL1Cache:   helper.Uint64ToPtr(100),
					ECCErrorsL2Cache:   helper.Uint64ToPtr(100),
					ECCErrorsDevice:    helper.Uint64ToPtr(100),
				},
			},
			ProvidedIDs: map[string]struct{}{
				"UUID2": {},
			},
			ExpectedResult: []*nvml.StatsData{
				{
					DeviceData: &nvml.DeviceData{
						UUID:       "UUID2",
						DeviceName: helper.StringToPtr("DeviceName1"),
						MemoryMiB:  helper.Uint64ToPtr(1),
						PowerW:     helper.UintToPtr(2),
						BAR1MiB:    helper.Uint64ToPtr(256),
					},
					PowerUsageW:        helper.UintToPtr(1),
					GPUUtilization:     helper.UintToPtr(1),
					MemoryUtilization:  helper.UintToPtr(1),
					EncoderUtilization: helper.UintToPtr(1),
					DecoderUtilization: helper.UintToPtr(1),
					TemperatureC:       helper.UintToPtr(1),
					UsedMemoryMiB:      helper.Uint64ToPtr(1),
					ECCErrorsL1Cache:   helper.Uint64ToPtr(100),
					ECCErrorsL2Cache:   helper.Uint64ToPtr(100),
					ECCErrorsDevice:    helper.Uint64ToPtr(100),
				},
			},
		},
		{
			Name: "Even are not provided in the map",
			ProvidedStats: []*nvml.StatsData{
				{
					DeviceData: &nvml.DeviceData{
						UUID:       "UUID1",
						DeviceName: helper.StringToPtr("DeviceName1"),
						MemoryMiB:  helper.Uint64ToPtr(1),
						PowerW:     helper.UintToPtr(2),
						BAR1MiB:    helper.Uint64ToPtr(256),
					},
					PowerUsageW:        helper.UintToPtr(1),
					GPUUtilization:     helper.UintToPtr(1),
					MemoryUtilization:  helper.UintToPtr(1),
					EncoderUtilization: helper.UintToPtr(1),
					DecoderUtilization: helper.UintToPtr(1),
					TemperatureC:       helper.UintToPtr(1),
					UsedMemoryMiB:      helper.Uint64ToPtr(1),
					ECCErrorsL1Cache:   helper.Uint64ToPtr(100),
					ECCErrorsL2Cache:   helper.Uint64ToPtr(100),
					ECCErrorsDevice:    helper.Uint64ToPtr(100),
				},
				{
					DeviceData: &nvml.DeviceData{
						UUID:       "UUID2",
						DeviceName: helper.StringToPtr("DeviceName1"),
						MemoryMiB:  helper.Uint64ToPtr(1),
						PowerW:     helper.UintToPtr(2),
						BAR1MiB:    helper.Uint64ToPtr(256),
					},
					PowerUsageW:        helper.UintToPtr(1),
					GPUUtilization:     helper.UintToPtr(1),
					MemoryUtilization:  helper.UintToPtr(1),
					EncoderUtilization: helper.UintToPtr(1),
					DecoderUtilization: helper.UintToPtr(1),
					TemperatureC:       helper.UintToPtr(1),
					UsedMemoryMiB:      helper.Uint64ToPtr(1),
					ECCErrorsL1Cache:   helper.Uint64ToPtr(100),
					ECCErrorsL2Cache:   helper.Uint64ToPtr(100),
					ECCErrorsDevice:    helper.Uint64ToPtr(100),
				},
				{
					DeviceData: &nvml.DeviceData{
						UUID:       "UUID3",
						DeviceName: helper.StringToPtr("DeviceName1"),
						MemoryMiB:  helper.Uint64ToPtr(1),
						PowerW:     helper.UintToPtr(2),
						BAR1MiB:    helper.Uint64ToPtr(256),
					},
					PowerUsageW:        helper.UintToPtr(1),
					GPUUtilization:     helper.UintToPtr(1),
					MemoryUtilization:  helper.UintToPtr(1),
					EncoderUtilization: helper.UintToPtr(1),
					DecoderUtilization: helper.UintToPtr(1),
					TemperatureC:       helper.UintToPtr(1),
					UsedMemoryMiB:      helper.Uint64ToPtr(1),
					ECCErrorsL1Cache:   helper.Uint64ToPtr(100),
					ECCErrorsL2Cache:   helper.Uint64ToPtr(100),
					ECCErrorsDevice:    helper.Uint64ToPtr(100),
				},
			},
			ProvidedIDs: map[string]struct{}{
				"UUID1": {},
				"UUID3": {},
			},
			ExpectedResult: []*nvml.StatsData{
				{
					DeviceData: &nvml.DeviceData{
						UUID:       "UUID1",
						DeviceName: helper.StringToPtr("DeviceName1"),
						MemoryMiB:  helper.Uint64ToPtr(1),
						PowerW:     helper.UintToPtr(2),
						BAR1MiB:    helper.Uint64ToPtr(256),
					},
					PowerUsageW:        helper.UintToPtr(1),
					GPUUtilization:     helper.UintToPtr(1),
					MemoryUtilization:  helper.UintToPtr(1),
					EncoderUtilization: helper.UintToPtr(1),
					DecoderUtilization: helper.UintToPtr(1),
					TemperatureC:       helper.UintToPtr(1),
					UsedMemoryMiB:      helper.Uint64ToPtr(1),
					ECCErrorsL1Cache:   helper.Uint64ToPtr(100),
					ECCErrorsL2Cache:   helper.Uint64ToPtr(100),
					ECCErrorsDevice:    helper.Uint64ToPtr(100),
				},
				{
					DeviceData: &nvml.DeviceData{
						UUID:       "UUID3",
						DeviceName: helper.StringToPtr("DeviceName1"),
						MemoryMiB:  helper.Uint64ToPtr(1),
						PowerW:     helper.UintToPtr(2),
						BAR1MiB:    helper.Uint64ToPtr(256),
					},
					PowerUsageW:        helper.UintToPtr(1),
					GPUUtilization:     helper.UintToPtr(1),
					MemoryUtilization:  helper.UintToPtr(1),
					EncoderUtilization: helper.UintToPtr(1),
					DecoderUtilization: helper.UintToPtr(1),
					TemperatureC:       helper.UintToPtr(1),
					UsedMemoryMiB:      helper.Uint64ToPtr(1),
					ECCErrorsL1Cache:   helper.Uint64ToPtr(100),
					ECCErrorsL2Cache:   helper.Uint64ToPtr(100),
					ECCErrorsDevice:    helper.Uint64ToPtr(100),
				},
			},
		},
		{
			Name: "No Stats were provided",
			ProvidedIDs: map[string]struct{}{
				"UUID1": {},
				"UUID2": {},
				"UUID3": {},
			},
		},
		{
			Name: "No Ids were provided",
			ProvidedStats: []*nvml.StatsData{
				{
					DeviceData: &nvml.DeviceData{
						UUID:       "UUID1",
						DeviceName: helper.StringToPtr("DeviceName1"),
						MemoryMiB:  helper.Uint64ToPtr(1),
						PowerW:     helper.UintToPtr(2),
						BAR1MiB:    helper.Uint64ToPtr(256),
					},
					PowerUsageW:        helper.UintToPtr(1),
					GPUUtilization:     helper.UintToPtr(1),
					MemoryUtilization:  helper.UintToPtr(1),
					EncoderUtilization: helper.UintToPtr(1),
					DecoderUtilization: helper.UintToPtr(1),
					TemperatureC:       helper.UintToPtr(1),
					UsedMemoryMiB:      helper.Uint64ToPtr(1),
					ECCErrorsL1Cache:   helper.Uint64ToPtr(100),
					ECCErrorsL2Cache:   helper.Uint64ToPtr(100),
					ECCErrorsDevice:    helper.Uint64ToPtr(100),
				},
				{
					DeviceData: &nvml.DeviceData{
						UUID:       "UUID2",
						DeviceName: helper.StringToPtr("DeviceName1"),
						MemoryMiB:  helper.Uint64ToPtr(1),
						PowerW:     helper.UintToPtr(2),
						BAR1MiB:    helper.Uint64ToPtr(256),
					},
					PowerUsageW:        helper.UintToPtr(1),
					GPUUtilization:     helper.UintToPtr(1),
					MemoryUtilization:  helper.UintToPtr(1),
					EncoderUtilization: helper.UintToPtr(1),
					DecoderUtilization: helper.UintToPtr(1),
					TemperatureC:       helper.UintToPtr(1),
					UsedMemoryMiB:      helper.Uint64ToPtr(1),
					ECCErrorsL1Cache:   helper.Uint64ToPtr(100),
					ECCErrorsL2Cache:   helper.Uint64ToPtr(100),
					ECCErrorsDevice:    helper.Uint64ToPtr(100),
				},
				{
					DeviceData: &nvml.DeviceData{
						UUID:       "UUID3",
						DeviceName: helper.StringToPtr("DeviceName1"),
						MemoryMiB:  helper.Uint64ToPtr(1),
						PowerW:     helper.UintToPtr(2),
						BAR1MiB:    helper.Uint64ToPtr(256),
					},
					PowerUsageW:        helper.UintToPtr(1),
					GPUUtilization:     helper.UintToPtr(1),
					MemoryUtilization:  helper.UintToPtr(1),
					EncoderUtilization: helper.UintToPtr(1),
					DecoderUtilization: helper.UintToPtr(1),
					TemperatureC:       helper.UintToPtr(1),
					UsedMemoryMiB:      helper.Uint64ToPtr(1),
					ECCErrorsL1Cache:   helper.Uint64ToPtr(100),
					ECCErrorsL2Cache:   helper.Uint64ToPtr(100),
					ECCErrorsDevice:    helper.Uint64ToPtr(100),
				},
			},
		},
	} {
		actualResult := filterStatsByID(testCase.ProvidedStats, testCase.ProvidedIDs)
		require.New(t).Equal(testCase.ExpectedResult, actualResult)
	}
}

func TestStatsForItem(t *testing.T) {
	for _, testCase := range []struct {
		Name           string
		Timestamp      time.Time
		ItemStat       *nvml.StatsData
		ExpectedResult *device.DeviceStats
	}{
		{
			Name:      "All fields in ItemStat are not nil",
			Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
			ItemStat: &nvml.StatsData{
				DeviceData: &nvml.DeviceData{
					UUID:       "UUID1",
					DeviceName: helper.StringToPtr("DeviceName1"),
					MemoryMiB:  helper.Uint64ToPtr(1),
					PowerW:     helper.UintToPtr(1),
					BAR1MiB:    helper.Uint64ToPtr(256),
				},
				PowerUsageW:        helper.UintToPtr(1),
				GPUUtilization:     helper.UintToPtr(1),
				MemoryUtilization:  helper.UintToPtr(1),
				EncoderUtilization: helper.UintToPtr(1),
				DecoderUtilization: helper.UintToPtr(1),
				TemperatureC:       helper.UintToPtr(1),
				UsedMemoryMiB:      helper.Uint64ToPtr(1),
				BAR1UsedMiB:        helper.Uint64ToPtr(1),
				ECCErrorsL1Cache:   helper.Uint64ToPtr(100),
				ECCErrorsL2Cache:   helper.Uint64ToPtr(100),
				ECCErrorsDevice:    helper.Uint64ToPtr(100),
			},
			ExpectedResult: &device.DeviceStats{
				Summary: &structs.StatValue{
					Unit:              MemoryStateUnit,
					Desc:              MemoryStateDesc,
					IntNumeratorVal:   helper.Int64ToPtr(1),
					IntDenominatorVal: helper.Int64ToPtr(1),
				},
				Stats: &structs.StatObject{
					Attributes: map[string]*structs.StatValue{
						PowerUsageAttr: {
							Unit:              PowerUsageUnit,
							Desc:              PowerUsageDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(1),
						},
						GPUUtilizationAttr: {
							Unit:            GPUUtilizationUnit,
							Desc:            GPUUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						MemoryUtilizationAttr: {
							Unit:            MemoryUtilizationUnit,
							Desc:            MemoryUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						EncoderUtilizationAttr: {
							Unit:            EncoderUtilizationUnit,
							Desc:            EncoderUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						DecoderUtilizationAttr: {
							Unit:            DecoderUtilizationUnit,
							Desc:            DecoderUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						TemperatureAttr: {
							Unit:            TemperatureUnit,
							Desc:            TemperatureDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						MemoryStateAttr: {
							Unit:              MemoryStateUnit,
							Desc:              MemoryStateDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(1),
						},
						BAR1StateAttr: {
							Unit:              BAR1StateUnit,
							Desc:              BAR1StateDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(256),
						},
						ECCErrorsL1CacheAttr: {
							Unit:            ECCErrorsL1CacheUnit,
							Desc:            ECCErrorsL1CacheDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
						ECCErrorsL2CacheAttr: {
							Unit:            ECCErrorsL2CacheUnit,
							Desc:            ECCErrorsL2CacheDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
						ECCErrorsDeviceAttr: {
							Unit:            ECCErrorsDeviceUnit,
							Desc:            ECCErrorsDeviceDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
					},
				},
				Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
			},
		},
		{
			Name:      "Power usage is nil",
			Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
			ItemStat: &nvml.StatsData{
				DeviceData: &nvml.DeviceData{
					UUID:       "UUID1",
					DeviceName: helper.StringToPtr("DeviceName1"),
					MemoryMiB:  helper.Uint64ToPtr(1),
					PowerW:     helper.UintToPtr(1),
					BAR1MiB:    helper.Uint64ToPtr(256),
				},
				PowerUsageW:        nil,
				GPUUtilization:     helper.UintToPtr(1),
				MemoryUtilization:  helper.UintToPtr(1),
				EncoderUtilization: helper.UintToPtr(1),
				DecoderUtilization: helper.UintToPtr(1),
				TemperatureC:       helper.UintToPtr(1),
				UsedMemoryMiB:      helper.Uint64ToPtr(1),
				BAR1UsedMiB:        helper.Uint64ToPtr(1),
				ECCErrorsL1Cache:   helper.Uint64ToPtr(100),
				ECCErrorsL2Cache:   helper.Uint64ToPtr(100),
				ECCErrorsDevice:    helper.Uint64ToPtr(100),
			},
			ExpectedResult: &device.DeviceStats{
				Summary: &structs.StatValue{
					Unit:              MemoryStateUnit,
					Desc:              MemoryStateDesc,
					IntNumeratorVal:   helper.Int64ToPtr(1),
					IntDenominatorVal: helper.Int64ToPtr(1),
				},
				Stats: &structs.StatObject{
					Attributes: map[string]*structs.StatValue{
						PowerUsageAttr: {
							Unit:      PowerUsageUnit,
							Desc:      PowerUsageDesc,
							StringVal: helper.StringToPtr(notAvailable),
						},
						GPUUtilizationAttr: {
							Unit:            GPUUtilizationUnit,
							Desc:            GPUUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						MemoryUtilizationAttr: {
							Unit:            MemoryUtilizationUnit,
							Desc:            MemoryUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						EncoderUtilizationAttr: {
							Unit:            EncoderUtilizationUnit,
							Desc:            EncoderUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						DecoderUtilizationAttr: {
							Unit:            DecoderUtilizationUnit,
							Desc:            DecoderUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						TemperatureAttr: {
							Unit:            TemperatureUnit,
							Desc:            TemperatureDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						MemoryStateAttr: {
							Unit:              MemoryStateUnit,
							Desc:              MemoryStateDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(1),
						},
						BAR1StateAttr: {
							Unit:              BAR1StateUnit,
							Desc:              BAR1StateDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(256),
						},
						ECCErrorsL1CacheAttr: {
							Unit:            ECCErrorsL1CacheUnit,
							Desc:            ECCErrorsL1CacheDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
						ECCErrorsL2CacheAttr: {
							Unit:            ECCErrorsL2CacheUnit,
							Desc:            ECCErrorsL2CacheDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
						ECCErrorsDeviceAttr: {
							Unit:            ECCErrorsDeviceUnit,
							Desc:            ECCErrorsDeviceDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
					},
				},
				Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
			},
		},
		{
			Name:      "PowerW is nil",
			Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
			ItemStat: &nvml.StatsData{
				DeviceData: &nvml.DeviceData{
					UUID:       "UUID1",
					DeviceName: helper.StringToPtr("DeviceName1"),
					MemoryMiB:  helper.Uint64ToPtr(1),
					PowerW:     nil,
					BAR1MiB:    helper.Uint64ToPtr(256),
				},
				PowerUsageW:        helper.UintToPtr(1),
				GPUUtilization:     helper.UintToPtr(1),
				MemoryUtilization:  helper.UintToPtr(1),
				EncoderUtilization: helper.UintToPtr(1),
				DecoderUtilization: helper.UintToPtr(1),
				TemperatureC:       helper.UintToPtr(1),
				UsedMemoryMiB:      helper.Uint64ToPtr(1),
				BAR1UsedMiB:        helper.Uint64ToPtr(1),
				ECCErrorsL1Cache:   helper.Uint64ToPtr(100),
				ECCErrorsL2Cache:   helper.Uint64ToPtr(100),
				ECCErrorsDevice:    helper.Uint64ToPtr(100),
			},
			ExpectedResult: &device.DeviceStats{
				Summary: &structs.StatValue{
					Unit:              MemoryStateUnit,
					Desc:              MemoryStateDesc,
					IntNumeratorVal:   helper.Int64ToPtr(1),
					IntDenominatorVal: helper.Int64ToPtr(1),
				},
				Stats: &structs.StatObject{
					Attributes: map[string]*structs.StatValue{
						PowerUsageAttr: {
							Unit:      PowerUsageUnit,
							Desc:      PowerUsageDesc,
							StringVal: helper.StringToPtr(notAvailable),
						},
						GPUUtilizationAttr: {
							Unit:            GPUUtilizationUnit,
							Desc:            GPUUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						MemoryUtilizationAttr: {
							Unit:            MemoryUtilizationUnit,
							Desc:            MemoryUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						EncoderUtilizationAttr: {
							Unit:            EncoderUtilizationUnit,
							Desc:            EncoderUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						DecoderUtilizationAttr: {
							Unit:            DecoderUtilizationUnit,
							Desc:            DecoderUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						TemperatureAttr: {
							Unit:            TemperatureUnit,
							Desc:            TemperatureDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						MemoryStateAttr: {
							Unit:              MemoryStateUnit,
							Desc:              MemoryStateDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(1),
						},
						BAR1StateAttr: {
							Unit:              BAR1StateUnit,
							Desc:              BAR1StateDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(256),
						},
						ECCErrorsL1CacheAttr: {
							Unit:            ECCErrorsL1CacheUnit,
							Desc:            ECCErrorsL1CacheDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
						ECCErrorsL2CacheAttr: {
							Unit:            ECCErrorsL2CacheUnit,
							Desc:            ECCErrorsL2CacheDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
						ECCErrorsDeviceAttr: {
							Unit:            ECCErrorsDeviceUnit,
							Desc:            ECCErrorsDeviceDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
					},
				},
				Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
			},
		},
		{
			Name:      "GPUUtilization is nil",
			Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
			ItemStat: &nvml.StatsData{
				DeviceData: &nvml.DeviceData{
					UUID:       "UUID1",
					DeviceName: helper.StringToPtr("DeviceName1"),
					MemoryMiB:  helper.Uint64ToPtr(1),
					PowerW:     helper.UintToPtr(1),
					BAR1MiB:    helper.Uint64ToPtr(256),
				},
				PowerUsageW:        helper.UintToPtr(1),
				GPUUtilization:     nil,
				MemoryUtilization:  helper.UintToPtr(1),
				EncoderUtilization: helper.UintToPtr(1),
				DecoderUtilization: helper.UintToPtr(1),
				TemperatureC:       helper.UintToPtr(1),
				UsedMemoryMiB:      helper.Uint64ToPtr(1),
				BAR1UsedMiB:        helper.Uint64ToPtr(1),
				ECCErrorsL1Cache:   helper.Uint64ToPtr(100),
				ECCErrorsL2Cache:   helper.Uint64ToPtr(100),
				ECCErrorsDevice:    helper.Uint64ToPtr(100),
			},
			ExpectedResult: &device.DeviceStats{
				Summary: &structs.StatValue{
					Unit:              MemoryStateUnit,
					Desc:              MemoryStateDesc,
					IntNumeratorVal:   helper.Int64ToPtr(1),
					IntDenominatorVal: helper.Int64ToPtr(1),
				},
				Stats: &structs.StatObject{
					Attributes: map[string]*structs.StatValue{
						PowerUsageAttr: {
							Unit:              PowerUsageUnit,
							Desc:              PowerUsageDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(1),
						},
						GPUUtilizationAttr: {
							Unit:      GPUUtilizationUnit,
							Desc:      GPUUtilizationDesc,
							StringVal: helper.StringToPtr(notAvailable),
						},
						MemoryUtilizationAttr: {
							Unit:            MemoryUtilizationUnit,
							Desc:            MemoryUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						EncoderUtilizationAttr: {
							Unit:            EncoderUtilizationUnit,
							Desc:            EncoderUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						DecoderUtilizationAttr: {
							Unit:            DecoderUtilizationUnit,
							Desc:            DecoderUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						TemperatureAttr: {
							Unit:            TemperatureUnit,
							Desc:            TemperatureDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						MemoryStateAttr: {
							Unit:              MemoryStateUnit,
							Desc:              MemoryStateDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(1),
						},
						BAR1StateAttr: {
							Unit:              BAR1StateUnit,
							Desc:              BAR1StateDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(256),
						},
						ECCErrorsL1CacheAttr: {
							Unit:            ECCErrorsL1CacheUnit,
							Desc:            ECCErrorsL1CacheDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
						ECCErrorsL2CacheAttr: {
							Unit:            ECCErrorsL2CacheUnit,
							Desc:            ECCErrorsL2CacheDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
						ECCErrorsDeviceAttr: {
							Unit:            ECCErrorsDeviceUnit,
							Desc:            ECCErrorsDeviceDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
					},
				},
				Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
			},
		},
		{
			Name:      "MemoryUtilization is nil",
			Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
			ItemStat: &nvml.StatsData{
				DeviceData: &nvml.DeviceData{
					UUID:       "UUID1",
					DeviceName: helper.StringToPtr("DeviceName1"),
					MemoryMiB:  helper.Uint64ToPtr(1),
					PowerW:     helper.UintToPtr(1),
					BAR1MiB:    helper.Uint64ToPtr(256),
				},
				PowerUsageW:        helper.UintToPtr(1),
				GPUUtilization:     helper.UintToPtr(1),
				MemoryUtilization:  nil,
				EncoderUtilization: helper.UintToPtr(1),
				DecoderUtilization: helper.UintToPtr(1),
				TemperatureC:       helper.UintToPtr(1),
				UsedMemoryMiB:      helper.Uint64ToPtr(1),
				BAR1UsedMiB:        helper.Uint64ToPtr(1),
				ECCErrorsL1Cache:   helper.Uint64ToPtr(100),
				ECCErrorsL2Cache:   helper.Uint64ToPtr(100),
				ECCErrorsDevice:    helper.Uint64ToPtr(100),
			},
			ExpectedResult: &device.DeviceStats{
				Summary: &structs.StatValue{
					Unit:              MemoryStateUnit,
					Desc:              MemoryStateDesc,
					IntNumeratorVal:   helper.Int64ToPtr(1),
					IntDenominatorVal: helper.Int64ToPtr(1),
				},
				Stats: &structs.StatObject{
					Attributes: map[string]*structs.StatValue{
						PowerUsageAttr: {
							Unit:              PowerUsageUnit,
							Desc:              PowerUsageDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(1),
						},
						GPUUtilizationAttr: {
							Unit:            GPUUtilizationUnit,
							Desc:            GPUUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						MemoryUtilizationAttr: {
							Unit:      MemoryUtilizationUnit,
							Desc:      MemoryUtilizationDesc,
							StringVal: helper.StringToPtr(notAvailable),
						},
						EncoderUtilizationAttr: {
							Unit:            EncoderUtilizationUnit,
							Desc:            EncoderUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						DecoderUtilizationAttr: {
							Unit:            DecoderUtilizationUnit,
							Desc:            DecoderUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						TemperatureAttr: {
							Unit:            TemperatureUnit,
							Desc:            TemperatureDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						MemoryStateAttr: {
							Unit:              MemoryStateUnit,
							Desc:              MemoryStateDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(1),
						},
						BAR1StateAttr: {
							Unit:              BAR1StateUnit,
							Desc:              BAR1StateDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(256),
						},
						ECCErrorsL1CacheAttr: {
							Unit:            ECCErrorsL1CacheUnit,
							Desc:            ECCErrorsL1CacheDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
						ECCErrorsL2CacheAttr: {
							Unit:            ECCErrorsL2CacheUnit,
							Desc:            ECCErrorsL2CacheDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
						ECCErrorsDeviceAttr: {
							Unit:            ECCErrorsDeviceUnit,
							Desc:            ECCErrorsDeviceDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
					},
				},
				Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
			},
		},
		{
			Name:      "EncoderUtilization is nil",
			Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
			ItemStat: &nvml.StatsData{
				DeviceData: &nvml.DeviceData{
					UUID:       "UUID1",
					DeviceName: helper.StringToPtr("DeviceName1"),
					MemoryMiB:  helper.Uint64ToPtr(1),
					PowerW:     helper.UintToPtr(1),
					BAR1MiB:    helper.Uint64ToPtr(256),
				},
				PowerUsageW:        helper.UintToPtr(1),
				GPUUtilization:     helper.UintToPtr(1),
				MemoryUtilization:  helper.UintToPtr(1),
				EncoderUtilization: nil,
				DecoderUtilization: helper.UintToPtr(1),
				TemperatureC:       helper.UintToPtr(1),
				UsedMemoryMiB:      helper.Uint64ToPtr(1),
				BAR1UsedMiB:        helper.Uint64ToPtr(1),
				ECCErrorsL1Cache:   helper.Uint64ToPtr(100),
				ECCErrorsL2Cache:   helper.Uint64ToPtr(100),
				ECCErrorsDevice:    helper.Uint64ToPtr(100),
			},
			ExpectedResult: &device.DeviceStats{
				Summary: &structs.StatValue{
					Unit:              MemoryStateUnit,
					Desc:              MemoryStateDesc,
					IntNumeratorVal:   helper.Int64ToPtr(1),
					IntDenominatorVal: helper.Int64ToPtr(1),
				},
				Stats: &structs.StatObject{
					Attributes: map[string]*structs.StatValue{
						PowerUsageAttr: {
							Unit:              PowerUsageUnit,
							Desc:              PowerUsageDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(1),
						},
						GPUUtilizationAttr: {
							Unit:            GPUUtilizationUnit,
							Desc:            GPUUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						MemoryUtilizationAttr: {
							Unit:            MemoryUtilizationUnit,
							Desc:            MemoryUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						EncoderUtilizationAttr: {
							Unit:      EncoderUtilizationUnit,
							Desc:      EncoderUtilizationDesc,
							StringVal: helper.StringToPtr(notAvailable),
						},
						DecoderUtilizationAttr: {
							Unit:            DecoderUtilizationUnit,
							Desc:            DecoderUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						TemperatureAttr: {
							Unit:            TemperatureUnit,
							Desc:            TemperatureDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						MemoryStateAttr: {
							Unit:              MemoryStateUnit,
							Desc:              MemoryStateDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(1),
						},
						BAR1StateAttr: {
							Unit:              BAR1StateUnit,
							Desc:              BAR1StateDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(256),
						},
						ECCErrorsL1CacheAttr: {
							Unit:            ECCErrorsL1CacheUnit,
							Desc:            ECCErrorsL1CacheDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
						ECCErrorsL2CacheAttr: {
							Unit:            ECCErrorsL2CacheUnit,
							Desc:            ECCErrorsL2CacheDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
						ECCErrorsDeviceAttr: {
							Unit:            ECCErrorsDeviceUnit,
							Desc:            ECCErrorsDeviceDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
					},
				},
				Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
			},
		},
		{
			Name:      "DecoderUtilization is nil",
			Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
			ItemStat: &nvml.StatsData{
				DeviceData: &nvml.DeviceData{
					UUID:       "UUID1",
					DeviceName: helper.StringToPtr("DeviceName1"),
					MemoryMiB:  helper.Uint64ToPtr(1),
					PowerW:     helper.UintToPtr(1),
					BAR1MiB:    helper.Uint64ToPtr(256),
				},
				PowerUsageW:        helper.UintToPtr(1),
				GPUUtilization:     helper.UintToPtr(1),
				MemoryUtilization:  helper.UintToPtr(1),
				EncoderUtilization: helper.UintToPtr(1),
				DecoderUtilization: nil,
				TemperatureC:       helper.UintToPtr(1),
				UsedMemoryMiB:      helper.Uint64ToPtr(1),
				BAR1UsedMiB:        helper.Uint64ToPtr(1),
				ECCErrorsL1Cache:   helper.Uint64ToPtr(100),
				ECCErrorsL2Cache:   helper.Uint64ToPtr(100),
				ECCErrorsDevice:    helper.Uint64ToPtr(100),
			},
			ExpectedResult: &device.DeviceStats{
				Summary: &structs.StatValue{
					Unit:              MemoryStateUnit,
					Desc:              MemoryStateDesc,
					IntNumeratorVal:   helper.Int64ToPtr(1),
					IntDenominatorVal: helper.Int64ToPtr(1),
				},
				Stats: &structs.StatObject{
					Attributes: map[string]*structs.StatValue{
						PowerUsageAttr: {
							Unit:              PowerUsageUnit,
							Desc:              PowerUsageDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(1),
						},
						GPUUtilizationAttr: {
							Unit:            GPUUtilizationUnit,
							Desc:            GPUUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						MemoryUtilizationAttr: {
							Unit:            MemoryUtilizationUnit,
							Desc:            MemoryUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						EncoderUtilizationAttr: {
							Unit:            EncoderUtilizationUnit,
							Desc:            EncoderUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						DecoderUtilizationAttr: {
							Unit:      DecoderUtilizationUnit,
							Desc:      DecoderUtilizationDesc,
							StringVal: helper.StringToPtr(notAvailable),
						},
						TemperatureAttr: {
							Unit:            TemperatureUnit,
							Desc:            TemperatureDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						MemoryStateAttr: {
							Unit:              MemoryStateUnit,
							Desc:              MemoryStateDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(1),
						},
						BAR1StateAttr: {
							Unit:              BAR1StateUnit,
							Desc:              BAR1StateDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(256),
						},
						ECCErrorsL1CacheAttr: {
							Unit:            ECCErrorsL1CacheUnit,
							Desc:            ECCErrorsL1CacheDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
						ECCErrorsL2CacheAttr: {
							Unit:            ECCErrorsL2CacheUnit,
							Desc:            ECCErrorsL2CacheDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
						ECCErrorsDeviceAttr: {
							Unit:            ECCErrorsDeviceUnit,
							Desc:            ECCErrorsDeviceDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
					},
				},
				Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
			},
		},
		{
			Name:      "Temperature is nil",
			Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
			ItemStat: &nvml.StatsData{
				DeviceData: &nvml.DeviceData{
					UUID:       "UUID1",
					DeviceName: helper.StringToPtr("DeviceName1"),
					MemoryMiB:  helper.Uint64ToPtr(1),
					PowerW:     helper.UintToPtr(1),
					BAR1MiB:    helper.Uint64ToPtr(256),
				},
				PowerUsageW:        helper.UintToPtr(1),
				GPUUtilization:     helper.UintToPtr(1),
				MemoryUtilization:  helper.UintToPtr(1),
				EncoderUtilization: helper.UintToPtr(1),
				DecoderUtilization: helper.UintToPtr(1),
				TemperatureC:       nil,
				UsedMemoryMiB:      helper.Uint64ToPtr(1),
				BAR1UsedMiB:        helper.Uint64ToPtr(1),
				ECCErrorsL1Cache:   helper.Uint64ToPtr(100),
				ECCErrorsL2Cache:   helper.Uint64ToPtr(100),
				ECCErrorsDevice:    helper.Uint64ToPtr(100),
			},
			ExpectedResult: &device.DeviceStats{
				Summary: &structs.StatValue{
					Unit:              MemoryStateUnit,
					Desc:              MemoryStateDesc,
					IntNumeratorVal:   helper.Int64ToPtr(1),
					IntDenominatorVal: helper.Int64ToPtr(1),
				},
				Stats: &structs.StatObject{
					Attributes: map[string]*structs.StatValue{
						PowerUsageAttr: {
							Unit:              PowerUsageUnit,
							Desc:              PowerUsageDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(1),
						},
						GPUUtilizationAttr: {
							Unit:            GPUUtilizationUnit,
							Desc:            GPUUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						MemoryUtilizationAttr: {
							Unit:            MemoryUtilizationUnit,
							Desc:            MemoryUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						EncoderUtilizationAttr: {
							Unit:            EncoderUtilizationUnit,
							Desc:            EncoderUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						DecoderUtilizationAttr: {
							Unit:            DecoderUtilizationUnit,
							Desc:            DecoderUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						TemperatureAttr: {
							Unit:      TemperatureUnit,
							Desc:      TemperatureDesc,
							StringVal: helper.StringToPtr(notAvailable),
						},
						MemoryStateAttr: {
							Unit:              MemoryStateUnit,
							Desc:              MemoryStateDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(1),
						},
						BAR1StateAttr: {
							Unit:              BAR1StateUnit,
							Desc:              BAR1StateDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(256),
						},
						ECCErrorsL1CacheAttr: {
							Unit:            ECCErrorsL1CacheUnit,
							Desc:            ECCErrorsL1CacheDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
						ECCErrorsL2CacheAttr: {
							Unit:            ECCErrorsL2CacheUnit,
							Desc:            ECCErrorsL2CacheDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
						ECCErrorsDeviceAttr: {
							Unit:            ECCErrorsDeviceUnit,
							Desc:            ECCErrorsDeviceDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
					},
				},
				Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
			},
		},
		{
			Name:      "UsedMemoryMiB is nil",
			Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
			ItemStat: &nvml.StatsData{
				DeviceData: &nvml.DeviceData{
					UUID:       "UUID1",
					DeviceName: helper.StringToPtr("DeviceName1"),
					MemoryMiB:  helper.Uint64ToPtr(1),
					PowerW:     helper.UintToPtr(1),
					BAR1MiB:    helper.Uint64ToPtr(256),
				},
				PowerUsageW:        helper.UintToPtr(1),
				GPUUtilization:     helper.UintToPtr(1),
				MemoryUtilization:  helper.UintToPtr(1),
				EncoderUtilization: helper.UintToPtr(1),
				DecoderUtilization: helper.UintToPtr(1),
				TemperatureC:       helper.UintToPtr(1),
				UsedMemoryMiB:      nil,
				BAR1UsedMiB:        helper.Uint64ToPtr(1),
				ECCErrorsL1Cache:   helper.Uint64ToPtr(100),
				ECCErrorsL2Cache:   helper.Uint64ToPtr(100),
				ECCErrorsDevice:    helper.Uint64ToPtr(100),
			},
			ExpectedResult: &device.DeviceStats{
				Summary: &structs.StatValue{
					Unit:      MemoryStateUnit,
					Desc:      MemoryStateDesc,
					StringVal: helper.StringToPtr(notAvailable),
				},
				Stats: &structs.StatObject{
					Attributes: map[string]*structs.StatValue{
						PowerUsageAttr: {
							Unit:              PowerUsageUnit,
							Desc:              PowerUsageDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(1),
						},
						GPUUtilizationAttr: {
							Unit:            GPUUtilizationUnit,
							Desc:            GPUUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						MemoryUtilizationAttr: {
							Unit:            MemoryUtilizationUnit,
							Desc:            MemoryUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						EncoderUtilizationAttr: {
							Unit:            EncoderUtilizationUnit,
							Desc:            EncoderUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						DecoderUtilizationAttr: {
							Unit:            DecoderUtilizationUnit,
							Desc:            DecoderUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						TemperatureAttr: {
							Unit:            TemperatureUnit,
							Desc:            TemperatureDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						MemoryStateAttr: {
							Unit:      MemoryStateUnit,
							Desc:      MemoryStateDesc,
							StringVal: helper.StringToPtr(notAvailable),
						},
						BAR1StateAttr: {
							Unit:              BAR1StateUnit,
							Desc:              BAR1StateDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(256),
						},
						ECCErrorsL1CacheAttr: {
							Unit:            ECCErrorsL1CacheUnit,
							Desc:            ECCErrorsL1CacheDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
						ECCErrorsL2CacheAttr: {
							Unit:            ECCErrorsL2CacheUnit,
							Desc:            ECCErrorsL2CacheDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
						ECCErrorsDeviceAttr: {
							Unit:            ECCErrorsDeviceUnit,
							Desc:            ECCErrorsDeviceDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
					},
				},
				Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
			},
		},
		{
			Name:      "MemoryMiB is nil",
			Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
			ItemStat: &nvml.StatsData{
				DeviceData: &nvml.DeviceData{
					UUID:       "UUID1",
					DeviceName: helper.StringToPtr("DeviceName1"),
					MemoryMiB:  nil,
					PowerW:     helper.UintToPtr(1),
					BAR1MiB:    helper.Uint64ToPtr(256),
				},
				PowerUsageW:        helper.UintToPtr(1),
				GPUUtilization:     helper.UintToPtr(1),
				MemoryUtilization:  helper.UintToPtr(1),
				EncoderUtilization: helper.UintToPtr(1),
				DecoderUtilization: helper.UintToPtr(1),
				TemperatureC:       helper.UintToPtr(1),
				UsedMemoryMiB:      helper.Uint64ToPtr(1),
				BAR1UsedMiB:        helper.Uint64ToPtr(1),
				ECCErrorsL1Cache:   helper.Uint64ToPtr(100),
				ECCErrorsL2Cache:   helper.Uint64ToPtr(100),
				ECCErrorsDevice:    helper.Uint64ToPtr(100),
			},
			ExpectedResult: &device.DeviceStats{
				Summary: &structs.StatValue{
					Unit:      MemoryStateUnit,
					Desc:      MemoryStateDesc,
					StringVal: helper.StringToPtr(notAvailable),
				},
				Stats: &structs.StatObject{
					Attributes: map[string]*structs.StatValue{
						PowerUsageAttr: {
							Unit:              PowerUsageUnit,
							Desc:              PowerUsageDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(1),
						},
						GPUUtilizationAttr: {
							Unit:            GPUUtilizationUnit,
							Desc:            GPUUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						MemoryUtilizationAttr: {
							Unit:            MemoryUtilizationUnit,
							Desc:            MemoryUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						EncoderUtilizationAttr: {
							Unit:            EncoderUtilizationUnit,
							Desc:            EncoderUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						DecoderUtilizationAttr: {
							Unit:            DecoderUtilizationUnit,
							Desc:            DecoderUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						TemperatureAttr: {
							Unit:            TemperatureUnit,
							Desc:            TemperatureDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						MemoryStateAttr: {
							Unit:      MemoryStateUnit,
							Desc:      MemoryStateDesc,
							StringVal: helper.StringToPtr(notAvailable),
						},
						BAR1StateAttr: {
							Unit:              BAR1StateUnit,
							Desc:              BAR1StateDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(256),
						},
						ECCErrorsL1CacheAttr: {
							Unit:            ECCErrorsL1CacheUnit,
							Desc:            ECCErrorsL1CacheDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
						ECCErrorsL2CacheAttr: {
							Unit:            ECCErrorsL2CacheUnit,
							Desc:            ECCErrorsL2CacheDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
						ECCErrorsDeviceAttr: {
							Unit:            ECCErrorsDeviceUnit,
							Desc:            ECCErrorsDeviceDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
					},
				},
				Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
			},
		},
		{
			Name:      "BAR1UsedMiB is nil",
			Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
			ItemStat: &nvml.StatsData{
				DeviceData: &nvml.DeviceData{
					UUID:       "UUID1",
					DeviceName: helper.StringToPtr("DeviceName1"),
					MemoryMiB:  helper.Uint64ToPtr(1),
					PowerW:     helper.UintToPtr(1),
					BAR1MiB:    helper.Uint64ToPtr(256),
				},
				PowerUsageW:        helper.UintToPtr(1),
				GPUUtilization:     helper.UintToPtr(1),
				MemoryUtilization:  helper.UintToPtr(1),
				EncoderUtilization: helper.UintToPtr(1),
				DecoderUtilization: helper.UintToPtr(1),
				TemperatureC:       helper.UintToPtr(1),
				UsedMemoryMiB:      helper.Uint64ToPtr(1),
				BAR1UsedMiB:        nil,
				ECCErrorsL1Cache:   helper.Uint64ToPtr(100),
				ECCErrorsL2Cache:   helper.Uint64ToPtr(100),
				ECCErrorsDevice:    helper.Uint64ToPtr(100),
			},
			ExpectedResult: &device.DeviceStats{
				Summary: &structs.StatValue{
					Unit:              MemoryStateUnit,
					Desc:              MemoryStateDesc,
					IntNumeratorVal:   helper.Int64ToPtr(1),
					IntDenominatorVal: helper.Int64ToPtr(1),
				},
				Stats: &structs.StatObject{
					Attributes: map[string]*structs.StatValue{
						PowerUsageAttr: {
							Unit:              PowerUsageUnit,
							Desc:              PowerUsageDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(1),
						},
						GPUUtilizationAttr: {
							Unit:            GPUUtilizationUnit,
							Desc:            GPUUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						MemoryUtilizationAttr: {
							Unit:            MemoryUtilizationUnit,
							Desc:            MemoryUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						EncoderUtilizationAttr: {
							Unit:            EncoderUtilizationUnit,
							Desc:            EncoderUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						DecoderUtilizationAttr: {
							Unit:            DecoderUtilizationUnit,
							Desc:            DecoderUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						TemperatureAttr: {
							Unit:            TemperatureUnit,
							Desc:            TemperatureDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						MemoryStateAttr: {
							Unit:              MemoryStateUnit,
							Desc:              MemoryStateDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(1),
						},
						BAR1StateAttr: {
							Unit:      BAR1StateUnit,
							Desc:      BAR1StateDesc,
							StringVal: helper.StringToPtr(notAvailable),
						},
						ECCErrorsL1CacheAttr: {
							Unit:            ECCErrorsL1CacheUnit,
							Desc:            ECCErrorsL1CacheDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
						ECCErrorsL2CacheAttr: {
							Unit:            ECCErrorsL2CacheUnit,
							Desc:            ECCErrorsL2CacheDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
						ECCErrorsDeviceAttr: {
							Unit:            ECCErrorsDeviceUnit,
							Desc:            ECCErrorsDeviceDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
					},
				},
				Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
			},
		},
		{
			Name:      "BAR1MiB is nil",
			Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
			ItemStat: &nvml.StatsData{
				DeviceData: &nvml.DeviceData{
					UUID:       "UUID1",
					DeviceName: helper.StringToPtr("DeviceName1"),
					MemoryMiB:  helper.Uint64ToPtr(1),
					PowerW:     helper.UintToPtr(1),
					BAR1MiB:    nil,
				},
				PowerUsageW:        helper.UintToPtr(1),
				GPUUtilization:     helper.UintToPtr(1),
				MemoryUtilization:  helper.UintToPtr(1),
				EncoderUtilization: helper.UintToPtr(1),
				DecoderUtilization: helper.UintToPtr(1),
				TemperatureC:       helper.UintToPtr(1),
				UsedMemoryMiB:      helper.Uint64ToPtr(1),
				BAR1UsedMiB:        helper.Uint64ToPtr(1),
				ECCErrorsL1Cache:   helper.Uint64ToPtr(100),
				ECCErrorsL2Cache:   helper.Uint64ToPtr(100),
				ECCErrorsDevice:    helper.Uint64ToPtr(100),
			},
			ExpectedResult: &device.DeviceStats{
				Summary: &structs.StatValue{
					Unit:              MemoryStateUnit,
					Desc:              MemoryStateDesc,
					IntNumeratorVal:   helper.Int64ToPtr(1),
					IntDenominatorVal: helper.Int64ToPtr(1),
				},
				Stats: &structs.StatObject{
					Attributes: map[string]*structs.StatValue{
						PowerUsageAttr: {
							Unit:              PowerUsageUnit,
							Desc:              PowerUsageDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(1),
						},
						GPUUtilizationAttr: {
							Unit:            GPUUtilizationUnit,
							Desc:            GPUUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						MemoryUtilizationAttr: {
							Unit:            MemoryUtilizationUnit,
							Desc:            MemoryUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						EncoderUtilizationAttr: {
							Unit:            EncoderUtilizationUnit,
							Desc:            EncoderUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						DecoderUtilizationAttr: {
							Unit:            DecoderUtilizationUnit,
							Desc:            DecoderUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						TemperatureAttr: {
							Unit:            TemperatureUnit,
							Desc:            TemperatureDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						MemoryStateAttr: {
							Unit:              MemoryStateUnit,
							Desc:              MemoryStateDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(1),
						},
						BAR1StateAttr: {
							Unit:      BAR1StateUnit,
							Desc:      BAR1StateDesc,
							StringVal: helper.StringToPtr(notAvailable),
						},
						ECCErrorsL1CacheAttr: {
							Unit:            ECCErrorsL1CacheUnit,
							Desc:            ECCErrorsL1CacheDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
						ECCErrorsL2CacheAttr: {
							Unit:            ECCErrorsL2CacheUnit,
							Desc:            ECCErrorsL2CacheDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
						ECCErrorsDeviceAttr: {
							Unit:            ECCErrorsDeviceUnit,
							Desc:            ECCErrorsDeviceDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
					},
				},
				Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
			},
		},
		{
			Name:      "ECCErrorsL1Cache is nil",
			Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
			ItemStat: &nvml.StatsData{
				DeviceData: &nvml.DeviceData{
					UUID:       "UUID1",
					DeviceName: helper.StringToPtr("DeviceName1"),
					MemoryMiB:  helper.Uint64ToPtr(1),
					PowerW:     helper.UintToPtr(1),
					BAR1MiB:    helper.Uint64ToPtr(256),
				},
				PowerUsageW:        helper.UintToPtr(1),
				GPUUtilization:     helper.UintToPtr(1),
				MemoryUtilization:  helper.UintToPtr(1),
				EncoderUtilization: helper.UintToPtr(1),
				DecoderUtilization: helper.UintToPtr(1),
				TemperatureC:       helper.UintToPtr(1),
				UsedMemoryMiB:      helper.Uint64ToPtr(1),
				BAR1UsedMiB:        helper.Uint64ToPtr(1),
				ECCErrorsL1Cache:   nil,
				ECCErrorsL2Cache:   helper.Uint64ToPtr(100),
				ECCErrorsDevice:    helper.Uint64ToPtr(100),
			},
			ExpectedResult: &device.DeviceStats{
				Summary: &structs.StatValue{
					Unit:              MemoryStateUnit,
					Desc:              MemoryStateDesc,
					IntNumeratorVal:   helper.Int64ToPtr(1),
					IntDenominatorVal: helper.Int64ToPtr(1),
				},
				Stats: &structs.StatObject{
					Attributes: map[string]*structs.StatValue{
						PowerUsageAttr: {
							Unit:              PowerUsageUnit,
							Desc:              PowerUsageDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(1),
						},
						GPUUtilizationAttr: {
							Unit:            GPUUtilizationUnit,
							Desc:            GPUUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						MemoryUtilizationAttr: {
							Unit:            MemoryUtilizationUnit,
							Desc:            MemoryUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						EncoderUtilizationAttr: {
							Unit:            EncoderUtilizationUnit,
							Desc:            EncoderUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						DecoderUtilizationAttr: {
							Unit:            DecoderUtilizationUnit,
							Desc:            DecoderUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						TemperatureAttr: {
							Unit:            TemperatureUnit,
							Desc:            TemperatureDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						MemoryStateAttr: {
							Unit:              MemoryStateUnit,
							Desc:              MemoryStateDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(1),
						},
						BAR1StateAttr: {
							Unit:              BAR1StateUnit,
							Desc:              BAR1StateDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(256),
						},
						ECCErrorsL1CacheAttr: {
							Unit:      ECCErrorsL1CacheUnit,
							Desc:      ECCErrorsL1CacheDesc,
							StringVal: helper.StringToPtr(notAvailable),
						},
						ECCErrorsL2CacheAttr: {
							Unit:            ECCErrorsL2CacheUnit,
							Desc:            ECCErrorsL2CacheDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
						ECCErrorsDeviceAttr: {
							Unit:            ECCErrorsDeviceUnit,
							Desc:            ECCErrorsDeviceDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
					},
				},
				Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
			},
		},
		{
			Name:      "ECCErrorsL2Cache is nil",
			Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
			ItemStat: &nvml.StatsData{
				DeviceData: &nvml.DeviceData{
					UUID:       "UUID1",
					DeviceName: helper.StringToPtr("DeviceName1"),
					MemoryMiB:  helper.Uint64ToPtr(1),
					PowerW:     helper.UintToPtr(1),
					BAR1MiB:    helper.Uint64ToPtr(256),
				},
				PowerUsageW:        helper.UintToPtr(1),
				GPUUtilization:     helper.UintToPtr(1),
				MemoryUtilization:  helper.UintToPtr(1),
				EncoderUtilization: helper.UintToPtr(1),
				DecoderUtilization: helper.UintToPtr(1),
				TemperatureC:       helper.UintToPtr(1),
				UsedMemoryMiB:      helper.Uint64ToPtr(1),
				BAR1UsedMiB:        helper.Uint64ToPtr(1),
				ECCErrorsL1Cache:   helper.Uint64ToPtr(100),
				ECCErrorsL2Cache:   nil,
				ECCErrorsDevice:    helper.Uint64ToPtr(100),
			},
			ExpectedResult: &device.DeviceStats{
				Summary: &structs.StatValue{
					Unit:              MemoryStateUnit,
					Desc:              MemoryStateDesc,
					IntNumeratorVal:   helper.Int64ToPtr(1),
					IntDenominatorVal: helper.Int64ToPtr(1),
				},
				Stats: &structs.StatObject{
					Attributes: map[string]*structs.StatValue{
						PowerUsageAttr: {
							Unit:              PowerUsageUnit,
							Desc:              PowerUsageDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(1),
						},
						GPUUtilizationAttr: {
							Unit:            GPUUtilizationUnit,
							Desc:            GPUUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						MemoryUtilizationAttr: {
							Unit:            MemoryUtilizationUnit,
							Desc:            MemoryUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						EncoderUtilizationAttr: {
							Unit:            EncoderUtilizationUnit,
							Desc:            EncoderUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						DecoderUtilizationAttr: {
							Unit:            DecoderUtilizationUnit,
							Desc:            DecoderUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						TemperatureAttr: {
							Unit:            TemperatureUnit,
							Desc:            TemperatureDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						MemoryStateAttr: {
							Unit:              MemoryStateUnit,
							Desc:              MemoryStateDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(1),
						},
						BAR1StateAttr: {
							Unit:              BAR1StateUnit,
							Desc:              BAR1StateDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(256),
						},
						ECCErrorsL1CacheAttr: {
							Unit:            ECCErrorsL1CacheUnit,
							Desc:            ECCErrorsL1CacheDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
						ECCErrorsL2CacheAttr: {
							Unit:      ECCErrorsL2CacheUnit,
							Desc:      ECCErrorsL2CacheDesc,
							StringVal: helper.StringToPtr(notAvailable),
						},
						ECCErrorsDeviceAttr: {
							Unit:            ECCErrorsDeviceUnit,
							Desc:            ECCErrorsDeviceDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
					},
				},
				Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
			},
		},
		{
			Name:      "ECCErrorsDevice is nil",
			Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
			ItemStat: &nvml.StatsData{
				DeviceData: &nvml.DeviceData{
					UUID:       "UUID1",
					DeviceName: helper.StringToPtr("DeviceName1"),
					MemoryMiB:  helper.Uint64ToPtr(1),
					PowerW:     helper.UintToPtr(1),
					BAR1MiB:    helper.Uint64ToPtr(256),
				},
				PowerUsageW:        helper.UintToPtr(1),
				GPUUtilization:     helper.UintToPtr(1),
				MemoryUtilization:  helper.UintToPtr(1),
				EncoderUtilization: helper.UintToPtr(1),
				DecoderUtilization: helper.UintToPtr(1),
				TemperatureC:       helper.UintToPtr(1),
				UsedMemoryMiB:      helper.Uint64ToPtr(1),
				BAR1UsedMiB:        helper.Uint64ToPtr(1),
				ECCErrorsL1Cache:   helper.Uint64ToPtr(100),
				ECCErrorsL2Cache:   helper.Uint64ToPtr(100),
				ECCErrorsDevice:    nil,
			},
			ExpectedResult: &device.DeviceStats{
				Summary: &structs.StatValue{
					Unit:              MemoryStateUnit,
					Desc:              MemoryStateDesc,
					IntNumeratorVal:   helper.Int64ToPtr(1),
					IntDenominatorVal: helper.Int64ToPtr(1),
				},
				Stats: &structs.StatObject{
					Attributes: map[string]*structs.StatValue{
						PowerUsageAttr: {
							Unit:              PowerUsageUnit,
							Desc:              PowerUsageDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(1),
						},
						GPUUtilizationAttr: {
							Unit:            GPUUtilizationUnit,
							Desc:            GPUUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						MemoryUtilizationAttr: {
							Unit:            MemoryUtilizationUnit,
							Desc:            MemoryUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						EncoderUtilizationAttr: {
							Unit:            EncoderUtilizationUnit,
							Desc:            EncoderUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						DecoderUtilizationAttr: {
							Unit:            DecoderUtilizationUnit,
							Desc:            DecoderUtilizationDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						TemperatureAttr: {
							Unit:            TemperatureUnit,
							Desc:            TemperatureDesc,
							IntNumeratorVal: helper.Int64ToPtr(1),
						},
						MemoryStateAttr: {
							Unit:              MemoryStateUnit,
							Desc:              MemoryStateDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(1),
						},
						BAR1StateAttr: {
							Unit:              BAR1StateUnit,
							Desc:              BAR1StateDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(256),
						},
						ECCErrorsL1CacheAttr: {
							Unit:            ECCErrorsL1CacheUnit,
							Desc:            ECCErrorsL1CacheDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
						ECCErrorsL2CacheAttr: {
							Unit:            ECCErrorsL2CacheUnit,
							Desc:            ECCErrorsL2CacheDesc,
							IntNumeratorVal: helper.Int64ToPtr(100),
						},
						ECCErrorsDeviceAttr: {
							Unit:      ECCErrorsDeviceUnit,
							Desc:      ECCErrorsDeviceDesc,
							StringVal: helper.StringToPtr(notAvailable),
						},
					},
				},
				Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
			},
		},
	} {
		actualResult := statsForItem(testCase.ItemStat, testCase.Timestamp)
		require.New(t).Equal(testCase.ExpectedResult, actualResult)
	}
}

func TestStatsForGroup(t *testing.T) {
	for _, testCase := range []struct {
		Name           string
		Timestamp      time.Time
		GroupStats     []*nvml.StatsData
		GroupName      string
		ExpectedResult *device.DeviceGroupStats
	}{
		{
			Name:      "make sure that all data is transformed correctly",
			Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
			GroupName: "DeviceName1",
			GroupStats: []*nvml.StatsData{
				{
					DeviceData: &nvml.DeviceData{
						UUID:       "UUID1",
						DeviceName: helper.StringToPtr("DeviceName1"),
						MemoryMiB:  helper.Uint64ToPtr(1),
						PowerW:     helper.UintToPtr(1),
						BAR1MiB:    helper.Uint64ToPtr(256),
					},
					PowerUsageW:        helper.UintToPtr(1),
					GPUUtilization:     helper.UintToPtr(1),
					MemoryUtilization:  helper.UintToPtr(1),
					EncoderUtilization: helper.UintToPtr(1),
					DecoderUtilization: helper.UintToPtr(1),
					TemperatureC:       helper.UintToPtr(1),
					UsedMemoryMiB:      helper.Uint64ToPtr(1),
					BAR1UsedMiB:        helper.Uint64ToPtr(1),
					ECCErrorsL1Cache:   helper.Uint64ToPtr(100),
					ECCErrorsL2Cache:   helper.Uint64ToPtr(100),
					ECCErrorsDevice:    helper.Uint64ToPtr(100),
				},
				{
					DeviceData: &nvml.DeviceData{
						UUID:       "UUID2",
						DeviceName: helper.StringToPtr("DeviceName2"),
						MemoryMiB:  helper.Uint64ToPtr(2),
						PowerW:     helper.UintToPtr(2),
						BAR1MiB:    helper.Uint64ToPtr(256),
					},
					PowerUsageW:        helper.UintToPtr(2),
					GPUUtilization:     helper.UintToPtr(2),
					MemoryUtilization:  helper.UintToPtr(2),
					EncoderUtilization: helper.UintToPtr(2),
					DecoderUtilization: helper.UintToPtr(2),
					TemperatureC:       helper.UintToPtr(2),
					UsedMemoryMiB:      helper.Uint64ToPtr(2),
					BAR1UsedMiB:        helper.Uint64ToPtr(2),
					ECCErrorsL1Cache:   helper.Uint64ToPtr(200),
					ECCErrorsL2Cache:   helper.Uint64ToPtr(200),
					ECCErrorsDevice:    helper.Uint64ToPtr(200),
				},
				{
					DeviceData: &nvml.DeviceData{
						UUID:       "UUID3",
						DeviceName: helper.StringToPtr("DeviceName3"),
						MemoryMiB:  helper.Uint64ToPtr(3),
						PowerW:     helper.UintToPtr(3),
						BAR1MiB:    helper.Uint64ToPtr(256),
					},
					PowerUsageW:        helper.UintToPtr(3),
					GPUUtilization:     helper.UintToPtr(3),
					MemoryUtilization:  helper.UintToPtr(3),
					EncoderUtilization: helper.UintToPtr(3),
					DecoderUtilization: helper.UintToPtr(3),
					TemperatureC:       helper.UintToPtr(3),
					UsedMemoryMiB:      helper.Uint64ToPtr(3),
					BAR1UsedMiB:        helper.Uint64ToPtr(3),
					ECCErrorsL1Cache:   helper.Uint64ToPtr(300),
					ECCErrorsL2Cache:   helper.Uint64ToPtr(300),
					ECCErrorsDevice:    helper.Uint64ToPtr(300),
				},
			},
			ExpectedResult: &device.DeviceGroupStats{
				Vendor: vendor,
				Type:   deviceType,
				Name:   "DeviceName1",
				InstanceStats: map[string]*device.DeviceStats{
					"UUID1": {
						Summary: &structs.StatValue{
							Unit:              MemoryStateUnit,
							Desc:              MemoryStateDesc,
							IntNumeratorVal:   helper.Int64ToPtr(1),
							IntDenominatorVal: helper.Int64ToPtr(1),
						},
						Stats: &structs.StatObject{
							Attributes: map[string]*structs.StatValue{
								PowerUsageAttr: {
									Unit:              PowerUsageUnit,
									Desc:              PowerUsageDesc,
									IntNumeratorVal:   helper.Int64ToPtr(1),
									IntDenominatorVal: helper.Int64ToPtr(1),
								},
								GPUUtilizationAttr: {
									Unit:            GPUUtilizationUnit,
									Desc:            GPUUtilizationDesc,
									IntNumeratorVal: helper.Int64ToPtr(1),
								},
								MemoryUtilizationAttr: {
									Unit:            MemoryUtilizationUnit,
									Desc:            MemoryUtilizationDesc,
									IntNumeratorVal: helper.Int64ToPtr(1),
								},
								EncoderUtilizationAttr: {
									Unit:            EncoderUtilizationUnit,
									Desc:            EncoderUtilizationDesc,
									IntNumeratorVal: helper.Int64ToPtr(1),
								},
								DecoderUtilizationAttr: {
									Unit:            DecoderUtilizationUnit,
									Desc:            DecoderUtilizationDesc,
									IntNumeratorVal: helper.Int64ToPtr(1),
								},
								TemperatureAttr: {
									Unit:            TemperatureUnit,
									Desc:            TemperatureDesc,
									IntNumeratorVal: helper.Int64ToPtr(1),
								},
								MemoryStateAttr: {
									Unit:              MemoryStateUnit,
									Desc:              MemoryStateDesc,
									IntNumeratorVal:   helper.Int64ToPtr(1),
									IntDenominatorVal: helper.Int64ToPtr(1),
								},
								BAR1StateAttr: {
									Unit:              BAR1StateUnit,
									Desc:              BAR1StateDesc,
									IntNumeratorVal:   helper.Int64ToPtr(1),
									IntDenominatorVal: helper.Int64ToPtr(256),
								},
								ECCErrorsL1CacheAttr: {
									Unit:            ECCErrorsL1CacheUnit,
									Desc:            ECCErrorsL1CacheDesc,
									IntNumeratorVal: helper.Int64ToPtr(100),
								},
								ECCErrorsL2CacheAttr: {
									Unit:            ECCErrorsL2CacheUnit,
									Desc:            ECCErrorsL2CacheDesc,
									IntNumeratorVal: helper.Int64ToPtr(100),
								},
								ECCErrorsDeviceAttr: {
									Unit:            ECCErrorsDeviceUnit,
									Desc:            ECCErrorsDeviceDesc,
									IntNumeratorVal: helper.Int64ToPtr(100),
								},
							},
						},
						Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
					},
					"UUID2": {
						Summary: &structs.StatValue{
							Unit:              MemoryStateUnit,
							Desc:              MemoryStateDesc,
							IntNumeratorVal:   helper.Int64ToPtr(2),
							IntDenominatorVal: helper.Int64ToPtr(2),
						},
						Stats: &structs.StatObject{
							Attributes: map[string]*structs.StatValue{
								PowerUsageAttr: {
									Unit:              PowerUsageUnit,
									Desc:              PowerUsageDesc,
									IntNumeratorVal:   helper.Int64ToPtr(2),
									IntDenominatorVal: helper.Int64ToPtr(2),
								},
								GPUUtilizationAttr: {
									Unit:            GPUUtilizationUnit,
									Desc:            GPUUtilizationDesc,
									IntNumeratorVal: helper.Int64ToPtr(2),
								},
								MemoryUtilizationAttr: {
									Unit:            MemoryUtilizationUnit,
									Desc:            MemoryUtilizationDesc,
									IntNumeratorVal: helper.Int64ToPtr(2),
								},
								EncoderUtilizationAttr: {
									Unit:            EncoderUtilizationUnit,
									Desc:            EncoderUtilizationDesc,
									IntNumeratorVal: helper.Int64ToPtr(2),
								},
								DecoderUtilizationAttr: {
									Unit:            DecoderUtilizationUnit,
									Desc:            DecoderUtilizationDesc,
									IntNumeratorVal: helper.Int64ToPtr(2),
								},
								TemperatureAttr: {
									Unit:            TemperatureUnit,
									Desc:            TemperatureDesc,
									IntNumeratorVal: helper.Int64ToPtr(2),
								},
								MemoryStateAttr: {
									Unit:              MemoryStateUnit,
									Desc:              MemoryStateDesc,
									IntNumeratorVal:   helper.Int64ToPtr(2),
									IntDenominatorVal: helper.Int64ToPtr(2),
								},
								BAR1StateAttr: {
									Unit:              BAR1StateUnit,
									Desc:              BAR1StateDesc,
									IntNumeratorVal:   helper.Int64ToPtr(2),
									IntDenominatorVal: helper.Int64ToPtr(256),
								},
								ECCErrorsL1CacheAttr: {
									Unit:            ECCErrorsL1CacheUnit,
									Desc:            ECCErrorsL1CacheDesc,
									IntNumeratorVal: helper.Int64ToPtr(200),
								},
								ECCErrorsL2CacheAttr: {
									Unit:            ECCErrorsL2CacheUnit,
									Desc:            ECCErrorsL2CacheDesc,
									IntNumeratorVal: helper.Int64ToPtr(200),
								},
								ECCErrorsDeviceAttr: {
									Unit:            ECCErrorsDeviceUnit,
									Desc:            ECCErrorsDeviceDesc,
									IntNumeratorVal: helper.Int64ToPtr(200),
								},
							},
						},
						Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
					},
					"UUID3": {
						Summary: &structs.StatValue{
							Unit:              MemoryStateUnit,
							Desc:              MemoryStateDesc,
							IntNumeratorVal:   helper.Int64ToPtr(3),
							IntDenominatorVal: helper.Int64ToPtr(3),
						},
						Stats: &structs.StatObject{
							Attributes: map[string]*structs.StatValue{
								PowerUsageAttr: {
									Unit:              PowerUsageUnit,
									Desc:              PowerUsageDesc,
									IntNumeratorVal:   helper.Int64ToPtr(3),
									IntDenominatorVal: helper.Int64ToPtr(3),
								},
								GPUUtilizationAttr: {
									Unit:            GPUUtilizationUnit,
									Desc:            GPUUtilizationDesc,
									IntNumeratorVal: helper.Int64ToPtr(3),
								},
								MemoryUtilizationAttr: {
									Unit:            MemoryUtilizationUnit,
									Desc:            MemoryUtilizationDesc,
									IntNumeratorVal: helper.Int64ToPtr(3),
								},
								EncoderUtilizationAttr: {
									Unit:            EncoderUtilizationUnit,
									Desc:            EncoderUtilizationDesc,
									IntNumeratorVal: helper.Int64ToPtr(3),
								},
								DecoderUtilizationAttr: {
									Unit:            DecoderUtilizationUnit,
									Desc:            DecoderUtilizationDesc,
									IntNumeratorVal: helper.Int64ToPtr(3),
								},
								TemperatureAttr: {
									Unit:            TemperatureUnit,
									Desc:            TemperatureDesc,
									IntNumeratorVal: helper.Int64ToPtr(3),
								},
								MemoryStateAttr: {
									Unit:              MemoryStateUnit,
									Desc:              MemoryStateDesc,
									IntNumeratorVal:   helper.Int64ToPtr(3),
									IntDenominatorVal: helper.Int64ToPtr(3),
								},
								BAR1StateAttr: {
									Unit:              BAR1StateUnit,
									Desc:              BAR1StateDesc,
									IntNumeratorVal:   helper.Int64ToPtr(3),
									IntDenominatorVal: helper.Int64ToPtr(256),
								},
								ECCErrorsL1CacheAttr: {
									Unit:            ECCErrorsL1CacheUnit,
									Desc:            ECCErrorsL1CacheDesc,
									IntNumeratorVal: helper.Int64ToPtr(300),
								},
								ECCErrorsL2CacheAttr: {
									Unit:            ECCErrorsL2CacheUnit,
									Desc:            ECCErrorsL2CacheDesc,
									IntNumeratorVal: helper.Int64ToPtr(300),
								},
								ECCErrorsDeviceAttr: {
									Unit:            ECCErrorsDeviceUnit,
									Desc:            ECCErrorsDeviceDesc,
									IntNumeratorVal: helper.Int64ToPtr(300),
								},
							},
						},
						Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
					},
				},
			},
		},
	} {
		actualResult := statsForGroup(testCase.GroupName, testCase.GroupStats, testCase.Timestamp)
		require.New(t).Equal(testCase.ExpectedResult, actualResult)
	}
}

func TestWriteStatsToChannel(t *testing.T) {
	for _, testCase := range []struct {
		Name                   string
		ExpectedWriteToChannel *device.StatsResponse
		Timestamp              time.Time
		Device                 *NvidiaDevice
	}{
		{
			Name:      "NVML wrapper returns error",
			Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
			ExpectedWriteToChannel: &device.StatsResponse{
				Error: errors.New(""),
			},
			Device: &NvidiaDevice{
				nvmlClient: &MockNvmlClient{
					StatsError: errors.New(""),
				},
				logger: hclog.NewNullLogger(),
			},
		},
		{
			Name:      "Check that stats with multiple DeviceNames are assigned to different groups",
			Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
			Device: &NvidiaDevice{
				devices: map[string]struct{}{
					"UUID1": {},
					"UUID2": {},
					"UUID3": {},
				},
				nvmlClient: &MockNvmlClient{
					StatsResponseReturned: []*nvml.StatsData{
						{
							DeviceData: &nvml.DeviceData{
								UUID:       "UUID1",
								DeviceName: helper.StringToPtr("DeviceName1"),
								MemoryMiB:  helper.Uint64ToPtr(1),
								PowerW:     helper.UintToPtr(1),
								BAR1MiB:    helper.Uint64ToPtr(256),
							},
							PowerUsageW:        helper.UintToPtr(1),
							GPUUtilization:     helper.UintToPtr(1),
							MemoryUtilization:  helper.UintToPtr(1),
							EncoderUtilization: helper.UintToPtr(1),
							DecoderUtilization: helper.UintToPtr(1),
							TemperatureC:       helper.UintToPtr(1),
							UsedMemoryMiB:      helper.Uint64ToPtr(1),
							BAR1UsedMiB:        helper.Uint64ToPtr(1),
							ECCErrorsL1Cache:   helper.Uint64ToPtr(100),
							ECCErrorsL2Cache:   helper.Uint64ToPtr(100),
							ECCErrorsDevice:    helper.Uint64ToPtr(100),
						},
						{
							DeviceData: &nvml.DeviceData{
								UUID:       "UUID2",
								DeviceName: helper.StringToPtr("DeviceName2"),
								MemoryMiB:  helper.Uint64ToPtr(2),
								PowerW:     helper.UintToPtr(2),
								BAR1MiB:    helper.Uint64ToPtr(256),
							},
							PowerUsageW:        helper.UintToPtr(2),
							GPUUtilization:     helper.UintToPtr(2),
							MemoryUtilization:  helper.UintToPtr(2),
							EncoderUtilization: helper.UintToPtr(2),
							DecoderUtilization: helper.UintToPtr(2),
							TemperatureC:       helper.UintToPtr(2),
							UsedMemoryMiB:      helper.Uint64ToPtr(2),
							BAR1UsedMiB:        helper.Uint64ToPtr(2),
							ECCErrorsL1Cache:   helper.Uint64ToPtr(200),
							ECCErrorsL2Cache:   helper.Uint64ToPtr(200),
							ECCErrorsDevice:    helper.Uint64ToPtr(200),
						},
						{
							DeviceData: &nvml.DeviceData{
								UUID:       "UUID3",
								DeviceName: helper.StringToPtr("DeviceName3"),
								MemoryMiB:  helper.Uint64ToPtr(3),
								PowerW:     helper.UintToPtr(3),
								BAR1MiB:    helper.Uint64ToPtr(256),
							},
							PowerUsageW:        helper.UintToPtr(3),
							GPUUtilization:     helper.UintToPtr(3),
							MemoryUtilization:  helper.UintToPtr(3),
							EncoderUtilization: helper.UintToPtr(3),
							DecoderUtilization: helper.UintToPtr(3),
							TemperatureC:       helper.UintToPtr(3),
							UsedMemoryMiB:      helper.Uint64ToPtr(3),
							BAR1UsedMiB:        helper.Uint64ToPtr(3),
							ECCErrorsL1Cache:   helper.Uint64ToPtr(300),
							ECCErrorsL2Cache:   helper.Uint64ToPtr(300),
							ECCErrorsDevice:    helper.Uint64ToPtr(300),
						},
					},
				},
				logger: hclog.NewNullLogger(),
			},
			ExpectedWriteToChannel: &device.StatsResponse{
				Groups: []*device.DeviceGroupStats{
					{
						Vendor: vendor,
						Type:   deviceType,
						Name:   "DeviceName1",
						InstanceStats: map[string]*device.DeviceStats{
							"UUID1": {
								Summary: &structs.StatValue{
									Unit:              MemoryStateUnit,
									Desc:              MemoryStateDesc,
									IntNumeratorVal:   helper.Int64ToPtr(1),
									IntDenominatorVal: helper.Int64ToPtr(1),
								},
								Stats: &structs.StatObject{
									Attributes: map[string]*structs.StatValue{
										PowerUsageAttr: {
											Unit:              PowerUsageUnit,
											Desc:              PowerUsageDesc,
											IntNumeratorVal:   helper.Int64ToPtr(1),
											IntDenominatorVal: helper.Int64ToPtr(1),
										},
										GPUUtilizationAttr: {
											Unit:            GPUUtilizationUnit,
											Desc:            GPUUtilizationDesc,
											IntNumeratorVal: helper.Int64ToPtr(1),
										},
										MemoryUtilizationAttr: {
											Unit:            MemoryUtilizationUnit,
											Desc:            MemoryUtilizationDesc,
											IntNumeratorVal: helper.Int64ToPtr(1),
										},
										EncoderUtilizationAttr: {
											Unit:            EncoderUtilizationUnit,
											Desc:            EncoderUtilizationDesc,
											IntNumeratorVal: helper.Int64ToPtr(1),
										},
										DecoderUtilizationAttr: {
											Unit:            DecoderUtilizationUnit,
											Desc:            DecoderUtilizationDesc,
											IntNumeratorVal: helper.Int64ToPtr(1),
										},
										TemperatureAttr: {
											Unit:            TemperatureUnit,
											Desc:            TemperatureDesc,
											IntNumeratorVal: helper.Int64ToPtr(1),
										},
										MemoryStateAttr: {
											Unit:              MemoryStateUnit,
											Desc:              MemoryStateDesc,
											IntNumeratorVal:   helper.Int64ToPtr(1),
											IntDenominatorVal: helper.Int64ToPtr(1),
										},
										BAR1StateAttr: {
											Unit:              BAR1StateUnit,
											Desc:              BAR1StateDesc,
											IntNumeratorVal:   helper.Int64ToPtr(1),
											IntDenominatorVal: helper.Int64ToPtr(256),
										},
										ECCErrorsL1CacheAttr: {
											Unit:            ECCErrorsL1CacheUnit,
											Desc:            ECCErrorsL1CacheDesc,
											IntNumeratorVal: helper.Int64ToPtr(100),
										},
										ECCErrorsL2CacheAttr: {
											Unit:            ECCErrorsL2CacheUnit,
											Desc:            ECCErrorsL2CacheDesc,
											IntNumeratorVal: helper.Int64ToPtr(100),
										},
										ECCErrorsDeviceAttr: {
											Unit:            ECCErrorsDeviceUnit,
											Desc:            ECCErrorsDeviceDesc,
											IntNumeratorVal: helper.Int64ToPtr(100),
										},
									},
								},
								Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
							},
						},
					},
					{
						Vendor: vendor,
						Type:   deviceType,
						Name:   "DeviceName2",
						InstanceStats: map[string]*device.DeviceStats{
							"UUID2": {
								Summary: &structs.StatValue{
									Unit:              MemoryStateUnit,
									Desc:              MemoryStateDesc,
									IntNumeratorVal:   helper.Int64ToPtr(2),
									IntDenominatorVal: helper.Int64ToPtr(2),
								},
								Stats: &structs.StatObject{
									Attributes: map[string]*structs.StatValue{
										PowerUsageAttr: {
											Unit:              PowerUsageUnit,
											Desc:              PowerUsageDesc,
											IntNumeratorVal:   helper.Int64ToPtr(2),
											IntDenominatorVal: helper.Int64ToPtr(2),
										},
										GPUUtilizationAttr: {
											Unit:            GPUUtilizationUnit,
											Desc:            GPUUtilizationDesc,
											IntNumeratorVal: helper.Int64ToPtr(2),
										},
										MemoryUtilizationAttr: {
											Unit:            MemoryUtilizationUnit,
											Desc:            MemoryUtilizationDesc,
											IntNumeratorVal: helper.Int64ToPtr(2),
										},
										EncoderUtilizationAttr: {
											Unit:            EncoderUtilizationUnit,
											Desc:            EncoderUtilizationDesc,
											IntNumeratorVal: helper.Int64ToPtr(2),
										},
										DecoderUtilizationAttr: {
											Unit:            DecoderUtilizationUnit,
											Desc:            DecoderUtilizationDesc,
											IntNumeratorVal: helper.Int64ToPtr(2),
										},
										TemperatureAttr: {
											Unit:            TemperatureUnit,
											Desc:            TemperatureDesc,
											IntNumeratorVal: helper.Int64ToPtr(2),
										},
										MemoryStateAttr: {
											Unit:              MemoryStateUnit,
											Desc:              MemoryStateDesc,
											IntNumeratorVal:   helper.Int64ToPtr(2),
											IntDenominatorVal: helper.Int64ToPtr(2),
										},
										BAR1StateAttr: {
											Unit:              BAR1StateUnit,
											Desc:              BAR1StateDesc,
											IntNumeratorVal:   helper.Int64ToPtr(2),
											IntDenominatorVal: helper.Int64ToPtr(256),
										},
										ECCErrorsL1CacheAttr: {
											Unit:            ECCErrorsL1CacheUnit,
											Desc:            ECCErrorsL1CacheDesc,
											IntNumeratorVal: helper.Int64ToPtr(200),
										},
										ECCErrorsL2CacheAttr: {
											Unit:            ECCErrorsL2CacheUnit,
											Desc:            ECCErrorsL2CacheDesc,
											IntNumeratorVal: helper.Int64ToPtr(200),
										},
										ECCErrorsDeviceAttr: {
											Unit:            ECCErrorsDeviceUnit,
											Desc:            ECCErrorsDeviceDesc,
											IntNumeratorVal: helper.Int64ToPtr(200),
										},
									},
								},
								Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
							},
						},
					},
					{
						Vendor: vendor,
						Type:   deviceType,
						Name:   "DeviceName3",
						InstanceStats: map[string]*device.DeviceStats{
							"UUID3": {
								Summary: &structs.StatValue{
									Unit:              MemoryStateUnit,
									Desc:              MemoryStateDesc,
									IntNumeratorVal:   helper.Int64ToPtr(3),
									IntDenominatorVal: helper.Int64ToPtr(3),
								},
								Stats: &structs.StatObject{
									Attributes: map[string]*structs.StatValue{
										PowerUsageAttr: {
											Unit:              PowerUsageUnit,
											Desc:              PowerUsageDesc,
											IntNumeratorVal:   helper.Int64ToPtr(3),
											IntDenominatorVal: helper.Int64ToPtr(3),
										},
										GPUUtilizationAttr: {
											Unit:            GPUUtilizationUnit,
											Desc:            GPUUtilizationDesc,
											IntNumeratorVal: helper.Int64ToPtr(3),
										},
										MemoryUtilizationAttr: {
											Unit:            MemoryUtilizationUnit,
											Desc:            MemoryUtilizationDesc,
											IntNumeratorVal: helper.Int64ToPtr(3),
										},
										EncoderUtilizationAttr: {
											Unit:            EncoderUtilizationUnit,
											Desc:            EncoderUtilizationDesc,
											IntNumeratorVal: helper.Int64ToPtr(3),
										},
										DecoderUtilizationAttr: {
											Unit:            DecoderUtilizationUnit,
											Desc:            DecoderUtilizationDesc,
											IntNumeratorVal: helper.Int64ToPtr(3),
										},
										TemperatureAttr: {
											Unit:            TemperatureUnit,
											Desc:            TemperatureDesc,
											IntNumeratorVal: helper.Int64ToPtr(3),
										},
										MemoryStateAttr: {
											Unit:              MemoryStateUnit,
											Desc:              MemoryStateDesc,
											IntNumeratorVal:   helper.Int64ToPtr(3),
											IntDenominatorVal: helper.Int64ToPtr(3),
										},
										BAR1StateAttr: {
											Unit:              BAR1StateUnit,
											Desc:              BAR1StateDesc,
											IntNumeratorVal:   helper.Int64ToPtr(3),
											IntDenominatorVal: helper.Int64ToPtr(256),
										},
										ECCErrorsL1CacheAttr: {
											Unit:            ECCErrorsL1CacheUnit,
											Desc:            ECCErrorsL1CacheDesc,
											IntNumeratorVal: helper.Int64ToPtr(300),
										},
										ECCErrorsL2CacheAttr: {
											Unit:            ECCErrorsL2CacheUnit,
											Desc:            ECCErrorsL2CacheDesc,
											IntNumeratorVal: helper.Int64ToPtr(300),
										},
										ECCErrorsDeviceAttr: {
											Unit:            ECCErrorsDeviceUnit,
											Desc:            ECCErrorsDeviceDesc,
											IntNumeratorVal: helper.Int64ToPtr(300),
										},
									},
								},
								Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
							},
						},
					},
				},
			},
		},
		{
			Name:      "Check that stats with multiple DeviceNames are assigned to different groups 2",
			Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
			Device: &NvidiaDevice{
				devices: map[string]struct{}{
					"UUID1": {},
					"UUID2": {},
					"UUID3": {},
				},
				nvmlClient: &MockNvmlClient{
					StatsResponseReturned: []*nvml.StatsData{
						{
							DeviceData: &nvml.DeviceData{
								UUID:       "UUID1",
								DeviceName: helper.StringToPtr("DeviceName1"),
								MemoryMiB:  helper.Uint64ToPtr(1),
								PowerW:     helper.UintToPtr(1),
								BAR1MiB:    helper.Uint64ToPtr(256),
							},
							PowerUsageW:        helper.UintToPtr(1),
							GPUUtilization:     helper.UintToPtr(1),
							MemoryUtilization:  helper.UintToPtr(1),
							EncoderUtilization: helper.UintToPtr(1),
							DecoderUtilization: helper.UintToPtr(1),
							TemperatureC:       helper.UintToPtr(1),
							UsedMemoryMiB:      helper.Uint64ToPtr(1),
							BAR1UsedMiB:        helper.Uint64ToPtr(1),
							ECCErrorsL1Cache:   helper.Uint64ToPtr(100),
							ECCErrorsL2Cache:   helper.Uint64ToPtr(100),
							ECCErrorsDevice:    helper.Uint64ToPtr(100),
						},
						{
							DeviceData: &nvml.DeviceData{
								UUID:       "UUID2",
								DeviceName: helper.StringToPtr("DeviceName2"),
								MemoryMiB:  helper.Uint64ToPtr(2),
								PowerW:     helper.UintToPtr(2),
								BAR1MiB:    helper.Uint64ToPtr(256),
							},
							PowerUsageW:        helper.UintToPtr(2),
							GPUUtilization:     helper.UintToPtr(2),
							MemoryUtilization:  helper.UintToPtr(2),
							EncoderUtilization: helper.UintToPtr(2),
							DecoderUtilization: helper.UintToPtr(2),
							TemperatureC:       helper.UintToPtr(2),
							UsedMemoryMiB:      helper.Uint64ToPtr(2),
							BAR1UsedMiB:        helper.Uint64ToPtr(2),
							ECCErrorsL1Cache:   helper.Uint64ToPtr(200),
							ECCErrorsL2Cache:   helper.Uint64ToPtr(200),
							ECCErrorsDevice:    helper.Uint64ToPtr(200),
						},
						{
							DeviceData: &nvml.DeviceData{
								UUID:       "UUID3",
								DeviceName: helper.StringToPtr("DeviceName2"),
								MemoryMiB:  helper.Uint64ToPtr(3),
								PowerW:     helper.UintToPtr(3),
								BAR1MiB:    helper.Uint64ToPtr(256),
							},
							PowerUsageW:        helper.UintToPtr(3),
							GPUUtilization:     helper.UintToPtr(3),
							MemoryUtilization:  helper.UintToPtr(3),
							EncoderUtilization: helper.UintToPtr(3),
							DecoderUtilization: helper.UintToPtr(3),
							TemperatureC:       helper.UintToPtr(3),
							UsedMemoryMiB:      helper.Uint64ToPtr(3),
							BAR1UsedMiB:        helper.Uint64ToPtr(3),
							ECCErrorsL1Cache:   helper.Uint64ToPtr(300),
							ECCErrorsL2Cache:   helper.Uint64ToPtr(300),
							ECCErrorsDevice:    helper.Uint64ToPtr(300),
						},
					},
				},
				logger: hclog.NewNullLogger(),
			},
			ExpectedWriteToChannel: &device.StatsResponse{
				Groups: []*device.DeviceGroupStats{
					{
						Vendor: vendor,
						Type:   deviceType,
						Name:   "DeviceName1",
						InstanceStats: map[string]*device.DeviceStats{
							"UUID1": {
								Summary: &structs.StatValue{
									Unit:              MemoryStateUnit,
									Desc:              MemoryStateDesc,
									IntNumeratorVal:   helper.Int64ToPtr(1),
									IntDenominatorVal: helper.Int64ToPtr(1),
								},
								Stats: &structs.StatObject{
									Attributes: map[string]*structs.StatValue{
										PowerUsageAttr: {
											Unit:              PowerUsageUnit,
											Desc:              PowerUsageDesc,
											IntNumeratorVal:   helper.Int64ToPtr(1),
											IntDenominatorVal: helper.Int64ToPtr(1),
										},
										GPUUtilizationAttr: {
											Unit:            GPUUtilizationUnit,
											Desc:            GPUUtilizationDesc,
											IntNumeratorVal: helper.Int64ToPtr(1),
										},
										MemoryUtilizationAttr: {
											Unit:            MemoryUtilizationUnit,
											Desc:            MemoryUtilizationDesc,
											IntNumeratorVal: helper.Int64ToPtr(1),
										},
										EncoderUtilizationAttr: {
											Unit:            EncoderUtilizationUnit,
											Desc:            EncoderUtilizationDesc,
											IntNumeratorVal: helper.Int64ToPtr(1),
										},
										DecoderUtilizationAttr: {
											Unit:            DecoderUtilizationUnit,
											Desc:            DecoderUtilizationDesc,
											IntNumeratorVal: helper.Int64ToPtr(1),
										},
										TemperatureAttr: {
											Unit:            TemperatureUnit,
											Desc:            TemperatureDesc,
											IntNumeratorVal: helper.Int64ToPtr(1),
										},
										MemoryStateAttr: {
											Unit:              MemoryStateUnit,
											Desc:              MemoryStateDesc,
											IntNumeratorVal:   helper.Int64ToPtr(1),
											IntDenominatorVal: helper.Int64ToPtr(1),
										},
										BAR1StateAttr: {
											Unit:              BAR1StateUnit,
											Desc:              BAR1StateDesc,
											IntNumeratorVal:   helper.Int64ToPtr(1),
											IntDenominatorVal: helper.Int64ToPtr(256),
										},
										ECCErrorsL1CacheAttr: {
											Unit:            ECCErrorsL1CacheUnit,
											Desc:            ECCErrorsL1CacheDesc,
											IntNumeratorVal: helper.Int64ToPtr(100),
										},
										ECCErrorsL2CacheAttr: {
											Unit:            ECCErrorsL2CacheUnit,
											Desc:            ECCErrorsL2CacheDesc,
											IntNumeratorVal: helper.Int64ToPtr(100),
										},
										ECCErrorsDeviceAttr: {
											Unit:            ECCErrorsDeviceUnit,
											Desc:            ECCErrorsDeviceDesc,
											IntNumeratorVal: helper.Int64ToPtr(100),
										},
									},
								},
								Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
							},
						},
					},
					{
						Vendor: vendor,
						Type:   deviceType,
						Name:   "DeviceName2",
						InstanceStats: map[string]*device.DeviceStats{
							"UUID3": {
								Summary: &structs.StatValue{
									Unit:              MemoryStateUnit,
									Desc:              MemoryStateDesc,
									IntNumeratorVal:   helper.Int64ToPtr(3),
									IntDenominatorVal: helper.Int64ToPtr(3),
								},
								Stats: &structs.StatObject{
									Attributes: map[string]*structs.StatValue{
										PowerUsageAttr: {
											Unit:              PowerUsageUnit,
											Desc:              PowerUsageDesc,
											IntNumeratorVal:   helper.Int64ToPtr(3),
											IntDenominatorVal: helper.Int64ToPtr(3),
										},
										GPUUtilizationAttr: {
											Unit:            GPUUtilizationUnit,
											Desc:            GPUUtilizationDesc,
											IntNumeratorVal: helper.Int64ToPtr(3),
										},
										MemoryUtilizationAttr: {
											Unit:            MemoryUtilizationUnit,
											Desc:            MemoryUtilizationDesc,
											IntNumeratorVal: helper.Int64ToPtr(3),
										},
										EncoderUtilizationAttr: {
											Unit:            EncoderUtilizationUnit,
											Desc:            EncoderUtilizationDesc,
											IntNumeratorVal: helper.Int64ToPtr(3),
										},
										DecoderUtilizationAttr: {
											Unit:            DecoderUtilizationUnit,
											Desc:            DecoderUtilizationDesc,
											IntNumeratorVal: helper.Int64ToPtr(3),
										},
										TemperatureAttr: {
											Unit:            TemperatureUnit,
											Desc:            TemperatureDesc,
											IntNumeratorVal: helper.Int64ToPtr(3),
										},
										MemoryStateAttr: {
											Unit:              MemoryStateUnit,
											Desc:              MemoryStateDesc,
											IntNumeratorVal:   helper.Int64ToPtr(3),
											IntDenominatorVal: helper.Int64ToPtr(3),
										},
										BAR1StateAttr: {
											Unit:              BAR1StateUnit,
											Desc:              BAR1StateDesc,
											IntNumeratorVal:   helper.Int64ToPtr(3),
											IntDenominatorVal: helper.Int64ToPtr(256),
										},
										ECCErrorsL1CacheAttr: {
											Unit:            ECCErrorsL1CacheUnit,
											Desc:            ECCErrorsL1CacheDesc,
											IntNumeratorVal: helper.Int64ToPtr(300),
										},
										ECCErrorsL2CacheAttr: {
											Unit:            ECCErrorsL2CacheUnit,
											Desc:            ECCErrorsL2CacheDesc,
											IntNumeratorVal: helper.Int64ToPtr(300),
										},
										ECCErrorsDeviceAttr: {
											Unit:            ECCErrorsDeviceUnit,
											Desc:            ECCErrorsDeviceDesc,
											IntNumeratorVal: helper.Int64ToPtr(300),
										},
									},
								},
								Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
							},
							"UUID2": {
								Summary: &structs.StatValue{
									Unit:              MemoryStateUnit,
									Desc:              MemoryStateDesc,
									IntNumeratorVal:   helper.Int64ToPtr(2),
									IntDenominatorVal: helper.Int64ToPtr(2),
								},
								Stats: &structs.StatObject{
									Attributes: map[string]*structs.StatValue{
										PowerUsageAttr: {
											Unit:              PowerUsageUnit,
											Desc:              PowerUsageDesc,
											IntNumeratorVal:   helper.Int64ToPtr(2),
											IntDenominatorVal: helper.Int64ToPtr(2),
										},
										GPUUtilizationAttr: {
											Unit:            GPUUtilizationUnit,
											Desc:            GPUUtilizationDesc,
											IntNumeratorVal: helper.Int64ToPtr(2),
										},
										MemoryUtilizationAttr: {
											Unit:            MemoryUtilizationUnit,
											Desc:            MemoryUtilizationDesc,
											IntNumeratorVal: helper.Int64ToPtr(2),
										},
										EncoderUtilizationAttr: {
											Unit:            EncoderUtilizationUnit,
											Desc:            EncoderUtilizationDesc,
											IntNumeratorVal: helper.Int64ToPtr(2),
										},
										DecoderUtilizationAttr: {
											Unit:            DecoderUtilizationUnit,
											Desc:            DecoderUtilizationDesc,
											IntNumeratorVal: helper.Int64ToPtr(2),
										},
										TemperatureAttr: {
											Unit:            TemperatureUnit,
											Desc:            TemperatureDesc,
											IntNumeratorVal: helper.Int64ToPtr(2),
										},
										MemoryStateAttr: {
											Unit:              MemoryStateUnit,
											Desc:              MemoryStateDesc,
											IntNumeratorVal:   helper.Int64ToPtr(2),
											IntDenominatorVal: helper.Int64ToPtr(2),
										},
										BAR1StateAttr: {
											Unit:              BAR1StateUnit,
											Desc:              BAR1StateDesc,
											IntNumeratorVal:   helper.Int64ToPtr(2),
											IntDenominatorVal: helper.Int64ToPtr(256),
										},
										ECCErrorsL1CacheAttr: {
											Unit:            ECCErrorsL1CacheUnit,
											Desc:            ECCErrorsL1CacheDesc,
											IntNumeratorVal: helper.Int64ToPtr(200),
										},
										ECCErrorsL2CacheAttr: {
											Unit:            ECCErrorsL2CacheUnit,
											Desc:            ECCErrorsL2CacheDesc,
											IntNumeratorVal: helper.Int64ToPtr(200),
										},
										ECCErrorsDeviceAttr: {
											Unit:            ECCErrorsDeviceUnit,
											Desc:            ECCErrorsDeviceDesc,
											IntNumeratorVal: helper.Int64ToPtr(200),
										},
									},
								},
								Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
							},
						},
					},
				},
			},
		},
		{
			Name:      "Check that only devices from NvidiaDevice.device map stats are reported",
			Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
			Device: &NvidiaDevice{
				devices: map[string]struct{}{
					"UUID1": {},
					"UUID2": {},
				},
				nvmlClient: &MockNvmlClient{
					StatsResponseReturned: []*nvml.StatsData{
						{
							DeviceData: &nvml.DeviceData{
								UUID:       "UUID1",
								DeviceName: helper.StringToPtr("DeviceName1"),
								MemoryMiB:  helper.Uint64ToPtr(1),
								PowerW:     helper.UintToPtr(1),
								BAR1MiB:    helper.Uint64ToPtr(256),
							},
							PowerUsageW:        helper.UintToPtr(1),
							GPUUtilization:     helper.UintToPtr(1),
							MemoryUtilization:  helper.UintToPtr(1),
							EncoderUtilization: helper.UintToPtr(1),
							DecoderUtilization: helper.UintToPtr(1),
							TemperatureC:       helper.UintToPtr(1),
							UsedMemoryMiB:      helper.Uint64ToPtr(1),
							BAR1UsedMiB:        helper.Uint64ToPtr(1),
							ECCErrorsL1Cache:   helper.Uint64ToPtr(100),
							ECCErrorsL2Cache:   helper.Uint64ToPtr(100),
							ECCErrorsDevice:    helper.Uint64ToPtr(100),
						},
						{
							DeviceData: &nvml.DeviceData{
								UUID:       "UUID2",
								DeviceName: helper.StringToPtr("DeviceName2"),
								MemoryMiB:  helper.Uint64ToPtr(2),
								PowerW:     helper.UintToPtr(2),
								BAR1MiB:    helper.Uint64ToPtr(256),
							},
							PowerUsageW:        helper.UintToPtr(2),
							GPUUtilization:     helper.UintToPtr(2),
							MemoryUtilization:  helper.UintToPtr(2),
							EncoderUtilization: helper.UintToPtr(2),
							DecoderUtilization: helper.UintToPtr(2),
							TemperatureC:       helper.UintToPtr(2),
							UsedMemoryMiB:      helper.Uint64ToPtr(2),
							BAR1UsedMiB:        helper.Uint64ToPtr(2),
							ECCErrorsL1Cache:   helper.Uint64ToPtr(200),
							ECCErrorsL2Cache:   helper.Uint64ToPtr(200),
							ECCErrorsDevice:    helper.Uint64ToPtr(200),
						},
						{
							DeviceData: &nvml.DeviceData{
								UUID:       "UUID3",
								DeviceName: helper.StringToPtr("DeviceName3"),
								MemoryMiB:  helper.Uint64ToPtr(3),
								PowerW:     helper.UintToPtr(3),
								BAR1MiB:    helper.Uint64ToPtr(256),
							},
							PowerUsageW:        helper.UintToPtr(3),
							GPUUtilization:     helper.UintToPtr(3),
							MemoryUtilization:  helper.UintToPtr(3),
							EncoderUtilization: helper.UintToPtr(3),
							DecoderUtilization: helper.UintToPtr(3),
							TemperatureC:       helper.UintToPtr(3),
							UsedMemoryMiB:      helper.Uint64ToPtr(3),
							BAR1UsedMiB:        helper.Uint64ToPtr(3),
							ECCErrorsL1Cache:   helper.Uint64ToPtr(300),
							ECCErrorsL2Cache:   helper.Uint64ToPtr(300),
							ECCErrorsDevice:    helper.Uint64ToPtr(300),
						},
					},
				},
				logger: hclog.NewNullLogger(),
			},
			ExpectedWriteToChannel: &device.StatsResponse{
				Groups: []*device.DeviceGroupStats{
					{
						Vendor: vendor,
						Type:   deviceType,
						Name:   "DeviceName1",
						InstanceStats: map[string]*device.DeviceStats{
							"UUID1": {
								Summary: &structs.StatValue{
									Unit:              MemoryStateUnit,
									Desc:              MemoryStateDesc,
									IntNumeratorVal:   helper.Int64ToPtr(1),
									IntDenominatorVal: helper.Int64ToPtr(1),
								},
								Stats: &structs.StatObject{
									Attributes: map[string]*structs.StatValue{
										PowerUsageAttr: {
											Unit:              PowerUsageUnit,
											Desc:              PowerUsageDesc,
											IntNumeratorVal:   helper.Int64ToPtr(1),
											IntDenominatorVal: helper.Int64ToPtr(1),
										},
										GPUUtilizationAttr: {
											Unit:            GPUUtilizationUnit,
											Desc:            GPUUtilizationDesc,
											IntNumeratorVal: helper.Int64ToPtr(1),
										},
										MemoryUtilizationAttr: {
											Unit:            MemoryUtilizationUnit,
											Desc:            MemoryUtilizationDesc,
											IntNumeratorVal: helper.Int64ToPtr(1),
										},
										EncoderUtilizationAttr: {
											Unit:            EncoderUtilizationUnit,
											Desc:            EncoderUtilizationDesc,
											IntNumeratorVal: helper.Int64ToPtr(1),
										},
										DecoderUtilizationAttr: {
											Unit:            DecoderUtilizationUnit,
											Desc:            DecoderUtilizationDesc,
											IntNumeratorVal: helper.Int64ToPtr(1),
										},
										TemperatureAttr: {
											Unit:            TemperatureUnit,
											Desc:            TemperatureDesc,
											IntNumeratorVal: helper.Int64ToPtr(1),
										},
										MemoryStateAttr: {
											Unit:              MemoryStateUnit,
											Desc:              MemoryStateDesc,
											IntNumeratorVal:   helper.Int64ToPtr(1),
											IntDenominatorVal: helper.Int64ToPtr(1),
										},
										BAR1StateAttr: {
											Unit:              BAR1StateUnit,
											Desc:              BAR1StateDesc,
											IntNumeratorVal:   helper.Int64ToPtr(1),
											IntDenominatorVal: helper.Int64ToPtr(256),
										},
										ECCErrorsL1CacheAttr: {
											Unit:            ECCErrorsL1CacheUnit,
											Desc:            ECCErrorsL1CacheDesc,
											IntNumeratorVal: helper.Int64ToPtr(100),
										},
										ECCErrorsL2CacheAttr: {
											Unit:            ECCErrorsL2CacheUnit,
											Desc:            ECCErrorsL2CacheDesc,
											IntNumeratorVal: helper.Int64ToPtr(100),
										},
										ECCErrorsDeviceAttr: {
											Unit:            ECCErrorsDeviceUnit,
											Desc:            ECCErrorsDeviceDesc,
											IntNumeratorVal: helper.Int64ToPtr(100),
										},
									},
								},
								Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
							},
						},
					},
					{
						Vendor: vendor,
						Type:   deviceType,
						Name:   "DeviceName2",
						InstanceStats: map[string]*device.DeviceStats{
							"UUID2": {
								Summary: &structs.StatValue{
									Unit:              MemoryStateUnit,
									Desc:              MemoryStateDesc,
									IntNumeratorVal:   helper.Int64ToPtr(2),
									IntDenominatorVal: helper.Int64ToPtr(2),
								},
								Stats: &structs.StatObject{
									Attributes: map[string]*structs.StatValue{
										PowerUsageAttr: {
											Unit:              PowerUsageUnit,
											Desc:              PowerUsageDesc,
											IntNumeratorVal:   helper.Int64ToPtr(2),
											IntDenominatorVal: helper.Int64ToPtr(2),
										},
										GPUUtilizationAttr: {
											Unit:            GPUUtilizationUnit,
											Desc:            GPUUtilizationDesc,
											IntNumeratorVal: helper.Int64ToPtr(2),
										},
										MemoryUtilizationAttr: {
											Unit:            MemoryUtilizationUnit,
											Desc:            MemoryUtilizationDesc,
											IntNumeratorVal: helper.Int64ToPtr(2),
										},
										EncoderUtilizationAttr: {
											Unit:            EncoderUtilizationUnit,
											Desc:            EncoderUtilizationDesc,
											IntNumeratorVal: helper.Int64ToPtr(2),
										},
										DecoderUtilizationAttr: {
											Unit:            DecoderUtilizationUnit,
											Desc:            DecoderUtilizationDesc,
											IntNumeratorVal: helper.Int64ToPtr(2),
										},
										TemperatureAttr: {
											Unit:            TemperatureUnit,
											Desc:            TemperatureDesc,
											IntNumeratorVal: helper.Int64ToPtr(2),
										},
										MemoryStateAttr: {
											Unit:              MemoryStateUnit,
											Desc:              MemoryStateDesc,
											IntNumeratorVal:   helper.Int64ToPtr(2),
											IntDenominatorVal: helper.Int64ToPtr(2),
										},
										BAR1StateAttr: {
											Unit:              BAR1StateUnit,
											Desc:              BAR1StateDesc,
											IntNumeratorVal:   helper.Int64ToPtr(2),
											IntDenominatorVal: helper.Int64ToPtr(256),
										},
										ECCErrorsL1CacheAttr: {
											Unit:            ECCErrorsL1CacheUnit,
											Desc:            ECCErrorsL1CacheDesc,
											IntNumeratorVal: helper.Int64ToPtr(200),
										},
										ECCErrorsL2CacheAttr: {
											Unit:            ECCErrorsL2CacheUnit,
											Desc:            ECCErrorsL2CacheDesc,
											IntNumeratorVal: helper.Int64ToPtr(200),
										},
										ECCErrorsDeviceAttr: {
											Unit:            ECCErrorsDeviceUnit,
											Desc:            ECCErrorsDeviceDesc,
											IntNumeratorVal: helper.Int64ToPtr(200),
										},
									},
								},
								Timestamp: time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC),
							},
						},
					},
				},
			},
		},
	} {
		channel := make(chan *device.StatsResponse, 1)
		testCase.Device.writeStatsToChannel(channel, testCase.Timestamp)
		actualResult := <-channel
		// writeStatsToChannel iterates over map keys
		// and insterts results to an array, so order of elements in output array
		// may be different
		// actualResult, expectedWriteToChannel arrays has to be sorted firsted
		sort.Slice(actualResult.Groups, func(i, j int) bool {
			return actualResult.Groups[i].Name < actualResult.Groups[j].Name
		})
		sort.Slice(testCase.ExpectedWriteToChannel.Groups, func(i, j int) bool {
			return testCase.ExpectedWriteToChannel.Groups[i].Name < testCase.ExpectedWriteToChannel.Groups[j].Name
		})
		require.New(t).Equal(testCase.ExpectedWriteToChannel, actualResult)
	}
}
