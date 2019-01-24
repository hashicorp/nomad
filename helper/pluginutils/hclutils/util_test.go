package hclutils

import (
	"testing"

	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/ast"
	hcl2 "github.com/hashicorp/hcl2/hcl"
	"github.com/hashicorp/hcl2/hcldec"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/kr/pretty"
	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/require"
	"github.com/ugorji/go/codec"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/gocty"
)

var (
	dockerSpec hcldec.Spec = hcldec.ObjectSpec(map[string]hcldec.Spec{
		"image": &hcldec.AttrSpec{
			Name:     "image",
			Type:     cty.String,
			Required: true,
		},
		"args": &hcldec.AttrSpec{
			Name: "args",
			Type: cty.List(cty.String),
		},
		"pids_limit": &hcldec.AttrSpec{
			Name: "pids_limit",
			Type: cty.Number,
		},
		"port_map": &hcldec.BlockAttrsSpec{
			TypeName:    "port_map",
			ElementType: cty.String,
		},

		"devices": &hcldec.BlockListSpec{
			TypeName: "devices",
			Nested: hcldec.ObjectSpec(map[string]hcldec.Spec{
				"host_path": &hcldec.AttrSpec{
					Name: "host_path",
					Type: cty.String,
				},
				"container_path": &hcldec.AttrSpec{
					Name: "container_path",
					Type: cty.String,
				},
				"cgroup_permissions": &hcldec.DefaultSpec{
					Primary: &hcldec.AttrSpec{
						Name: "cgroup_permissions",
						Type: cty.String,
					},
					Default: &hcldec.LiteralSpec{
						Value: cty.StringVal(""),
					},
				},
			}),
		},
	},
	)
)

type dockerConfig struct {
	Image     string            `cty:"image"`
	Args      []string          `cty:"args"`
	PidsLimit *int64            `cty:"pids_limit"`
	PortMap   map[string]string `cty:"port_map"`
	Devices   []DockerDevice    `cty:"devices"`
}

type DockerDevice struct {
	HostPath          string `cty:"host_path"`
	ContainerPath     string `cty:"container_path"`
	CgroupPermissions string `cty:"cgroup_permissions"`
}

func hclConfigToInterface(t *testing.T, config string) interface{} {
	t.Helper()

	// Parse as we do in the jobspec parser
	root, err := hcl.Parse(config)
	if err != nil {
		t.Fatalf("failed to hcl parse the config: %v", err)
	}

	// Top-level item should be a list
	list, ok := root.Node.(*ast.ObjectList)
	if !ok {
		t.Fatalf("root should be an object")
	}

	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, list.Items[0]); err != nil {
		t.Fatalf("failed to decode object: %v", err)
	}

	var m2 map[string]interface{}
	if err := mapstructure.WeakDecode(m, &m2); err != nil {
		t.Fatalf("failed to weak decode object: %v", err)
	}

	return m2["config"]
}

func jsonConfigToInterface(t *testing.T, config string) interface{} {
	t.Helper()

	// Decode from json
	dec := codec.NewDecoderBytes([]byte(config), structs.JsonHandle)

	var m map[string]interface{}
	err := dec.Decode(&m)
	if err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	return m["Config"]
}

