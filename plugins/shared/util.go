package shared

import (
	"bytes"
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
// body and return the interpolated value.
func ParseHclInterface(val interface{}, spec hcldec.Spec, ctx *hcl.EvalContext) (cty.Value, hcl.Diagnostics) {
	// Encode to json
	var buf bytes.Buffer
	enc := codec.NewEncoder(&buf, structs.JsonHandle)
	err := enc.Encode(val)
	if err != nil {
		// Convert to a hcl diagnostics message
		return cty.NilVal, hcl.Diagnostics([]*hcl.Diagnostic{
			{
				Severity: hcl.DiagError,
				Summary:  "Failed to JSON encode value",
				Detail:   fmt.Sprintf("JSON encoding failed: %v", err),
			}})
	}

	// Parse the json as hcl2
	hclFile, diag := hjson.Parse(buf.Bytes(), "")
	if diag.HasErrors() {
		return cty.NilVal, diag
	}

	value, decDiag := hcldec.Decode(hclFile.Body, spec, ctx)
	diag = diag.Extend(decDiag)
	if diag.HasErrors() {
		return cty.NilVal, diag
	}

	return value, diag
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
