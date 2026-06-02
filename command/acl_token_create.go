// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/go-set/v3"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper"
	"github.com/posener/complete"
)

type ACLTokenCreateCommand struct {
	Meta

	roleNames []string
	roleIDs   []string
}

func (c *ACLTokenCreateCommand) Help() string {
	helpText := `
Usage: nomad acl token create [options] [<path>]

  Create is used to issue new ACL tokens. Requires a management token.

  By default, Nomad generates the AccessorID and SecretID automatically. To
  upload (restore) a client token with pre-specified IDs — for example, when
  recovering tokens from a backup — provide -accessor and supply the SecretID
  via a file at <path>, or pass "-" to read it from stdin. Only client tokens
  may be uploaded; management tokens must always be created fresh.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Create Options:

  -name=""
    Sets the human readable name for the ACL token.

  -type="client"
    Sets the type of token. Must be one of "client" (default), or "management".

  -global=false
    Toggles the global mode of the token. Global tokens are replicated to all regions.

  -policy=""
    Specifies a policy to associate with the token. Can be specified multiple times,
    but only with client type tokens.

  -role-id
     ID of an ACL role to associate with this token. ACL roles are
     created with "nomad acl role create". May be specified multiple times,
     but only with client type tokens.

  -role-name
     Name of an ACL role to associate with this token. ACL roles are created
     with "nomad acl role create". May be specified multiple times, but only
     with client type tokens.

  -ttl
    Specifies the time-to-live of the created ACL token. This takes the form of
    a time duration such as "5m" and "1h". By default, tokens will be created
    without a TTL and therefore never expire.

  -accessor=""
    Pre-specified AccessorID (UUID) for the token. When provided, the SecretID
    must be supplied via <path> (a file or "-" for stdin). The token is uploaded
    rather than generated. Only valid for client tokens.

  -json
    Output the ACL token information in JSON format.

  -t
    Format and display the ACL token information using a Go template.
`
	return strings.TrimSpace(helpText)
}

func (c *ACLTokenCreateCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"name":      complete.PredictAnything,
			"type":      complete.PredictAnything,
			"global":    complete.PredictNothing,
			"policy":    complete.PredictAnything,
			"role-id":   complete.PredictAnything,
			"role-name": complete.PredictAnything,
			"ttl":       complete.PredictAnything,
			"accessor":  complete.PredictAnything,
			"-json":     complete.PredictNothing,
			"-t":        complete.PredictAnything,
		})
}

func (c *ACLTokenCreateCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *ACLTokenCreateCommand) Synopsis() string {
	return "Create a new ACL token"
}

func (c *ACLTokenCreateCommand) Name() string { return "acl token create" }

func (c *ACLTokenCreateCommand) Run(args []string) int {
	var name, tokenType, ttl, tmpl, accessorID string
	var global, json bool
	var policies []string
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.StringVar(&name, "name", "", "")
	flags.StringVar(&tokenType, "type", "client", "")
	flags.BoolVar(&global, "global", false, "")
	flags.StringVar(&ttl, "ttl", "", "")
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&tmpl, "t", "", "")
	flags.StringVar(&accessorID, "accessor", "", "")
	flags.Var((funcVar)(func(s string) error {
		policies = append(policies, s)
		return nil
	}), "policy", "")
	flags.Var((funcVar)(func(s string) error {
		c.roleNames = append(c.roleNames, s)
		return nil
	}), "role-name", "")
	flags.Var((funcVar)(func(s string) error {
		c.roleIDs = append(c.roleIDs, s)
		return nil
	}), "role-id", "")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	args = flags.Args()
	if l := len(args); l > 1 {
		c.Ui.Error("This command takes up to one argument")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	// If -accessor is set, the caller must also supply the SecretID via a file
	// or stdin (positional argument).
	if accessorID != "" && len(args) == 0 {
		c.Ui.Error("-accessor requires a SecretID supplied as a file path or \"-\" for stdin")
		c.Ui.Error(commandErrorText(c))
		return 1
	}
	if accessorID == "" && len(args) == 1 {
		c.Ui.Error("A SecretID file path was provided but -accessor was not set")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	if accessorID != "" && !helper.IsUUID(accessorID) {
		c.Ui.Error("-accessor value must be a valid UUID")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	var secretID string
	if len(args) == 1 {
		var raw []byte
		var err error
		switch args[0] {
		case "-":
			raw, err = io.ReadAll(os.Stdin)
		default:
			raw, err = os.ReadFile(args[0])
		}
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error reading SecretID: %v", err))
			return 1
		}
		secretID = strings.TrimSpace(string(raw))
		if !helper.IsUUID(secretID) {
			c.Ui.Error("SecretID must be a valid UUID")
			c.Ui.Error(commandErrorText(c))
			return 1
		}
	}

	// Set up the token.
	tk := &api.ACLToken{
		Name:     name,
		Type:     tokenType,
		Policies: policies,
		Roles:    generateACLTokenRoleLinks(c.roleNames, c.roleIDs),
		Global:   global,
	}

	// If the user set a TTL flag value, convert this to a time duration and
	// add it to our token request object.
	if ttl != "" {
		ttlDuration, err := time.ParseDuration(ttl)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to parse TTL as time duration: %s", err))
			return 1
		}
		tk.ExpirationTTL = ttlDuration
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Show warning if policy doesn't exist
	for _, policy := range tk.Policies {
		_, _, err := client.ACLPolicies().Info(policy, nil)
		if err != nil {
			c.Ui.Warn(fmt.Sprintf("Error fetching info on %s policy: %s", policy, err))
		}
	}

	var token *api.ACLToken

	if accessorID != "" {
		tk.AccessorID = accessorID
		tk.SecretID = secretID
		token, _, err = client.ACLTokens().Upload(tk, nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error uploading token: %s", err))
			return 1
		}
	} else {
		token, _, err = client.ACLTokens().Create(tk, nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error creating token: %s", err))
			return 1
		}
	}

	if json || len(tmpl) > 0 {
		out, err := Format(json, tmpl, token)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}

		c.Ui.Output(out)
		return 0
	}
	// Format the output
	outputACLToken(c.Ui, token)
	return 0
}

// generateACLTokenRoleLinks takes the command input role links by ID and name
// and coverts this to the relevant API object. It handles de-duplicating
// entries to the best effort, so this doesn't need to be done on the leader.
func generateACLTokenRoleLinks(roleNames, roleIDs []string) []*api.ACLTokenRoleLink {
	var tokenLinks []*api.ACLTokenRoleLink

	roleNameSet := set.From[string](roleNames).Slice()
	roleNameFn := func(name string) *api.ACLTokenRoleLink { return &api.ACLTokenRoleLink{Name: name} }

	roleIDsSet := set.From[string](roleIDs).Slice()
	roleIDFn := func(id string) *api.ACLTokenRoleLink { return &api.ACLTokenRoleLink{ID: id} }

	tokenLinks = append(tokenLinks, helper.ConvertSlice(roleNameSet, roleNameFn)...)
	tokenLinks = append(tokenLinks, helper.ConvertSlice(roleIDsSet, roleIDFn)...)

	return tokenLinks
}