func TestParseHclInterface_Hcl(t *testing.T) {
	defaultCtx := &hcl2.EvalContext{
		Functions: GetStdlibFuncs(),
	}
	variableCtx := &hcl2.EvalContext{
		Functions: GetStdlibFuncs(),
		Variables: map[string]cty.Value{
			"NOMAD_ALLOC_INDEX": cty.NumberIntVal(2),
			"NOMAD_META_hello":  cty.StringVal("world"),
		},
	}

	// XXX Useful for determining what cty thinks the type is
	//implied, err := gocty.ImpliedType(&dockerConfig{})
	//if err != nil {
	//t.Fatalf("implied type failed: %v", err)
	//}

	//t.Logf("Implied type: %v", implied.GoString())

	cases := []struct {
		name         string
		config       interface{}
		spec         hcldec.Spec
		ctx          *hcl2.EvalContext
		expected     interface{}
		expectedType interface{}
	}{
		{
			name: "single string attr",
			config: hclConfigToInterface(t, `
			config {
				image = "redis:3.2"
			}`),
			spec: dockerSpec,
			ctx:  defaultCtx,
			expected: &dockerConfig{
				Image:   "redis:3.2",
				Devices: []DockerDevice{},
			},
			expectedType: &dockerConfig{},
		},
		{
			name: "single string attr json",
			config: jsonConfigToInterface(t, `
						{
							"Config": {
								"image": "redis:3.2"
			                }
						}`),
			spec: dockerSpec,
			ctx:  defaultCtx,
			expected: &dockerConfig{
				Image:   "redis:3.2",
				Devices: []DockerDevice{},
			},
			expectedType: &dockerConfig{},
		},
		{
			name: "number attr",
			config: hclConfigToInterface(t, `
						config {
							image = "redis:3.2"
							pids_limit  = 2
						}`),
			spec: dockerSpec,
			ctx:  defaultCtx,
			expected: &dockerConfig{
				Image:     "redis:3.2",
				PidsLimit: helper.Int64ToPtr(2),
				Devices:   []DockerDevice{},
			},
			expectedType: &dockerConfig{},
		},
		{
			name: "number attr json",
			config: jsonConfigToInterface(t, `
						{
							"Config": {
								"image": "redis:3.2",
								"pids_limit": "2"
			                }
						}`),
			spec: dockerSpec,
			ctx:  defaultCtx,
			expected: &dockerConfig{
				Image:     "redis:3.2",
				PidsLimit: helper.Int64ToPtr(2),
				Devices:   []DockerDevice{},
			},
			expectedType: &dockerConfig{},
		},
		{
			name: "number attr interpolated",
			config: hclConfigToInterface(t, `
						config {
							image = "redis:3.2"
							pids_limit  = "${2 + 2}"
						}`),
			spec: dockerSpec,
			ctx:  defaultCtx,
			expected: &dockerConfig{
				Image:     "redis:3.2",
				PidsLimit: helper.Int64ToPtr(4),
				Devices:   []DockerDevice{},
			},
			expectedType: &dockerConfig{},
		},
		{
			name: "number attr interploated json",
			config: jsonConfigToInterface(t, `
						{
							"Config": {
								"image": "redis:3.2",
								"pids_limit": "${2 + 2}"
			                }
						}`),
			spec: dockerSpec,
			ctx:  defaultCtx,
			expected: &dockerConfig{
				Image:     "redis:3.2",
				PidsLimit: helper.Int64ToPtr(4),
				Devices:   []DockerDevice{},
			},
			expectedType: &dockerConfig{},
		},
		{
			name: "multi attr",
			config: hclConfigToInterface(t, `
						config {
							image = "redis:3.2"
							args = ["foo", "bar"]
						}`),
			spec: dockerSpec,
			ctx:  defaultCtx,
			expected: &dockerConfig{
				Image:   "redis:3.2",
				Args:    []string{"foo", "bar"},
				Devices: []DockerDevice{},
			},
			expectedType: &dockerConfig{},
		},
		{
			name: "multi attr json",
			config: jsonConfigToInterface(t, `
						{
							"Config": {
								"image": "redis:3.2",
								"args": ["foo", "bar"]
			                }
						}`),
			spec: dockerSpec,
			ctx:  defaultCtx,
			expected: &dockerConfig{
				Image:   "redis:3.2",
				Args:    []string{"foo", "bar"},
				Devices: []DockerDevice{},
			},
			expectedType: &dockerConfig{},
		},
		{
			name: "multi attr variables",
			config: hclConfigToInterface(t, `
						config {
							image = "redis:3.2"
							args = ["${NOMAD_META_hello}", "${NOMAD_ALLOC_INDEX}"]
							pids_limit = "${NOMAD_ALLOC_INDEX + 2}"
						}`),
			spec: dockerSpec,
			ctx:  variableCtx,
			expected: &dockerConfig{
				Image:     "redis:3.2",
				Args:      []string{"world", "2"},
				PidsLimit: helper.Int64ToPtr(4),
				Devices:   []DockerDevice{},
			},
			expectedType: &dockerConfig{},
		},
		{
			name: "multi attr variables json",
			config: jsonConfigToInterface(t, `
						{
							"Config": {
								"image": "redis:3.2",
								"args": ["foo", "bar"]
			                }
						}`),
			spec: dockerSpec,
			ctx:  defaultCtx,
			expected: &dockerConfig{
				Image:   "redis:3.2",
				Args:    []string{"foo", "bar"},
				Devices: []DockerDevice{},
			},
			expectedType: &dockerConfig{},
		},
		{
			name: "port_map",
			config: hclConfigToInterface(t, `
			config {
				image = "redis:3.2"
				port_map {
					foo = "db"
					bar = "db2"
				}
			}`),
			spec: dockerSpec,
			ctx:  defaultCtx,
			expected: &dockerConfig{
				Image: "redis:3.2",
				PortMap: map[string]string{
					"foo": "db",
					"bar": "db2",
				},
				Devices: []DockerDevice{},
			},
			expectedType: &dockerConfig{},
		},
		{
			name: "port_map json",
			config: jsonConfigToInterface(t, `
							{
								"Config": {
									"image": "redis:3.2",
									"port_map": [{
										"foo": "db",
										"bar": "db2"
									}]
				                }
							}`),
			spec: dockerSpec,
			ctx:  defaultCtx,
			expected: &dockerConfig{
				Image: "redis:3.2",
				PortMap: map[string]string{
					"foo": "db",
					"bar": "db2",
				},
				Devices: []DockerDevice{},
			},
			expectedType: &dockerConfig{},
		},
		{
			name: "devices",
			config: hclConfigToInterface(t, `
						config {
							image = "redis:3.2"
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
			spec: dockerSpec,
			ctx:  defaultCtx,
			expected: &dockerConfig{
				Image: "redis:3.2",
				Devices: []DockerDevice{
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
			},
			expectedType: &dockerConfig{},
		},
		{
			name: "devices json",
			config: jsonConfigToInterface(t, `
							{
								"Config": {
									"image": "redis:3.2",
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
			spec: dockerSpec,
			ctx:  defaultCtx,
			expected: &dockerConfig{
				Image: "redis:3.2",
				Devices: []DockerDevice{
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
			},
			expectedType: &dockerConfig{},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Logf("Val: % #v", pretty.Formatter(c.config))
			// Parse the interface
			ctyValue, diag := ParseHclInterface(c.config, c.spec, c.ctx)
			if diag.HasErrors() {
				for _, err := range diag.Errs() {
					t.Error(err)
				}
				t.FailNow()
			}

			// Convert cty-value to go structs
			require.NoError(t, gocty.FromCtyValue(ctyValue, c.expectedType))

			require.EqualValues(t, c.expected, c.expectedType)

		})
	}
}
