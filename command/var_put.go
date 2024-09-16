// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-set/v3"
	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/ast"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper"
	"github.com/mitchellh/cli"
	"github.com/mitchellh/mapstructure"
	"github.com/posener/complete"
)

// Detect characters that are not valid identifiers to emit a warning when they
// are used in as a variable key.
var invalidIdentifier = regexp.MustCompile(`[^_\pN\pL]`)

type VarPutCommand struct {
	Meta

	contents  []byte
	inFmt     string
	outFmt    string
	tmpl      string
	testStdin io.Reader // for tests
	verbose   func(string)
}

func (c *VarPutCommand) Help() string {
	helpText := `
Usage:
nomad var put [options] <variable spec file reference> [<key>=<value>]...
nomad var put [options] <path to store variable> [<variable spec file reference>] [<key>=<value>]...

  The 'var put' command is used to create or update an existing variable.
  Variable metadata and items can be supplied using a variable specification,
  by using command arguments, or by a combination of the two techniques.

  An entire variable specification can be provided to the command via standard
  input (stdin) by setting the first argument to "-" or from a file by using an
  @-prefixed path to a variable specification file. When providing variable
  data via stdin, you must provide the "-in" flag with the format of the
  specification, either "hcl" or "json"

  Items to be stored in the variable can be supplied using the specification,
  as a series of key-value pairs, or both. The value for a key-value pair can
  be a string, an @-prefixed file reference, or a '-' to get the value from
  stdin. Item values provided from file references or stdin are consumed as-is
  with no additional processing and do not require the input format to be
  specified.

  Values supplied as command line arguments supersede values provided in
  any variable specification piped into the command or loaded from file.

  If ACLs are enabled, this command requires the 'variables:write' capability
  for the destination namespace and path.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Var put Options:

  -check-index
     If set, the variable is only acted upon if the server-side version's index
     matches the provided value. When a variable specification contains
     a modify index, that modify index is used as the check-index for the
     check-and-set operation and can be overridden using this flag.

  -force
     Perform this operation regardless of the state or index of the variable
     on the server-side.

  -in (hcl | json)
     Parser to use for data supplied via standard input or when the variable
     specification's type can not be known using the file extension. Defaults
     to "json".

  -out (go-template | hcl | json | none | table)
     Format to render created or updated variable. Defaults to "none" when
     stdout is a terminal and "json" when the output is redirected.

  -template
     Template to render output with. Required when format is "go-template",
     invalid for other formats.

  -verbose
     Provides additional information via standard error to preserve standard
     output (stdout) for redirected output.

`
	return strings.TrimSpace(helpText)
}

func (c *VarPutCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-in":  complete.PredictSet("hcl", "json"),
			"-out": complete.PredictSet("none", "hcl", "json", "go-template", "table"),
		},
	)
}

func (c *VarPutCommand) AutocompleteArgs() complete.Predictor {
	return VariablePathPredictor(c.Meta.Client)
}

func (c *VarPutCommand) Synopsis() string {
	return "Create or update a variable"
}

func (c *VarPutCommand) Name() string { return "var put" }

