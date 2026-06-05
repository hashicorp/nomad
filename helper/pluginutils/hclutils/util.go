// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package hclutils

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"math/big"

	"github.com/hashicorp/go-msgpack/v2/codec"
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

// CtyValueToMapInterface converts a decoded cty value into a Go
// map[string]interface{}.
//
// ParseHclInterface returns a cty.Value and callers sometimes need a generic
// map payload (for example for plugin config maps). This helper converts that
// value recursively into native Go values.
func CtyValueToMapInterface(val cty.Value) (map[string]any, error) {
	if !val.IsKnown() {
		return nil, fmt.Errorf("value is not known")
	}

	if val.IsNull() {
		return nil, nil
	}

	t := val.Type()
	if !t.IsMapType() && !t.IsObjectType() {
		return nil, fmt.Errorf("expected map/object cty value, got %s", t.FriendlyName())
	}

	v, err := ctyValueToInterface(val)
	if err != nil {
		return nil, err
	}

	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected map/object cty value, got %T", v)
	}

	return m, nil
}

func ctyValueToInterface(val cty.Value) (interface{}, error) {
	t := val.Type()

	if val.IsNull() {
		return nil, nil
	}

	if !val.IsKnown() {
		return nil, fmt.Errorf("value is not known")
	}

	switch {
	case t.IsPrimitiveType():
		switch t {
		case cty.String:
			return val.AsString(), nil
		case cty.Number:
			if val.RawEquals(cty.PositiveInfinity) {
				return math.Inf(1), nil
			}
			if val.RawEquals(cty.NegativeInfinity) {
				return math.Inf(-1), nil
			}
			return smallestNumber(val.AsBigFloat()), nil
		case cty.Bool:
			return val.True(), nil
		default:
			panic("unsupported primitive type")
		}

	case t.IsListType(), t.IsSetType(), t.IsTupleType():
		result := []interface{}{}

		it := val.ElementIterator()
		for it.Next() {
			_, ev := it.Element()
			evi, err := ctyValueToInterface(ev)
			if err != nil {
				return nil, err
			}
			result = append(result, evi)
		}
		return result, nil

	case t.IsMapType():
		result := map[string]interface{}{}

		it := val.ElementIterator()
		for it.Next() {
			ek, ev := it.Element()

			evv, err := ctyValueToInterface(ev)
			if err != nil {
				return nil, err
			}

			result[ek.AsString()] = evv
		}
		return result, nil

	case t.IsObjectType():
		result := map[string]interface{}{}

		for k := range t.AttributeTypes() {
			av := val.GetAttr(k)
			avv, err := ctyValueToInterface(av)
			if err != nil {
				return nil, err
			}

			result[k] = avv
		}
		return result, nil

	case t.IsCapsuleType():
		return val.EncapsulatedValue(), nil

	default:
		return nil, fmt.Errorf("cannot serialize %s", t.FriendlyName())
	}
}

func smallestNumber(b *big.Float) interface{} {
	if v, acc := b.Int64(); acc == big.Exact {
		if int64(int(v)) == v {
			return int(v)
		}
		return v
	}

	v, _ := b.Float64()
	return v
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
