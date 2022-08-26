package command

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

type VarGetCommand struct {
	Meta
	outFmt string
	tmpl   string
}

func (c *VarGetCommand) Help() string {
	helpText := `
Usage: nomad var get [options] <path>

  The 'var get' command is used to get the contents of an existing secure
  variable.

  If ACLs are enabled, this command requires a token with the 'var:read'
  capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Read Options:

  -output ( go-template | hcl | json | table )
     Format to render the secure variable in. When using "go-template",
     provide the template content with the "-template" option. Defaults
     to "table" when stdout is a terminal and to "json" when stdout is
	 redirected.

  -template
     Template to render output with. Required when output is "go-template".

  -exit-code-not-found
     Exit code to use when the secure variable is not found. Defaults to
     1.
`
	return strings.TrimSpace(helpText)
}

func (c *VarGetCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-format":              complete.PredictSet("table", "hcl", "json", "go-template"),
			"-template":            complete.PredictAnything,
			"-exit-code-not-found": complete.PredictAnything,
		},
	)
}

func (c *VarGetCommand) AutocompleteArgs() complete.Predictor {
	return SecureVariablePathPredictor(c.Meta.Client)
}

func (c *VarGetCommand) Synopsis() string {
	return "Read a secure variable"
}

func (c *VarGetCommand) Name() string { return "var read" }

func (c *VarGetCommand) Run(args []string) int {
	var exitCodeNotFound int
	var out string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) != 0 {
		flags.StringVar(&c.outFmt, "output", "table", "")
	} else {
		flags.StringVar(&c.outFmt, "output", "json", "")
	}
	flags.StringVar(&c.tmpl, "template", "", "")
	flags.IntVar(&exitCodeNotFound, "exit-code-not-found", 1, "")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got one argument
	args = flags.Args()
	if l := len(args); !(l == 1 || l == 2) {
		c.Ui.Error("This command takes one or two arguments:\n  <path>\n <path> <item key>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	if err := c.validateOutputFlag(); err != nil {
		c.Ui.Error(err.Error())
		return 1
	}

	path := args[0]

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	qo := &api.QueryOptions{
		Namespace: c.Meta.namespace,
	}

	sv, _, err := client.SecureVariables().Read(path, qo)
	if err != nil {
		if err.Error() == "secure variable not found" {
			c.Ui.Warn("Secure variable not found")
			return exitCodeNotFound
		}
		c.Ui.Error(fmt.Sprintf("Error retrieving secure variable: %s", err))
		return 1
	}

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
	default:
		// the renderSVAsUiTable func writes directly to the ui and doesn't error.
		renderSVAsUiTable(sv, c)
		return 0
	}

	c.Ui.Output(out)
	return 0
}

func (c *VarGetCommand) validateOutputFlag() error {
	switch c.outFmt {
	case "none": // noop
	case "json": // noop
	case "hcl": //noop
	case "go-template": //noop
	default:
		return errors.New(`Invalid value for "-output"; valid values are [go-template, hcl, json, none]`)
	}
	if c.outFmt == "go-template" && c.tmpl == "" {
		return errors.New(`A template must be supplied using '-template' when using go-template formatting`)
	}
	if c.outFmt != "go-template" && c.tmpl != "" {
		return errors.New(`The '-template' flag is only valid when using 'go-template' formatting`)
	}
	return nil
}

func (c *VarGetCommand) GetConcurrentUI() cli.ConcurrentUi {
	return c.GetConcurrentUI()
}
