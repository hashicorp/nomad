package jobspec2

// This file is copied verbatim from Packer: https://github.com/hashicorp/packer/blob/7a1680df97e028c4a75622effe08f6610d0ee5b4/hcl2template/types.variables.go
// with few changes. Packer references in comments are preserved to reduce the diff between files.

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/ext/typeexpr"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/nomad/jobspec2/addrs"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"
)

// A consistent detail message for all "not a valid identifier" diagnostics.
const badIdentifierDetail = "A name must start with a letter or underscore and may contain only letters, digits, underscores, and dashes."

// Local represents a single entry from a "locals" block in a file.
// The "locals" block itself is not represented, because it serves only to
// provide context for us to interpret its contents.
type LocalBlock struct {
	Name string
	Expr hcl.Expression
}

// VariableAssignment represents a way a variable was set: the expression
// setting it and the value of that expression. It helps pinpoint were
// something was set in diagnostics.
type VariableAssignment struct {
	// From tells were it was taken from, command/varfile/env/default
	From  string
	Value cty.Value
	Expr  hcl.Expression
}

type Variable struct {
	// Values contains possible values for the variable; The last value set
	// from these will be the one used. If none is set; an error will be
	// returned by Value().
	Values []VariableAssignment

	// Validations contains all variables validation rules to be applied to the
	// used value. Only the used value - the last value from Values - is
	// validated.
	Validations []*VariableValidation

	// Cty Type of the variable. If the default value or a collected value is
	// not of this type nor can be converted to this type an error diagnostic
	// will show up. This allows us to assume that values are valid later in
	// code.
	//
	// When a default value - and no type - is passed in the variable
	// declaration, the type of the default variable will be used. This will
	// allow to ensure that users set this variable correctly.
	Type cty.Type
	// Common name of the variable
	Name string
	// Description of the variable
	Description string

	Range hcl.Range
}

func (v *Variable) GoString() string {
	b := &strings.Builder{}
	fmt.Fprintf(b, "{type:%s", v.Type.GoString())
	for _, vv := range v.Values {
		fmt.Fprintf(b, ",%s:%s", vv.From, vv.Value)
	}
	fmt.Fprintf(b, "}")
	return b.String()
}

// validateValue ensures that all of the configured custom validations for a
// variable value are passing.
func (v *Variable) validateValue(val VariableAssignment) (diags hcl.Diagnostics) {
	if len(v.Validations) == 0 {
		return nil
	}

	hclCtx := &hcl.EvalContext{
		Variables: map[string]cty.Value{
			"var": cty.ObjectVal(map[string]cty.Value{
				v.Name: val.Value,
			}),
		},
		Functions: Functions("", false),
	}

	for _, validation := range v.Validations {
		const errInvalidCondition = "Invalid variable validation result"

		if validation.Condition == nil {
			diags = append(diags, &hcl.Diagnostic{
				Severity:    hcl.DiagError,
				Summary:     "Invalid variable validation specification",
				Detail:      "validation requires a condition.",
				Subject:     validation.DeclRange.Ptr(),
				EvalContext: hclCtx,
			})
			continue
		}

		result, moreDiags := validation.Condition.Value(hclCtx)
		diags = append(diags, moreDiags...)
		if !result.IsKnown() {
			continue // We'll wait until we've learned more, then.
		}
		if result.IsNull() {
			diags = append(diags, &hcl.Diagnostic{
				Severity:    hcl.DiagError,
				Summary:     errInvalidCondition,
				Detail:      "Validation condition expression must return either true or false, not null.",
				Subject:     validation.Condition.Range().Ptr(),
				Expression:  validation.Condition,
				EvalContext: hclCtx,
			})
			continue
		}
		var err error
		result, err = convert.Convert(result, cty.Bool)
		if err != nil {
			diags = append(diags, &hcl.Diagnostic{
				Severity:    hcl.DiagError,
				Summary:     errInvalidCondition,
				Detail:      fmt.Sprintf("Invalid validation condition result value: %s.", err),
				Subject:     validation.Condition.Range().Ptr(),
				Expression:  validation.Condition,
				EvalContext: hclCtx,
			})
			continue
		}

		if result.False() {
			subj := validation.DeclRange.Ptr()
			if val.Expr != nil {
				subj = val.Expr.Range().Ptr()
			}
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("Invalid value for %s variable", val.From),
				Detail:   fmt.Sprintf("%s\n\nThis was checked by the validation rule at %s.", validation.ErrorMessage, validation.DeclRange.String()),
				Subject:  subj,
			})
		}
	}

	return diags
}

