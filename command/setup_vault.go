// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/hashicorp/vault/api"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

// Ensure SetupVaultCommand satisfies the cli.Command interface.
var _ cli.Command = &SetupVaultCommand{}

//go:embed asset/vault-wi-default-auth-method-config.json
var vaultAuthConfigBody []byte

//go:embed asset/vault-wi-default-policy.hcl
var vaultPolicyBody []byte

//go:embed asset/vault-wi-default-role.json
var vaultRoleBody []byte

const (
	vaultRole       = "nomad-workloads"
	vaultPolicyName = "nomad-workloads"
	vaultNamespace  = "nomad-workloads"
	vaultAud        = "vault.io"
	vaultPath       = "jwt-nomad"
)

type SetupVaultCommand struct {
	Meta

	vClient  *api.Client
	vLogical *api.Logical
	ns       string

	jwksURL string

	destroy bool
	autoYes bool
}

// Help satisfies the cli.Command Help function.
func (s *SetupVaultCommand) Help() string {
	helpText := `
Usage: nomad setup vault [options]

  This command sets up Vault for allowing Nomad workloads to authenticate
  themselves using Workload Identity.

  This command requires acl:write permissions for Vault and respects
  VAULT_TOKEN, VAULT_ADDR, and other Vault-related environment variables
  as documented in https://developer.hashicorp.com/vault/docs/commands#environment-variables.

  WARNING: This command is an experimental feature and may change its behavior
  in future versions of Nomad.

Setup Vault options:

  -jwks-url <url>
    URL of Nomad's JWKS endpoint contacted by Vault to verify JWT
    signatures. Defaults to http://localhost:4646/.well-known/jwks.json.

  -destroy
    Removes all configuration components this command created from the
    Vault cluster.

  -y
    Automatically answers "yes" to all the questions, making the setup
    non-interactive. Defaults to "false".

`
	return strings.TrimSpace(helpText)
}

func (s *SetupVaultCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(s.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-jwks-url": complete.PredictAnything,
			"-destroy":  complete.PredictSet("true", "false"),
			"-y":        complete.PredictSet("true", "false"),
		})
}

