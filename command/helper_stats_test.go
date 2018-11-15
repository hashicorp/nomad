package command

import (
	"sort"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeviceQualifiedID(t *testing.T) {

	require := require.New(t)

	require.Equal("vendor/type/name[id]", deviceQualifiedID("vendor", "type", "name", "id"))
	require.Equal("vendor/type[id]", deviceQualifiedID("vendor", "type", "", "id"))
	require.Equal("vendor[id]", deviceQualifiedID("vendor", "", "", "id"))
}

func TestBuildDeviceStatsSummaryMap(t *testing.T) {
	hostDeviceStats := []*api.DeviceGroupStats{
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

	require.EqualValues(t, expected, buildDeviceStatsSummaryMap(hostDeviceStats))
}

func TestFormatDeviceStats(t *testing.T) {
	statValue := func(v string) *api.StatValue {
		return &api.StatValue{
			StringVal: helper.StringToPtr(v),
		}
	}

	stat := &api.StatObject{
		Attributes: map[string]*api.StatValue{
			"k0": statValue("v0"),
		},
		Nested: map[string]*api.StatObject{
			"nested1": {
				Attributes: map[string]*api.StatValue{
					"k1_0": statValue("v1_0"),
					"k1_1": statValue("v1_1"),
				},
				Nested: map[string]*api.StatObject{
					"nested1_1": {
						Attributes: map[string]*api.StatValue{
							"k11_0": statValue("v11_0"),
							"k11_1": statValue("v11_1"),
						},
					},
				},
			},
			"nested2": {
				Attributes: map[string]*api.StatValue{
					"k2": statValue("v2"),
				},
			},
		},
	}

	result := []string{"preseedkey|pressededvalue"}
	formatDeviceStats(stat, "", &result)

	// check array is appended only
	require.Equal(t, "preseedkey|pressededvalue", result[0])

	// check rest of values
	sort.Strings(result)
	expected := []string{
		"k0|v0",
		"nested1.k1_0|v1_0",
		"nested1.k1_1|v1_1",
		"nested1.nested1_1.k11_0|v11_0",
		"nested1.nested1_1.k11_1|v11_1",
		"nested2.k2|v2",
		"preseedkey|pressededvalue",
	}

	require.Equal(t, expected, result)
}

func TestNodeStatusCommand_GetDeviceResourcesForNode(t *testing.T) {
	hostDeviceStats := []*api.DeviceGroupStats{
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
	}

	node := &api.Node{
		NodeResources: &api.NodeResources{
			Devices: []*api.NodeDeviceResource{
				{
					Vendor: "vendor2",
					Type:   "type2",
					Instances: []*api.NodeDevice{
						{ID: "id1"},
						{ID: "id2"},
					},
				},
				{
					Vendor: "vendor1",
					Type:   "type1",
					Name:   "name1",
					Instances: []*api.NodeDevice{
						{ID: "id1"},
						{ID: "id2"},
					},
				},
			},
		},
	}

	formattedDevices := getDeviceResourcesForNode(hostDeviceStats, node)
	sort.Strings(formattedDevices)
	expected := []string{
		"vendor1/type1/name1[id1]|stat1",
		"vendor1/type1/name1[id2]|2",
		"vendor2/type2[id1]|stat3",
		"vendor2/type2[id2]|4",
	}

	assert.Equal(t, expected, formattedDevices)
}
