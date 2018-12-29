package command

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type SentinelApplyCommand struct {
	Meta
}

func (c *SentinelApplyCommand) Help() string {
	helpText := `
Usage: nomad sentinel apply [options] <name> <file>

  Apply is used to write a new Sentinel policy or update an existing one.
  The name of the policy and file must be specified. The file will be read
  from stdin by specifying "-".

General Options:

  ` + generalOptionsUsage() + `

Apply Options:

  -description
    Sets a human readable description for the policy.

  -scope (default: submit-job)
    Sets the scope of the policy and when it should be enforced.

  -level (default: advisory)
    Sets the enforcment level of the policy. Must be one of advisory,
    soft-mandatory, hard-mandatory.

`
	return strings.TrimSpace(helpText)
}

func (c *SentinelApplyCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-description": complete.PredictAnything,
			"-scope":       complete.PredictAnything,
			"-level":       complete.PredictAnything,
		})
}

func (c *SentinelApplyCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *SentinelApplyCommand) Synopsis() string {
	return "Create a new or update existing Sentinel policies"
}

func (c *SentinelApplyCommand) Run(args []string) int {
	var description, scope, enfLevel string
	var err error
	flags := c.Meta.FlagSet("sentinel apply", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.StringVar(&description, "description", "", "")
	flags.StringVar(&scope, "scope", "submit-job", "")
	flags.StringVar(&enfLevel, "level", "advisory", "")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got exactly two arguments
	args = flags.Args()
	if l := len(args); l != 2 {
		c.Ui.Error(c.Help())
		return 1
	}

	// Get the name and file
	policyName := args[0]

	// Read the file contents
	file := args[1]
	var rawPolicy []byte
	if file == "-" {
		rawPolicy, err = ioutil.ReadAll(os.Stdin)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to read stdin: %v", err))
			return 1
		}
	} else {
		rawPolicy, err = ioutil.ReadFile(file)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to read file: %v", err))
			return 1
		}
	}

	// Construct the policy
	sp := &api.SentinelPolicy{
		Name:             policyName,
		Description:      description,
		Scope:            scope,
		EnforcementLevel: enfLevel,
		Policy:           string(rawPolicy),
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Get the list of policies
	_, err = client.SentinelPolicies().Upsert(sp, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error writing Sentinel policy: %s", err))
		return 1
	}

	c.Ui.Output(fmt.Sprintf("Successfully wrote %q Sentinel policy!",
		policyName))
	return 0
}
