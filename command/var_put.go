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
	"github.com/hashicorp/nomad/command/kvbuilder"
	"github.com/mitchellh/mapstructure"
	"github.com/posener/complete"
)

type VarPutCommand struct {
	Meta

	contents  []byte
	format    string
	testStdin io.Reader // for tests
}

func (c *VarPutCommand) Help() string {
	helpText := `
Usage: nomad var put [options] <path> [<key>=<value>]...

  The 'var put' command is used to create or update an existing secure variable.

  If ACLs are enabled, this command requires a token with the 'var:write'
  capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Apply Options:

  -format (hcl | json)
     Parser to use for data supplied via standard input or when the type can
     not be know using the file extension

  -force
     Replace any existing value at the specified path without checking
`
	return strings.TrimSpace(helpText)
}

func (c *VarPutCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-format": complete.PredictSet("hcl", "json"),
		},
	)
}

func (c *VarPutCommand) AutocompleteArgs() complete.Predictor {
	return SecureVariablePathPredictor(c.Meta.Client)
}

func (c *VarPutCommand) Synopsis() string {
	return "Create or update a secure variable"
}

func (c *VarPutCommand) Name() string { return "var put" }

func (c *VarPutCommand) Run(args []string) int {
	var forceOverwrite bool

	f := c.Meta.FlagSet(c.Name(), FlagSetClient)
	f.Usage = func() { c.Ui.Output(c.Help()) }
	f.StringVar(&c.format, "format", "json", "")
	f.BoolVar(&forceOverwrite, "force", false, "")

	if err := f.Parse(args); err != nil {
		c.Ui.Error(err.Error())
		return 1
	}

	args = f.Args()

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

	var err error

	arg := args[0]
	var path string
	c.Ui.Error("***** Handling Arg 1 *****")

	switch {
	// Handle first argument: can be -, @file, «var path»
	case isArgStdinRef(arg):
		c.Ui.Info("Reading whole variable specification from stdin")
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			c.contents, err = io.ReadAll(os.Stdin)
			if err != nil {
				c.Ui.Error(fmt.Sprintf("Error reading from stdin: %s", err))
				return 1
			}
		}
	case isArgFileRef(arg):
		c.Ui.Info(fmt.Sprintf("Reading whole variable specification from %q", arg))
		c.contents, err = os.ReadFile(arg)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error reading %q: %s", arg, err))
			return 1
		}
	default:
		path = sanitizePath(arg)
		c.Ui.Info(fmt.Sprintf("Writing to path %q", path))
	}

	args = args[1:]
	c.Ui.Error("***** Handling Arg 2 *****")
	switch {
	// Handle second argument: can be -, @file, or kv
	case len(args) == 0:
		// no-op
	case isArgStdinRef(args[0]):
		c.Ui.Info("Reading variable specification from stdin")
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
		c.Ui.Info(fmt.Sprintf("Reading variable specification from %q", arg))
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
		c.Ui.Error(fmt.Sprintf("Failed to parse secure variable data: %s", err))
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
			c.Ui.Info(fmt.Sprintf("Setting item %q", k))
			sv.Items[k] = vs
		}
	}
	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}
	fmt.Println(sv)

	var createFn func(*api.SecureVariable, *api.WriteOptions) (*api.WriteMeta, error)
	createFn = client.SecureVariables().CheckedUpdate
	if forceOverwrite {
		createFn = client.SecureVariables().Update
	}
	_, err = createFn(sv, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error creating secure variable: %s", err))
		return 1
	}

	c.Ui.Output(fmt.Sprintf("Successfully created secure variable %q!", sv.Path))
	return 0
}

// makeVariable creates a variable based on whether or not there is data in
// content and the format is set
func (c *VarPutCommand) makeVariable(path string) (*api.SecureVariable, error) {
	var err error
	out := new(api.SecureVariable)
	if len(c.contents) == 0 {
		out.Path = path
		out.Namespace = c.Meta.namespace
		out.Items = make(map[string]string)
		return out, nil
	}
	switch c.format {
	case "json":
		fmt.Println("json")
		c.Ui.Info("unmarhaslling json")
		err = json.Unmarshal(c.contents, out)
		if err != nil {
			return nil, fmt.Errorf("error unmarshaling json: %w", err)
		}
	case "hcl":
		fmt.Println("json")

		out, err = parseSecureVariableSpec(c.contents)
		if err != nil {
			return nil, fmt.Errorf("error parsing hcl: %w", err)
		}
	case "":
		fmt.Println("format flag required")
		return nil, errors.New("format flag required")
	default:
		return nil, fmt.Errorf("unknown format flag value")
	}
	// Step on the namespace in the object if one is provided by flag
	if c.Meta.namespace != "" {
		out.Namespace = c.Meta.namespace
	}
	c.Ui.Error(fmt.Sprintf("%#v\n", out))
	return out, nil
}

// parseSecureVariableSpec is used to parse the secure variable specification
// from HCL
func parseSecureVariableSpec(input []byte) (*api.SecureVariable, error) {
	root, err := hcl.ParseBytes(input)
	if err != nil {
		return nil, err
	}

	// Top-level item should be a list
	list, ok := root.Node.(*ast.ObjectList)
	if !ok {
		return nil, fmt.Errorf("error parsing: root should be an object")
	}

	var out api.SecureVariable
	if err := parseSecureVariableSpecImpl(&out, list); err != nil {
		return nil, err
	}

	return &out, nil
}

// parseSecureVariableSpecImpl parses the secure variable taking as input the AST tree
func parseSecureVariableSpecImpl(result *api.SecureVariable, list *ast.ObjectList) error {
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
	builder := &kvbuilder.Builder{Stdin: stdin}
	if err := builder.Add(args...); err != nil {
		return nil, err
	}
	return builder.Map(), nil
}
