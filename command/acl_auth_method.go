// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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

// outputAuthMethod can be used to output the auth method to the UI within the
// passed meta object.
func outputAuthMethod(meta Meta, authMethod *api.ACLAuthMethod) {
	meta.Ui.Output(formatAuthMethod(authMethod))
	if authMethod.Config != nil {
		meta.Ui.Output(meta.Colorize().Color("\n[bold]Auth Method Config[reset]\n"))
		meta.Ui.Output(formatAuthMethodConfig(authMethod.Config))
	}
}

// formatAuthMethod formats and converts the ACL auth method API object into a
// string KV representation suitable for console output.
func formatAuthMethod(authMethod *api.ACLAuthMethod) string {
	out := []string{
		fmt.Sprintf("Name|%s", authMethod.Name),
		fmt.Sprintf("Type|%s", authMethod.Type),
		fmt.Sprintf("Locality|%s", authMethod.TokenLocality),
		fmt.Sprintf("MaxTokenTTL|%s", authMethod.MaxTokenTTL.String()),
		fmt.Sprintf("Default|%t", authMethod.Default),
		fmt.Sprintf("Create Index|%d", authMethod.CreateIndex),
		fmt.Sprintf("Modify Index|%d", authMethod.ModifyIndex),
	}
	return formatKV(out)
}

func formatAuthMethodConfig(config *api.ACLAuthMethodConfig) string {
	out := []string{
		fmt.Sprintf("JWT Validation Public Keys|%s", strings.Join(config.JWTValidationPubKeys, ",")),
		fmt.Sprintf("JWKS URL|%s", config.JWKSURL),
		fmt.Sprintf("OIDC Discovery URL|%s", config.OIDCDiscoveryURL),
		fmt.Sprintf("OIDC Client ID|%s", config.OIDCClientID),
		fmt.Sprintf("OIDC Client Secret|%s", config.OIDCClientSecret),
		fmt.Sprintf("OIDC Scopes|%s", strings.Join(config.OIDCScopes, ",")),
		fmt.Sprintf("Bound audiences|%s", strings.Join(config.BoundAudiences, ",")),
		fmt.Sprintf("Bound issuer|%s", strings.Join(config.BoundIssuer, ",")),
		fmt.Sprintf("Allowed redirects URIs|%s", strings.Join(config.AllowedRedirectURIs, ",")),
		fmt.Sprintf("Discovery CA pem|%s", strings.Join(config.DiscoveryCaPem, ",")),
		fmt.Sprintf("JWKS CA cert|%s", config.JWKSCACert),
		fmt.Sprintf("Signing algorithms|%s", strings.Join(config.SigningAlgs, ",")),
		fmt.Sprintf("Expiration Leeway|%s", config.ExpirationLeeway.String()),
		fmt.Sprintf("NotBefore Leeway|%s", config.NotBeforeLeeway.String()),
		fmt.Sprintf("ClockSkew Leeway|%s", config.ClockSkewLeeway.String()),
		fmt.Sprintf("Claim mappings|%s", strings.Join(formatMap(config.ClaimMappings), "; ")),
		fmt.Sprintf("List claim mappings|%s", strings.Join(formatMap(config.ListClaimMappings), "; ")),
	}
	return formatKV(out)
}

func formatMap(m map[string]string) []string {
	out := []string{}
	for k, v := range m {
		out = append(out, fmt.Sprintf("{%s: %s}", k, v))
	}
	return out
}