func (c *VarPutCommand) Run(args []string) int {
	var force, enforce, doVerbose bool
	var path, checkIndexStr string
	var checkIndex uint64
	var err error

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	flags.BoolVar(&force, "force", false, "")
	flags.BoolVar(&doVerbose, "verbose", false, "")
	flags.StringVar(&checkIndexStr, "check-index", "", "")
	flags.StringVar(&c.inFmt, "in", "json", "")
	flags.StringVar(&c.tmpl, "template", "", "")

	if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) != 0 {
		flags.StringVar(&c.outFmt, "out", "none", "")
	} else {
		flags.StringVar(&c.outFmt, "out", "json", "")
	}

	if err := flags.Parse(args); err != nil {
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	args = flags.Args()

	// Manage verbose output
	verbose := func(_ string) {} //no-op
	if doVerbose {
		verbose = func(msg string) {
			c.Ui.Warn(msg)
		}
	}
	c.verbose = verbose

	// Parse the check-index
	checkIndex, enforce, err = parseCheckIndex(checkIndexStr)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error parsing check-index value %q: %v", checkIndexStr, err))
		return 1
	}

	if c.Meta.namespace == "*" {
		c.Ui.Error(errWildcardNamespaceNotAllowed)
		return 1
	}

	// Pull our fake stdin if needed
	stdin := (io.Reader)(os.Stdin)
	if c.testStdin != nil {
		stdin = c.testStdin
	}

	switch {
	case len(args) < 1:
		c.Ui.Error(fmt.Sprintf("Not enough arguments (expected >1, got %d)", len(args)))
		c.Ui.Error(commandErrorText(c))
		return 1
	case len(args) == 1 && !isArgStdinRef(args[0]) && !isArgFileRef(args[0]):
		c.Ui.Error("Must supply data")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	if err = c.validateInputFlag(); err != nil {
		c.Ui.Error(err.Error())
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	if err := c.validateOutputFlag(); err != nil {
		c.Ui.Error(err.Error())
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	arg := args[0]
	switch {
	// Handle first argument: can be -, @file, «var path»
	case isArgStdinRef(arg):

		// read the specification into memory from stdin
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			c.contents, err = io.ReadAll(os.Stdin)
			if err != nil {
				c.Ui.Error(fmt.Sprintf("Error reading from stdin: %s", err))
				return 1
			}
		}
		verbose(fmt.Sprintf("Reading whole %s variable specification from stdin", strings.ToUpper(c.inFmt)))

	case isArgFileRef(arg):
		// ArgFileRefs start with "@" so we need to peel that off
		// detect format based on file extension
		specPath := arg[1:]
		err = c.setParserForFileArg(specPath)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}
		verbose(fmt.Sprintf("Reading whole %s variable specification from %q", strings.ToUpper(c.inFmt), specPath))
		c.contents, err = os.ReadFile(specPath)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error reading %q: %s", specPath, err))
			return 1
		}
	default:
		path = sanitizePath(arg)
		verbose(fmt.Sprintf("Writing to path %q", path))
	}

	args = args[1:]
	switch {
	// Handle second argument: can be -, @file, or kv
	case len(args) == 0:
		// no-op
	case isArgStdinRef(args[0]):
		verbose(fmt.Sprintf("Creating variable %q using specification from stdin", path))
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			c.contents, err = io.ReadAll(os.Stdin)
			if err != nil {
				c.Ui.Error(fmt.Sprintf("Error reading from stdin: %s", err))
				return 1
			}
		}
		args = args[1:]

	case isArgFileRef(args[0]):
		arg := args[0]
		err = c.setParserForFileArg(arg)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}
		verbose(fmt.Sprintf("Creating variable %q from specification file %q", path, arg))
		fPath := arg[1:]
		c.contents, err = os.ReadFile(fPath)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("error reading %q: %s", fPath, err))
			return 1
		}
		args = args[1:]
	default:
		// no-op - should be KV arg
	}

	sv, err := c.makeVariable(path)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to parse variable data: %s", err))
		return 1
	}

	var warnings *multierror.Error
	if len(args) > 0 {
		data, err := parseArgsData(stdin, args)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to parse K=V data: %s", err))
			return 1
		}

		for k, v := range data {
			vs := v.(string)
			if vs == "" {
				if _, ok := sv.Items[k]; ok {
					verbose(fmt.Sprintf("Removed item %q", k))
					delete(sv.Items, k)
				} else {
					verbose(fmt.Sprintf("Item %q does not exist, continuing...", k))
				}
				continue
			}
			if err := warnInvalidIdentifier(k); err != nil {
				warnings = multierror.Append(warnings, err)
			}
			sv.Items[k] = vs
		}
	}
	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	if enforce {
		sv.ModifyIndex = checkIndex
	}

	if force {
		sv, _, err = client.Variables().Update(sv, nil)
	} else {
		sv, _, err = client.Variables().CheckedUpdate(sv, nil)
	}
	if err != nil {
		if handled := handleCASError(err, c); handled {
			return 1
		}
		c.Ui.Error(fmt.Sprintf("Error creating variable: %s", err))
		return 1
	}

	successMsg := fmt.Sprintf(
		"Created variable %q with modify index %v", sv.Path, sv.ModifyIndex)

	if warnings != nil {
		c.Ui.Warn(c.FormatWarnings(
			"Variable",
			helper.MergeMultierrorWarnings(warnings),
		))
	}

	var out string
	switch c.outFmt {
	case "json":
		out = sv.AsPrettyJSON()
	case "hcl":
		out = renderAsHCL(sv)
	case "go-template":
		if out, err = renderWithGoTemplate(sv, c.tmpl); err != nil {
			c.Ui.Error(err.Error())
			return 1
		}
	case "table":
		// the renderSVAsUiTable func writes directly to the ui and doesn't error.
		verbose(successMsg)
		renderSVAsUiTable(sv, c)
		return 0
	default:
		c.Ui.Output(successMsg)
		return 0
	}
	verbose(successMsg)
	c.Ui.Output(out)
	return 0
}

