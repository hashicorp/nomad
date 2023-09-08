// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

// Ensure ACLRoleUpdateCommand satisfies the cli.Command interface.
var _ cli.Command = &ACLRoleUpdateCommand{}

// ACLRoleUpdateCommand implements cli.Command.
type ACLRoleUpdateCommand struct {
	Meta

	name        string
	description string
	policyNames []string
	noMerge     bool
	json        bool
	tmpl        string
}

// Help satisfies the cli.Command Help function.
func (a *ACLRoleUpdateCommand) Help() string {
	helpText := `
Usage: nomad acl role update [options] <acl_role_id>

  Update is used to update an existing ACL token. Requires a management token.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Update Options:

  -name
    Sets the human readable name for the ACL role. The name must be between
    1-128 characters.

  -description
    A free form text description of the role that must not exceed 256
    characters.

  -policy
    Specifies a policy to associate with the role identified by their name. This
    flag can be specified multiple times.

  -no-merge
    Do not merge the current role information with what is provided to the
    command. Instead overwrite all fields with the exception of the role ID
    which is immutable.

  -json
    Output the ACL role in a JSON format.

  -t
    Format and display the ACL role using a Go template.
`

	return strings.TrimSpace(helpText)
}

func (a *ACLRoleUpdateCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(a.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-name":        complete.PredictAnything,
			"-description": complete.PredictAnything,
			"-no-merge":    complete.PredictNothing,
			"-policy":      complete.PredictAnything,
			"-json":        complete.PredictNothing,
			"-t":           complete.PredictAnything,
		})
}

func (a *ACLRoleUpdateCommand) AutocompleteArgs() complete.Predictor { return complete.PredictNothing }

// Synopsis satisfies the cli.Command Synopsis function.
func (a *ACLRoleUpdateCommand) Synopsis() string { return "Update an existing ACL role" }

// Name returns the name of this command.
func (*ACLRoleUpdateCommand) Name() string { return "acl role update" }

// Run satisfies the cli.Command Run function.
func (a *ACLRoleUpdateCommand) Run(args []string) int {

	flags := a.Meta.FlagSet(a.Name(), FlagSetClient)
	flags.Usage = func() { a.Ui.Output(a.Help()) }
	flags.StringVar(&a.name, "name", "", "")
	flags.StringVar(&a.description, "description", "", "")
	flags.Var((funcVar)(func(s string) error {
		a.policyNames = append(a.policyNames, s)
		return nil
	}), "policy", "")
	flags.BoolVar(&a.noMerge, "no-merge", false, "")
	flags.BoolVar(&a.json, "json", false, "")
	flags.StringVar(&a.tmpl, "t", "", "")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got exactly one argument which is expected to be the ACL
	// role ID.
	if len(flags.Args()) != 1 {
		a.Ui.Error("This command takes one argument: <acl_role_id>")
		a.Ui.Error(commandErrorText(a))
		return 1
	}

	// Get the HTTP client.
	client, err := a.Meta.Client()
	if err != nil {
		a.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	aclRoleID := flags.Args()[0]

	// Read the current role in both cases, so we can fail better if not found.
	currentRole, _, err := client.ACLRoles().Get(aclRoleID, nil)
	if err != nil {
		a.Ui.Error(fmt.Sprintf("Error when retrieving ACL role: %v", err))
		return 1
	}

	var updatedRole api.ACLRole

	// Depending on whether we are merging or not, we need to take a different
	// approach.
	switch a.noMerge {
	case true:

		// Perform some basic validation on the submitted role information to
		// avoid sending API and RPC requests which will fail basic validation.
		if a.name == "" {
			a.Ui.Error("ACL role name must be specified using the -name flag")
			return 1
		}
		if len(a.policyNames) < 1 {
			a.Ui.Error("At least one policy name must be specified using the -policy flag")
			return 1
		}

		updatedRole = api.ACLRole{
			ID:          aclRoleID,
			Name:        a.name,
			Description: a.description,
			Policies:    aclRolePolicyNamesToPolicyLinks(a.policyNames),
		}
	default:
		// Check that the operator specified at least one flag to update the ACL
		// role with.
		if len(a.policyNames) == 0 && a.name == "" && a.description == "" {
			a.Ui.Error("Please provide at least one flag to update the ACL role")
			a.Ui.Error(commandErrorText(a))
			return 1
		}

		updatedRole = *currentRole

		// If the operator specified a name or description, overwrite the
		// existing value as these are simple strings.
		if a.name != "" {
			updatedRole.Name = a.name
		}
		if a.description != "" {
			updatedRole.Description = a.description
		}

		// In order to merge the policy updates, we need to identify if the
		// specified policy names already exist within the ACL role linking.
		for _, policyName := range a.policyNames {

			// Track whether we found the policy name already in the ACL role
			// linking.
			var found bool

			for _, existingLinkedPolicy := range currentRole.Policies {
				if policyName == existingLinkedPolicy.Name {
					found = true
					break
				}
			}

			// If the policy name was not found, append this new link to the
			// updated role.
			if !found {
				updatedRole.Policies = append(updatedRole.Policies, &api.ACLRolePolicyLink{Name: policyName})
			}
		}
	}

	// Update the ACL role with the new information via the API.
	updatedACLRoleRead, _, err := client.ACLRoles().Update(&updatedRole, nil)
	if err != nil {
		a.Ui.Error(fmt.Sprintf("Error updating ACL role: %s", err))
		return 1
	}

	if a.json || len(a.tmpl) > 0 {
		out, err := Format(a.json, a.tmpl, updatedACLRoleRead)
		if err != nil {
			a.Ui.Error(err.Error())
			return 1
		}

		a.Ui.Output(out)
		return 0
	}

	// Format the output
	a.Ui.Output(formatACLRole(updatedACLRoleRead))
	return 0
}