// Value returns the last found value from the list of variable settings.
func (v *Variable) Value() (cty.Value, hcl.Diagnostics) {
	if len(v.Values) == 0 {
		return cty.UnknownVal(v.Type), hcl.Diagnostics{&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  fmt.Sprintf("Unset variable %q", v.Name),
			Detail: "A used variable must be set or have a default value; see " +
				"https://www.nomadproject.io/docs/job-specification/hcl2/variables for " +
				"details.",
			Context: v.Range.Ptr(),
		}}
	}
	val := v.Values[len(v.Values)-1]
	return val.Value, v.validateValue(v.Values[len(v.Values)-1])
}

type Variables map[string]*Variable

func (variables Variables) Keys() []string {
	keys := make([]string, 0, len(variables))
	for key := range variables {
		keys = append(keys, key)
	}
	return keys
}

func (variables Variables) Values() (map[string]cty.Value, hcl.Diagnostics) {
	res := map[string]cty.Value{}
	var diags hcl.Diagnostics
	for k, v := range variables {
		value, moreDiags := v.Value()
		diags = append(diags, moreDiags...)
		res[k] = value
	}
	return res, diags
}

// decodeVariable decodes a variable key and value into Variables
func (variables *Variables) decodeVariable(key string, attr *hcl.Attribute, ectx *hcl.EvalContext) hcl.Diagnostics {
	var diags hcl.Diagnostics

	if (*variables) == nil {
		(*variables) = Variables{}
	}

	if _, found := (*variables)[key]; found {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Duplicate variable",
			Detail:   "Duplicate " + key + " variable definition found.",
			Subject:  attr.NameRange.Ptr(),
		})
		return diags
	}

	value, moreDiags := attr.Expr.Value(ectx)
	diags = append(diags, moreDiags...)
	if moreDiags.HasErrors() {
		return diags
	}

	(*variables)[key] = &Variable{
		Name: key,
		Values: []VariableAssignment{{
			From:  "default",
			Value: value,
			Expr:  attr.Expr,
		}},
		Type:  value.Type(),
		Range: attr.Range,
	}

	return diags
}

var variableBlockSchema = &hcl.BodySchema{
	Attributes: []hcl.AttributeSchema{
		{
			Name: "description",
		},
		{
			Name: "default",
		},
		{
			Name: "type",
		},
	},
	Blocks: []hcl.BlockHeaderSchema{
		{
			Type: "validation",
		},
	},
}

// decodeVariableBlock decodes a "variables" section the way packer 1 used to
func (variables *Variables) decodeVariableBlock(block *hcl.Block, ectx *hcl.EvalContext) hcl.Diagnostics {
	if (*variables) == nil {
		(*variables) = Variables{}
	}

	if _, found := (*variables)[block.Labels[0]]; found {

		return []*hcl.Diagnostic{{
			Severity: hcl.DiagError,
			Summary:  "Duplicate variable",
			Detail:   "Duplicate " + block.Labels[0] + " variable definition found.",
			Context:  block.DefRange.Ptr(),
		}}
	}

	name := block.Labels[0]

	content, diags := block.Body.Content(variableBlockSchema)
	if !hclsyntax.ValidIdentifier(name) {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid variable name",
			Detail:   badIdentifierDetail,
			Subject:  &block.LabelRanges[0],
		})
	}

	v := &Variable{
		Name:  name,
		Range: block.DefRange,
	}

	if attr, exists := content.Attributes["description"]; exists {
		valDiags := gohcl.DecodeExpression(attr.Expr, nil, &v.Description)
		diags = append(diags, valDiags...)
	}

	if t, ok := content.Attributes["type"]; ok {
		tp, moreDiags := typeexpr.Type(t.Expr)
		diags = append(diags, moreDiags...)
		if moreDiags.HasErrors() {
			return diags
		}

		v.Type = tp
	}

	if def, ok := content.Attributes["default"]; ok {
		defaultValue, moreDiags := def.Expr.Value(ectx)
		diags = append(diags, moreDiags...)
		if moreDiags.HasErrors() {
			return diags
		}

		if v.Type != cty.NilType {
			var err error
			defaultValue, err = convert.Convert(defaultValue, v.Type)
			if err != nil {
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "Invalid default value for variable",
					Detail:   fmt.Sprintf("This default value is not compatible with the variable's type constraint: %s.", err),
					Subject:  def.Expr.Range().Ptr(),
				})
				defaultValue = cty.DynamicVal
			}
		}

		v.Values = append(v.Values, VariableAssignment{
			From:  "default",
			Value: defaultValue,
			Expr:  def.Expr,
		})

		// It's possible no type attribute was assigned so lets make sure we
		// have a valid type otherwise there could be issues parsing the value.
		if v.Type == cty.NilType {
			v.Type = defaultValue.Type()
		}
	}

	for _, block := range content.Blocks {
		switch block.Type {
		case "validation":
			vv, moreDiags := decodeVariableValidationBlock(v.Name, block)
			diags = append(diags, moreDiags...)
			v.Validations = append(v.Validations, vv)
		}
	}

	(*variables)[name] = v

	return diags
}

