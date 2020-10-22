package jobspec2

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/ext/dynblock"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	hcljson "github.com/hashicorp/hcl/v2/json"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/jobspec2/hclutil"
	"github.com/zclconf/go-cty/cty"
)

func Parse(path string, r io.Reader) (*api.Job, error) {
	return ParseWithArgs(path, r, nil, false)
}

func toVars(vars map[string]string) cty.Value {
	attrs := make(map[string]cty.Value, len(vars))
	for k, v := range vars {
		attrs[k] = cty.StringVal(v)
	}

	return cty.ObjectVal(attrs)
}

func ParseWithArgs(path string, r io.Reader, vars map[string]string, allowFS bool) (*api.Job, error) {
	if path == "" {
		if f, ok := r.(*os.File); ok {
			path = f.Name()
		}
	}
	basedir := filepath.Dir(path)

	// Copy the reader into an in-memory buffer first since HCL requires it.
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		return nil, err
	}

	evalContext := &hcl.EvalContext{
		Functions: Functions(basedir, allowFS),
		Variables: map[string]cty.Value{
			"vars": toVars(vars),
		},
		UnknownVariable: func(expr string) (cty.Value, error) {
			v := "${" + expr + "}"
			return cty.StringVal(v), nil
		},
	}
	var result struct {
		Job jobWrapper `hcl:"job,block"`
	}
	err := decode(path, buf.Bytes(), evalContext, &result)
	if err != nil {
		return nil, err
	}

	normalizeJob(&result.Job)
	return result.Job.Job, nil
}

func decode(filename string, src []byte, ctx *hcl.EvalContext, target interface{}) error {
	var file *hcl.File
	var diags hcl.Diagnostics

	if !isJSON(src) {
		file, diags = hclsyntax.ParseConfig(src, filename, hcl.Pos{Line: 1, Column: 1})
	} else {
		file, diags = hcljson.Parse(src, filename)

	}
	if diags.HasErrors() {
		return diags
	}

	body := hclutil.BlocksAsAttrs(file.Body)
	body = dynblock.Expand(body, ctx)
	diags = hclDecoder.DecodeBody(body, ctx, target)
	if diags.HasErrors() {
		var str strings.Builder
		for i, diag := range diags {
			if i != 0 {
				str.WriteByte('\n')
			}
			str.WriteString(diag.Error())
		}
		return errors.New(str.String())
	}
	diags = append(diags, decodeMapInterfaceType(target, ctx)...)
	return nil
}

func isJSON(src []byte) bool {
	for _, c := range src {
		if c == ' ' {
			continue
		}

		return c == '{'
	}
	return false
}
