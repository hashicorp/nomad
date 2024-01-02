// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hclutils_test

import (
	"testing"

	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/nomad/drivers/docker"
	"github.com/hashicorp/nomad/helper/pluginutils/hclspecutils"
	"github.com/hashicorp/nomad/helper/pluginutils/hclutils"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestParseHclInterface_Hcl(t *testing.T) {
	dockerDriver := new(docker.Driver)
	dockerSpec, err := dockerDriver.TaskConfigSchema()
	require.NoError(t, err)
	dockerDecSpec, diags := hclspecutils.Convert(dockerSpec)
	require.False(t, diags.HasErrors())

	vars := map[string]cty.Value{
		"NOMAD_ALLOC_INDEX": cty.NumberIntVal(2),
		"NOMAD_META_hello":  cty.StringVal("world"),
	}

	cases := []struct {
		name         string
		config       interface{}
		spec         hcldec.Spec
		vars         map[string]cty.Value
		expected     interface{}
		expectedType interface{}
	}{
		{
			name: "single string attr",
			config: hclutils.HclConfigToInterface(t, `
			config {
				image = "redis:7"
			}`),
			spec: dockerDecSpec,
			expected: &docker.TaskConfig{
				Image:            "redis:7",
				Devices:          []docker.DockerDevice{},
				Mounts:           []docker.DockerMount{},
				MountsList:       []docker.DockerMount{},
				CPUCFSPeriod:     100000,
				ImagePullTimeout: "5m",
			},
			expectedType: &docker.TaskConfig{},
		},
		{
			name: "single string attr json",
			config: hclutils.JsonConfigToInterface(t, `
						{
							"Config": {
								"image": "redis:7"
			                }
						}`),
			spec: dockerDecSpec,
			expected: &docker.TaskConfig{
				Image:            "redis:7",
				Devices:          []docker.DockerDevice{},
				Mounts:           []docker.DockerMount{},
				MountsList:       []docker.DockerMount{},
				CPUCFSPeriod:     100000,
				ImagePullTimeout: "5m",
			},
			expectedType: &docker.TaskConfig{},
		},
		{
			name: "number attr",
			config: hclutils.HclConfigToInterface(t, `
						config {
							image = "redis:7"
							pids_limit  = 2
						}`),
			spec: dockerDecSpec,
			expected: &docker.TaskConfig{
				Image:            "redis:7",
				PidsLimit:        2,
				Devices:          []docker.DockerDevice{},
				Mounts:           []docker.DockerMount{},
				MountsList:       []docker.DockerMount{},
				CPUCFSPeriod:     100000,
				ImagePullTimeout: "5m",
			},
			expectedType: &docker.TaskConfig{},
		},
		{
			name: "number attr json",
			config: hclutils.JsonConfigToInterface(t, `
						{
							"Config": {
								"image": "redis:7",
								"pids_limit": "2"
			                }
						}`),
			spec: dockerDecSpec,
			expected: &docker.TaskConfig{
				Image:            "redis:7",
				PidsLimit:        2,
				Devices:          []docker.DockerDevice{},
				Mounts:           []docker.DockerMount{},
				MountsList:       []docker.DockerMount{},
				CPUCFSPeriod:     100000,
				ImagePullTimeout: "5m",
			},
			expectedType: &docker.TaskConfig{},
		},
		{
			name: "number attr interpolated",
			config: hclutils.HclConfigToInterface(t, `
						config {
							image = "redis:7"
							pids_limit  = "${2 + 2}"
						}`),
			spec: dockerDecSpec,
			expected: &docker.TaskConfig{
				Image:            "redis:7",
				PidsLimit:        4,
				Devices:          []docker.DockerDevice{},
				Mounts:           []docker.DockerMount{},
				MountsList:       []docker.DockerMount{},
				CPUCFSPeriod:     100000,
				ImagePullTimeout: "5m",
			},
			expectedType: &docker.TaskConfig{},
		},
		{
			name: "number attr interploated json",
			config: hclutils.JsonConfigToInterface(t, `
						{
							"Config": {
								"image": "redis:7",
								"pids_limit": "${2 + 2}"
			                }
						}`),
			spec: dockerDecSpec,
			expected: &docker.TaskConfig{
				Image:            "redis:7",
				PidsLimit:        4,
				Devices:          []docker.DockerDevice{},
				Mounts:           []docker.DockerMount{},
				MountsList:       []docker.DockerMount{},
				CPUCFSPeriod:     100000,
				ImagePullTimeout: "5m",
			},
			expectedType: &docker.TaskConfig{},
		},
		{
			name: "multi attr",
			config: hclutils.HclConfigToInterface(t, `
						config {
							image = "redis:7"
							args = ["foo", "bar"]
						}`),
			spec: dockerDecSpec,
			expected: &docker.TaskConfig{
				Image:            "redis:7",
				Args:             []string{"foo", "bar"},
				Devices:          []docker.DockerDevice{},
				Mounts:           []docker.DockerMount{},
				MountsList:       []docker.DockerMount{},
				CPUCFSPeriod:     100000,
				ImagePullTimeout: "5m",
			},
			expectedType: &docker.TaskConfig{},
		},
		{
			name: "multi attr json",
			config: hclutils.JsonConfigToInterface(t, `
						{
							"Config": {
								"image": "redis:7",
								"args": ["foo", "bar"]
			                }
						}`),
			spec: dockerDecSpec,
			expected: &docker.TaskConfig{
				Image:            "redis:7",
				Args:             []string{"foo", "bar"},
				Devices:          []docker.DockerDevice{},
				Mounts:           []docker.DockerMount{},
				MountsList:       []docker.DockerMount{},
				CPUCFSPeriod:     100000,
				ImagePullTimeout: "5m",
			},
			expectedType: &docker.TaskConfig{},
		},
		{
			name: "multi attr variables",
			config: hclutils.HclConfigToInterface(t, `
						config {
							image = "redis:7"
							args = ["${NOMAD_META_hello}", "${NOMAD_ALLOC_INDEX}"]
							pids_limit = "${NOMAD_ALLOC_INDEX + 2}"
						}`),
			spec: dockerDecSpec,
			vars: vars,
			expected: &docker.TaskConfig{
				Image:            "redis:7",
				Args:             []string{"world", "2"},
				PidsLimit:        4,
				Devices:          []docker.DockerDevice{},
				Mounts:           []docker.DockerMount{},
				MountsList:       []docker.DockerMount{},
				CPUCFSPeriod:     100000,
				ImagePullTimeout: "5m",
			},
			expectedType: &docker.TaskConfig{},
		},
		{
			name: "multi attr variables json",
			config: hclutils.JsonConfigToInterface(t, `
						{
							"Config": {
								"image": "redis:7",
								"args": ["foo", "bar"]
			                }
						}`),
			spec: dockerDecSpec,
			expected: &docker.TaskConfig{
				Image:            "redis:7",
				Args:             []string{"foo", "bar"},
				Devices:          []docker.DockerDevice{},
				Mounts:           []docker.DockerMount{},
				MountsList:       []docker.DockerMount{},
				CPUCFSPeriod:     100000,
				ImagePullTimeout: "5m",
			},
			expectedType: &docker.TaskConfig{},
		},
		{
			name: "port_map",
			config: hclutils.HclConfigToInterface(t, `
			config {
				image = "redis:7"
				port_map {
					foo = 1234
					bar = 5678
				}
			}`),
			spec: dockerDecSpec,
			expected: &docker.TaskConfig{
				Image: "redis:7",
				PortMap: map[string]int{
					"foo": 1234,
					"bar": 5678,
				},
				Devices:          []docker.DockerDevice{},
				Mounts:           []docker.DockerMount{},
				MountsList:       []docker.DockerMount{},
				CPUCFSPeriod:     100000,
				ImagePullTimeout: "5m",
			},
			expectedType: &docker.TaskConfig{},
		},
		{
			name: "port_map json",
			config: hclutils.JsonConfigToInterface(t, `
							{
								"Config": {
									"image": "redis:7",
									"port_map": [{
										"foo": 1234,
										"bar": 5678
									}]
				                }
							}`),
			spec: dockerDecSpec,
			expected: &docker.TaskConfig{
				Image: "redis:7",
				PortMap: map[string]int{
					"foo": 1234,
					"bar": 5678,
				},
				Devices:          []docker.DockerDevice{},
				Mounts:           []docker.DockerMount{},
				MountsList:       []docker.DockerMount{},
				CPUCFSPeriod:     100000,
				ImagePullTimeout: "5m",
			},
			expectedType: &docker.TaskConfig{},
		},
		{
			name: "devices",
			config: hclutils.HclConfigToInterface(t, `
						config {
							image = "redis:7"
							devices = [
								{
									host_path = "/dev/sda1"
									container_path = "/dev/xvdc"
									cgroup_permissions = "r"
								},
								{
									host_path = "/dev/sda2"
									container_path = "/dev/xvdd"
								}
							]
						}`),
			spec: dockerDecSpec,
			expected: &docker.TaskConfig{
				Image: "redis:7",
				Devices: []docker.DockerDevice{
					{
						HostPath:          "/dev/sda1",
						ContainerPath:     "/dev/xvdc",
						CgroupPermissions: "r",
					},
					{
						HostPath:      "/dev/sda2",
						ContainerPath: "/dev/xvdd",
					},
				},
				Mounts:           []docker.DockerMount{},
				MountsList:       []docker.DockerMount{},
				CPUCFSPeriod:     100000,
				ImagePullTimeout: "5m",
			},
			expectedType: &docker.TaskConfig{},
		},
		{
			name: "docker_logging",
			config: hclutils.HclConfigToInterface(t, `
				config {
					image = "redis:7"
					network_mode = "host"
					dns_servers = ["169.254.1.1"]
					logging {
					    type = "syslog"
					    config {
						tag  = "driver-test"
					    }
					}
				}`),
			spec: dockerDecSpec,
			expected: &docker.TaskConfig{
				Image:       "redis:7",
				NetworkMode: "host",
				DNSServers:  []string{"169.254.1.1"},
				Logging: docker.DockerLogging{
					Type: "syslog",
					Config: map[string]string{
						"tag": "driver-test",
					},
				},
				Devices:          []docker.DockerDevice{},
				Mounts:           []docker.DockerMount{},
				MountsList:       []docker.DockerMount{},
				CPUCFSPeriod:     100000,
				ImagePullTimeout: "5m",
			},
			expectedType: &docker.TaskConfig{},
		},
		{
			name: "docker_json",
			config: hclutils.JsonConfigToInterface(t, `
					{
						"Config": {
							"image": "redis:7",
							"devices": [
								{
									"host_path": "/dev/sda1",
									"container_path": "/dev/xvdc",
									"cgroup_permissions": "r"
								},
								{
									"host_path": "/dev/sda2",
									"container_path": "/dev/xvdd"
								}
							]
				}
					}`),
			spec: dockerDecSpec,
			expected: &docker.TaskConfig{
				Image: "redis:7",
				Devices: []docker.DockerDevice{
					{
						HostPath:          "/dev/sda1",
						ContainerPath:     "/dev/xvdc",
						CgroupPermissions: "r",
					},
					{
						HostPath:      "/dev/sda2",
						ContainerPath: "/dev/xvdd",
					},
				},
				Mounts:           []docker.DockerMount{},
				MountsList:       []docker.DockerMount{},
				CPUCFSPeriod:     100000,
				ImagePullTimeout: "5m",
			},
			expectedType: &docker.TaskConfig{},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Logf("Val: % #v", pretty.Formatter(c.config))
			// Parse the interface
			ctyValue, diag, errs := hclutils.ParseHclInterface(c.config, c.spec, c.vars)
			if diag.HasErrors() {
				for _, err := range errs {
					t.Error(err)
				}
				t.FailNow()
			}

			// Test encoding
			taskConfig := &drivers.TaskConfig{}
			require.NoError(t, taskConfig.EncodeDriverConfig(ctyValue))

			// Test decoding
			require.NoError(t, taskConfig.DecodeDriverConfig(c.expectedType))

			require.EqualValues(t, c.expected, c.expectedType)

		})
	}
}

