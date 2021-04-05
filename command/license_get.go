package command

import (
	"fmt"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type LicenseGetCommand struct {
	Meta
}

func (c *LicenseGetCommand) Help() string {
	helpText := `
Usage: nomad license get [options]

  Gets a new license in Servers and Clients

  When ACLs are enabled, this command requires a token with the
  'operator:read' capability.

  -stale=[true|false]
	By default the license get command will be forwarded to the Nomad leader.
	If -stale is set to true, the command will not be forwarded to the
	leader and will return the license from the specific server being
	contacted. This option may be useful during upgrade scenarios when a server
	is given a new file license and is a follower so the new license has not
	yet been propagated to raft.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace)

	return helpText
}

func (c *LicenseGetCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-stale": complete.PredictAnything,
		})
}

func (c *LicenseGetCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *LicenseGetCommand) Synopsis() string {
	return "Retrieve the current Nomad Enterprise License"
}

func (c *LicenseGetCommand) Name() string { return "license get" }

func (c *LicenseGetCommand) Run(args []string) int {
	var stale bool

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	flags.BoolVar(&stale, "stale", false, "")

	if err := flags.Parse(args); err != nil {
		c.Ui.Error(fmt.Sprintf("Error parsing flags: %s", err))
		return 1
	}

	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	q := &api.QueryOptions{
		AllowStale: stale,
	}
	resp, _, err := client.Operator().LicenseGet(q)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error getting license: %v", err))
		return 1
	}

	return OutputLicenseReply(c.Ui, resp)
}
