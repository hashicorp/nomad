// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hcl

import (
	"fmt"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/gocty"
)

// DecodeDuration is the decode function for time.Duration types. It supports
// both string and numeric values. String values are parsed using
// time.ParseDuration. Numeric values are expected to be in nanoseconds.
//
// This function duplicates that found within the jobspec2 package to avoid
// license conflicts.
func DecodeDuration(expr hcl.Expression, ctx *hcl.EvalContext, val any) hcl.Diagnostics {
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
