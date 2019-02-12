package hclutils

import (
	"testing"

	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/ast"
	"github.com/hashicorp/nomad/helper/pluginutils/hclspecutils"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/require"
	"github.com/ugorji/go/codec"
	"github.com/zclconf/go-cty/cty"
)

type HCLParserBuilder struct {
	t         *testing.T
	configStr string
	format    string
	spec      *hclspec.Spec
	vars      map[string]cty.Value
}

func NewConfigParser(t *testing.T) *HCLParserBuilder {
	return &HCLParserBuilder{t: t}
}

func (b *HCLParserBuilder) Json(configStr string) *HCLParserBuilder {
	b.configStr = configStr
	b.format = "json"
	return b
}

func (b *HCLParserBuilder) Hcl(configStr string) *HCLParserBuilder {
	b.configStr = configStr
	b.format = "hcl"
	return b
}

func (b *HCLParserBuilder) Vars(vars map[string]cty.Value) *HCLParserBuilder {
	b.vars = vars
	return b
}

func (b *HCLParserBuilder) Spec(spec *hclspec.Spec) *HCLParserBuilder {
	b.spec = spec
	return b
}

func (b *HCLParserBuilder) Parse(taskConfig interface{}) {
	decSpec, diags := hclspecutils.Convert(b.spec)
	require.Empty(b.t, diags)

	var config interface{}
	switch b.format {
	case "hcl":
		config = HclConfigToInterface(b.t, b.configStr)
	case "json":
		config = JsonConfigToInterface(b.t, b.configStr)
	default:
		require.Fail(b.t, "unexpected format: "+b.format)
	}

	ctyValue, diag := ParseHclInterface(config, decSpec, b.vars)
	require.Empty(b.t, diag)

	// encode
	dtc := &drivers.TaskConfig{}
	require.NoError(b.t, dtc.EncodeDriverConfig(ctyValue))

	// decode
	require.NoError(b.t, dtc.DecodeDriverConfig(taskConfig))
}

func HclConfigToInterface(t *testing.T, config string) interface{} {
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

func JsonConfigToInterface(t *testing.T, config string) interface{} {
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
