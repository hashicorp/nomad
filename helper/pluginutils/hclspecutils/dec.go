// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hclspecutils

import (
	"fmt"

	hcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
)

var (
	// nilSpecDiagnostic is the diagnostic value returned if a nil value is
	// given
	nilSpecDiagnostic = &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "nil spec given",
		Detail:   "Can not convert a nil specification. Pass a valid spec",
	}

	// emptyPos is the position used when parsing hcl expressions
	emptyPos = hcl.Pos{
		Line:   0,
		Column: 0,
		Byte:   0,
	}

	// specCtx is the context used to evaluate expressions.
	specCtx = &hcl.EvalContext{
		Functions: specFuncs,
	}
)

// Convert converts a Spec to an hcl specification.
func Convert(spec *hclspec.Spec) (hcldec.Spec, hcl.Diagnostics) {
	if spec == nil {
		return nil, hcl.Diagnostics([]*hcl.Diagnostic{nilSpecDiagnostic})
	}

	return decodeSpecBlock(spec, "")
}

// decodeSpecBlock is the recursive entry point that converts between the two
// spec types.
func decodeSpecBlock(spec *hclspec.Spec, impliedName string) (hcldec.Spec, hcl.Diagnostics) {
	switch spec.Block.(type) {

	case *hclspec.Spec_Object:
		return decodeObjectSpec(spec.GetObject())

	case *hclspec.Spec_Array:
		return decodeArraySpec(spec.GetArray())

	case *hclspec.Spec_Attr:
		return decodeAttrSpec(spec.GetAttr(), impliedName)

	case *hclspec.Spec_BlockValue:
		return decodeBlockSpec(spec.GetBlockValue(), impliedName)

	case *hclspec.Spec_BlockAttrs:
		return decodeBlockAttrsSpec(spec.GetBlockAttrs(), impliedName)

	case *hclspec.Spec_BlockList:
		return decodeBlockListSpec(spec.GetBlockList(), impliedName)

	case *hclspec.Spec_BlockSet:
		return decodeBlockSetSpec(spec.GetBlockSet(), impliedName)

	case *hclspec.Spec_BlockMap:
		return decodeBlockMapSpec(spec.GetBlockMap(), impliedName)

	case *hclspec.Spec_Default:
		return decodeDefaultSpec(spec.GetDefault())

	case *hclspec.Spec_Literal:
		return decodeLiteralSpec(spec.GetLiteral())

	default:
		// Should never happen, because the above cases should be exhaustive
		// for our schema.
		var diags hcl.Diagnostics
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid spec block",
			Detail:   fmt.Sprintf("Blocks of type %T are not expected here.", spec.Block),
		})
		return nil, diags
	}
}

func decodeObjectSpec(obj *hclspec.Object) (hcldec.Spec, hcl.Diagnostics) {
	var diags hcl.Diagnostics
	spec := make(hcldec.ObjectSpec)
	for attr, block := range obj.GetAttributes() {
		propSpec, propDiags := decodeSpecBlock(block, attr)
		diags = append(diags, propDiags...)
		spec[attr] = propSpec
	}

	return spec, diags
}

func decodeArraySpec(a *hclspec.Array) (hcldec.Spec, hcl.Diagnostics) {
	values := a.GetValues()
	var diags hcl.Diagnostics
	spec := make(hcldec.TupleSpec, 0, len(values))
	for _, block := range values {
		elemSpec, elemDiags := decodeSpecBlock(block, "")
		diags = append(diags, elemDiags...)
		spec = append(spec, elemSpec)
	}

	return spec, diags
}

func decodeAttrSpec(attr *hclspec.Attr, impliedName string) (hcldec.Spec, hcl.Diagnostics) {
	// Convert the string type to an hcl.Expression
	typeExpr, diags := hclsyntax.ParseExpression([]byte(attr.GetType()), "proto", emptyPos)
	if diags.HasErrors() {
		return nil, diags
	}

	spec := &hcldec.AttrSpec{
		Name:     impliedName,
		Required: attr.GetRequired(),
	}

	if n := attr.GetName(); n != "" {
		spec.Name = n
	}

	var typeDiags hcl.Diagnostics
	spec.Type, typeDiags = evalTypeExpr(typeExpr)
	diags = append(diags, typeDiags...)

	if spec.Name == "" {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Missing name in attribute spec",
			Detail:   "The name attribute is required, to specify the attribute name that is expected in an input HCL file.",
		})
		return nil, diags
	}

	return spec, diags
}

func decodeBlockSpec(block *hclspec.Block, impliedName string) (hcldec.Spec, hcl.Diagnostics) {
	spec := &hcldec.BlockSpec{
		TypeName: impliedName,
		Required: block.GetRequired(),
	}

	if n := block.GetName(); n != "" {
		spec.TypeName = n
	}

	nested, diags := decodeBlockNestedSpec(block.GetNested())
	spec.Nested = nested
	return spec, diags
}

