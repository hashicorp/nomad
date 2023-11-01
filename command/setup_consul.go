// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/hashicorp/consul/api"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

// Ensure SetupConsulCommand satisfies the cli.Command interface.
var _ cli.Command = &SetupConsulCommand{}

//go:embed asset/consul-wi-default-auth-method-config.json
var consulAuthConfigBody []byte

//go:embed asset/consul-wi-default-policy.hcl
var consulPolicyBody []byte

const (
	consulAuthMethodServicesName = "nomad-services"
	consulAuthMethodServicesDesc = "Login method for Nomad services using workload identities"
	consulAuthMethodTasksName    = "nomad-tasks"
	consulAuthMethodTaskDesc     = "Login method for Nomad tasks using workload identities"
	consulRoleTasks              = "role-nomad-tasks"
	consulPolicyName             = "policy-nomad-tasks"
	consulNamespace              = "nomad-workloads"
	consulAud                    = "consul.io"
)

type SetupConsulCommand struct {
	Meta

	// client is the Consul API client shared by all functions in the command
	// to reuse the same connection.
	client    *api.Client
	clientCfg *api.Config

	jwksURL string

	consulEnt bool
	cleanup   bool
	autoYes   bool
}

// Help satisfies the cli.Command Help function.
func (s *SetupConsulCommand) Help() string {
	helpText := `
Usage: nomad setup consul [options]

  This command sets up Consul for allowing Nomad workloads to authenticate
  themselves using Workload Identity.

  This command requires acl:write permissions for Consul and respects
  CONSUL_HTTP_TOKEN, CONSUL_HTTP_ADDR, and other Consul-related
  environment variables as documented in
  https://developer.hashicorp.com/nomad/docs/runtime/environment#summary.

  WARNING: This command is an experimental feature and may change its behavior
  in future versions of Nomad.

Setup Consul options:

  -jwks-url <url>
    URL of Nomad's JWKS endpoint contacted by Consul to verify JWT
    signatures. Defaults to http://localhost:4646/.well-known/jwks.json.

  -cleanup
    Removes all configuration components this command created from the
    Consul cluster.

  -y
    Automatically answers "yes" to all the questions, making the setup
    non-interactive. Defaults to "false".

`
	return strings.TrimSpace(helpText)
}

func (s *SetupConsulCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(s.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-jwks-url": complete.PredictAnything,
			"-cleanup":  complete.PredictSet("true", "false"),
			"-y":        complete.PredictSet("true", "false"),
		})
}