func TestParseNullFields(t *testing.T) {
	spec := hclspec.NewObject(map[string]*hclspec.Spec{
		"array_field":   hclspec.NewAttr("array_field", "list(string)", false),
		"string_field":  hclspec.NewAttr("string_field", "string", false),
		"boolean_field": hclspec.NewAttr("boolean_field", "bool", false),
		"number_field":  hclspec.NewAttr("number_field", "number", false),
		"block_field": hclspec.NewBlock("block_field", false, hclspec.NewObject((map[string]*hclspec.Spec{
			"f": hclspec.NewAttr("f", "string", true),
		}))),
		"block_list_field": hclspec.NewBlockList("block_list_field", hclspec.NewObject((map[string]*hclspec.Spec{
			"f": hclspec.NewAttr("f", "string", true),
		}))),
	})

	type Sub struct {
		F string `codec:"f"`
	}

	type TaskConfig struct {
		Array     []string `codec:"array_field"`
		String    string   `codec:"string_field"`
		Boolean   bool     `codec:"boolean_field"`
		Number    int64    `codec:"number_field"`
		Block     Sub      `codec:"block_field"`
		BlockList []Sub    `codec:"block_list_field"`
	}

	cases := []struct {
		name     string
		json     string
		expected TaskConfig
	}{
		{
			"omitted fields",
			`{"Config": {}}`,
			TaskConfig{BlockList: []Sub{}},
		},
		{
			"explicitly nil",
			`{"Config": {
                            "array_field": null,
                            "string_field": null,
			    "boolean_field": null,
                            "number_field": null,
                            "block_field": null,
                            "block_list_field": null}}`,
			TaskConfig{BlockList: []Sub{}},
		},
		{
			// for checking that the fields are actually set
			"explicitly set to not null",
			`{"Config": {
                            "array_field": ["a"],
                            "string_field": "a",
                            "boolean_field": true,
                            "number_field": 5,
                            "block_field": [{"f": "a"}],
                            "block_list_field": [{"f": "a"}, {"f": "b"}]}}`,
			TaskConfig{
				Array:     []string{"a"},
				String:    "a",
				Boolean:   true,
				Number:    5,
				Block:     Sub{"a"},
				BlockList: []Sub{{"a"}, {"b"}},
			},
		},
	}

	parser := hclutils.NewConfigParser(spec)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var tc TaskConfig
			parser.ParseJson(t, c.json, &tc)

			require.EqualValues(t, c.expected, tc)
		})
	}
}