var variableValidationBlockSchema = &hcl.BodySchema{
	Attributes: []hcl.AttributeSchema{
		{
			Name:     "condition",
			Required: true,
		},
		{
			Name:     "error_message",
			Required: true,
		},
	},
}

// VariableValidation represents a configuration-defined validation rule
// for a particular input variable, given as a "validation" block inside
// a "variable" block.
type VariableValidation struct {
	// Condition is an expression that refers to the variable being tested and
	// contains no other references. The expression must return true to
	// indicate that the value is valid or false to indicate that it is
	// invalid. If the expression produces an error, that's considered a bug in
	// the block defining the validation rule, not an error in the caller.
	Condition hcl.Expression

	// ErrorMessage is one or more full sentences, which _should_ be in English
	// for consistency with the rest of the error message output but can in
	// practice be in any language as long as it ends with a period. The
	// message should describe what is required for the condition to return
	// true in a way that would make sense to a caller of the module.
	ErrorMessage string

	DeclRange hcl.Range
}

func decodeVariableValidationBlock(varName string, block *hcl.Block) (*VariableValidation, hcl.Diagnostics) {
	var diags hcl.Diagnostics
	vv := &VariableValidation{
		DeclRange: block.DefRange,
	}

	content, moreDiags := block.Body.Content(variableValidationBlockSchema)
	diags = append(diags, moreDiags...)

	if attr, exists := content.Attributes["condition"]; exists {
		vv.Condition = attr.Expr

		// The validation condition must refer to the variable itself and
		// nothing else; to ensure that the variable declaration can't create
		// additional edges in the dependency graph.
		goodRefs := 0
		for _, traversal := range vv.Condition.Variables() {

			ref, moreDiags := addrs.ParseRef(traversal)
			if !moreDiags.HasErrors() {
				if addr, ok := ref.Subject.(addrs.InputVariable); ok {
					if addr.Name == varName {
						goodRefs++
						continue // Reference is valid
					}
				}
			}

			// If we fall out here then the reference is invalid.
			diags = diags.Append(&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Invalid reference in variable validation",
				Detail:   fmt.Sprintf("The condition for variable %q can only refer to the variable itself, using var.%s.", varName, varName),
				Subject:  traversal.SourceRange().Ptr(),
			})
		}
		if goodRefs < 1 {
			diags = diags.Append(&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Invalid variable validation condition",
				Detail:   fmt.Sprintf("The condition for variable %q must refer to var.%s in order to test incoming values.", varName, varName),
				Subject:  attr.Expr.Range().Ptr(),
			})
		}
	}

	if attr, exists := content.Attributes["error_message"]; exists {
		moreDiags := gohcl.DecodeExpression(attr.Expr, nil, &vv.ErrorMessage)
		diags = append(diags, moreDiags...)
		if !moreDiags.HasErrors() {
			const errSummary = "Invalid validation error message"
			switch {
			case vv.ErrorMessage == "":
				diags = diags.Append(&hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  errSummary,
					Detail:   "An empty string is not a valid nor useful error message.",
					Subject:  attr.Expr.Range().Ptr(),
				})
			case !looksLikeSentences(vv.ErrorMessage):
				// Because we're going to include this string verbatim as part
				// of a bigger error message written in our usual style, we'll
				// require the given error message to conform to that. We might
				// relax this in future if e.g. we start presenting these error
				// messages in a different way, or if Packer starts supporting
				// producing error messages in other human languages, etc. For
				// pragmatism we also allow sentences ending with exclamation
				// points, but we don't mention it explicitly here because
				// that's not really consistent with the Packer UI writing
				// style.
				diags = diags.Append(&hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  errSummary,
					Detail:   "Validation error message must be at least one full sentence starting with an uppercase letter ( if the alphabet permits it ) and ending with a period or question mark.",
					Subject:  attr.Expr.Range().Ptr(),
				})
			}
		}
	}

	return vv, diags
}

