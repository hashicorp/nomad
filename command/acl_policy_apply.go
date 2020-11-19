package command

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type ACLPolicyApplyCommand struct {
	Meta
}

func (c *ACLPolicyApplyCommand) Help() string {
	helpText := `
Usage: nomad acl policy apply [options] <name> <path>

  Apply is used to create or update an ACL policy. The policy is
  sourced from <path> or from stdin if path is "-".

  This command requires a management ACL token.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Apply Options:

  -description
    Specifies a human readable description for the policy.

`
	return strings.TrimSpace(helpText)
}

func (c *ACLPolicyApplyCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{})
}

func (c *ACLPolicyApplyCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *ACLPolicyApplyCommand) Synopsis() string {
	return "Create or update an ACL policy"
}

func (c *ACLPolicyApplyCommand) Name() string { return "acl policy apply" }

func (c *ACLPolicyApplyCommand) Run(args []string) int {
	var description string
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.StringVar(&description, "description", "", "")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got two arguments
	args = flags.Args()
	if l := len(args); l != 2 {
		c.Ui.Error("This command takes two arguments: <name> and <path>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	// Get the policy name
	policyName := args[0]

	// Read the file contents
	file := args[1]
	var rawPolicy []byte
	var err error
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
	ap := &api.ACLPolicy{
		Name:        policyName,
		Description: description,
		Rules:       string(rawPolicy),
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Upsert the policy
	_, err = client.ACLPolicies().Upsert(ap, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error writing ACL policy: %s", err))
		return 1
	}

	c.Ui.Output(fmt.Sprintf("Successfully wrote %q ACL policy!",
		policyName))
	return 0
}
