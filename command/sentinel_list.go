package command

import (
	"fmt"
	"strings"

	"github.com/posener/complete"
)

type SentinelListCommand struct {
	Meta
}

func (c *SentinelListCommand) Help() string {
	helpText := `
Usage: nomad sentinel list [options]

  List is used to display all the installed Sentinel policies.

  Sentinel commands are only available when ACLs are enabled. This command
  requires a management token.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `
  
ListOptions:
-json
	Output the latest quota information in a JSON format.
-t
	Format and display quota information using a Go template.
`
	return strings.TrimSpace(helpText)
}

func (c *SentinelListCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-t":    complete.PredictAnything,
			"-json": complete.PredictNothing,
		})
}

func (c *SentinelListCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *SentinelListCommand) Synopsis() string {
	return "Display all Sentinel policies"
}

func (c *SentinelListCommand) Name() string { return "sentinel list" }

func (c *SentinelListCommand) Run(args []string) int {
	var json bool
	var tmpl string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&tmpl, "t", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	if args = flags.Args(); len(args) > 0 {
		c.Ui.Error("This command takes no arguments")
		c.Ui.Error(commandErrorText(c))
	}
	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Get the list of policies
	policies, _, err := client.SentinelPolicies().List(nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error listing Sentinel policies: %s", err))
		return 1
	}

	if len(policies) == 0 {
		c.Ui.Output("No policies found")
		return 0
	}

	if json || len(tmpl) > 0 {
		out, err := Format(json, tmpl, policies)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}

		c.Ui.Output(out)
		return 0
	}

	out := []string{}
	out = append(out, "Name|Scope|Enforcement Level|Description")
	for _, p := range policies {
		line := fmt.Sprintf("%s|%s|%s|%s", p.Name, p.Scope, p.EnforcementLevel, p.Description)
		out = append(out, line)
	}
	c.Ui.Output(formatList(out))
	return 0
}
