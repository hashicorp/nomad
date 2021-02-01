package jobspec2

import (
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/ext/dynblock"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/jobspec2/hclutil"
	"github.com/zclconf/go-cty/cty"
)

const (
	variablesLabel = "variables"
	variableLabel  = "variable"
	localsLabel    = "locals"
	vaultLabel     = "vault"
	taskLabel      = "task"

	inputVariablesAccessor = "var"
	localsAccessor         = "local"
)

type jobConfig struct {
	JobID string `hcl:",label"`
	Job   *api.Job

	ParseConfig *ParseConfig

	Vault *api.Vault  `hcl:"vault,block"`
	Tasks []*api.Task `hcl:"task,block"`

	InputVariables Variables
	LocalVariables Variables

	LocalBlocks []*LocalBlock
}

func newJobConfig(parseConfig *ParseConfig) *jobConfig {
	return &jobConfig{
		ParseConfig: parseConfig,

		InputVariables: Variables{},
		LocalVariables: Variables{},
	}
}

var jobConfigSchema = &hcl.BodySchema{
	Blocks: []hcl.BlockHeaderSchema{
		{Type: variablesLabel},
		{Type: variableLabel, LabelNames: []string{"name"}},
		{Type: localsLabel},
		{Type: "job", LabelNames: []string{"name"}},
	},
}

func (c *jobConfig) decodeBody(body hcl.Body) hcl.Diagnostics {
	content, diags := body.Content(jobConfigSchema)
	if len(diags) != 0 {
		return diags
	}

	diags = append(diags, c.decodeInputVariables(content)...)
	diags = append(diags, c.parseLocalVariables(content)...)
	diags = append(diags, c.collectInputVariableValues(c.ParseConfig.Envs, c.ParseConfig.parsedVarFiles, toVars(c.ParseConfig.ArgVars))...)

	_, moreDiags := c.InputVariables.Values()
	diags = append(diags, moreDiags...)
	_, moreDiags = c.LocalVariables.Values()
	diags = append(diags, moreDiags...)
	diags = append(diags, c.evaluateLocalVariables(c.LocalBlocks)...)

	nctx := c.EvalContext()

	diags = append(diags, c.decodeJob(content, nctx)...)
	return diags
}

// decodeInputVariables looks in the found blocks for 'variables' and
// 'variable' blocks. It should be called firsthand so that other blocks can
// use the variables.
func (c *jobConfig) decodeInputVariables(content *hcl.BodyContent) hcl.Diagnostics {
	var diags hcl.Diagnostics

	for _, block := range content.Blocks {
		switch block.Type {
		case variableLabel:
			moreDiags := c.InputVariables.decodeVariableBlock(block, nil)
			diags = append(diags, moreDiags...)
		case variablesLabel:
			attrs, moreDiags := block.Body.JustAttributes()
			diags = append(diags, moreDiags...)
			for key, attr := range attrs {
				moreDiags = c.InputVariables.decodeVariable(key, attr, nil)
				diags = append(diags, moreDiags...)
			}
		}
	}
	return diags
}

// parseLocalVariables looks in the found blocks for 'locals' blocks. It
// should be called after parsing input variables so that they can be
// referenced.
func (c *jobConfig) parseLocalVariables(content *hcl.BodyContent) hcl.Diagnostics {
	var diags hcl.Diagnostics

	for _, block := range content.Blocks {
		switch block.Type {
		case localsLabel:
			attrs, moreDiags := block.Body.JustAttributes()
			diags = append(diags, moreDiags...)
			for name, attr := range attrs {
				if _, found := c.LocalVariables[name]; found {
					diags = append(diags, &hcl.Diagnostic{
						Severity: hcl.DiagError,
						Summary:  "Duplicate value in " + localsLabel,
						Detail:   "Duplicate " + name + " definition found.",
						Subject:  attr.NameRange.Ptr(),
						Context:  block.DefRange.Ptr(),
					})
					return diags
				}
				c.LocalBlocks = append(c.LocalBlocks, &LocalBlock{
					Name: name,
					Expr: attr.Expr,
				})
			}
		}
	}

	return diags
}

