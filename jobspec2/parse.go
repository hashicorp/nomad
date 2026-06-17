// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: MPL-2.0

package jobspec2

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	hcljson "github.com/hashicorp/hcl/v2/json"
	"github.com/hashicorp/nomad/api"
	"github.com/zclconf/go-cty/cty"
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

// ParseResult contains the result of parsing a job specification,
// including the job itself and metadata about declared variables.
type ParseResult struct {
	Job        *api.Job
	Submission *api.JobSubmission
	Variables  Variables
}

// ParseWithConfig returns the parsed job and any diagnostic errors. This will
// get used for the Job Parse API or similar consumers.
func ParseWithConfig(args *ParseConfig) (*api.Job, error) {
	c, err := parseWithConfigImpl(args)
	if err != nil {
		return nil, err
	}
	return c.Job, nil
}

// ParseWithConfigEx is an extended version of ParseWithConfig that returns
// additional HCL variable type information. This allows callers like the `job
// run` CLI or the Terraform provider to distinguish between simple and complex
// variable types when constructing JobSubmission objects.
func ParseWithConfigEx(args *ParseConfig) (*ParseResult, error) {
	c, err := parseWithConfigImpl(args)
	if err != nil {
		return nil, err
	}
	sub, err := submissionFromJob(args, c)
	if err != nil {
		return nil, err
	}

	return &ParseResult{
		Job:        c.Job,
		Variables:  c.InputVariables,
		Submission: sub,
	}, nil
}

func parseWithConfigImpl(args *ParseConfig) (*jobConfig, error) {
	args.normalize()

	c := newJobConfig(args)
	err := decode(c)
	if err != nil {
		return nil, err
	}

	normalizeJob(c)
	return c, nil
}

type ParseConfig struct {
	Path    string
	BaseDir string

	// Body is the HCL body
	Body []byte

	// AllowFS enables HCL functions that require file system access
	AllowFS bool

	// ArgVars is the CLI -var arguments
	ArgVars []string

	// VarFiles is the paths of variable data files that should be read during
	// parsing.
	VarFiles []string

	// VarContent is the content of variable data known without reading an
	// actual var file during parsing.
	VarContent string

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
	config := c.ParseConfig

	file, diags := parseHCLOrJSON(config.Body, config.Path)

	for _, varFile := range config.VarFiles {
		parsedVarFile, ds := parseFile(varFile)
		if parsedVarFile == nil || ds.HasErrors() {
			return fmt.Errorf("unable to parse var file: %v", ds.Error())
		}

		config.parsedVarFiles = append(config.parsedVarFiles, parsedVarFile)
		diags = append(diags, ds...)
	}

	if config.VarContent != "" {
		hclFile, hclDiagnostics := parseHCLOrJSON([]byte(config.VarContent), "input.hcl")
		if hclDiagnostics.HasErrors() {
			return fmt.Errorf("unable to parse var content: %v", hclDiagnostics.Error())
		}
		config.parsedVarFiles = append(config.parsedVarFiles, hclFile)
	}

	// Return early if the input job or variable files are not valid.
	// Decoding and evaluating invalid files may result in unexpected results.
	if diags.HasErrors() {
		return diags
	}

	diags = append(diags, c.decodeBody(file.Body)...)

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

	diags = append(diags, decodeMapInterfaceType(&c.Job, c.EvalContext())...)
	diags = append(diags, decodeMapInterfaceType(&c.Tasks, c.EvalContext())...)
	diags = append(diags, decodeMapInterfaceType(&c.Vault, c.EvalContext())...)
	diags = append(diags, decodeMapInterfaceType(&c.Secrets, c.EvalContext())...)

	if diags.HasErrors() {
		return diags
	}

	return nil
}

func parseFile(path string) (*hcl.File, hcl.Diagnostics) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, hcl.Diagnostics{
			&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Failed to read file",
				Detail:   fmt.Sprintf("failed to read %q: %v", path, err),
			},
		}
	}

	return parseHCLOrJSON(body, path)
}

