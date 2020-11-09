package jobspec2

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/ext/dynblock"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	hcljson "github.com/hashicorp/hcl/v2/json"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/jobspec2/hclutil"
)

func Parse(path string, r io.Reader) (*api.Job, error) {
	if path == "" {
		if f, ok := r.(*os.File); ok {
			path = f.Name()
		}
	}

	var buf bytes.Buffer
	_, err := io.Copy(&buf, r)
	if err != nil {
		return nil, err
	}

	return ParseWithConfig(&ParseConfig{
		Path:    path,
		Body:    buf.Bytes(),
		AllowFS: false,
		Strict:  true,
	})
}

func ParseWithConfig(args *ParseConfig) (*api.Job, error) {
	args.normalize()

	c := newJobConfig(args)
	err := decode(c)
	if err != nil {
		return nil, err
	}

	normalizeJob(c)
	return c.Job, nil
}

type ParseConfig struct {
	Path    string
	BaseDir string

	// Body is the HCL body
	Body []byte

	// AllowFS enables HCL functions that require file system accecss
	AllowFS bool

	// ArgVars is the CLI -var arguments
	ArgVars []string

	// VarFiles is the paths of variable data files
	VarFiles []string

	// Envs represent process environment variable
	Envs []string

	Strict bool

	// parsedVarFiles represent parsed HCL AST of the passed EnvVars
	parsedVarFiles []*hcl.File
}

func (c *ParseConfig) normalize() {
	if c.BaseDir == "" {
		c.BaseDir = filepath.Dir(c.Path)
	}
}

func decode(c *jobConfig) error {
	var file *hcl.File
	var diags hcl.Diagnostics

	pc := c.ParseConfig

	if !isJSON(pc.Body) {
		file, diags = hclsyntax.ParseConfig(pc.Body, pc.Path, hcl.Pos{Line: 1, Column: 1})
	} else {
		file, diags = hcljson.Parse(pc.Body, pc.Path)

	}

	parsedVarFiles, mdiags := parseVarFiles(pc.VarFiles)
	pc.parsedVarFiles = parsedVarFiles
	diags = append(diags, mdiags...)

	if diags.HasErrors() {
		return diags
	}

	body := hclutil.BlocksAsAttrs(file.Body)
	body = dynblock.Expand(body, c.EvalContext())
	diags = c.decodeBody(body)
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
	diags = append(diags, decodeMapInterfaceType(&c, c.EvalContext())...)
	return nil
}

func parseVarFiles(paths []string) ([]*hcl.File, hcl.Diagnostics) {
	if len(paths) == 0 {
		return nil, nil
	}

	files := make([]*hcl.File, 0, len(paths))
	var diags hcl.Diagnostics

	for _, p := range paths {
		body, err := ioutil.ReadFile(p)
		if err != nil {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Failed to read file",
				Detail:   fmt.Sprintf("failed to read %q: %v", p, err),
			})
			continue
		}

		var file *hcl.File
		var mdiags hcl.Diagnostics
		if !isJSON(body) {
			file, mdiags = hclsyntax.ParseConfig(body, p, hcl.Pos{Line: 1, Column: 1})
		} else {
			file, mdiags = hcljson.Parse(body, p)

		}

		files = append(files, file)
		diags = append(diags, mdiags...)
	}

	return files, diags
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
