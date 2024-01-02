// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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

  The 'var get' command is used to get the contents of an existing variable.

  If ACLs are enabled, this command requires a token with the 'variables:read'
  capability for the target variable's namespace and path.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Get Options:

  -item <item key>
     Print only the value of the given item. Specifying this option will
     take precedence over other formatting directives. The result will not
     have a trailing newline making it ideal for piping to other processes.

  -out ( go-template | hcl | json | none | table )
     Format to render the variable in. When using "go-template", you must
     provide the template content with the "-template" option. Defaults
     to "table" when stdout is a terminal and to "json" when stdout is
     redirected.

  -template
     Template to render output with. Required when output is "go-template".

`
	return strings.TrimSpace(helpText)
}

func (c *VarGetCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-out":      complete.PredictSet("go-template", "hcl", "json", "none", "table"),
			"-template": complete.PredictAnything,
		},
	)
}

func (c *VarGetCommand) AutocompleteArgs() complete.Predictor {
	return VariablePathPredictor(c.Meta.Client)
}

func (c *VarGetCommand) Synopsis() string {
	return "Read a variable"
}

func (c *VarGetCommand) Name() string { return "var get" }

func (c *VarGetCommand) Run(args []string) int {
	var out, item string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	flags.StringVar(&item, "item", "", "")
	flags.StringVar(&c.tmpl, "template", "", "")

	if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) != 0 {
		flags.StringVar(&c.outFmt, "out", "table", "")
	} else {
		flags.StringVar(&c.outFmt, "out", "json", "")
	}

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got one argument
	args = flags.Args()
	if len(args) != 1 {
		c.Ui.Error("This command takes one argument: <path>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	if err := c.validateOutputFlag(); err != nil {
		c.Ui.Error(err.Error())
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	if c.Meta.namespace == "*" {
		c.Ui.Error(errWildcardNamespaceNotAllowed)
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

	sv, _, err := client.Variables().Read(path, qo)
	if err != nil {
		if err.Error() == "variable not found" {
			c.Ui.Warn(errVariableNotFound)
			return 1
		}
		c.Ui.Error(fmt.Sprintf("Error retrieving variable: %s", err))
		return 1
	}
	// If the user provided an item key, return that value instead of the whole
	// object
	if item != "" {
		if v, ok := sv.Items[item]; ok {
			fmt.Print(v)
			return 0
		} else {
			c.Ui.Error(fmt.Sprintf("Variable does not contain %q item", args[1]))
			return 1
		}
	}

	// Output whole object
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
	case "none":
		// exit without more output
		return 0
	default:
		// the renderSVAsUiTable func writes directly to the ui and doesn't error.
		renderSVAsUiTable(sv, c)
		return 0
	}

	c.Ui.Output(out)
	return 0
}

func (c *VarGetCommand) validateOutputFlag() error {
	if c.outFmt != "go-template" && c.tmpl != "" {
		return errors.New(errUnexpectedTemplate)
	}
	switch c.outFmt {
	case "hcl", "json", "none", "table":
		return nil
	case "go-template": //noop - needs more validation
		if c.tmpl == "" {
			return errors.New(errMissingTemplate)
		}
		return nil
	default:
		return errors.New(errInvalidOutFormat)
	}
}

func (c *VarGetCommand) GetConcurrentUI() cli.ConcurrentUi {
	return cli.ConcurrentUi{Ui: c.Ui}
}
