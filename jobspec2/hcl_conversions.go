package jobspec2

import (
	"fmt"
	"reflect"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/nomad/api"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/gocty"
)

var hclDecoder *gohcl.Decoder

func init() {
	hclDecoder = &gohcl.Decoder{}

	// time conversion
	d := time.Duration(0)
	hclDecoder.RegisterExprDecoder(reflect.TypeOf(d), decodeDuration)
	hclDecoder.RegisterExprDecoder(reflect.TypeOf(&d), decodeDuration)

	// affinity
	hclDecoder.RegisterBlockDecoder(reflect.TypeOf(api.Affinity{}), decodeAffinity)
	hclDecoder.RegisterSchema(reflect.TypeOf(api.Affinity{}), hcldec.ImpliedSchema(affinitySpec), false)

	hclDecoder.RegisterBlockDecoder(reflect.TypeOf(api.Constraint{}), decodeConstraint)
	hclDecoder.RegisterSchema(reflect.TypeOf(api.Constraint{}), hcldec.ImpliedSchema(constraintSpec), false)

}

func decodeDuration(expr hcl.Expression, ctx *hcl.EvalContext, val interface{}) hcl.Diagnostics {
	srcVal, diags := expr.Value(ctx)

	if srcVal.Type() == cty.String {
		dur, err := time.ParseDuration(srcVal.AsString())
		if err != nil {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Unsuitable value type",
				Detail:   fmt.Sprintf("Unsuitable duration value: %s", err.Error()),
				Subject:  expr.StartRange().Ptr(),
				Context:  expr.Range().Ptr(),
			})
			return diags
		}

		srcVal = cty.NumberIntVal(int64(dur))
	}

	if srcVal.Type() != cty.Number {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Unsuitable value type",
			Detail:   fmt.Sprintf("Unsuitable value: expected a string but found %s", srcVal.Type()),
			Subject:  expr.StartRange().Ptr(),
			Context:  expr.Range().Ptr(),
		})
		return diags

	}

	err := gocty.FromCtyValue(srcVal, val)
	if err != nil {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Unsuitable value type",
			Detail:   fmt.Sprintf("Unsuitable value: %s", err.Error()),
			Subject:  expr.StartRange().Ptr(),
			Context:  expr.Range().Ptr(),
		})
	}

	return diags
}

var affinitySpec = hcldec.ObjectSpec{
	"attribute": &hcldec.AttrSpec{"attribute", cty.String, false},
	"value":     &hcldec.AttrSpec{"value", cty.String, false},
	"operator":  &hcldec.AttrSpec{"operator", cty.String, false},
	"weight":    &hcldec.AttrSpec{"weight", cty.Number, false},

	api.ConstraintVersion:        &hcldec.AttrSpec{api.ConstraintVersion, cty.String, false},
	api.ConstraintSemver:         &hcldec.AttrSpec{api.ConstraintSemver, cty.String, false},
	api.ConstraintRegex:          &hcldec.AttrSpec{api.ConstraintRegex, cty.String, false},
	api.ConstraintSetContains:    &hcldec.AttrSpec{api.ConstraintSetContains, cty.String, false},
	api.ConstraintSetContainsAll: &hcldec.AttrSpec{api.ConstraintSetContainsAll, cty.String, false},
	api.ConstraintSetContainsAny: &hcldec.AttrSpec{api.ConstraintSetContainsAny, cty.String, false},
}

func schemaAffinity() (schema *hcl.BodySchema, partial bool) {
	return
}

func decodeAffinity(body hcl.Body, ctx *hcl.EvalContext, val interface{}) hcl.Diagnostics {
	a := val.(*api.Affinity)
	v, diags := hcldec.Decode(body, affinitySpec, ctx)
	if len(diags) != 0 {
		return diags
	}

	attr := func(attr string) string {
		a := v.GetAttr(attr)
		if a.IsNull() {
			return ""
		}
		return a.AsString()
	}
	a.LTarget = attr("attribute")
	a.RTarget = attr("value")
	a.Operand = attr("operator")
	weight := v.GetAttr("weight")
	if !weight.IsNull() {
		w, _ := weight.AsBigFloat().Int64()
		a.Weight = int8ToPtr(int8(w))
	}

	// If "version" is provided, set the operand
	// to "version" and the value to the "RTarget"
	if affinity := attr(api.ConstraintVersion); affinity != "" {
		a.Operand = api.ConstraintVersion
		a.RTarget = affinity
	}

	// If "semver" is provided, set the operand
	// to "semver" and the value to the "RTarget"
	if affinity := attr(api.ConstraintSemver); affinity != "" {
		a.Operand = api.ConstraintSemver
		a.RTarget = affinity
	}

	// If "regexp" is provided, set the operand
	// to "regexp" and the value to the "RTarget"
	if affinity := attr(api.ConstraintRegex); affinity != "" {
		a.Operand = api.ConstraintRegex
		a.RTarget = affinity
	}

	// If "set_contains_any" is provided, set the operand
	// to "set_contains_any" and the value to the "RTarget"
	if affinity := attr(api.ConstraintSetContainsAny); affinity != "" {
		a.Operand = api.ConstraintSetContainsAny
		a.RTarget = affinity
	}

	// If "set_contains_all" is provided, set the operand
	// to "set_contains_all" and the value to the "RTarget"
	if affinity := attr(api.ConstraintSetContainsAll); affinity != "" {
		a.Operand = api.ConstraintSetContainsAll
		a.RTarget = affinity
	}

	// set_contains is a synonym of set_contains_all
	if affinity := attr(api.ConstraintSetContains); affinity != "" {
		a.Operand = api.ConstraintSetContains
		a.RTarget = affinity
	}

	if a.Operand == "" {
		a.Operand = "="
	}
	return diags
}

