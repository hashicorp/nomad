// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package jobspec2

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/hashicorp/go-cty-funcs/cidr"
	"github.com/hashicorp/go-cty-funcs/crypto"
	"github.com/hashicorp/go-cty-funcs/encoding"
	"github.com/hashicorp/go-cty-funcs/filesystem"
	"github.com/hashicorp/go-cty-funcs/uuid"
	"github.com/hashicorp/hcl/v2/ext/tryfunc"
	"github.com/hashicorp/hcl/v2/ext/typeexpr"
	"github.com/mitchellh/go-homedir"
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
		"abspath":     guardFS(allowFS, filesystem.AbsPathFunc),
		"basename":    guardFS(allowFS, filesystem.BasenameFunc),
		"dirname":     guardFS(allowFS, filesystem.DirnameFunc),
		"file":        guardFS(allowFS, filesystem.MakeFileFunc(basedir, false)),
		"fileescaped": guardFS(allowFS, fileEscaped(basedir)),
		"filebase64":  guardFS(allowFS, filesystem.MakeFileFunc(basedir, true)),
		"fileexists":  guardFS(allowFS, filesystem.MakeFileExistsFunc(basedir)),
		"fileset":     guardFS(allowFS, filesystem.MakeFileSetFunc(basedir)),
		"pathexpand":  guardFS(allowFS, filesystem.PathExpandFunc),
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

var escapeRegex = regexp.MustCompile(`(\%+|\$+){`)

// fileEscaped is a slightly stripped-down alternative implementation of
// go-cty's filesystem that escapes the file contents so they won't be further
// interpolated.
func fileEscaped(baseDir string) function.Function {

	return function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "path",
				Type: cty.String,
			},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			path := args[0].AsString()
			buf, err := readFileBytes(baseDir, path)
			if err != nil {
				return cty.UnknownVal(cty.String), err
			}

			if !utf8.Valid(buf) {
				return cty.UnknownVal(cty.String), fmt.Errorf("contents of %s are not valid UTF-8; use the filebase64 function to obtain the Base64 encoded contents or the other file functions (e.g. filemd5, filesha256) to obtain file hashing results instead", path)
			}

			src := string(buf)
			src = escape(src)

			return cty.StringVal(src), nil
		},
	})
}

func escape(src string) string {
	src = escapeRegex.ReplaceAllStringFunc(src, func(in string) string {
		switch {
		case strings.HasPrefix(in, "%{"):
			return "%" + in
		case strings.HasPrefix(in, "${"):
			return "$" + in
		default:
			return in
		}
	})

	return src
}

func readFileBytes(baseDir, path string) ([]byte, error) {
	path, err := homedir.Expand(path)
	if err != nil {
		return nil, fmt.Errorf("failed to expand ~: %s", err)
	}

	if !filepath.IsAbs(path) {
		path = filepath.Join(baseDir, path)
	}

	// Ensure that the path is canonical for the host OS
	path = filepath.Clean(path)

	src, err := ioutil.ReadFile(path)
	if err != nil {
		// ReadFile does not return Terraform-user-friendly error
		// messages, so we'll provide our own.
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no file exists at %s", path)
		}
		return nil, fmt.Errorf("failed to read %s", path)
	}

	return src, nil
}
