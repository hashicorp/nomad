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
	vaultAuthMethodName = "nomad-tasks"
	vaultAuthMethodDesc = "Login method for Nomad tasks using workload identities"
	vaultRoleTasks      = "role-nomad-tasks"
	vaultPolicyName     = "policy-nomad-tasks"
	vaultNamespace      = "nomad-workloads"
	vaultAud            = "vault.io"
)

type SetupVaultCommand struct {
	Meta

	vClient  *api.Client
	vLogical *api.Logical

	jwksURL string

	vaultEnt bool
	cleanup  bool
	autoYes  bool
}

// Help satisfies the cli.Command Help function.
func (s *SetupVaultCommand) Help() string {
	helpText := `
Usage: nomad setup vault [options]

  This command sets up Vault for allowing Nomad workloads to authenticate
  themselves using Workload Identity.

  This command requires acl:write permissions for Vault and respects
  VAULT_TOKEN, VAULT_ADDR, and other Consul-related environment variables
  as documented in https://developer.hashicorp.com/nomad/docs/runtime/environment#summary.

  WARNING: This command is an experimental feature and may change its behavior
  in future versions of Nomad.

Setup Vault options:

  -jwks-url <url>
    URL of Nomad's JWKS endpoint contacted by Vault to verify JWT
    signatures. Defaults to http://localhost:4646/.well-known/jwks.json.

  -cleanup
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
			"-cleanup":  complete.PredictSet("true", "false"),
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
	flags.BoolVar(&s.cleanup, "cleanup", false, "")
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

	if !s.cleanup {
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
		s.Ui.Error(fmt.Sprintf("Error initializing Consul client: %s", err))
		return 1
	}
	s.vLogical = s.vClient.Logical()

	// check if we're connecting to Vault ent
	if _, err := s.vLogical.Read("/sys/license/status"); err == nil {
		s.vaultEnt = true
	}

	// Setup Vault client namespace.
	if s.vaultEnt {
		if s.vClient.Namespace() != "" {
			// Confirm VAULT_NAMESPACE will be used.
			if !s.autoYes {
				if !s.askQuestion(fmt.Sprintf("Is %q the correct Vault namespace to use? [Y/n]", s.vClient.Namespace())) {
					s.Ui.Warn(`
Please set the VAULT_NAMESPACE environment variable to the Vault namespace to use and re-run the command.`)
					return 0
				}
			}
		} else {
			// Update client with default namespace if VAULT_NAMESPACE is not
			// defined.
			s.vClient.SetNamespace(vaultNamespace)
		}
	}

	if s.cleanup {
		return s.removeConfiguredComponents()
	}

	/*
		Namespace creation
	*/
	// 	if s.vaultEnt {
	// 		ns := s.vClient.Namespace()
	// 		namespaceMsg := `
	// Since you're running Vault Enterprise, we will additionally create
	// a namespace %q and bind the auth methods to that namespace.
	// 	`
	// 		if s.namespaceExists(ns) {
	// 			s.Ui.Info(fmt.Sprintf("[✔] Namespace %q already exists.", ns))
	// 		} else {
	// 			s.Ui.Output(fmt.Sprintf(namespaceMsg, ns))
	//
	// 			var createNamespace bool
	// 			if !s.autoYes {
	// 				createNamespace = s.askQuestion(
	// 					fmt.Sprintf("Create the namespace %q in your Vault cluster? [Y/n]", ns))
	// 				if !createNamespace {
	// 					s.handleNo()
	// 				}
	// 			} else {
	// 				createNamespace = true
	// 			}
	//
	// 			if createNamespace {
	// 				err = s.createNamespace(ns)
	// 				if err != nil {
	// 					s.Ui.Error(err.Error())
	// 					return 1
	// 				}
	// 			}
	// 		}
	// 	}

	/*
		JWT Auth
	*/
	s.Ui.Output(`
Nomad workloads authenticate using JSON Web Tokens. For that authentication to
work, your Vault cluster needs to have JWT credential backend enabled.
`)
	if s.jwtEnabled() {
		s.Ui.Info("[✔] JWT Auth already enabled.")
	} else {
		var enableJWT bool
		if !s.autoYes {
			enableJWT = s.askQuestion("Enable JWT credential backend in your Vault cluster? [Y/n]")
			if !enableJWT {
				s.handleNo()
			}
		} else {
			enableJWT = true
		}

		if enableJWT {
			err := s.enableJWT()
			if err != nil {
				s.Ui.Error(err.Error())
				return 1
			}
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
		s.Ui.Output(fmt.Sprintf("These are the rules for the policy %q that we will create:\n", vaultPolicyName))
		s.Ui.Output(string(vaultPolicyBody))

		var createPolicy bool
		if !s.autoYes {
			createPolicy = s.askQuestion(
				"Create the above policy in your Vault cluster? [Y/n]",
			)
			if !createPolicy {
				s.handleNo()
			}

		} else {
			createPolicy = true
		}

		if createPolicy {
			err = s.createPolicy()
			if err != nil {
				s.Ui.Error(err.Error())
				return 1
			}
		}
	}

	if s.roleExists() {
		s.Ui.Info(fmt.Sprintf("[✔] Role %q already exists.", vaultRoleTasks))
	} else {
		s.Ui.Output(fmt.Sprintf(`
We will now create an ACL role called %q associated with the policy above.
`,
			vaultRoleTasks))

		var createRole bool
		if !s.autoYes {
			createRole = s.askQuestion(
				"Create role in your Vault cluster? [Y/n]",
			)
			if !createRole {
				s.handleNo()
			}
		} else {
			createRole = true
		}

		if createRole {
			err = s.createRoleForTasks()
			if err != nil {
				s.Ui.Error(err.Error())
				return 1
			}
		}
	}

	/*
		Auth method creation
	*/
	authMethodMsg := `
Nomad needs two JWT auth methods: one for Consul services, and one for tasks.
The method for services will be called %q and the method for
tasks %q.
	`
	s.Ui.Output(fmt.Sprintf(authMethodMsg, consulAuthMethodServicesName, consulAuthMethodTasksName))

	// if s.authMethodExists(consulAuthMethodServicesName) {
	// 	s.Ui.Info(fmt.Sprintf("[✔] Auth method %q already exists.", consulAuthMethodServicesName))
	// } else {

	// 	authMethodMsg := "This is the %q method configuration:\n"
	// 	s.Ui.Output(fmt.Sprintf(authMethodMsg, consulAuthMethodServicesName))

	// 	servicesAuthMethod, err := s.renderAuthMethod(consulAuthMethodServicesName, consulAuthMethodServicesDesc)
	// 	if err != nil {
	// 		s.Ui.Error(err.Error())
	// 		return 1
	// 	}
	// 	jsConf, _ := json.MarshalIndent(servicesAuthMethod, "", "    ")

	// 	s.Ui.Output(string(jsConf))

	// 	var createServicesAuthMethod bool
	// 	if !s.autoYes {
	// 		createServicesAuthMethod = s.askQuestion(
	// 			fmt.Sprintf("Create %q auth method in your Consul cluster? [Y/n]", consulAuthMethodServicesName))
	// 		if !createServicesAuthMethod {
	// 			s.handleNo()
	// 		}
	// 	} else {
	// 		createServicesAuthMethod = true
	// 	}

	// 	if createServicesAuthMethod {
	// 		err = s.createAuthMethod(servicesAuthMethod)
	// 		if err != nil {
	// 			s.Ui.Error(err.Error())
	// 			return 1
	// 		}
	// 	}
	// }

	// if s.authMethodExists(consulAuthMethodTasksName) {
	// 	s.Ui.Info(fmt.Sprintf("[✔] Auth method %q already exists.", consulAuthMethodTasksName))
	// } else {

	// 	authMethodMsg := `
	// This is the %q method configuration:
	// `
	// 	s.Ui.Output(fmt.Sprintf(authMethodMsg, consulAuthMethodTasksName))

	// 	tasksAuthMethod, err := s.renderAuthMethod(consulAuthMethodTasksName, consulAuthMethodTaskDesc)
	// 	if err != nil {
	// 		s.Ui.Error(err.Error())
	// 		return 1
	// 	}
	// 	jsConf, _ := json.MarshalIndent(tasksAuthMethod, "", "    ")

	// 	s.Ui.Output(string(jsConf))

	// 	var createTasksAuthMethod bool
	// 	if !s.autoYes {
	// 		createTasksAuthMethod = s.askQuestion(
	// 			fmt.Sprintf("Create %q auth method in your Consul cluster? [Y/n]", consulAuthMethodTasksName))
	// 		if !createTasksAuthMethod {
	// 			s.handleNo()
	// 		}
	// 	} else {
	// 		createTasksAuthMethod = true
	// 	}

	// 	if createTasksAuthMethod {
	// 		err = s.createAuthMethod(tasksAuthMethod)
	// 		if err != nil {
	// 			s.Ui.Error(err.Error())
	// 			return 1
	// 		}
	// 	}
	// }

	s.Ui.Output(`
Congratulations, your Vault cluster is now setup and ready to accept Nomad
workloads with Workload Identity!

You need to adjust your Nomad client configuration in the following way:

vault {
  enabled = true
  address = "<Vault address>"

  # Vault Enterprise only.
  # namespace = "<namespace>"

  jwt_auth_backend_path = "jwt/"
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

func (s *SetupVaultCommand) jwtEnabled() bool {
	auth, _ := s.vClient.Sys().ListAuth()
	_, ok := auth["jwt/"]
	return ok
}

func (s *SetupVaultCommand) enableJWT() error {
	err := s.vClient.Sys().EnableAuthWithOptions("jwt", &api.MountInput{Type: "jwt"})
	if err != nil {
		return fmt.Errorf("[✘] Could not enable JWT credential backend: %w", err)
	}
	s.Ui.Info("[✔] Enabled JWT credential backend.")
	return nil
}

func (s *SetupVaultCommand) roleExists() bool {
	existingRoles, err := s.vLogical.List("/auth/jwt/role")
	if err != nil {
		panic(err)
	}
	if existingRoles != nil {
		return slices.Contains(existingRoles.Data["keys"].([]interface{}), vaultRoleTasks)
	}
	return false
}

func (s *SetupVaultCommand) createRoleForTasks() error {
	role := map[string]any{}
	err := json.Unmarshal(vaultRoleBody, &role)
	if err != nil {
		return fmt.Errorf("[✘] Default auth config text could not be deserialized: %w", err)
	}

	role["bound_audiences"] = vaultAud
	role["token_policies"] = []string{vaultPolicyName}

	buf, err := json.Marshal(role)
	if err != nil {
		return fmt.Errorf("[✘] Role could not be interpolated with args: %w", err)
	}

	path := "auth/jwt/role/" + vaultRoleTasks

	_, err = s.vLogical.WriteBytes(path, buf)
	if err != nil {
		return fmt.Errorf("[✘] Could not create Vault role: %w", err)
	}

	s.Ui.Info(fmt.Sprintf("[✔] Created role %q.", vaultRoleTasks))
	return nil
}

func (s *SetupVaultCommand) policyExists() bool {
	existingPolicies, _ := s.vClient.Sys().ListPolicies()
	return slices.Contains(existingPolicies, vaultPolicyName)
}

func (s *SetupVaultCommand) createPolicy() error {
	secret, err := s.vLogical.Read("sys/auth/jwt")
	if err != nil {
		return fmt.Errorf("[✘] Could not retrieve JWT accessor: %w", err)
	}
	accessor := secret.Data["accessor"].(string)

	policyTextStr := string(vaultPolicyBody)
	policyTextStr = strings.ReplaceAll(policyTextStr, "auth_jwt_X", accessor)
	encoded := base64.StdEncoding.EncodeToString([]byte(policyTextStr))

	finalText := fmt.Sprintf(`{"policy": "%s"}`, encoded)
	buf := []byte(finalText)

	path := "sys/policies/acl/" + vaultPolicyName
	_, err = s.vLogical.WriteBytes(path, buf)
	if err != nil {
		return fmt.Errorf("[✘] Could not create Vault policy: %w", err)
	}

	s.Ui.Info(fmt.Sprintf("[✔] Created policy %q.", vaultPolicyName))

	return nil
}

// func (s *SetupVaultCommand) authMethodExists(authMethodName string) bool {
// 	existingMethods, _, _ := s.client.ACL().AuthMethodList(nil)
// 	return slices.ContainsFunc(
// 		existingMethods,
// 		func(m *api.ACLAuthMethodListEntry) bool { return m.Name == authMethodName })
// }
//
// func (s *SetupVaultCommand) renderAuthMethod(name string, desc string) (*api.ACLAuthMethod, error) {
// 	authConfig := map[string]any{}
// 	err := json.Unmarshal(consulAuthConfigBody, &authConfig)
// 	if err != nil {
// 		return nil, fmt.Errorf("default auth config text could not be deserialized: %v", err)
// 	}
//
// 	authConfig["JWKSURL"] = s.jwksURL
// 	authConfig["BoundAudiences"] = []string{consulAud}
// 	authConfig["JWTSupportedAlgs"] = []string{"RS256"}
//
// 	method := &api.ACLAuthMethod{
// 		Name:          name,
// 		Type:          "jwt",
// 		DisplayName:   name,
// 		Description:   desc,
// 		TokenLocality: "local",
// 		Config:        authConfig,
// 	}
// 	if s.vaultEnt && (s.clientCfg.Namespace == "" || s.clientCfg.Namespace == "default") {
// 		method.NamespaceRules = []*api.ACLAuthMethodNamespaceRule{{
// 			Selector:      "",
// 			BindNamespace: "${value.nomad_namespace}",
// 		}}
// 	}
//
// 	return method, nil
// }
//
// func (s *SetupVaultCommand) createAuthMethod(authMethod *api.ACLAuthMethod) error {
// 	_, _, err := s.client.ACL().AuthMethodCreate(authMethod, nil)
// 	if err != nil {
// 		if strings.Contains(err.Error(), "error checking JWKSURL") {
// 			s.Ui.Error(fmt.Sprintf(
// 				"error: Nomad JWKS endpoint unreachable, verify that Nomad is running and that the JWKS URL %s is reachable by Consul", s.jwksURL,
// 			))
// 			os.Exit(1)
// 		}
// 		return fmt.Errorf("[✘] Could not create Consul auth method: %w", err)
// 	}
//
// 	s.Ui.Info(fmt.Sprintf("[✔] Created auth method %q.", authMethod.Name))
// 	return nil
// }
//
// func (s *SetupVaultCommand) namespaceExists(ns string) bool {
// 	nsClient := s.client.Namespaces()
//
// 	existingNamespaces, _, _ := nsClient.List(nil)
// 	return slices.ContainsFunc(
// 		existingNamespaces,
// 		func(n *api.Namespace) bool { return n.Name == ns })
// }
//
// func (s *SetupVaultCommand) createNamespace(ns string) error {
// 	nsClient := s.client.Namespaces()
// 	namespace := &api.Namespace{
// 		Name: ns,
// 		Meta: map[string]string{
// 			"created-by": "nomad-setup",
// 		},
// 	}
//
// 	_, _, err := nsClient.Create(namespace, nil)
// 	if err != nil {
// 		return fmt.Errorf("[✘] Could not write namespace %q: %w", ns, err)
// 	}
// 	s.Ui.Info(fmt.Sprintf("[✔] Created namespace %q.", ns))
// 	return nil
// }
//

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
 By answering "no" to any of these questions, you are risking an incorrect Consul
 cluster configuration. Nomad workloads with Workload Identity will not be able
 to authenticate unless you create missing configuration yourself.
 `)

	exitCode := 0
	if s.autoYes || s.askQuestion("Remove everything this command creates? [Y/n]") {
		exitCode = s.removeConfiguredComponents()
	}

	s.Ui.Output(s.Colorize().Color(`
 Consul cluster has [bold][underline]not[reset] been configured for authenticating Nomad tasks and
 services using workload identitiies.

 Run the command again to finish the configuration process.`))
	os.Exit(exitCode)
}

func (s *SetupVaultCommand) removeConfiguredComponents() int {
	exitCode := 0
	componentsToRemove := map[string][]string{}

	// 	authMethods := []string{}
	// 	for _, authMethod := range []string{consulAuthMethodServicesName, consulAuthMethodTasksName} {
	// 		if s.authMethodExists(authMethod) {
	// 			authMethods = append(authMethods, authMethod)
	// 		}
	// 	}
	// 	if len(authMethods) > 0 {
	// 		componentsToRemove["Auth methods"] = authMethods
	// 	}
	//
	if s.policyExists() {
		componentsToRemove["Policy"] = []string{consulPolicyName}
	}

	if s.roleExists() {
		componentsToRemove["Role"] = []string{consulRoleTasks}
	}
	if s.jwtEnabled() {
		componentsToRemove["Credential backend"] = []string{"JWT"}
	}
	//
	// 	if s.vaultEnt {
	// 		ns, _, err := s.client.Namespaces().Read(s.clientCfg.Namespace, nil)
	// 		if err != nil {
	// 			s.Ui.Error(fmt.Sprintf("[✘] Failed to fetch namespace %q: %v", ns.Name, err.Error()))
	// 			exitCode = 1
	// 		} else if ns != nil && ns.Meta["created-by"] == "nomad-setup" {
	// 			componentsToRemove["Namespace"] = []string{ns.Name}
	// 		}
	// 	}
	if exitCode != 0 {
		return exitCode
	}

	q := `The following items will be deleted:
%s`
	if len(componentsToRemove) == 0 {
		s.Ui.Output("Nothing to delete.")
		return 0
	}

	if !s.autoYes {
		s.Ui.Warn(fmt.Sprintf(q, printMap(componentsToRemove)))
	}

	if s.autoYes || s.askQuestion("Remove all the items listed above? [Y/n]") {

		for _, policy := range componentsToRemove["Policy"] {
			err := s.vClient.Sys().DeletePolicy(policy)
			if err != nil {
				s.Ui.Error(fmt.Sprintf("[✘] Failed to delete policy %q: %v", policy, err.Error()))
				exitCode = 1
			} else {
				s.Ui.Info(fmt.Sprintf("[✔] Deleted policy %q.", policy))
			}
		}

		for _, role := range componentsToRemove["Role"] {
			_, err := s.vLogical.Delete(fmt.Sprintf("/auth/jwt/role/%s", role))
			if err != nil {
				s.Ui.Error(fmt.Sprintf("[✘] Failed to delete role %q: %v", role, err.Error()))
				exitCode = 1
			} else {
				s.Ui.Info(fmt.Sprintf("[✔] Deleted role %q.", role))
			}
		}
		//
		// 		for _, authMethod := range componentsToRemove["Auth methods"] {
		// 			_, err := s.client.ACL().AuthMethodDelete(authMethod, nil)
		// 			if err != nil {
		// 				s.Ui.Error(fmt.Sprintf("[✘] Failed to delete auth method %q: %v", authMethod, err.Error()))
		// 				exitCode = 1
		// 			} else {
		// 				s.Ui.Info(fmt.Sprintf("[✔] Deleted auth method %q.", authMethod))
		// 			}
		// 		}

		if _, ok := componentsToRemove["Credential backend"]; ok {
			if err := s.vClient.Sys().DisableAuth("jwt"); err != nil {
				s.Ui.Error(fmt.Sprintf("[✘] Failed to disable JWT credential backend: %v", err.Error()))
				exitCode = 1
			} else {
				s.Ui.Info("[✔] Disabled JWT credential backend")
			}
		}

		//
		// 		for _, ns := range componentsToRemove["Namespace"] {
		// 			_, err := s.client.Namespaces().Delete(ns, nil)
		// 			if err != nil {
		// 				s.Ui.Error(fmt.Sprintf("[✘] Failed to delete namespace %q: %v", ns, err.Error()))
		// 				exitCode = 1
		// 			} else {
		// 				s.Ui.Info(fmt.Sprintf("[✔] Deleted namespace %q.", ns))
		// 			}
		// 		}
	}

	return exitCode
}