var constraintSpec = hcldec.ObjectSpec{
	"attribute": &hcldec.AttrSpec{"attribute", cty.String, false},
	"value":     &hcldec.AttrSpec{"value", cty.String, false},
	"operator":  &hcldec.AttrSpec{"operator", cty.String, false},

	api.ConstraintDistinctProperty:  &hcldec.AttrSpec{api.ConstraintDistinctProperty, cty.String, false},
	api.ConstraintDistinctHosts:     &hcldec.AttrSpec{api.ConstraintDistinctHosts, cty.Bool, false},
	api.ConstraintRegex:             &hcldec.AttrSpec{api.ConstraintRegex, cty.String, false},
	api.ConstraintVersion:           &hcldec.AttrSpec{api.ConstraintVersion, cty.String, false},
	api.ConstraintSemver:            &hcldec.AttrSpec{api.ConstraintSemver, cty.String, false},
	api.ConstraintSetContains:       &hcldec.AttrSpec{api.ConstraintSetContains, cty.String, false},
	api.ConstraintSetContainsAll:    &hcldec.AttrSpec{api.ConstraintSetContainsAll, cty.String, false},
	api.ConstraintSetContainsAny:    &hcldec.AttrSpec{api.ConstraintSetContainsAny, cty.String, false},
	api.ConstraintAttributeIsSet:    &hcldec.AttrSpec{api.ConstraintAttributeIsSet, cty.String, false},
	api.ConstraintAttributeIsNotSet: &hcldec.AttrSpec{api.ConstraintAttributeIsNotSet, cty.String, false},
}

func decodeConstraint(body hcl.Body, ctx *hcl.EvalContext, val interface{}) hcl.Diagnostics {
	c := val.(*api.Constraint)

	v, diags := hcldec.Decode(body, constraintSpec, ctx)
	if len(diags) != 0 {
		return diags
	}

	attr := func(attr string) string {
		a := v.GetAttr(attr)
		if a.IsNull() {
			return ""
		}
		return a.AsString()
	}

	c.LTarget = attr("attribute")
	c.RTarget = attr("value")
	c.Operand = attr("operator")

	// If "version" is provided, set the operand
	// to "version" and the value to the "RTarget"
	if constraint := attr(api.ConstraintVersion); constraint != "" {
		c.Operand = api.ConstraintVersion
		c.RTarget = constraint
	}

	// If "semver" is provided, set the operand
	// to "semver" and the value to the "RTarget"
	if constraint := attr(api.ConstraintSemver); constraint != "" {
		c.Operand = api.ConstraintSemver
		c.RTarget = constraint
	}

	// If "regexp" is provided, set the operand
	// to "regexp" and the value to the "RTarget"
	if constraint := attr(api.ConstraintRegex); constraint != "" {
		c.Operand = api.ConstraintRegex
		c.RTarget = constraint
	}

	// If "set_contains" is provided, set the operand
	// to "set_contains" and the value to the "RTarget"
	if constraint := attr(api.ConstraintSetContains); constraint != "" {
		c.Operand = api.ConstraintSetContains
		c.RTarget = constraint
	}

	if d := v.GetAttr(api.ConstraintDistinctHosts); !d.IsNull() && d.True() {
		c.Operand = api.ConstraintDistinctHosts
	}

	if property := attr(api.ConstraintDistinctProperty); property != "" {
		c.Operand = api.ConstraintDistinctProperty
		c.LTarget = property
	}

	if c.Operand == "" {
		c.Operand = "="
	}
	return diags
}
