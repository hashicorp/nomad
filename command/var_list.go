package command

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type VarListCommand struct {
	Prefix string
	Meta
}

func (c *VarListCommand) Help() string {
	helpText := `
Usage: nomad var list [options] <prefix>

  List is used to list available secure variables.

  If ACLs are enabled, this command requires a token with the 'namespace:read'
  capability. Any secure variables for namespaces that the token does not have
  access to will be filtered from the results.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

List Options:

  -json
    Output the secure variables in JSON format.

  -t
    Format and display the secure variables using a Go template.
`
	return strings.TrimSpace(helpText)
}

func (c *VarListCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json": complete.PredictNothing,
			"-t":    complete.PredictAnything,
		},
	)
}

func (c *VarListCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *VarListCommand) Synopsis() string {
	return "List secure variable specifications"
}

func (c *VarListCommand) Name() string { return "var list" }
func (c *VarListCommand) Run(args []string) int {
	var json bool
	var tmpl string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&tmpl, "t", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got no arguments
	args = flags.Args()
	if l := len(args); l > 1 {
		c.Ui.Error("This command takes either no arguments or one: <prefix>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	prefix := ""
	if len(args) == 1 {
		prefix = args[0]
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	qo := &api.QueryOptions{}

	vars, _, err := client.SecureVariables().PrefixList(prefix, qo)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error retrieving vars: %s", err))
		return 1
	}

	if json || len(tmpl) > 0 {
		out, err := Format(json, tmpl, vars)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}

		c.Ui.Output(out)
		return 0
	}

	c.Ui.Output(formatVarStubs(vars))
	return 0
}

func formatVarStubs(vars []*api.SecureVariableMetadata) string {
	if len(vars) == 0 {
		return "No secure variables found"
	}

	// Sort the output by variable namespace, path
	sort.Slice(vars, func(i, j int) bool {
		if vars[i].Namespace == vars[j].Namespace {
			return vars[i].Path < vars[j].Path
		}
		return vars[i].Namespace < vars[j].Namespace
	})

	rows := make([]string, len(vars)+1)
	rows[0] = "Namespace|Path|Last Updated"
	for i, sv := range vars {
		rows[i+1] = fmt.Sprintf("%s|%s|%s",
			sv.Namespace,
			sv.Path,
			time.Unix(0, sv.ModifyTime),
		)
	}
	return formatList(rows)
}