func decodeBlockAttrsSpec(block *hclspec.BlockAttrs, impliedName string) (hcldec.Spec, hcl.Diagnostics) {
	// Convert the string type to an hcl.Expression
	typeExpr, diags := hclsyntax.ParseExpression([]byte(block.GetType()), "proto", emptyPos)
	if diags.HasErrors() {
		return nil, diags
	}

	spec := &hcldec.BlockAttrsSpec{
		TypeName: impliedName,
		Required: block.GetRequired(),
	}

	if n := block.GetName(); n != "" {
		spec.TypeName = n
	}

	var typeDiags hcl.Diagnostics
	spec.ElementType, typeDiags = evalTypeExpr(typeExpr)
	diags = append(diags, typeDiags...)

	if spec.TypeName == "" {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Missing name in block_attrs spec",
			Detail:   "The name attribute is required, to specify the block attr name that is expected in an input HCL file.",
		})
		return nil, diags
	}

	return spec, diags
}

func decodeBlockListSpec(block *hclspec.BlockList, impliedName string) (hcldec.Spec, hcl.Diagnostics) {
	spec := &hcldec.BlockListSpec{
		TypeName: impliedName,
		MinItems: int(block.GetMinItems()),
		MaxItems: int(block.GetMaxItems()),
	}

	if n := block.GetName(); n != "" {
		spec.TypeName = n
	}

	nested, diags := decodeBlockNestedSpec(block.GetNested())
	spec.Nested = nested

	if spec.TypeName == "" {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Missing name in block_list spec",
			Detail:   "The name attribute is required, to specify the block type name that is expected in an input HCL file.",
		})
		return nil, diags
	}

	return spec, diags
}

func decodeBlockSetSpec(block *hclspec.BlockSet, impliedName string) (hcldec.Spec, hcl.Diagnostics) {
	spec := &hcldec.BlockSetSpec{
		TypeName: impliedName,
		MinItems: int(block.GetMinItems()),
		MaxItems: int(block.GetMaxItems()),
	}

	if n := block.GetName(); n != "" {
		spec.TypeName = n
	}

	nested, diags := decodeBlockNestedSpec(block.GetNested())
	spec.Nested = nested

	if spec.TypeName == "" {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Missing name in block_set spec",
			Detail:   "The name attribute is required, to specify the block type name that is expected in an input HCL file.",
		})
		return nil, diags
	}

	return spec, diags
}

func decodeBlockMapSpec(block *hclspec.BlockMap, impliedName string) (hcldec.Spec, hcl.Diagnostics) {
	spec := &hcldec.BlockMapSpec{
		TypeName:   impliedName,
		LabelNames: block.GetLabels(),
	}

	if n := block.GetName(); n != "" {
		spec.TypeName = n
	}

	nested, diags := decodeBlockNestedSpec(block.GetNested())
	spec.Nested = nested

	if spec.TypeName == "" {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Missing name in block_map spec",
			Detail:   "The name attribute is required, to specify the block type name that is expected in an input HCL file.",
		})
		return nil, diags
	}
	if len(spec.LabelNames) < 1 {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid block label name list",
			Detail:   "A block_map must have at least one label specified.",
		})
		return nil, diags
	}

	return spec, diags
}

func decodeBlockNestedSpec(spec *hclspec.Spec) (hcldec.Spec, hcl.Diagnostics) {
	if spec == nil {
		return nil, hcl.Diagnostics([]*hcl.Diagnostic{
			{
				Severity: hcl.DiagError,
				Summary:  "Missing spec block",
				Detail:   "A block spec must have exactly one child spec specifying how to decode block contents.",
			}})
	}

	return decodeSpecBlock(spec, "")
}

func decodeLiteralSpec(l *hclspec.Literal) (hcldec.Spec, hcl.Diagnostics) {
	// Convert the string value to an hcl.Expression
	valueExpr, diags := hclsyntax.ParseExpression([]byte(l.GetValue()), "proto", emptyPos)
	if diags.HasErrors() {
		return nil, diags
	}

	value, valueDiags := valueExpr.Value(specCtx)
	diags = append(diags, valueDiags...)
	if diags.HasErrors() {
		return nil, diags
	}

	return &hcldec.LiteralSpec{
		Value: value,
	}, diags
}

func decodeDefaultSpec(d *hclspec.Default) (hcldec.Spec, hcl.Diagnostics) {
	// Parse the primary
	primary, diags := decodeSpecBlock(d.GetPrimary(), "")
	if diags.HasErrors() {
		return nil, diags
	}

	// Parse the default
	def, defDiags := decodeSpecBlock(d.GetDefault(), "")
	diags = append(diags, defDiags...)
	if diags.HasErrors() {
		return nil, diags
	}

	spec := &hcldec.DefaultSpec{
		Primary: primary,
		Default: def,
	}

	return spec, diags
}