// makeVariable creates a variable based on whether or not there is data in
// content and the format is set.
func (c *VarPutCommand) makeVariable(path string) (*api.Variable, error) {
	var err error
	out := new(api.Variable)
	if len(c.contents) == 0 {
		out.Path = path
		out.Namespace = c.Meta.namespace
		out.Items = make(map[string]string)
		return out, nil
	}

	switch c.inFmt {
	case "json":
		err = json.Unmarshal(c.contents, out)
		if err != nil {
			return nil, fmt.Errorf("error unmarshaling json: %w", err)
		}
	case "hcl":
		out, err = parseVariableSpec(c.contents, c.verbose)
		if err != nil {
			return nil, fmt.Errorf("error parsing hcl: %w", err)
		}
	case "":
		return nil, errors.New("format flag required")
	default:
		return nil, fmt.Errorf("unknown format flag value")
	}

	// It is possible a specification file was used which did not declare any
	// items. Therefore, default the entry to avoid panics and ensure this type
	// of use is valid.
	if out.Items == nil {
		out.Items = make(map[string]string)
	}

	// Handle cases where values are provided by CLI flags that modify the
	// the created variable. Typical of a "copy" operation, it is a convenience
	// to reset the Create and Modify metadata to zero.
	var resetIndex bool

	// Step on the namespace in the object if one is provided by flag
	if c.Meta.namespace != "" && c.Meta.namespace != out.Namespace {
		out.Namespace = c.Meta.namespace
		resetIndex = true
	}

	// Step on the path in the object if one is provided by argument.
	if path != "" && path != out.Path {
		out.Path = path
		resetIndex = true
	}

	if resetIndex {
		out.CreateIndex = 0
		out.CreateTime = 0
		out.ModifyIndex = 0
		out.ModifyTime = 0
	}
	return out, nil
}

// parseVariableSpec is used to parse the variable specification
// from HCL
func parseVariableSpec(input []byte, verbose func(string)) (*api.Variable, error) {
	root, err := hcl.ParseBytes(input)
	if err != nil {
		return nil, err
	}

	// Top-level item should be a list
	list, ok := root.Node.(*ast.ObjectList)
	if !ok {
		return nil, fmt.Errorf("error parsing: root should be an object")
	}

	var out api.Variable
	if err := parseVariableSpecImpl(&out, list); err != nil {
		return nil, err
	}
	return &out, nil
}

