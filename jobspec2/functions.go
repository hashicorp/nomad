// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package jobspec2

import (
	"fmt"

	"github.com/hashicorp/go-cty-funcs/cidr"
	"github.com/hashicorp/go-cty-funcs/crypto"
	"github.com/hashicorp/go-cty-funcs/encoding"
	"github.com/hashicorp/go-cty-funcs/filesystem"
	"github.com/hashicorp/go-cty-funcs/uuid"
	"github.com/hashicorp/hcl/v2/ext/tryfunc"
	"github.com/hashicorp/hcl/v2/ext/typeexpr"
	ctyyaml "github.com/zclconf/go-cty-yaml"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/function/stdlib"
)

// Functions returns the set of functions that should be used to when
// evaluating expressions in the receiving scope.
//
// basedir is used with file functions and allows a user to reference a file
// using local path. Usually basedir is the directory in which the config file
// is located
func Functions(basedir string, allowFS bool) map[string]function.Function {
	funcs := map[string]function.Function{
		"abs":             stdlib.AbsoluteFunc,
		"base64decode":    encoding.Base64DecodeFunc,
		"base64encode":    encoding.Base64EncodeFunc,
		"bcrypt":          crypto.BcryptFunc,
		"can":             tryfunc.CanFunc,
		"ceil":            stdlib.CeilFunc,
		"chomp":           stdlib.ChompFunc,
		"chunklist":       stdlib.ChunklistFunc,
		"cidrhost":        cidr.HostFunc,
		"cidrnetmask":     cidr.NetmaskFunc,
		"cidrsubnet":      cidr.SubnetFunc,
		"cidrsubnets":     cidr.SubnetsFunc,
		"coalesce":        stdlib.CoalesceFunc,
		"coalescelist":    stdlib.CoalesceListFunc,
		"compact":         stdlib.CompactFunc,
		"concat":          stdlib.ConcatFunc,
		"contains":        stdlib.ContainsFunc,
		"convert":         typeexpr.ConvertFunc,
		"csvdecode":       stdlib.CSVDecodeFunc,
		"distinct":        stdlib.DistinctFunc,
		"element":         stdlib.ElementFunc,
		"flatten":         stdlib.FlattenFunc,
		"floor":           stdlib.FloorFunc,
		"format":          stdlib.FormatFunc,
		"formatdate":      stdlib.FormatDateFunc,
		"formatlist":      stdlib.FormatListFunc,
		"indent":          stdlib.IndentFunc,
		"index":           stdlib.IndexFunc,
		"join":            stdlib.JoinFunc,
		"jsondecode":      stdlib.JSONDecodeFunc,
		"jsonencode":      stdlib.JSONEncodeFunc,
		"keys":            stdlib.KeysFunc,
		"length":          stdlib.LengthFunc,
		"log":             stdlib.LogFunc,
		"lookup":          stdlib.LookupFunc,
		"lower":           stdlib.LowerFunc,
		"max":             stdlib.MaxFunc,
		"md5":             crypto.Md5Func,
		"merge":           stdlib.MergeFunc,
		"min":             stdlib.MinFunc,
		"parseint":        stdlib.ParseIntFunc,
		"pow":             stdlib.PowFunc,
		"range":           stdlib.RangeFunc,
		"reverse":         stdlib.ReverseFunc,
		"replace":         stdlib.ReplaceFunc,
		"regex_replace":   stdlib.RegexReplaceFunc,
		"rsadecrypt":      crypto.RsaDecryptFunc,
		"setintersection": stdlib.SetIntersectionFunc,
		"setproduct":      stdlib.SetProductFunc,
		"setunion":        stdlib.SetUnionFunc,
		"sha1":            crypto.Sha1Func,
		"sha256":          crypto.Sha256Func,
		"sha512":          crypto.Sha512Func,
		"signum":          stdlib.SignumFunc,
		"slice":           stdlib.SliceFunc,
		"sort":            stdlib.SortFunc,
		"split":           stdlib.SplitFunc,
		"strlen":          stdlib.StrlenFunc,
		"strrev":          stdlib.ReverseFunc,
		"substr":          stdlib.SubstrFunc,
		"timeadd":         stdlib.TimeAddFunc,
		"title":           stdlib.TitleFunc,
		"trim":            stdlib.TrimFunc,
		"trimprefix":      stdlib.TrimPrefixFunc,
		"trimspace":       stdlib.TrimSpaceFunc,
		"trimsuffix":      stdlib.TrimSuffixFunc,
		"try":             tryfunc.TryFunc,
		"upper":           stdlib.UpperFunc,
		"urlencode":       encoding.URLEncodeFunc,
		"uuidv4":          uuid.V4Func,
		"uuidv5":          uuid.V5Func,
		"values":          stdlib.ValuesFunc,
		"yamldecode":      ctyyaml.YAMLDecodeFunc,
		"yamlencode":      ctyyaml.YAMLEncodeFunc,
		"zipmap":          stdlib.ZipmapFunc,

		// filesystem calls
		"abspath":    guardFS(allowFS, filesystem.AbsPathFunc),
		"basename":   guardFS(allowFS, filesystem.BasenameFunc),
		"dirname":    guardFS(allowFS, filesystem.DirnameFunc),
		"file":       guardFS(allowFS, filesystem.MakeFileFunc(basedir, false)),
		"filebase64": guardFS(allowFS, filesystem.MakeFileFunc(basedir, true)),
		"fileexists": guardFS(allowFS, filesystem.MakeFileExistsFunc(basedir)),
		"fileset":    guardFS(allowFS, filesystem.MakeFileSetFunc(basedir)),
		"pathexpand": guardFS(allowFS, filesystem.PathExpandFunc),
	}

	return funcs
}

func guardFS(allowFS bool, fn function.Function) function.Function {
	if allowFS {
		return fn
	}

	spec := &function.Spec{
		Params:   fn.Params(),
		VarParam: fn.VarParam(),
		Type: func([]cty.Value) (cty.Type, error) {
			return cty.DynamicPseudoType, fmt.Errorf("filesystem function disabled")
		},
		Impl: func([]cty.Value, cty.Type) (cty.Value, error) {
			return cty.DynamicVal, fmt.Errorf("filesystem functions disabled")
		},
	}

	return function.New(spec)
}