func (s *SetupVaultCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

// Synopsis satisfies the cli.Command Synopsis function.
func (s *SetupVaultCommand) Synopsis() string { return "Setup a Vault cluster for Nomad integration" }

// Name returns the name of this command.
func (s *SetupVaultCommand) Name() string { return "setup vault" }

// Run satisfies the cli.Command Run function.
func (s *SetupVaultCommand) Run(args []string) int {

	flags := s.Meta.FlagSet(s.Name(), FlagSetClient)
	flags.Usage = func() { s.Ui.Output(s.Help()) }
	flags.BoolVar(&s.destroy, "destroy", false, "")
	flags.BoolVar(&s.autoYes, "y", false, "")
	flags.StringVar(&s.jwksURL, "jwks-url", "http://localhost:4646/.well-known/jwks.json", "")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got no arguments.
	if len(flags.Args()) != 0 {
		s.Ui.Error("This command takes no arguments")
		s.Ui.Error(commandErrorText(s))
		return 1
	}

	if !isTty() && !s.autoYes {
		s.Ui.Error("This command requires -y option when running in non-interactive mode")
		return 1
	}

	if !s.destroy {
		s.Ui.Output(`
This command will walk you through configuring all the components required for
Nomad workloads to authenticate themselves against Vault ACL using their
respective workload identities.

First we need to connect to Vault.
`)
	}

	clientCfg := api.DefaultConfig()
	if !s.autoYes {
		if !s.askQuestion(fmt.Sprintf("Is %q the correct address of your Vault cluster? [Y/n]", clientCfg.Address)) {
			s.Ui.Warn(`
Please set the VAULT_ADDR environment variable to your Vault cluster address and re-run the command.`)
			return 0
		}
	}

	// Get the Vault client.
	var err error
	s.vClient, err = api.NewClient(clientCfg)
	if err != nil {
		s.Ui.Error(fmt.Sprintf("Error initializing Vault client: %s", err))
		return 1
	}
	s.vLogical = s.vClient.Logical()

	// ent check: if we're not in empty namespace or the license check returns
	// non-nil (license checks will only ever work from default namespace),
	// we're connected to ent
	var ent bool
	clientNamespace := s.vClient.Namespace()
	license, _ := s.vClient.Logical().Read("/sys/license/status")
	ent = clientNamespace != "" || license != nil

	// Setup Vault client namespace.
	if ent {
		if clientNamespace != "" {
			// Confirm VAULT_NAMESPACE will be used.
			if !s.autoYes {
				if !s.askQuestion(fmt.Sprintf("Is %q the correct Vault namespace to use? [Y/n]", clientNamespace)) {
					s.Ui.Warn(`
Please set the VAULT_NAMESPACE environment variable to the Vault namespace to use and re-run the command.`)
					return 0
				}
			}
			s.ns = clientNamespace
		} else {
			// Set default namespace if VAULT_NAMESPACE is not defined.
			s.ns = vaultNamespace
			s.vClient.SetNamespace(s.ns)
		}
	}

	if s.destroy {
		return s.removeConfiguredComponents()
	}

	/*
		Namespace creation and setup
	*/
	if ent {
		namespaceMsg := `
Since you're running Vault Enterprise, we will additionally create
a namespace %q and create all configuration within that namespace.
`
		if s.namespaceExists(s.ns, false) {
			s.Ui.Info(fmt.Sprintf("[✔] Namespace %q already exists.", s.ns))
		} else {
			s.Ui.Output(fmt.Sprintf(namespaceMsg, s.ns))

			if !s.autoYes && !s.askQuestion(
				fmt.Sprintf("Create the namespace %q in your Vault cluster? [Y/n]", s.ns)) {
				s.handleNo()
			}

			err = s.createNamespace(s.ns)
			if err != nil {
				s.Ui.Error(err.Error())
				return 1
			}
		}
	}

	/*
		Auth method creation
	*/
	s.Ui.Output(`
We will now enable the JWT credential backend and create a JWT auth method that
Nomad workloads will use. 
`)

	if s.authMethodExists() {
		s.Ui.Info(fmt.Sprintf("[✔] JWT auth method %q already exists.", vaultPath))
	} else {

		s.Ui.Output("This is the method configuration:\n")
		authMethodConf, err := s.renderAuthMethod()
		if err != nil {
			s.Ui.Error(err.Error())
			return 1
		}
		jsConf, _ := json.MarshalIndent(authMethodConf, "", "    ")

		s.Ui.Output(string(jsConf))

		if !s.autoYes && !s.askQuestion("Create JWT auth method in your Vault cluster? [Y/n]") {
			s.handleNo()
		}

		err = s.createAuthMethod(authMethodConf)
		if err != nil {
			s.Ui.Error(err.Error())
			return 1
		}
	}

	/*
		Policy & role creation
	*/
	s.Ui.Output(`
We need to create a role that Nomad workloads will assume while authenticating,
and a policy associated with that role.
	`)

	if s.policyExists() {
		s.Ui.Info(fmt.Sprintf("[✔] Policy %q already exists.", vaultPolicyName))
	} else {
		s.Ui.Output(fmt.Sprintf(`
These are the rules for the policy %q that we will create. It uses a templated
policy to allow Nomad tasks to access secrets in the path
"secrets/data/<job namespace>/<job name>":
`, vaultPolicyName))

		policyBody, err := s.renderPolicy()
		if err != nil {
			s.Ui.Error(err.Error())
			return 1
		}
		s.Ui.Output(policyBody)

		if !s.autoYes && !s.askQuestion("Create the above policy in your Vault cluster? [Y/n]") {
			s.handleNo()
		}

		err = s.createPolicy(policyBody)
		if err != nil {
			s.Ui.Error(err.Error())
			return 1
		}
	}

	if s.roleExists() {
		s.Ui.Info(fmt.Sprintf("[✔] Role %q already exists.", vaultRole))
	} else {
		s.Ui.Output(fmt.Sprintf(`
We will now create an ACL role called %q associated with the policy above.
`,
			vaultRole))

		roleBody, err := s.renderRole()
		if err != nil {
			s.Ui.Error(err.Error())
			return 1
		}

		roleJS, _ := json.MarshalIndent(roleBody, "", "    ")
		s.Ui.Output(string(roleJS))

		if !s.autoYes && !s.askQuestion("Create role in your Vault cluster? [Y/n]") {
			s.handleNo()
		}

		err = s.createRole(roleBody)
		if err != nil {
			s.Ui.Error(err.Error())
			return 1
		}
	}

	s.Ui.Output(`
Congratulations, your Vault cluster is now setup and ready to accept Nomad
workloads with Workload Identity!

You need to adjust your Nomad client configuration in the following way:

vault {
  enabled = true
  address = "<Vault address>"

  # Vault Enterprise only.
  # namespace = "<namespace>"

  jwt_auth_backend_path = "jwt-nomad/"
}

And your Nomad server configuration in the following way:

vault {
  enabled = true

  default_identity {
    aud = ["vault.io"]
    ttl = "1h"
  }
}`)
	return 0
}

func (s *SetupVaultCommand) roleExists() bool {
	existingRoles, _ := s.vLogical.List(fmt.Sprintf("/auth/%s/role", vaultPath))
	if existingRoles != nil {
		return slices.Contains(existingRoles.Data["keys"].([]any), vaultRole)
	}
	return false
}

func (s *SetupVaultCommand) renderRole() (map[string]any, error) {
	role := map[string]any{}
	err := json.Unmarshal(vaultRoleBody, &role)
	if err != nil {
		return role, fmt.Errorf("[✘] Role data could not be deserialized: %w", err)
	}

	role["bound_audiences"] = vaultAud

	return role, nil
}

func (s *SetupVaultCommand) createRole(role map[string]any) error {
	buf, err := json.Marshal(role)
	if err != nil {
		return fmt.Errorf("[✘] Role could not be interpolated with args: %w", err)
	}

	path := fmt.Sprintf("auth/%s/role/%s", vaultPath, vaultRole)

	_, err = s.vLogical.WriteBytes(path, buf)
	if err != nil {
		return fmt.Errorf("[✘] Could not create Vault role: %w", err)
	}

	s.Ui.Info(fmt.Sprintf("[✔] Created role %q.", vaultRole))
	return nil
}

func (s *SetupVaultCommand) policyExists() bool {
	existingPolicies, _ := s.vClient.Sys().ListPolicies()
	return slices.Contains(existingPolicies, vaultPolicyName)
}

func (s *SetupVaultCommand) renderPolicy() (string, error) {
	secret, err := s.vLogical.Read("sys/auth/" + vaultPath)
	if err != nil {
		return "", fmt.Errorf("[✘] Could not retrieve JWT accessor: %w", err)
	}
	accessor := secret.Data["accessor"].(string)

	policyTextStr := string(vaultPolicyBody)
	return strings.ReplaceAll(policyTextStr, "auth_jwt_X", accessor), nil
}

func (s *SetupVaultCommand) createPolicy(policyText string) error {
	encoded := base64.StdEncoding.EncodeToString([]byte(policyText))
	policyBody := fmt.Sprintf(`{"policy": "%s"}`, encoded)
	buf := []byte(policyBody)

	path := "sys/policies/acl/" + vaultPolicyName
	_, err := s.vLogical.WriteBytes(path, buf)
	if err != nil {
		return fmt.Errorf("[✘] Could not create Vault policy: %w", err)
	}

	s.Ui.Info(fmt.Sprintf("[✔] Created policy %q.", vaultPolicyName))

	return nil
}

func (s *SetupVaultCommand) authMethodExists() bool {
	existingConf, _ := s.vLogical.Read(fmt.Sprintf("/auth/%s/config", vaultPath))
	return existingConf != nil
}

func (s *SetupVaultCommand) renderAuthMethod() (map[string]any, error) {
	authConfig := map[string]any{}
	err := json.Unmarshal(vaultAuthConfigBody, &authConfig)
	if err != nil {
		return nil, fmt.Errorf("default auth config text could not be deserialized: %v", err)
	}

	authConfig["jwks_url"] = s.jwksURL
	authConfig["default_role"] = vaultRole

	return authConfig, nil
}

func (s *SetupVaultCommand) createAuthMethod(authConfig map[string]any) error {
	err := s.vClient.Sys().EnableAuthWithOptions(vaultPath, &api.MountInput{Type: "jwt"})
	if err != nil {
		return fmt.Errorf("[✘] Could not enable JWT credential backend: %w", err)
	}

	buf, err := json.Marshal(authConfig)
	if err != nil {
		return fmt.Errorf("auth method could not be interpolated with args: %w", err)
	}
	_, err = s.vLogical.WriteBytes(fmt.Sprintf("auth/%s/config", vaultPath), buf)
	if err != nil {
		if strings.Contains(err.Error(), "error checking jwks URL") {
			s.Ui.Error(fmt.Sprintf(
				"error: Nomad JWKS endpoint unreachable, verify that Nomad is running and that the JWKS URL %s is reachable by Vault", s.jwksURL,
			))
			os.Exit(1)
		}
		return fmt.Errorf("[✘] Could not create Vault auth method: %w", err)
	}

	s.Ui.Info(fmt.Sprintf("[✔] Created JWT auth method %q.", vaultPath))
	return nil
}

// namespaceExists takes checks if ns exists. if destroy is true, it will check
// for custom metadata presence to prevent deleting a namespace we didn't
// create.
func (s *SetupVaultCommand) namespaceExists(ns string, destroy bool) bool {
	s.vClient.SetNamespace("")
	defer s.vClient.SetNamespace(s.ns)

	existingNamespace, _ := s.vLogical.Read(fmt.Sprintf("/sys/namespaces/%s", ns))
	if destroy && existingNamespace != nil {
		if m, ok := existingNamespace.Data["custom_metadata"]; ok {
			if mm, ok := m.(map[string]any)["created-by"]; ok {
				return mm == "nomad-setup"
			}
		}
	} else {
		return existingNamespace != nil
	}
	return false
}

func (s *SetupVaultCommand) createNamespace(ns string) error {
	s.vClient.SetNamespace("")
	defer s.vClient.SetNamespace(s.ns)

	_, err := s.vLogical.Write(
		"/sys/namespaces/"+ns,
		map[string]any{
			"custom_metadata": map[string]string{
				"created-by": "nomad-setup",
			},
		},
	)
	if err != nil {
		return fmt.Errorf("[✘] Could not write namespace %q: %w", ns, err)
	}
	s.Ui.Info(fmt.Sprintf("[✔] Created namespace %q.", ns))
	return nil
}

// askQuestion asks question to user until they provide a valid response.
func (s *SetupVaultCommand) askQuestion(question string) bool {
	for {
		answer, err := s.Ui.Ask(s.Colorize().Color(fmt.Sprintf("[?] %s", question)))
		if err != nil {
			if err.Error() != "interrupted" {
				s.Ui.Output(err.Error())
				os.Exit(1)
			}
			os.Exit(0)
		}

		switch strings.TrimSpace(strings.ToLower(answer)) {
		case "", "y", "yes":
			return true
		case "n", "no":
			return false
		default:
			s.Ui.Output(fmt.Sprintf(`%q is not a valid response, please answer "yes" or "no".`, answer))
			continue
		}
	}
}

func (s *SetupVaultCommand) handleNo() {
	s.Ui.Warn(`
By answering "no" to any of these questions, you are risking an incorrect Vault
cluster configuration. Nomad workloads with Workload Identity will not be able
to authenticate unless you create missing configuration yourself.
 `)

	exitCode := 0
	if s.autoYes || s.askQuestion("Remove everything this command creates? [Y/n]") {
		exitCode = s.removeConfiguredComponents()
	}

	s.Ui.Output(s.Colorize().Color(`
Vault cluster has [bold][underline]not[reset] been configured for authenticating Nomad tasks and
services using workload identities.

Run the command again to finish the configuration process.`))
	os.Exit(exitCode)
}

func (s *SetupVaultCommand) removeConfiguredComponents() int {
	s.vClient.SetNamespace(s.ns)
	exitCode := 0
	componentsToRemove := map[string]string{}

	if s.policyExists() {
		componentsToRemove["Policy"] = vaultPolicyName
	}
	if s.roleExists() {
		componentsToRemove["Role"] = vaultRole
	}
	if s.authMethodExists() {
		componentsToRemove["JWT auth method"] = vaultPath
	}
	if s.namespaceExists(s.ns, true) {
		componentsToRemove["Namespace"] = s.ns
	}

	if len(componentsToRemove) == 0 {
		s.Ui.Output("Nothing to delete.")
		return 0
	}

	q := `The following items will be deleted:
%s`
	if !s.autoYes {
		s.Ui.Warn(fmt.Sprintf(q, printMapOfStrings(componentsToRemove)))
	}

	if s.autoYes || s.askQuestion("Remove all the items listed above? [Y/n]") {

		if policy, ok := componentsToRemove["Policy"]; ok {
			err := s.vClient.Sys().DeletePolicy(policy)
			if err != nil {
				s.Ui.Error(fmt.Sprintf("[✘] Failed to delete policy %q: %v", policy, err.Error()))
				exitCode = 1
			} else {
				s.Ui.Info(fmt.Sprintf("[✔] Deleted policy %q.", policy))
			}
		}

		if role, ok := componentsToRemove["Role"]; ok {
			_, err := s.vLogical.Delete(fmt.Sprintf("/auth/%s/role/%s", vaultPath, role))
			if err != nil {
				s.Ui.Error(fmt.Sprintf("[✘] Failed to delete role %q: %v", role, err.Error()))
				exitCode = 1
			} else {
				s.Ui.Info(fmt.Sprintf("[✔] Deleted role %q.", role))
			}
		}

		if _, ok := componentsToRemove["JWT auth method"]; ok {
			if err := s.vClient.Sys().DisableAuth(vaultPath); err != nil {
				s.Ui.Error(fmt.Sprintf("[✘] Failed to disable JWT auth method %q %v", vaultPath, err.Error()))
				exitCode = 1
			} else {
				s.Ui.Info(fmt.Sprintf("[✔] Disabled JWT auth method %q.", vaultPath))
			}
		}

		if ns, ok := componentsToRemove["Namespace"]; ok {
			s.vClient.SetNamespace("")
			_, err := s.vLogical.Delete(fmt.Sprintf("/sys/namespaces/%s", ns))
			if err != nil {
				s.Ui.Error(fmt.Sprintf("[✘] Failed to delete namespace %q: %v", ns, err.Error()))
				exitCode = 1
			} else {
				s.Ui.Info(fmt.Sprintf("[✔] Deleted namespace %q.", ns))
			}
		}
	}

	return exitCode
}

func printMapOfStrings(m map[string]string) string {
	var output string

	for k, v := range m {
		if v != "" {
			output += fmt.Sprintf("  * %s: %q\n", k, v)
		} else {
			output += fmt.Sprintf("  * %s\n", k)
		}
	}

	return output
}