func (c *jobConfig) decodeTopLevelExtras(content *hcl.BodyContent, ctx *hcl.EvalContext) hcl.Diagnostics {
	var diags hcl.Diagnostics

	var foundVault *hcl.Block
	for _, b := range content.Blocks {
		if b.Type == vaultLabel {
			if foundVault != nil {
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  fmt.Sprintf("Duplicate %s block", b.Type),
					Detail: fmt.Sprintf(
						"Only one block of type %q is allowed. Previous definition was at %s.",
						b.Type, foundVault.DefRange.String(),
					),
					Subject: &b.DefRange,
				})
				continue
			}
			foundVault = b

			v := &api.Vault{}
			diags = append(diags, hclDecoder.DecodeBody(b.Body, ctx, v)...)
			c.Vault = v

		} else if b.Type == taskLabel {
			t := &api.Task{}
			diags = append(diags, hclDecoder.DecodeBody(b.Body, ctx, t)...)
			if len(b.Labels) == 1 {
				t.Name = b.Labels[0]
				c.Tasks = append(c.Tasks, t)
			}
		}
	}

	return diags
}

func (c *jobConfig) evaluateLocalVariables(locals []*LocalBlock) hcl.Diagnostics {
	var diags hcl.Diagnostics

	if len(locals) > 0 && c.LocalVariables == nil {
		c.LocalVariables = Variables{}
	}

	var retry, previousL int
	for len(locals) > 0 {
		local := locals[0]
		moreDiags := c.evaluateLocalVariable(local)
		if moreDiags.HasErrors() {
			if len(locals) == 1 {
				// If this is the only local left there's no need
				// to try evaluating again
				return append(diags, moreDiags...)
			}
			if previousL == len(locals) {
				if retry == 100 {
					// To get to this point, locals must have a circle dependency
					return append(diags, moreDiags...)
				}
				retry++
			}
			previousL = len(locals)

			// If local uses another local that has not been evaluated yet this could be the reason of errors
			// Push local to the end of slice to be evaluated later
			locals = append(locals, local)
		} else {
			retry = 0
			diags = append(diags, moreDiags...)
		}
		// Remove local from slice
		locals = append(locals[:0], locals[1:]...)
	}

	return diags
}

func (c *jobConfig) evaluateLocalVariable(local *LocalBlock) hcl.Diagnostics {
	var diags hcl.Diagnostics

	value, moreDiags := local.Expr.Value(c.EvalContext())
	diags = append(diags, moreDiags...)
	if moreDiags.HasErrors() {
		return diags
	}
	c.LocalVariables[local.Name] = &Variable{
		Name: local.Name,
		Values: []VariableAssignment{{
			Value: value,
			Expr:  local.Expr,
			From:  "default",
		}},
		Type: value.Type(),
	}

	return diags
}

func (c *jobConfig) decodeJob(content *hcl.BodyContent, ctx *hcl.EvalContext) hcl.Diagnostics {
	var diags hcl.Diagnostics

	c.Job = &api.Job{}

	var found *hcl.Block
	for _, b := range content.Blocks {
		if b.Type != "job" {
			continue
		}

		body := hclutil.BlocksAsAttrs(b.Body)
		body = dynblock.Expand(body, ctx)

		if found != nil {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("Duplicate %s block", b.Type),
				Detail: fmt.Sprintf(
					"Only one block of type %q is allowed. Previous definition was at %s.",
					b.Type, found.DefRange.String(),
				),
				Subject: &b.DefRange,
			})
			continue
		}
		found = b

		c.JobID = b.Labels[0]

		metaAttr, body, mdiags := decodeAsAttribute(body, ctx, "meta")
		diags = append(diags, mdiags...)

		extra, remain, mdiags := body.PartialContent(&hcl.BodySchema{
			Blocks: []hcl.BlockHeaderSchema{
				{Type: "vault"},
				{Type: "task", LabelNames: []string{"name"}},
			},
		})

		diags = append(diags, mdiags...)
		diags = append(diags, c.decodeTopLevelExtras(extra, ctx)...)
		diags = append(diags, hclDecoder.DecodeBody(remain, ctx, c.Job)...)

		if metaAttr != nil {
			c.Job.Meta = metaAttr
		}
	}

	if found == nil {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Missing job block",
			Detail:   "A job block is required",
		})
	}

	return diags

}

func (c *jobConfig) EvalContext() *hcl.EvalContext {
	vars, _ := c.InputVariables.Values()
	locals, _ := c.LocalVariables.Values()
	return &hcl.EvalContext{
		Functions: Functions(c.ParseConfig.BaseDir, c.ParseConfig.AllowFS),
		Variables: map[string]cty.Value{
			inputVariablesAccessor: cty.ObjectVal(vars),
			localsAccessor:         cty.ObjectVal(locals),
		},
		UnknownVariable: func(expr string) (cty.Value, error) {
			v := "${" + expr + "}"
			return cty.StringVal(v), nil
		},
	}
}

func toVars(vars []string) map[string]string {
	attrs := make(map[string]string, len(vars))
	for _, arg := range vars {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) == 2 {
			attrs[parts[0]] = parts[1]
		}
	}

	return attrs
}
