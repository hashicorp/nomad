package command

import (
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper"
	"github.com/stretchr/testify/require"
)

func TestDeviceQualifiedID(t *testing.T) {

	require := require.New(t)

	require.Equal("vendor/type/name[id]", deviceQualifiedID("vendor", "type", "name", "id"))
	require.Equal("vendor/type[id]", deviceQualifiedID("vendor", "type", "", "id"))
	require.Equal("vendor[id]", deviceQualifiedID("vendor", "", "", "id"))
}

func TestBuildDeviceStatsSummaryMap(t *testing.T) {
	hostStats := &api.HostStats{
		DeviceStats: []*api.DeviceGroupStats{
			{
				Vendor: "vendor1",
				Type:   "type1",
				Name:   "name1",
				InstanceStats: map[string]*api.DeviceStats{
					"id1": {
						Summary: &api.StatValue{
							StringVal: helper.StringToPtr("stat1"),
						},
					},
					"id2": {
						Summary: &api.StatValue{
							IntNumeratorVal: helper.Int64ToPtr(2),
						},
					},
				},
			},
			{
				Vendor: "vendor2",
				Type:   "type2",
				InstanceStats: map[string]*api.DeviceStats{
					"id1": {
						Summary: &api.StatValue{
							StringVal: helper.StringToPtr("stat3"),
						},
					},
					"id2": {
						Summary: &api.StatValue{
							IntNumeratorVal: helper.Int64ToPtr(4),
						},
					},
				},
			},
		},
	}

	expected := map[string]*api.StatValue{
		"vendor1/type1/name1[id1]": {
			StringVal: helper.StringToPtr("stat1"),
		},
		"vendor1/type1/name1[id2]": {
			IntNumeratorVal: helper.Int64ToPtr(2),
		},
		"vendor2/type2[id1]": {
			StringVal: helper.StringToPtr("stat3"),
		},
		"vendor2/type2[id2]": {
			IntNumeratorVal: helper.Int64ToPtr(4),
		},
	}

	require.EqualValues(t, expected, buildDeviceStatsSummaryMap(hostStats))
}
