package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/cli"
)

// Ensure ACLAuthMethodCommand satisfies the cli.Command interface.
var _ cli.Command = &ACLAuthMethodCommand{}

// ACLAuthMethodCommand implements cli.Command.
type ACLAuthMethodCommand struct {
	Meta
}

// Help satisfies the cli.Command Help function.
func (a *ACLAuthMethodCommand) Help() string {
	helpText := `
Usage: nomad acl auth-method <subcommand> [options] [args]

  This command groups subcommands for interacting with ACL auth methods.

  Create an ACL auth method:

      $ nomad acl auth-method create -name="name" -type="OIDC" -max-token-ttl="3600s"

  List all ACL auth methods:

      $ nomad acl auth-method list

  Lookup a specific ACL auth method:

      $ nomad acl auth-method info <acl_auth_method_name>

  Update an ACL auth method:

      $ nomad acl auth-method update -type="updated-type" <acl_auth_method_name>

  Delete an ACL auth method:

      $ nomad acl auth-method delete <acl_auth_method_name>

  Please see the individual subcommand help for detailed usage information.
`
	return strings.TrimSpace(helpText)
}

// Synopsis satisfies the cli.Command Synopsis function.
func (a *ACLAuthMethodCommand) Synopsis() string { return "Interact with ACL auth methods" }

// Name returns the name of this command.
func (a *ACLAuthMethodCommand) Name() string { return "acl auth-method" }

// Run satisfies the cli.Command Run function.
func (a *ACLAuthMethodCommand) Run(_ []string) int { return cli.RunResultHelp }

// formatAuthMethod formats and converts the ACL auth method API object into a
// string KV representation suitable for console output.
func formatAuthMethod(authMethod *api.ACLAuthMethod) string {
	out := []string{
		fmt.Sprintf("Name|%s", authMethod.Name),
		fmt.Sprintf("Type|%s", authMethod.Type),
		fmt.Sprintf("Locality|%s", authMethod.TokenLocality),
		fmt.Sprintf("MaxTokenTTL|%s", authMethod.MaxTokenTTL.String()),
		fmt.Sprintf("Default|%t", authMethod.Default),
	}

	if authMethod.Config != nil {
		out = append(out, formatAuthMethodConfig(authMethod.Config)...)
	}
	out = append(out,
		[]string{fmt.Sprintf("Create Index|%d", authMethod.CreateIndex),
			fmt.Sprintf("Modify Index|%d", authMethod.ModifyIndex),
		}...,
	)

	return formatKV(out)
}

func formatAuthMethodConfig(config *api.ACLAuthMethodConfig) []string {
	return []string{
		fmt.Sprintf("OIDC Discovery URL|%s", config.OIDCDiscoveryURL),
		fmt.Sprintf("OIDC Client ID|%s", config.OIDCClientID),
		fmt.Sprintf("OIDC Client Secret|%s", config.OIDCClientSecret),
		fmt.Sprintf("Bound audiences|%s", strings.Join(config.BoundAudiences, ",")),
		fmt.Sprintf("Allowed redirects URIs|%s", strings.Join(config.AllowedRedirectURIs, ",")),
		fmt.Sprintf("Discovery CA pem|%s", strings.Join(config.DiscoveryCaPem, ",")),
		fmt.Sprintf("Signing algorithms|%s", strings.Join(config.SigningAlgs, ",")),
		fmt.Sprintf("Claim mappings|%s", formatMap(config.ClaimMappings)),
		fmt.Sprintf("List claim mappings|%s", formatMap(config.ListClaimMappings)),
	}
}

func formatMap(m map[string]string) string {
	out := []string{}
	for k, v := range m {
		out = append(out, fmt.Sprintf("%s/%s", k, v))
	}
	return formatKV(out)
}
