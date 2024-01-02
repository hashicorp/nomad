// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hclspecutils

import (
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/function/stdlib"
)

// specFuncs exposes the stdlib functions.
var specFuncs = map[string]function.Function{
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