func parseHCLOrJSON(src []byte, filename string) (*hcl.File, hcl.Diagnostics) {
	if isJSON(src) {
		return hcljson.Parse(src, filename)
	}

	return hclsyntax.ParseConfig(src, filename, hcl.Pos{Line: 1, Column: 1})
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

const (
	formatJSON = "json"
	formatHCL2 = "hcl2"
)

func submissionFromJob(args *ParseConfig, j *jobConfig) (*api.JobSubmission, error) {
	format := formatHCL2
	if isJSON(args.Body) {
		format = formatJSON
	}

	// combine any -var-file data into one big blob
	varFileCat, readVarFileErr := extractVarFiles(args.VarFiles)
	if readVarFileErr != nil {
		return nil, fmt.Errorf("Failed to read var file(s): %w", readVarFileErr)
	}

	if varFileCat != "" && args.VarContent != "" {
		varFileCat = strings.TrimRight(varFileCat, "\n") + "\n\n" + args.VarContent
	} else if varFileCat == "" {
		varFileCat = args.VarContent
	}

	// Extract variables declared by the -var flag and as environment
	// variables. Merge the two maps ensuring that variables defined by -var
	// flags take precedence over the environment
	extractedVarFlags := extractVarFlags(args.ArgVars)
	extractedEnvVars := extractJobSpecEnvVars(args.Envs)
	maps.Copy(extractedEnvVars, extractedVarFlags)

	// Separate simple and complex variables from -var based on types from
	// schema. Simple types (string, number, bool) go to VariableFlags as
	// strings. Complex types (lists, maps, objects) go to Variables in HCL
	// format.
	simpleVars, complexVarsHCL := separateVariables(extractedEnvVars, j.InputVariables)

	// Combine var-file with complex variables from flags

	if varFileCat != "" && complexVarsHCL != "" {
		varFileCat = strings.TrimRight(varFileCat, "\n") + "\n\n" + complexVarsHCL
	} else if varFileCat == "" {
		varFileCat = complexVarsHCL
	}

	sub := &api.JobSubmission{
		Source:        string(args.Body),
		Format:        format,
		VariableFlags: simpleVars,
		Variables:     varFileCat,
	}
	return sub, nil
}

// extractVarFlags is used to parse the values of -var command line arguments
// and turn them into a map to be used for submission. The result is never
// nil for convenience.
func extractVarFlags(slice []string) map[string]string {
	m := make(map[string]string, len(slice))
	for _, s := range slice {
		if tokens := strings.SplitN(s, "=", 2); len(tokens) == 1 {
			m[tokens[0]] = ""
		} else {
			m[tokens[0]] = tokens[1]
		}
	}
	return m
}

// extractJobSpecEnvVars is used to extract Nomad specific HCL variables from
// the OS environment. The input envVars parameter is expected to be generated
// from the os.Environment function call. The result is never nil for
// convenience.
func extractJobSpecEnvVars(envVars []string) map[string]string {

	m := make(map[string]string)

	for _, raw := range envVars {
		if !strings.HasPrefix(raw, VarEnvPrefix) {
			continue
		}

		// Trim the prefix, so we just have the raw key=value variable
		// remaining.
		raw = raw[len(VarEnvPrefix):]

		// Identify the index of the equals sign which is where we split the
		// variable k/v pair. -1 indicates the equals sign is not found and
		// therefore the var is not valid.
		if eq := strings.Index(raw, "="); eq == -1 {
			continue
		} else if raw[:eq] != "" {
			m[raw[:eq]] = raw[eq+1:]
		}
	}

	return m
}

// extractVarFiles concatenates the content of each file in filenames and
// returns it all as one big content blob
func extractVarFiles(filenames []string) (string, error) {
	var sb strings.Builder
	for _, filename := range filenames {
		b, err := os.ReadFile(filename)
		if err != nil {
			return "", err
		}
		sb.WriteString(string(b))
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

// separateVariables splits variables into simple (string-safe) and complex
// (HCL-required) based on their schema.
func separateVariables(varFlags map[string]string, declaredVars Variables) (simple map[string]string, complex string) {
	simple = make(map[string]string)
	var complexVars []string

	for name, value := range varFlags {
		variable, declared := declaredVars[name]
		if !declared {
			simple[name] = value
			continue
		}
		// Simple types can be safely represented as strings in
		// VariableFlags. Complex types (lists, maps, objects) require HCL
		// format in the Variables field to preserve their structure and type
		// information.
		switch variable.Type {
		case cty.NilType, cty.String, cty.Number, cty.Bool:
			simple[name] = value
		default:
			// unquoted HCL format for appending to Variables field
			complexVars = append(complexVars, fmt.Sprintf("%s = %s", name, value))
		}
	}

	if len(complexVars) > 0 {
		complex = strings.Join(complexVars, "\n")
	}

	return simple, complex
}