// looksLikeSentence is a simple heuristic that encourages writing error
// messages that will be presentable when included as part of a larger error
// diagnostic whose other text is written in the UI writing style.
//
// This is intentionally not a very strong validation since we're assuming that
// authors want to write good messages and might just need a nudge about
// Packer's specific style, rather than that they are going to try to work
// around these rules to write a lower-quality message.
func looksLikeSentences(s string) bool {
	if len(s) < 1 {
		return false
	}
	runes := []rune(s) // HCL guarantees that all strings are valid UTF-8
	first := runes[0]
	last := runes[len(runes)-1]

	// If the first rune is a letter then it must be an uppercase letter. To
	// sorts of nudge people into writing sentences. For alphabets that don't
	// have the notion of 'upper', this does nothing.
	if unicode.IsLetter(first) && !unicode.IsUpper(first) {
		return false
	}

	// The string must be at least one full sentence, which implies having
	// sentence-ending punctuation.
	return last == '.' || last == '?' || last == '!'
}

// Prefix your environment variables with VarEnvPrefix so that Packer can see
// them.
const VarEnvPrefix = "NOMAD_VAR_"

func (c *jobConfig) collectInputVariableValues(env []string, files []*hcl.File, argv map[string]string) hcl.Diagnostics {
	var diags hcl.Diagnostics
	variables := c.InputVariables

	for _, raw := range env {
		if !strings.HasPrefix(raw, VarEnvPrefix) {
			continue
		}
		raw = raw[len(VarEnvPrefix):] // trim the prefix

		eq := strings.Index(raw, "=")
		if eq == -1 {
			// Seems invalid, so we'll ignore it.
			continue
		}

		name := raw[:eq]
		value := raw[eq+1:]

		variable, found := variables[name]
		if !found {
			// this variable was not defined in the hcl files, let's skip it !
			continue
		}

		fakeFilename := fmt.Sprintf("<value for var.%s from env>", name)
		expr, moreDiags := expressionFromVariableDefinition(fakeFilename, value, variable.Type)
		diags = append(diags, moreDiags...)
		if moreDiags.HasErrors() {
			continue
		}

		val, valDiags := expr.Value(nil)
		diags = append(diags, valDiags...)
		if variable.Type != cty.NilType {
			var err error
			val, err = convert.Convert(val, variable.Type)
			if err != nil {
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "Invalid value for variable",
					Detail:   fmt.Sprintf("The value for %s is not compatible with the variable's type constraint: %s.", name, err),
					Subject:  expr.Range().Ptr(),
				})
				val = cty.DynamicVal
			}
		}
		variable.Values = append(variable.Values, VariableAssignment{
			From:  "env",
			Value: val,
			Expr:  expr,
		})
	}

	// Define the severity of variable passed that are undefined.
	undefSev := hcl.DiagWarning
	if c.ParseConfig.Strict {
		undefSev = hcl.DiagError
	}

	// files will contain files found in the folder then files passed as
	// arguments.
	for _, file := range files {
		// Before we do our real decode, we'll probe to see if there are any
		// blocks of type "variable" in this body, since it's a common mistake
		// for new users to put variable declarations in pkrvars rather than
		// variable value definitions, and otherwise our error message for that
		// case is not so helpful.
		{
			content, _, _ := file.Body.PartialContent(&hcl.BodySchema{
				Blocks: []hcl.BlockHeaderSchema{
					{
						Type:       "variable",
						LabelNames: []string{"name"},
					},
				},
			})
			for _, block := range content.Blocks {
				name := block.Labels[0]
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "Variable declaration in a .var file",
					Detail: fmt.Sprintf("A .var file is used to assign "+
						"values to variables that have already been declared "+
						"in job files, not to declare new variables. To "+
						"declare variable %q, place this block in one of your"+
						" job files\n\nTo set a "+
						"value for this variable in %s, use the definition "+
						"syntax instead:\n    %s = <value>",
						name, block.TypeRange.Filename, name),
					Subject: &block.TypeRange,
				})
			}
			if diags.HasErrors() {
				// If we already found problems then JustAttributes below will find
				// the same problems with less-helpful messages, so we'll bail for
				// now to let the user focus on the immediate problem.
				return diags
			}
		}

		attrs, moreDiags := file.Body.JustAttributes()
		diags = append(diags, moreDiags...)

		for name, attr := range attrs {
			variable, found := variables[name]
			if !found {
				diags = append(diags, &hcl.Diagnostic{
					Severity: undefSev,
					Summary:  "Undefined variable",
					Detail: fmt.Sprintf("A %q variable was set but was "+
						"not found in known variables. To declare "+
						"variable %q, place this block in your "+
						"job files",
						name, name),
					Context: attr.Range.Ptr(),
				})
				continue
			}

			val, moreDiags := attr.Expr.Value(nil)
			diags = append(diags, moreDiags...)

			if variable.Type != cty.NilType {
				var err error
				val, err = convert.Convert(val, variable.Type)
				if err != nil {
					diags = append(diags, &hcl.Diagnostic{
						Severity: hcl.DiagError,
						Summary:  "Invalid value for variable",
						Detail:   fmt.Sprintf("The value for %s is not compatible with the variable's type constraint: %s.", name, err),
						Subject:  attr.Expr.Range().Ptr(),
					})
					val = cty.DynamicVal
				}
			}

			variable.Values = append(variable.Values, VariableAssignment{
				From:  "varfile",
				Value: val,
				Expr:  attr.Expr,
			})
		}
	}

	// Finally we process values given explicitly on the command line.
	for name, value := range argv {
		variable, found := variables[name]
		if !found {
			diags = append(diags, &hcl.Diagnostic{
				Severity: undefSev,
				Summary:  "Undefined -var variable",
				Detail: fmt.Sprintf("A %q variable was passed in the command "+
					"line but was not found in known variables. "+
					"To declare variable %q, place this block in your"+
					" job file",
					name, name),
			})
			continue
		}

		fakeFilename := fmt.Sprintf("<value for var.%s from arguments>", name)
		expr, moreDiags := expressionFromVariableDefinition(fakeFilename, value, variable.Type)
		diags = append(diags, moreDiags...)
		if moreDiags.HasErrors() {
			continue
		}

		val, valDiags := expr.Value(nil)
		diags = append(diags, valDiags...)

		if variable.Type != cty.NilType {
			var err error
			val, err = convert.Convert(val, variable.Type)
			if err != nil {
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "Invalid argument value for -var variable",
					Detail:   fmt.Sprintf("The received arg value for %s is not compatible with the variable's type constraint: %s.", name, err),
					Subject:  expr.Range().Ptr(),
				})
				val = cty.DynamicVal
			}
		}

		variable.Values = append(variable.Values, VariableAssignment{
			From:  "cmd",
			Value: val,
			Expr:  expr,
		})
	}

	return diags
}

// expressionFromVariableDefinition creates an hclsyntax.Expression that is capable of evaluating the specified value for a given cty.Type.
// The specified filename is to identify the source of where value originated from in the diagnostics report, if there is an error.
func expressionFromVariableDefinition(filename string, value string, variableType cty.Type) (hclsyntax.Expression, hcl.Diagnostics) {
	switch variableType {
	case cty.String, cty.Number, cty.NilType:
		// when the type is nil (not set in a variable block) we default to
		// interpreting everything as a string literal.
		return &hclsyntax.LiteralValueExpr{Val: cty.StringVal(value)}, nil
	default:
		return hclsyntax.ParseExpression([]byte(value), filename, hcl.Pos{Line: 1, Column: 1})
	}
}
