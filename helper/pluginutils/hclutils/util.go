package hclutils

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/hashicorp/hcl2/hcl"
	hjson "github.com/hashicorp/hcl2/hcl/json"
	"github.com/hashicorp/hcl2/hcldec"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/ugorji/go/codec"
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

func formattedDiagnosticErrors(diag hcl.Diagnostics) []error {
	var errs []error
	for _, d := range diag {
		if d.Summary == "Extraneous JSON object property" {
			d.Summary = "Invalid label"
		}
		err := errors.New(fmt.Sprintf("%s: %s", d.Summary, d.Detail))
		errs = append(errs, err)
	}
	return errs
}