func TestParseUnknown(t *testing.T) {
	spec := hclspec.NewObject(map[string]*hclspec.Spec{
		"string_field":   hclspec.NewAttr("string_field", "string", false),
		"map_field":      hclspec.NewAttr("map_field", "map(string)", false),
		"list_field":     hclspec.NewAttr("list_field", "map(string)", false),
		"map_list_field": hclspec.NewAttr("map_list_field", "list(map(string))", false),
	})
	cSpec, diags := hclspecutils.Convert(spec)
	require.False(t, diags.HasErrors())

	cases := []struct {
		name string
		hcl  string
	}{
		{
			"string field",
			`config {  string_field = "${MYENV}" }`,
		},
		{
			"map_field",
			`config { map_field { key = "${MYENV}" }}`,
		},
		{
			"list_field",
			`config { list_field = ["${MYENV}"]}`,
		},
		{
			"map_list_field",
			`config { map_list_field { key = "${MYENV}"}}`,
		},
	}

	vars := map[string]cty.Value{}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			inter := hclutils.HclConfigToInterface(t, c.hcl)

			ctyValue, diag, errs := hclutils.ParseHclInterface(inter, cSpec, vars)
			t.Logf("parsed: %# v", pretty.Formatter(ctyValue))

			require.NotNil(t, errs)
			require.True(t, diag.HasErrors())
			require.Contains(t, errs[0].Error(), "no variable named")
		})
	}
}

func TestParseInvalid(t *testing.T) {
	dockerDriver := new(docker.Driver)
	dockerSpec, err := dockerDriver.TaskConfigSchema()
	require.NoError(t, err)
	spec, diags := hclspecutils.Convert(dockerSpec)
	require.False(t, diags.HasErrors())

	cases := []struct {
		name string
		hcl  string
	}{
		{
			"invalid_field",
			`config { image = "redis:7" bad_key = "whatever"}`,
		},
	}

	vars := map[string]cty.Value{}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			inter := hclutils.HclConfigToInterface(t, c.hcl)

			ctyValue, diag, errs := hclutils.ParseHclInterface(inter, spec, vars)
			t.Logf("parsed: %# v", pretty.Formatter(ctyValue))

			require.NotNil(t, errs)
			require.True(t, diag.HasErrors())
			require.Contains(t, errs[0].Error(), "Invalid label")
		})
	}
}
