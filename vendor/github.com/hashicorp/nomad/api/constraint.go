package api

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/zclconf/go-cty/cty"
)

const (
	ConstraintDistinctProperty  = "distinct_property"
	ConstraintDistinctHosts     = "distinct_hosts"
	ConstraintRegex             = "regexp"
	ConstraintVersion           = "version"
	ConstraintSemver            = "semver"
	ConstraintSetContains       = "set_contains"
	ConstraintSetContainsAll    = "set_contains_all"
	ConstraintSetContainsAny    = "set_contains_any"
	ConstraintAttributeIsSet    = "is_set"
	ConstraintAttributeIsNotSet = "is_not_set"
)

// Constraint is used to serialize a job placement constraint.
type Constraint struct {
	LTarget string `hcl:"attribute,optional"`
	RTarget string `hcl:"value,optional"`
	Operand string `hcl:"operator,optional"`
}

var constraintSpec = hcldec.ObjectSpec{
	"attribute": &hcldec.AttrSpec{"attribute", cty.String, false},
	"value":     &hcldec.AttrSpec{"value", cty.String, false},
	"operator":  &hcldec.AttrSpec{"operator", cty.String, false},

	ConstraintDistinctProperty:  &hcldec.AttrSpec{ConstraintDistinctProperty, cty.String, false},
	ConstraintDistinctHosts:     &hcldec.AttrSpec{ConstraintDistinctHosts, cty.Bool, false},
	ConstraintRegex:             &hcldec.AttrSpec{ConstraintRegex, cty.String, false},
	ConstraintVersion:           &hcldec.AttrSpec{ConstraintVersion, cty.String, false},
	ConstraintSemver:            &hcldec.AttrSpec{ConstraintSemver, cty.String, false},
	ConstraintSetContains:       &hcldec.AttrSpec{ConstraintSetContains, cty.String, false},
	ConstraintSetContainsAll:    &hcldec.AttrSpec{ConstraintSetContainsAll, cty.String, false},
	ConstraintSetContainsAny:    &hcldec.AttrSpec{ConstraintSetContainsAny, cty.String, false},
	ConstraintAttributeIsSet:    &hcldec.AttrSpec{ConstraintAttributeIsSet, cty.String, false},
	ConstraintAttributeIsNotSet: &hcldec.AttrSpec{ConstraintAttributeIsNotSet, cty.String, false},
}

func (c Constraint) HCLSchema() (schema *hcl.BodySchema, partial bool) {
	return hcldec.ImpliedSchema(constraintSpec), false
}

func (c *Constraint) DecodeHCL(body hcl.Body, ctx *hcl.EvalContext) hcl.Diagnostics {
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
	if constraint := attr(ConstraintVersion); constraint != "" {
		c.Operand = ConstraintVersion
		c.RTarget = constraint
	}

	// If "semver" is provided, set the operand
	// to "semver" and the value to the "RTarget"
	if constraint := attr(ConstraintSemver); constraint != "" {
		c.Operand = ConstraintSemver
		c.RTarget = constraint
	}

	// If "regexp" is provided, set the operand
	// to "regexp" and the value to the "RTarget"
	if constraint := attr(ConstraintRegex); constraint != "" {
		c.Operand = ConstraintRegex
		c.RTarget = constraint
	}

	// If "set_contains" is provided, set the operand
	// to "set_contains" and the value to the "RTarget"
	if constraint := attr(ConstraintSetContains); constraint != "" {
		c.Operand = ConstraintSetContains
		c.RTarget = constraint
	}

	if d := v.GetAttr(ConstraintDistinctHosts); !d.IsNull() && d.True() {
		c.Operand = ConstraintDistinctHosts
	}

	if property := attr(ConstraintDistinctProperty); property != "" {
		c.Operand = ConstraintDistinctProperty
		c.LTarget = property
	}

	if c.Operand == "" {
		c.Operand = "="
	}
	return diags
}

// NewConstraint generates a new job placement constraint.
func NewConstraint(left, operand, right string) *Constraint {
	return &Constraint{
		LTarget: left,
		RTarget: right,
		Operand: operand,
	}
}
