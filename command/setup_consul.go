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
	consulAuthMethodServices = "nomad-services"
	consulAuthMethodTasks    = "nomad-tasks"
	consulRoleTasks          = "role-nomad-tasks"
	consulPolicyName         = "policy-nomad-tasks"
	consulNamespace          = "nomad-prod"
	consulAud                = "consul.io"
)

type SetupConsulCommand struct {
	Meta

	// client is the Consul API client shared by all functions in the command to
	// reuse the same connection.
	client *api.Client

	jwksURL string

	consulEnt bool
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

Setup Consul options:

  -jwks-url
    URL of Nomad's JWKS endpoint contacted by Consul to verify JWT
    signatures. Defaults to http://localhost:4646/.well-known/jwks.json. 

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

	var err error

	s.Ui.Output(`
This command will walk you through configuring all the components required for 
Nomad workloads to authenticate themselves against Consul ACL using their 
respective workload identities. 

First we need to connect to Consul. 
`)

	cfg := api.DefaultConfig()
	if !s.autoYes {
		if !s.askQuestion(fmt.Sprintf("Is %q the correct address of your Consul cluster? [Y/n]", cfg.Address)) {
			s.Ui.Warn(`
Please set the CONSUL_HTTP_ADDR environment variable to your Consul cluster address and re-run the command.`)
			return 0
		}
	}

	// Get the Consul client.
	s.client, err = api.NewClient(cfg)
	if err != nil {
		s.Ui.Error(fmt.Sprintf("Error initializing Consul client: %s", err))
		return 1
	}

	// check if we're connecting to Consul ent
	if _, err := s.client.Operator().LicenseGet(nil); err == nil {
		s.consulEnt = true
	}

	/*
		Auth method creation
	*/

	if s.authMethodExists(consulAuthMethodServices) {
		s.Ui.Info(fmt.Sprintf("[ ] auth method with name %q already exists", consulAuthMethodServices))
	} else {

		authMethodMsg := `
Nomad needs two JWT auth methods: one for Consul services, and one for tasks. 
The method for services will be called %[1]q and the method for 
tasks %[2]q, and they will both be of JWT type.

This is the %[1]q method configuration:
`
		s.Ui.Output(fmt.Sprintf(authMethodMsg, consulAuthMethodServices, consulAuthMethodTasks))

		servicesAuthMethod, err := s.renderAuthMethod(consulAuthMethodServices)
		if err != nil {
			s.Ui.Error(err.Error())
			return 1
		}
		jsConf, _ := json.MarshalIndent(servicesAuthMethod, "", "    ")

		s.Ui.Output(string(jsConf))

		var createServicesAuthMethod bool
		if !s.autoYes {
			createServicesAuthMethod = s.askQuestion(
				fmt.Sprintf("Create %q auth method in your Consul cluster? [Y/n]", consulAuthMethodServices))
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

	if s.authMethodExists(consulAuthMethodTasks) {
		s.Ui.Info(fmt.Sprintf("[ ] auth method with name %q already exists", consulAuthMethodTasks))
	} else {

		authMethodMsg := `
This is the %q method configuration:
`
		s.Ui.Output(fmt.Sprintf(authMethodMsg, consulAuthMethodTasks))

		tasksAuthMethod, err := s.renderAuthMethod(consulAuthMethodTasks)
		if err != nil {
			s.Ui.Error(err.Error())
			return 1
		}
		jsConf, _ := json.MarshalIndent(tasksAuthMethod, "", "    ")

		s.Ui.Output(string(jsConf))

		var createTasksAuthMethod bool
		if !s.autoYes {
			createTasksAuthMethod = s.askQuestion(
				fmt.Sprintf("Create %q auth method in your Consul cluster? [Y/n]", consulAuthMethodTasks))
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

	if s.consulEnt {
		if s.namespaceExists() {
			s.Ui.Info(fmt.Sprintf("[ ] namespace %q already exists", consulNamespace))
		} else {
			namespaceMsg := `
Since you're running Consul Enterprise, we will additionally create
a namespace %q and bind the auth methods to that namespace.
	 `
			s.Ui.Output(fmt.Sprintf(namespaceMsg, consulNamespace))

			var createNamespace bool
			if !s.autoYes {
				createNamespace = s.askQuestion(
					fmt.Sprintf("Create the namespace %q in your Consul cluster? [Y/n]", consulNamespace))
				if !createNamespace {
					s.handleNo()
				}
			} else {
				createNamespace = true
			}

			if createNamespace {
				err = s.createNamespace()
				if err != nil {
					s.Ui.Error(err.Error())
					return 1
				}
			}
		}
	}

	/*
		Binding rules creation
	*/

	servicesBindingRule := &api.ACLBindingRule{
		Description: "Binding rule for Nomad services authenticated using a workload identity",
		AuthMethod:  consulAuthMethodServices,
		BindType:    "service",
		BindName:    "${value.nomad_service}",
	}

	tasksBindingRule := &api.ACLBindingRule{
		Description: "Binding rule for Nomad tasks authenticated using a workload identity",
		AuthMethod:  consulAuthMethodTasks,
		BindType:    "role",
		BindName:    "nomad-${value.nomad_namespace}-templates",
	}

	if s.bindingRuleExists(servicesBindingRule) {
		s.Ui.Info(fmt.Sprintf("[ ] binding rule for auth method %q already exists", servicesBindingRule.AuthMethod))
	} else {

		s.Ui.Output(`
Consul uses binding rules to map claims between Nomad's JWTs and Consul service
identities and ACL roles, so we need to create the following binding rules:
`)
		jsServicesBindingRule, _ := json.MarshalIndent(servicesBindingRule, "", "    ")
		s.Ui.Output(string(jsServicesBindingRule))

		var createServicesBindingRule bool
		if !s.autoYes {
			createServicesBindingRule = s.askQuestion(
				"Create this binding rule in your Consul cluster? [Y/n]",
			)
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
		s.Ui.Info(fmt.Sprintf("[ ] binding rule for auth method %q already exists", tasksBindingRule.AuthMethod))
	} else {

		jsTasksBindingRule, _ := json.MarshalIndent(tasksBindingRule, "", "    ")
		s.Ui.Output(string(jsTasksBindingRule))

		var createTasksBindingRule bool
		if !s.autoYes {
			createTasksBindingRule = s.askQuestion(
				"Create this binding rule in your Consul cluster? [Y/n]",
			)
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

	if s.policyExists() {
		s.Ui.Info(fmt.Sprintf("[ ] policy %q already exists", consulPolicyName))
	} else {
		s.Ui.Output(`
The step above bound Nomad tasks to a Consul ACL role. Now we need to create the
role and the associated ACL policy that defines what tasks are allowed to access
in Consul. Below is the body of the policy we will create:
`)
		s.Ui.Output(string(consulPolicyBody))

		var createPolicy bool
		if !s.autoYes {
			createPolicy = s.askQuestion(
				"Create the above policy in your Consul cluster? [Y/n]",
			)
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
		s.Ui.Info(fmt.Sprintf("[ ] role %q already exists", consulRoleTasks))
	} else {
		s.Ui.Output(fmt.Sprintf(
			"\nAnd finally, we will create an ACL role called %q associated with the policy above.",
			consulRoleTasks))

		var createRole bool
		if !s.autoYes {
			createRole = s.askQuestion(
				"Create the above role in your Consul cluster? [Y/n]",
			)
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

and the configuration of your Nomad servers as follows:

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
}

`)

	return 0
}

func (s *SetupConsulCommand) authMethodExists(authMethodName string) bool {
	existingMethods, _, _ := s.client.ACL().AuthMethodList(nil)
	return slices.ContainsFunc(
		existingMethods,
		func(m *api.ACLAuthMethodListEntry) bool { return m.Name == authMethodName })
}

func (s *SetupConsulCommand) renderAuthMethod(authMethodName string) (*api.ACLAuthMethod, error) {
	authConfig := map[string]any{}
	err := json.Unmarshal(consulAuthConfigBody, &authConfig)
	if err != nil {
		return nil, fmt.Errorf("default auth config text could not be deserialized: %v", err)
	}

	authConfig["JWKSURL"] = s.jwksURL
	authConfig["BoundAudiences"] = []string{consulAud}
	authConfig["JWTSupportedAlgs"] = []string{"RS256"}

	method := &api.ACLAuthMethod{
		Name:          authMethodName,
		Type:          "jwt",
		DisplayName:   authMethodName,
		Description:   "login method for Nomad workload identities (WI)",
		TokenLocality: "local",
		Config:        authConfig,
	}
	if s.consulEnt {
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
		return fmt.Errorf("[✘] could not create Consul auth method: %w", err)
	}

	s.Ui.Info(fmt.Sprintf("[✔] Created auth method %q", authMethod.Name))
	return nil
}

func (s *SetupConsulCommand) namespaceExists() bool {
	nsClient := s.client.Namespaces()

	existingNamespaces, _, _ := nsClient.List(nil)
	return slices.ContainsFunc(
		existingNamespaces,
		func(n *api.Namespace) bool { return n.Name == consulNamespace })
}

func (s *SetupConsulCommand) createNamespace() error {
	nsClient := s.client.Namespaces()
	namespace := &api.Namespace{Name: consulNamespace}

	_, _, err := nsClient.Create(namespace, nil)
	if err != nil {
		return fmt.Errorf("[✘] could not write namespace %q: %w", consulNamespace, err)
	}
	s.Ui.Info(fmt.Sprintf("[✔] Created namespace %q", consulNamespace))
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
		return fmt.Errorf("[✘] could not create Consul binding rule: %w", err)
	}

	s.Ui.Info(fmt.Sprintf("[✔] Created binding rule for auth method %q", rule.AuthMethod))
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
		Description: "role for Nomad templates w/ workload identities (WI)",
		Policies:    []*api.ACLLink{{Name: consulPolicyName}},
	}, nil)
	if err != nil {
		return fmt.Errorf("[✘] could not create Consul role: %w", err)
	}

	s.Ui.Info(fmt.Sprintf("[✔] Created role %q", consulRoleTasks))
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
		return fmt.Errorf("[✘] could not create Consul policy: %w", err)
	}

	s.Ui.Info(fmt.Sprintf("[✔] Created policy %q", consulPolicyName))

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
}