func (s *SetupConsulCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

// Synopsis satisfies the cli.Command Synopsis function.
func (s *SetupConsulCommand) Synopsis() string { return "Setup a Consul cluster for Nomad integration" }

// Name returns the name of this command.
func (s *SetupConsulCommand) Name() string { return "setup consul" }

// Run satisfies the cli.Command Run function.
func (s *SetupConsulCommand) Run(args []string) int {

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
Nomad workloads to authenticate themselves against Consul ACL using their
respective workload identities.

First we need to connect to Consul.
`)
	}

	s.clientCfg = api.DefaultConfig()
	if !s.autoYes {
		if !s.askQuestion(fmt.Sprintf("Is %q the correct address of your Consul cluster? [Y/n]", s.clientCfg.Address)) {
			s.Ui.Warn(`
Please set the CONSUL_HTTP_ADDR environment variable to your Consul cluster address and re-run the command.`)
			return 0
		}
	}

	// Get the Consul client.
	var err error
	s.client, err = api.NewClient(s.clientCfg)
	if err != nil {
		s.Ui.Error(fmt.Sprintf("Error initializing Consul client: %s", err))
		return 1
	}

	// check if we're connecting to Consul ent
	if _, err := s.client.Operator().LicenseGet(nil); err == nil {
		s.consulEnt = true
	}

	if s.consulEnt {
		if s.clientCfg.Namespace != "" {
			// Confirm CONSUL_NAMESPACE will be used.
			if !s.autoYes {
				if !s.askQuestion(fmt.Sprintf("Is %q the correct Consul namespace to use? [Y/n]", s.clientCfg.Namespace)) {
					s.Ui.Warn(`
Please set the CONSUL_NAMESPACE environment variable to the Consul namespace to use and re-run the command.`)
					return 0
				}
			}
		} else {
			// Update client with default namespace if CONSUL_NAMESPACE is not
			// defined.
			s.clientCfg.Namespace = consulNamespace
			s.client, err = api.NewClient(s.clientCfg)
			if err != nil {
				s.Ui.Error(fmt.Sprintf("Error initializing Consul client with namespace: %s", err))
				return 1
			}
		}

		/*
			Namespace creation
		*/
		ns := s.clientCfg.Namespace
		namespaceMsg := `
Since you're running Consul Enterprise, we will additionally create
a namespace %q and bind the auth methods to that namespace.
`
		if !s.cleanup {
			if s.namespaceExists(s.clientCfg.Namespace) {
				s.Ui.Info(fmt.Sprintf("[✔] Namespace %q already exists.", ns))
			} else {
				s.Ui.Output(fmt.Sprintf(namespaceMsg, ns))

				var createNamespace bool
				if !s.autoYes {
					createNamespace = s.askQuestion(
						fmt.Sprintf("Create the namespace %q in your Consul cluster? [Y/n]", ns))
					if !createNamespace {
						s.handleNo()
					}
				} else {
					createNamespace = true
				}

				if createNamespace {
					err = s.createNamespace(ns)
					if err != nil {
						s.Ui.Error(err.Error())
						return 1
					}
				}
			}
		}
	}

	if s.cleanup {
		s.removeConfiguredComponents()
		return 0
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

	if s.authMethodExists(consulAuthMethodServicesName) {
		s.Ui.Info(fmt.Sprintf("[✔] Auth method %q already exists.", consulAuthMethodServicesName))
	} else {

		authMethodMsg := "This is the %q method configuration:\n"
		s.Ui.Output(fmt.Sprintf(authMethodMsg, consulAuthMethodServicesName))

		servicesAuthMethod, err := s.renderAuthMethod(consulAuthMethodServicesName, consulAuthMethodServicesDesc)
		if err != nil {
			s.Ui.Error(err.Error())
			return 1
		}
		jsConf, _ := json.MarshalIndent(servicesAuthMethod, "", "    ")

		s.Ui.Output(string(jsConf))

		var createServicesAuthMethod bool
		if !s.autoYes {
			createServicesAuthMethod = s.askQuestion(
				fmt.Sprintf("Create %q auth method in your Consul cluster? [Y/n]", consulAuthMethodServicesName))
			if !createServicesAuthMethod {
				s.handleNo()
			}
		} else {
			createServicesAuthMethod = true
		}

		if createServicesAuthMethod {
			err = s.createAuthMethod(servicesAuthMethod)
			if err != nil {
				s.Ui.Error(err.Error())
				return 1
			}
		}
	}

	if s.authMethodExists(consulAuthMethodTasksName) {
		s.Ui.Info(fmt.Sprintf("[✔] Auth method %q already exists.", consulAuthMethodTasksName))
	} else {

		authMethodMsg := `
This is the %q method configuration:
`
		s.Ui.Output(fmt.Sprintf(authMethodMsg, consulAuthMethodTasksName))

		tasksAuthMethod, err := s.renderAuthMethod(consulAuthMethodTasksName, consulAuthMethodTaskDesc)
		if err != nil {
			s.Ui.Error(err.Error())
			return 1
		}
		jsConf, _ := json.MarshalIndent(tasksAuthMethod, "", "    ")

		s.Ui.Output(string(jsConf))

		var createTasksAuthMethod bool
		if !s.autoYes {
			createTasksAuthMethod = s.askQuestion(
				fmt.Sprintf("Create %q auth method in your Consul cluster? [Y/n]", consulAuthMethodTasksName))
			if !createTasksAuthMethod {
				s.handleNo()
			}
		} else {
			createTasksAuthMethod = true
		}

		if createTasksAuthMethod {
			err = s.createAuthMethod(tasksAuthMethod)
			if err != nil {
				s.Ui.Error(err.Error())
				return 1
			}
		}
	}

	/*
		Binding rules creation
	*/

	servicesBindingRule := &api.ACLBindingRule{
		Description: "Binding rule for Nomad services authenticated using a workload identity",
		AuthMethod:  consulAuthMethodServicesName,
		BindType:    "service",
		BindName:    "${value.nomad_service}",
	}

	tasksBindingRule := &api.ACLBindingRule{
		Description: "Binding rule for Nomad tasks authenticated using a workload identity",
		AuthMethod:  consulAuthMethodTasksName,
		BindType:    "role",
		BindName:    "nomad-${value.nomad_namespace}-templates",
	}

	s.Ui.Output(`
Consul uses binding rules to map claims between Nomad's JWTs to Consul service
identities and ACL roles, so we need to create a binding rule for each of the
auth methods above.
`)

	if s.bindingRuleExists(servicesBindingRule) {
		s.Ui.Info(fmt.Sprintf("[✔] Binding rule for auth method %q already exists.", servicesBindingRule.AuthMethod))
	} else {

		s.Ui.Output(fmt.Sprintf("This is the binding rule for the %q auth method:\n", consulAuthMethodServicesName))

		jsServicesBindingRule, _ := json.MarshalIndent(servicesBindingRule, "", "    ")
		s.Ui.Output(string(jsServicesBindingRule))

		var createServicesBindingRule bool
		if !s.autoYes {
			createServicesBindingRule = s.askQuestion(
				"Create this binding rule in your Consul cluster? [Y/n]",
			)
			if !createServicesBindingRule {
				s.handleNo()
			}
		} else {
			createServicesBindingRule = true
		}

		if createServicesBindingRule {
			err = s.createBindingRules(servicesBindingRule)
			if err != nil {
				s.Ui.Error(err.Error())
				return 1
			}
		}
	}

	if s.bindingRuleExists(tasksBindingRule) {
		s.Ui.Info(fmt.Sprintf("[✔] Binding rule for auth method %q already exists.", tasksBindingRule.AuthMethod))
	} else {

		s.Ui.Output(fmt.Sprintf(`
This is the binding rule for the %q auth method:
`, consulAuthMethodTasksName))

		jsTasksBindingRule, _ := json.MarshalIndent(tasksBindingRule, "", "    ")
		s.Ui.Output(string(jsTasksBindingRule))

		var createTasksBindingRule bool
		if !s.autoYes {
			createTasksBindingRule = s.askQuestion(
				"Create this binding rule in your Consul cluster? [Y/n]",
			)
			if !createTasksBindingRule {
				s.handleNo()
			}
		} else {
			createTasksBindingRule = true
		}

		if createTasksBindingRule {
			err = s.createBindingRules(tasksBindingRule)
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
The step above bound Nomad tasks to a Consul ACL role. Now we need to create the
role and the associated ACL policy that defines what tasks are allowed to access
in Consul.
`)

	if s.policyExists() {
		s.Ui.Info(fmt.Sprintf("[✔] Policy %q already exists.", consulPolicyName))
	} else {
		s.Ui.Output(fmt.Sprintf("These are the rules for the policy %q that we will create:\n", consulPolicyName))
		s.Ui.Output(string(consulPolicyBody))

		var createPolicy bool
		if !s.autoYes {
			createPolicy = s.askQuestion(
				"Create the above policy in your Consul cluster? [Y/n]",
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
		s.Ui.Info(fmt.Sprintf("[✔] Role %q already exists.", consulRoleTasks))
	} else {
		s.Ui.Output(fmt.Sprintf(`
And finally, we will create an ACL role called %q associated
with the policy above.
`,
			consulRoleTasks))

		var createRole bool
		if !s.autoYes {
			createRole = s.askQuestion(
				"Create role in your Consul cluster? [Y/n]",
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

	s.Ui.Output(`
Congratulations, your Consul cluster is now setup and ready to accept Nomad
workloads with Workload Identity!

You need to adjust your Nomad client configuration in the following way:

consul {
  enabled = true
  address = "<Consul address>"

  # Nomad agents still need a Consul token in order to register themselves
  # for automated clustering. It is recommended to set the token using the
  # CONSUL_HTTP_TOKEN environment variable instead of writing it in the
  # configuration file.
}

And the configuration of your Nomad servers as follows:

consul {
  enabled = true
  address = "<Consul address>"

  # Nomad agents still need a Consul token in order to register themselves
  # for automated clustering. It is recommended to set the token using the
  # CONSUL_HTTP_TOKEN environment variable instead of writing it in the
  # configuration file.

  service_identity {
    aud = ["consul.io"]
    ttl = "1h"
  }

  task_identity {
    aud = ["consul.io"]
    ttl = "1h"
  }
}`)

	return 0
}

func (s *SetupConsulCommand) authMethodExists(authMethodName string) bool {
	existingMethods, _, _ := s.client.ACL().AuthMethodList(nil)
	return slices.ContainsFunc(
		existingMethods,
		func(m *api.ACLAuthMethodListEntry) bool { return m.Name == authMethodName })
}

func (s *SetupConsulCommand) renderAuthMethod(name string, desc string) (*api.ACLAuthMethod, error) {
	authConfig := map[string]any{}
	err := json.Unmarshal(consulAuthConfigBody, &authConfig)
	if err != nil {
		return nil, fmt.Errorf("default auth config text could not be deserialized: %v", err)
	}

	authConfig["JWKSURL"] = s.jwksURL
	authConfig["BoundAudiences"] = []string{consulAud}
	authConfig["JWTSupportedAlgs"] = []string{"RS256"}

	method := &api.ACLAuthMethod{
		Name:          name,
		Type:          "jwt",
		DisplayName:   name,
		Description:   desc,
		TokenLocality: "local",
		Config:        authConfig,
	}
	if s.consulEnt && (s.clientCfg.Namespace == "" || s.clientCfg.Namespace == "default") {
		method.NamespaceRules = []*api.ACLAuthMethodNamespaceRule{{
			Selector:      "",
			BindNamespace: "${value.nomad_namespace}",
		}}
	}

	return method, nil
}

func (s *SetupConsulCommand) createAuthMethod(authMethod *api.ACLAuthMethod) error {
	_, _, err := s.client.ACL().AuthMethodCreate(authMethod, nil)
	if err != nil {
		if strings.Contains(err.Error(), "error checking JWKSURL") {
			s.Ui.Error(fmt.Sprintf(
				"error: Nomad JWKS endpoint unreachable, verify that Nomad is running and that the JWKS URL %s is reachable by Consul", s.jwksURL,
			))
			os.Exit(1)
		}
		return fmt.Errorf("[✘] Could not create Consul auth method: %w", err)
	}

	s.Ui.Info(fmt.Sprintf("[✔] Created auth method %q.", authMethod.Name))
	return nil
}

func (s *SetupConsulCommand) namespaceExists(ns string) bool {
	nsClient := s.client.Namespaces()

	existingNamespaces, _, _ := nsClient.List(nil)
	return slices.ContainsFunc(
		existingNamespaces,
		func(n *api.Namespace) bool { return n.Name == ns })
}

func (s *SetupConsulCommand) createNamespace(ns string) error {
	nsClient := s.client.Namespaces()
	namespace := &api.Namespace{
		Name: ns,
		Meta: map[string]string{
			"created-by": "nomad-setup",
		},
	}

	_, _, err := nsClient.Create(namespace, nil)
	if err != nil {
		return fmt.Errorf("[✘] Could not write namespace %q: %w", ns, err)
	}
	s.Ui.Info(fmt.Sprintf("[✔] Created namespace %q.", ns))
	return nil
}

func (s *SetupConsulCommand) bindingRuleExists(rule *api.ACLBindingRule) bool {
	existingRules, _, _ := s.client.ACL().BindingRuleList("", nil)
	return slices.ContainsFunc(
		existingRules,
		func(r *api.ACLBindingRule) bool { return r.AuthMethod == rule.AuthMethod })
}

func (s *SetupConsulCommand) createBindingRules(rule *api.ACLBindingRule) error {
	_, _, err := s.client.ACL().BindingRuleCreate(rule, nil)
	if err != nil {
		return fmt.Errorf("[✘] Could not create Consul binding rule: %w", err)
	}

	s.Ui.Info(fmt.Sprintf("[✔] Created binding rule for auth method %q.", rule.AuthMethod))

	return nil
}

func (s *SetupConsulCommand) roleExists() bool {
	existingRoles, _, _ := s.client.ACL().RoleList(nil)
	return slices.ContainsFunc(
		existingRoles,
		func(r *api.ACLRole) bool { return r.Name == consulRoleTasks })
}

func (s *SetupConsulCommand) createRoleForTasks() error {
	_, _, err := s.client.ACL().RoleCreate(&api.ACLRole{
		Name:        consulRoleTasks,
		Description: "Role for Nomad tasks using workload identities",
		Policies:    []*api.ACLLink{{Name: consulPolicyName}},
	}, nil)
	if err != nil {
		return fmt.Errorf("[✘] Could not create Consul role: %w", err)
	}

	s.Ui.Info(fmt.Sprintf("[✔] Created role %q.", consulRoleTasks))
	return nil
}

func (s *SetupConsulCommand) policyExists() bool {
	existingPolicies, _, _ := s.client.ACL().PolicyList(nil)
	return slices.ContainsFunc(
		existingPolicies,
		func(p *api.ACLPolicyListEntry) bool { return p.Name == consulPolicyName })
}

func (s *SetupConsulCommand) createPolicy() error {
	_, _, err := s.client.ACL().PolicyCreate(&api.ACLPolicy{
		Name:  consulPolicyName,
		Rules: string(consulPolicyBody),
	}, nil)
	if err != nil {
		return fmt.Errorf("[✘] Could not create Consul policy: %w", err)
	}

	s.Ui.Info(fmt.Sprintf("[✔] Created policy %q.", consulPolicyName))

	return nil
}

// askQuestion asks question to user until they provide a valid response.
func (s *SetupConsulCommand) askQuestion(question string) bool {
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

func (s *SetupConsulCommand) handleNo() {
	s.Ui.Warn(`
By answering "no" to any of these questions, you are risking an incorrect Consul
cluster configuration. Nomad workloads with Workload Identity will not be able
to authenticate unless you create missing configuration yourself.
`)

	if s.autoYes || s.askQuestion("Remove everything this command creates? [Y/n]") {
		s.removeConfiguredComponents()
	}

	s.Ui.Output(s.Colorize().Color(`
Consul cluster has [bold][underline]not[reset] been configured for authenticating Nomad tasks and
services using workload identitiies.

Run the command again to finish the configuration process.`))
	os.Exit(0)
}

func (s *SetupConsulCommand) removeConfiguredComponents() {
	componentsToRemove := map[string][]string{}

	authMethods := []string{}
	for _, authMethod := range []string{consulAuthMethodServicesName, consulAuthMethodTasksName} {
		if s.authMethodExists(authMethod) {
			authMethods = append(authMethods, authMethod)
		}
	}
	if len(authMethods) > 0 {
		componentsToRemove["Auth methods"] = authMethods
	}

	serviceMethodRules, _, err := s.client.ACL().BindingRuleList(consulAuthMethodServicesName, nil)
	if err != nil {
		s.Ui.Error(fmt.Sprintf("[✘] Failed to fetch binding rules for method: %q", consulAuthMethodServicesName))
	}
	taskMethodRules, _, err := s.client.ACL().BindingRuleList(consulAuthMethodTasksName, nil)
	if err != nil {
		s.Ui.Error(fmt.Sprintf("[✘] Failed to fetch binding rules for method: %q", consulAuthMethodTasksName))
	}

	ruleIDs := []string{}
	for _, b := range serviceMethodRules {
		ruleIDs = append(ruleIDs, b.ID)
	}
	for _, b := range taskMethodRules {
		ruleIDs = append(ruleIDs, b.ID)
	}
	if len(ruleIDs) > 0 {
		componentsToRemove["Binding rules"] = ruleIDs
	}

	if s.policyExists() {
		componentsToRemove["Policy"] = []string{consulPolicyName}
	}

	if s.roleExists() {
		componentsToRemove["Role"] = []string{consulRoleTasks}
	}

	if s.consulEnt && s.namespaceExists(s.clientCfg.Namespace) {
		componentsToRemove["Namespace"] = []string{s.clientCfg.Namespace}
	}

	q := `The following items will be deleted:
%s`
	if len(componentsToRemove) == 0 {
		s.Ui.Output("Nothing to delete.")
		return
	}

	if !s.autoYes {
		s.Ui.Warn(fmt.Sprintf(q, printMap(componentsToRemove)))
	}

	if s.autoYes || s.askQuestion("Remove all the items listed above? [Y/n]") {

		for _, policy := range componentsToRemove["Policy"] {
			p, _, err := s.client.ACL().PolicyReadByName(policy, nil)
			if err != nil {
				s.Ui.Error(fmt.Sprintf("[✘] Failed to fetch policy %q: %v", policy, err.Error()))
			} else if p != nil {
				_, err := s.client.ACL().PolicyDelete(p.ID, nil)
				if err != nil {
					s.Ui.Error(fmt.Sprintf("[✘] Failed to delete policy %q: %v", policy, err.Error()))
				} else {
					s.Ui.Info(fmt.Sprintf("[✔] Deleted policy %q.", p.ID))
				}
			}
		}

		for _, role := range componentsToRemove["Role"] {
			r, _, err := s.client.ACL().RoleReadByName(role, nil)
			if err != nil {
				s.Ui.Error(fmt.Sprintf("[✘] Failed to fetch role %q: %v", role, err.Error()))
			} else if r != nil {
				_, err := s.client.ACL().RoleDelete(r.ID, nil)
				if err != nil {
					s.Ui.Error(fmt.Sprintf("[✘] Failed to delete role %q: %v", r.ID, err.Error()))
				} else {
					s.Ui.Info(fmt.Sprintf("[✔] Deleted role %q.", role))
				}
			}
		}

		for _, b := range append(serviceMethodRules, taskMethodRules...) {
			_, err := s.client.ACL().BindingRuleDelete(b.ID, nil)
			if err != nil {
				s.Ui.Error(fmt.Sprintf("[✘] Failed to delete binding rule %q: %v", b.ID, err.Error()))
			} else {
				s.Ui.Info(fmt.Sprintf("[✔] Deleted binding rule %q.", b.ID))
			}
		}

		for _, authMethod := range componentsToRemove["Auth methods"] {
			_, err := s.client.ACL().AuthMethodDelete(authMethod, nil)
			if err != nil {
				s.Ui.Error(fmt.Sprintf("[✘] Failed to delete auth method %q: %v", authMethod, err.Error()))
			} else {
				s.Ui.Info(fmt.Sprintf("[✔] Deleted auth method %q.", authMethod))
			}
		}

		for _, namespace := range componentsToRemove["Namespace"] {
			ns, _, err := s.client.Namespaces().Read(namespace, nil)
			if err != nil {
				s.Ui.Error(fmt.Sprintf("[✘] Failed to delete namespace %q: %v", namespace, err.Error()))
			}

			if ns.Meta["created-by"] == "nomad-setup" {
				_, err := s.client.Namespaces().Delete(ns.Name, nil)
				if err != nil {
					s.Ui.Error(fmt.Sprintf("[✘] Failed to delete namespace %q: %v", ns.Name, err.Error()))
				} else {
					s.Ui.Info(fmt.Sprintf("[✔] Deleted namespace %q.", ns.Name))
				}
			}
		}
	}
}

func printMap(m map[string][]string) string {
	var output string

	for k, v := range m {
		output += fmt.Sprintf("  * %s: %s\n", k, strings.Join(v, ", "))
	}

	return output
}
