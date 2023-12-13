// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hclutils

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/hashicorp/go-msgpack/codec"
	hcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hcldec"
	hjson "github.com/hashicorp/hcl/v2/json"

	"github.com/hashicorp/nomad/nomad/structs"

	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/function/stdlib"
)

// ParseHclInterface is used to convert an interface value representing a hcl2
// body and return the interpolated value. Vars may be nil if there are no
// variables to interpolate.
func ParseHclInterface(val interface{}, spec hcldec.Spec, vars map[string]cty.Value) (cty.Value, hcl.Diagnostics, []error) {
	evalCtx := &hcl.EvalContext{
		Variables: vars,
		Functions: GetStdlibFuncs(),
	}

	// Encode to json
	var buf bytes.Buffer
	enc := codec.NewEncoder(&buf, structs.JsonHandle)
	err := enc.Encode(val)
	if err != nil {
		// Convert to a hcl diagnostics message
		errorMessage := fmt.Sprintf("Label encoding failed: %v", err)
		return cty.NilVal,
			hcl.Diagnostics([]*hcl.Diagnostic{{
				Severity: hcl.DiagError,
				Summary:  "Failed to encode label value",
				Detail:   errorMessage,
			}}),
			[]error{errors.New(errorMessage)}
	}

	// Parse the json as hcl2
	hclFile, diag := hjson.Parse(buf.Bytes(), "")
	if diag.HasErrors() {
		return cty.NilVal, diag, formattedDiagnosticErrors(diag)
	}

	value, decDiag := hcldec.Decode(hclFile.Body, spec, evalCtx)
	diag = diag.Extend(decDiag)
	if diag.HasErrors() {
		return cty.NilVal, diag, formattedDiagnosticErrors(diag)
	}

	return value, diag, nil
}

// GetStdlibFuncs returns the set of stdlib functions.
func GetStdlibFuncs() map[string]function.Function {
	return map[string]function.Function{
		"abs":        stdlib.AbsoluteFunc,
		"coalesce":   stdlib.CoalesceFunc,
		"concat":     stdlib.ConcatFunc,
		"hasindex":   stdlib.HasIndexFunc,
		"int":        stdlib.IntFunc,
		"jsondecode": stdlib.JSONDecodeFunc,
		"jsonencode": stdlib.JSONEncodeFunc,
		"length":     stdlib.LengthFunc,
		"lower":      stdlib.LowerFunc,
		"max":        stdlib.MaxFunc,
		"min":        stdlib.MinFunc,
		"reverse":    stdlib.ReverseFunc,
		"strlen":     stdlib.StrlenFunc,
		"substr":     stdlib.SubstrFunc,
		"upper":      stdlib.UpperFunc,
	}
}

// TODO: update hcl2 library with better diagnostics formatting for streamed configs
// - should be arbitrary labels not JSON https://github.com/hashicorp/hcl2/blob/4fba5e1a75e382aed7f7a7993f2c4836a5e1cd52/hcl/json/structure.go#L66
// - should not print diagnostic subject https://github.com/hashicorp/hcl2/blob/4fba5e1a75e382aed7f7a7993f2c4836a5e1cd52/hcl/diagnostic.go#L77
func formattedDiagnosticErrors(diag hcl.Diagnostics) []error {
	var errs []error
	for _, d := range diag {
		if d.Summary == "Extraneous JSON object property" {
			d.Summary = "Invalid label"
		}
		err := fmt.Errorf("%s: %s", d.Summary, d.Detail)
		errs = append(errs, err)
	}
	return errs
}
