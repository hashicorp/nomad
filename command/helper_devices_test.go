// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeviceQualifiedID(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)

	require.Equal("vendor/type/name[id]", deviceQualifiedID("vendor", "type", "name", "id"))
	require.Equal("vendor/type[id]", deviceQualifiedID("vendor", "type", "", "id"))
	require.Equal("vendor[id]", deviceQualifiedID("vendor", "", "", "id"))
}

func TestBuildDeviceStatsSummaryMap(t *testing.T) {
	ci.Parallel(t)

	hostDeviceStats := []*api.DeviceGroupStats{
		{
			Vendor: "vendor1",
			Type:   "type1",
			Name:   "name1",
			InstanceStats: map[string]*api.DeviceStats{
				"id1": {
					Summary: &api.StatValue{
						StringVal: pointer.Of("stat1"),
					},
				},
				"id2": {
					Summary: &api.StatValue{
						IntNumeratorVal: pointer.Of(int64(2)),
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
						StringVal: pointer.Of("stat3"),
					},
				},
				"id2": {
					Summary: &api.StatValue{
						IntNumeratorVal: pointer.Of(int64(4)),
					},
				},
			},
		},
	}

	expected := map[string]*api.StatValue{
		"vendor1/type1/name1[id1]": {
			StringVal: pointer.Of("stat1"),
		},
		"vendor1/type1/name1[id2]": {
			IntNumeratorVal: pointer.Of(int64(2)),
		},
		"vendor2/type2[id1]": {
			StringVal: pointer.Of("stat3"),
		},
		"vendor2/type2[id2]": {
			IntNumeratorVal: pointer.Of(int64(4)),
		},
	}

	require.EqualValues(t, expected, buildDeviceStatsSummaryMap(hostDeviceStats))
}

func TestFormatDeviceStats(t *testing.T) {
	ci.Parallel(t)

	statValue := func(v string) *api.StatValue {
		return &api.StatValue{
			StringVal: pointer.Of(v),
		}
	}

	stat := &api.StatObject{
		Attributes: map[string]*api.StatValue{
			"a0": statValue("va0"),
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

	result := formatDeviceStats("TestDeviceID", stat)

	// check that device id always appears first
	require.Equal(t, "Device|TestDeviceID", result[0])

	// check rest of values
	expected := []string{
		"Device|TestDeviceID",
		"a0|va0",
		"k0|v0",
		"nested1.k1_0|v1_0",
		"nested1.k1_1|v1_1",
		"nested1.nested1_1.k11_0|v11_0",
		"nested1.nested1_1.k11_1|v11_1",
		"nested2.k2|v2",
	}

	require.Equal(t, expected, result)
}

func TestNodeStatusCommand_GetDeviceResourcesForNode(t *testing.T) {
	ci.Parallel(t)

	hostDeviceStats := []*api.DeviceGroupStats{
		{
			Vendor: "vendor1",
			Type:   "type1",
			Name:   "name1",
			InstanceStats: map[string]*api.DeviceStats{
				"id1": {
					Summary: &api.StatValue{
						StringVal: pointer.Of("stat1"),
					},
				},
				"id2": {
					Summary: &api.StatValue{
						IntNumeratorVal: pointer.Of(int64(2)),
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
						StringVal: pointer.Of("stat3"),
					},
				},
				"id2": {
					Summary: &api.StatValue{
						IntNumeratorVal: pointer.Of(int64(4)),
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
	expected := []string{
		"vendor1/type1/name1[id1]|stat1",
		"vendor1/type1/name1[id2]|2",
		"vendor2/type2[id1]|stat3",
		"vendor2/type2[id2]|4",
	}

	assert.Equal(t, expected, formattedDevices)
}

func TestNodeStatusCommand_GetDeviceResources(t *testing.T) {
	ci.Parallel(t)

	hostDeviceStats := []*api.DeviceGroupStats{
		{
			Vendor: "vendor1",
			Type:   "type1",
			Name:   "name1",
			InstanceStats: map[string]*api.DeviceStats{
				"id1": {
					Summary: &api.StatValue{
						StringVal: pointer.Of("stat1"),
					},
				},
				"id2": {
					Summary: &api.StatValue{
						IntNumeratorVal: pointer.Of(int64(2)),
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
						StringVal: pointer.Of("stat3"),
					},
				},
				"id2": {
					Summary: &api.StatValue{
						IntNumeratorVal: pointer.Of(int64(4)),
					},
				},
			},
		},
	}

	formattedDevices := getDeviceResources(hostDeviceStats)
	expected := []string{
		"vendor1/type1/name1[id1]|stat1",
		"vendor1/type1/name1[id2]|2",
		"vendor2/type2[id1]|stat3",
		"vendor2/type2[id2]|4",
	}

	assert.Equal(t, expected, formattedDevices)
}
func TestGetDeviceAttributes(t *testing.T) {
	ci.Parallel(t)

	d := &api.NodeDeviceResource{
		Vendor: "Vendor",
		Type:   "Type",
		Name:   "Name",

		Attributes: map[string]*api.Attribute{
			"utilization": {
				FloatVal: pointer.Of(float64(0.78)),
				Unit:     "%",
			},
			"filesystem": {
				StringVal: pointer.Of("ext4"),
			},
		},
	}

	formattedDevices := getDeviceAttributes(d)
	expected := []string{
		"Device Group|Vendor/Type/Name",
		"filesystem|ext4",
		"utilization|0.78 %",
	}

	assert.Equal(t, expected, formattedDevices)
}
