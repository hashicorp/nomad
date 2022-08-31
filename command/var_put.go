package command

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/ast"
	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/cli"
	"github.com/mitchellh/mapstructure"
	"github.com/posener/complete"
)

type VarPutCommand struct {
	Meta

	contents  []byte
	inFmt     string
	outFmt    string
	tmpl      string
	testStdin io.Reader // for tests
}

func (c *VarPutCommand) Help() string {
	helpText := `
Usage: nomad var put [options] <path> [<key>=<value>]...

  The 'var put' command is used to create or update an existing variable.

  If ACLs are enabled, this command requires a token with the 'var:write'
  capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Apply Options:
  -check-index
     If set, the variable is only purged if the server side version's modify
     index matches the provided value.

  -in (hcl | json)
     Parser to use for data supplied via standard input or when the type can
     not be known using the file's extension. Defaults to "json".

  -out (hcl | json | none | table)
     Format to render created or updated variable. Defaults to "none" when
     stdout is a terminal and "json" when the output is redirected.

  -template
     Template to render output with. Required when format is "go-template",
     invalid for other formats.

  -force
     Replace any existing value at the specified path regardless of modify index.

  -v
     Verbose output. Additional output is written to stderr to preserve stdout
     for redirected output.

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
	var err error
	var forceOverwrite, enforce, doVerbose bool
	var path, checkIndexStr string
	var checkIndex uint64

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	flags.StringVar(&c.inFmt, "in", "json", "")
	flags.StringVar(&checkIndexStr, "check-index", "", "")
	flags.BoolVar(&forceOverwrite, "force", false, "")
	flags.BoolVar(&doVerbose, "v", false, "")

	if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) != 0 {
		flags.StringVar(&c.outFmt, "out", "none", "")
	} else {
		flags.StringVar(&c.outFmt, "out", "json", "")
	}

	if err := flags.Parse(args); err != nil {
		c.Ui.Error(err.Error())
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
	// Parse the check-index
	checkIndex, enforce, err = parseCheckIndex(checkIndexStr)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error parsing check-index value %q: %v", checkIndexStr, err))
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
		return 1
	case len(args) == 1 && !isArgStdinRef(args[0]) && !isArgFileRef(args[0]):
		c.Ui.Error("Must supply data")
		return 1
	}

	if err = c.validateInputFlag(); err != nil {
		c.Ui.Error(err.Error())
		return 1
	}

	if err := c.validateOutputFlag(); err != nil {
		c.Ui.Error(err.Error())
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
		filepath := arg[1:]
		// detect format based on extension
		p := strings.Split(filepath, ".")
		if len(p) < 2 {
			c.Ui.Error(fmt.Sprintf("Unable to determine format of %s; Use the -in flag to specify it.", filepath))
			return 1
		}
		// detect format based on file extension
		switch strings.ToLower(p[len(p)-1]) {
		case "json":
			c.inFmt = "json"
		case "hcl":
			c.inFmt = "hcl"
		default:
			c.Ui.Error(fmt.Sprintf("Unable to determine format of %s; Use the -in flag to specify it.", filepath))
			return 1
		}

		verbose(fmt.Sprintf("Reading whole %s variable specification from %q", strings.ToUpper(c.inFmt), filepath))
		c.contents, err = os.ReadFile(filepath)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error reading %q: %s", filepath, err))
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
					c.Ui.Warn(fmt.Sprintf("Removed item %q", k))
					delete(sv.Items, k)
				} else {
					c.Ui.Warn(fmt.Sprintf("Item %q does not exist, continuing...", k))
				}
				continue
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

	if forceOverwrite {
		sv, _, err = client.Variables().Update(sv, nil)
	} else {
		sv, _, err = client.Variables().CheckedUpdate(sv, nil)
	}
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error creating variable: %s", err))
		return 1
	}

	verbose(fmt.Sprintf("Created variable %q with modify index %v", sv.Path, sv.ModifyIndex))

	var out string
	switch c.outFmt {
	case "json":
		out = sv.AsJSON()
	case "hcl":
		out = renderAsHCL(sv)
	case "go-template":
		if out, err = renderWithGoTemplate(sv, c.tmpl); err != nil {
			c.Ui.Error(err.Error())
			return 1
		}
	case "table":
		// the renderSVAsUiTable func writes directly to the ui and doesn't error.
		renderSVAsUiTable(sv, c)
		return 0
	default:
		return 0
	}
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
		out, err = parseVariableSpec(c.contents)
		if err != nil {
			return nil, fmt.Errorf("error parsing hcl: %w", err)
		}
	case "":
		fmt.Println("format flag required")
		return nil, errors.New("format flag required")
	default:
		return nil, fmt.Errorf("unknown format flag value")
	}

	// Handle cases where values are provided by CLI flags that modify the
	// the created variable. Typical of a "copy" operation, it is a convenience
	// to reset the Create and Modify metadata to zero.
	var resetIndex bool

	// Step on the namespace in the object if one is provided by flag
	if c.Meta.namespace != "" && c.Meta.namespace != out.Path {
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
func parseVariableSpec(input []byte) (*api.Variable, error) {
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
	fmt.Printf("\n\n\n%+#v\n\n\n", out)
	return &out, nil
}

// parseVariableSpecImpl parses the variable taking as input the AST tree
func parseVariableSpecImpl(result *api.Variable, list *ast.ObjectList) error {
	// Decode the full thing into a map[string]interface for ease
	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, list); err != nil {
		return err
	}

	delete(m, "items")

	// Decode the rest
	if err := mapstructure.WeakDecode(m, result); err != nil {
		return err
	}

	if itemsO := list.Filter("items"); len(itemsO.Items) > 0 {
		for _, o := range itemsO.Elem().Items {
			var m map[string]interface{}
			if err := hcl.DecodeObject(&m, o.Val); err != nil {
				return err
			}
			if err := mapstructure.WeakDecode(m, &result.Items); err != nil {
				return err
			}
		}
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

func (c *VarPutCommand) validateInputFlag() error {
	switch c.inFmt {
	case "hcl": // noop
	case "json": // noop
	default:
		return errors.New(`Invalid value for "-in"; valid values are [hcl, json]`)
	}
	return nil
}

func (c *VarPutCommand) validateOutputFlag() error {
	switch c.outFmt {
	case "none": // noop
	case "json": // noop
	case "hcl": //noop
	case "go-template": //noop
	case "table": //noop
	default:
		return errors.New(`Invalid value for "-out"; valid values are [go-template, hcl, json, none, table]`)
	}
	if c.outFmt == "go-template" && c.tmpl == "" {
		return errors.New(`A template must be supplied using '-template' when using go-template formatting`)
	}
	if c.outFmt != "go-template" && c.tmpl != "" {
		return errors.New(`The '-template' flag is only valid when using 'go-template' formatting`)
	}
	return nil
}