// parseVariableSpecImpl parses the variable taking as input the AST tree
func parseVariableSpecImpl(result *api.Variable, list *ast.ObjectList) error {
	// Decode the full thing into a map[string]interface for ease
	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, list); err != nil {
		return err
	}

	// Check for invalid keys
	valid := []string{
		"namespace",
		"path",
		"create_index",
		"modify_index",
		"create_time",
		"modify_time",
		"items",
	}
	if err := helper.CheckHCLKeys(list, valid); err != nil {
		return err
	}

	for _, index := range []string{"create_index", "modify_index"} {
		if value, ok := m[index]; ok {
			vInt, ok := value.(int)
			if !ok {
				return fmt.Errorf("%s must be integer; got (%T) %[2]v", index, value)
			}
			idx := uint64(vInt)
			n := strings.ReplaceAll(strings.Title(strings.ReplaceAll(index, "_", " ")), " ", "")
			m[n] = idx
			delete(m, index)
		}
	}

	for _, index := range []string{"create_time", "modify_time"} {
		if value, ok := m[index]; ok {
			vInt, ok := value.(int)
			if !ok {
				return fmt.Errorf("%s must be a int64; got a (%T) %[2]v", index, value)
			}
			n := strings.ReplaceAll(strings.Title(strings.ReplaceAll(index, "_", " ")), " ", "")
			m[n] = vInt
			delete(m, index)
		}
	}

	// Decode the rest
	if err := mapstructure.WeakDecode(m, result); err != nil {
		return err
	}

	return nil
}

func isArgFileRef(a string) bool {
	return strings.HasPrefix(a, "@") && !strings.HasPrefix(a, "\\@")
}

func isArgStdinRef(a string) bool {
	return a == "-"
}

// sanitizePath removes any leading or trailing things from a "path".
func sanitizePath(s string) string {
	return strings.Trim(strings.TrimSpace(s), "/")
}

// parseArgsData parses the given args in the format key=value into a map of
// the provided arguments. The given reader can also supply key=value pairs.
func parseArgsData(stdin io.Reader, args []string) (map[string]interface{}, error) {
	builder := &KVBuilder{Stdin: stdin}
	if err := builder.Add(args...); err != nil {
		return nil, err
	}
	return builder.Map(), nil
}

func (c *VarPutCommand) GetConcurrentUI() cli.ConcurrentUi {
	return cli.ConcurrentUi{Ui: c.Ui}
}

func (c *VarPutCommand) setParserForFileArg(arg string) error {
	switch filepath.Ext(arg) {
	case ".json":
		c.inFmt = "json"
	case ".hcl":
		c.inFmt = "hcl"
	default:
		return fmt.Errorf("Unable to determine format of %s; Use the -in flag to specify it.", arg)
	}
	return nil
}

func (c *VarPutCommand) validateInputFlag() error {
	switch c.inFmt {
	case "hcl", "json":
		return nil
	default:
		return errors.New(errInvalidInFormat)
	}
}

func (c *VarPutCommand) validateOutputFlag() error {
	if c.outFmt != "go-template" && c.tmpl != "" {
		return errors.New(errUnexpectedTemplate)
	}
	switch c.outFmt {
	case "none", "json", "hcl", "table":
		return nil
	case "go-template":
		if c.tmpl == "" {
			return errors.New(errMissingTemplate)
		}
		return nil
	default:
		return errors.New(errInvalidOutFormat)
	}
}

func warnInvalidIdentifier(in string) error {
	invalid := invalidIdentifier.FindAllString(in, -1)
	if len(invalid) == 0 {
		return nil
	}

	// Use %s instead of %q to avoid escaping characters.
	return fmt.Errorf(
		`"%s" contains characters %s that require the 'index' function for direct access in templates`,
		in,
		formatInvalidVarKeyChars(invalid),
	)
}

func formatInvalidVarKeyChars(invalid []string) string {
	// Deduplicate characters
	chars := set.From(invalid)

	// Sort the characters for output
	charList := make([]string, 0, chars.Size())

	for k := range chars.Items() {
		// Use %s instead of %q to avoid escaping characters.
		charList = append(charList, fmt.Sprintf(`"%s"`, k))
	}

	slices.Sort(charList)

	// Build string
	return fmt.Sprintf("[%s]", strings.Join(charList, ","))
}
