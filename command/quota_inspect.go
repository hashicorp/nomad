package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type QuotaInspectCommand struct {
	Meta
}

type inspectedQuota struct {
	Spec     *api.QuotaSpec
	Usages   map[string]*api.QuotaUsage
	Failures map[string]string `json:"UsageLookupErrors"`
}

func (c *QuotaInspectCommand) Help() string {
	helpText := `
Usage: nomad quota inspect [options] <quota>

  Inspect is used to view raw information about a particular quota.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Inspect Options:

  -t
    Format and display the namespaces using a Go template.
`

	return strings.TrimSpace(helpText)
}

func (c *QuotaInspectCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-t": complete.PredictAnything,
		})
}

func (c *QuotaInspectCommand) AutocompleteArgs() complete.Predictor {
	return QuotaPredictor(c.Meta.Client)
}

func (c *QuotaInspectCommand) Synopsis() string {
	return "Inspect a quota specification"
}

func (c *QuotaInspectCommand) Name() string { return "quota inspect" }

func (c *QuotaInspectCommand) Run(args []string) int {
	var tmpl string
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.StringVar(&tmpl, "t", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got one arguments
	args = flags.Args()
	if l := len(args); l != 1 {
		c.Ui.Error("This command takes one argument: <quota>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	name := args[0]

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Do a prefix lookup
	quotas := client.Quotas()
	spec, possible, err := getQuota(quotas, name)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error retrieving quota: %s", err))
		return 1
	}

	if len(possible) != 0 {
		c.Ui.Error(fmt.Sprintf("Prefix matched multiple quotas\n\n%s", formatQuotaSpecs(possible)))
		return 1
	}

	// Get the quota usages
	usages, failures := quotaUsages(spec, quotas)

	failuresConverted := make(map[string]string, len(failures))
	for r, e := range failures {
		failuresConverted[r] = e.Error()
	}

	data := &inspectedQuota{
		Spec:     spec,
		Usages:   usages,
		Failures: failuresConverted,
	}

	out, err := Format(len(tmpl) == 0, tmpl, data)
	if err != nil {
		c.Ui.Error(err.Error())
		return 1
	}

	c.Ui.Output(out)
	return 0
}
