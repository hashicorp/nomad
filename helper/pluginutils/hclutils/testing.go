// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hclutils

import (
	"testing"

	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/ast"
	"github.com/hashicorp/nomad/helper/pluginutils/hclspecutils"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

type HCLParser struct {
	spec *hclspec.Spec
	vars map[string]cty.Value
}

// NewConfigParser return a helper for parsing drivers TaskConfig
// Parser is an immutable object can be used in multiple tests
func NewConfigParser(spec *hclspec.Spec) *HCLParser {
	return &HCLParser{
		spec: spec,
	}
}

// WithVars returns a new parser that uses passed vars when interpolated strings in config
func (b *HCLParser) WithVars(vars map[string]cty.Value) *HCLParser {
	return &HCLParser{
		spec: b.spec,
		vars: vars,
	}
}

// ParseJson parses the json config string and decode it into the `out` parameter.
// out parameter should be a golang reference to a driver specific TaskConfig reference.
// The function terminates and reports errors if any is found during conversion.
//
//	var tc *TaskConfig
//	hclutils.NewConfigParser(spec).ParseJson(t, configString, &tc)
func (b *HCLParser) ParseJson(t *testing.T, configStr string, out interface{}) {
	config := JsonConfigToInterface(t, configStr)
	b.parse(t, config, out)
}

// ParseHCL parses the hcl config string and decode it into the `out` parameter.
// out parameter should be a golang reference to a driver specific TaskConfig reference.
// The function terminates and reports errors if any is found during conversion.
//
// # Sample invocation would be
//
// ```
// var tc *TaskConfig
// hclutils.NewConfigParser(spec).ParseHCL(t, configString, &tc)
// ```
func (b *HCLParser) ParseHCL(t *testing.T, configStr string, out interface{}) {
	config := HclConfigToInterface(t, configStr)
	b.parse(t, config, out)
}

func (b *HCLParser) parse(t *testing.T, config, out interface{}) {
	decSpec, diags := hclspecutils.Convert(b.spec)
	require.Empty(t, diags)

	ctyValue, diag, errs := ParseHclInterface(config, decSpec, b.vars)
	if len(errs) > 1 {
		t.Error("unexpected errors parsing file")
		for _, err := range errs {
			t.Errorf(" * %v", err)

		}
		t.FailNow()
	}
	require.Empty(t, diag)

	// encode
	dtc := &drivers.TaskConfig{}
	require.NoError(t, dtc.EncodeDriverConfig(ctyValue))

	// decode
	require.NoError(t, dtc.DecodeDriverConfig(out))
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
