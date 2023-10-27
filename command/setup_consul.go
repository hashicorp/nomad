// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

// Ensure SetupConsulCommand satisfies the cli.Command interface.
var _ cli.Command = &SetupConsulCommand{}

//go:embed asset/consul-wi-default-auth-method-config.json
var authConfigBody []byte

//go:embed asset/consul-wi-default-policy.hcl
var policyBody []byte

const (
	authMethodServices = "nomad-services"
	authMethodTasks    = "nomad-tasks"
	roleTasks          = "role-nomad-tasks"
	policyName         = "policy-nomad-tasks"
	defaultTTL         = 8 * time.Hour
	aud                = "consul.io"
)

type SetupConsulCommand struct {
	Meta

	// client is the Consul API client shared by all functions in the command to
	// reuse the same connection.
	client *api.Client

	jwksURL string

	// if set, answers "Yes" to all the interactive questions
	autoYes bool
}

// Help satisfies the cli.Command Help function.
func (s *SetupConsulCommand) Help() string {
	helpText := `
Usage: nomad setup consul [options]

  This command sets up Consul for allowing Nomad workloads to authenticate
  themselves using Workload Identity.

  Setup Consul for Nomad:

	$ nomad setup consul -y -jwks="http://nomad.example/.well-known/jwks.json"
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
func (s *SetupConsulCommand) Synopsis() string { return "Interact with setup helpers" }

// Name returns the name of this command.
func (s *SetupConsulCommand) Name() string { return "setup" }

// Run satisfies the cli.Command Run function.
func (s *SetupConsulCommand) Run(args []string) int {

	flags := s.Meta.FlagSet(s.Name(), FlagSetClient)
	flags.Usage = func() { s.Ui.Output(s.Help()) }
	flags.BoolVar(&s.autoYes, "y", false, "")
	flags.StringVar(&s.jwksURL, "jwks", "http://localhost:4646/.well-known/jwks.json", "")
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

	// Get the Consul client.
	cfg := api.DefaultConfig()
	s.client, err = api.NewClient(cfg)
	if err != nil {
		s.Ui.Error(fmt.Sprintf("Error initializing Consul client: %s", err))
		return 1
	}

	s.Ui.Output(`
This command will walk you through configuring all the components required for 
Nomad workloads to authenticate themselves against Consul ACL using their 
respective Workload Identities. 

First we need to create a JWT auth method for Nomad services. Here is the auth
method configuration we will create:
`)

	authMethodConf, err := s.renderAuthMethodConf(authMethodServices)
	if err != nil {
		s.Ui.Error(err.Error())
		return 1
	}

	jsConf, _ := json.MarshalIndent(authMethodConf, "", "\t")
	s.Ui.Output(string(jsConf))

	var createAuthMethod bool
	if !s.autoYes {
		createAuthMethod = s.askQuestion(
			fmt.Sprintf(
				"Should we create the %s auth method in your Consul cluster? [Y/n]",
				authMethodServices,
			))
	} else {
		createAuthMethod = true
	}

	if createAuthMethod {
		err = s.createAuthMethod(authMethodServices, authMethodConf)
		if err != nil {
			s.Ui.Error(err.Error())
			return 1
		}
	}

	s.Ui.Output(`
In order to map claims between Nomad's JWTs and Consul ACL, we need to create
the following binding rule:
{
	"Description": "binding rule for Nomad workload identities (WI)",
	"AuthMethod": "nomad-services",
	"BindType": "service",
	"BindName": "${value.nomad_service}"
}
`)

	var createBindingRule bool
	if !s.autoYes {
		createBindingRule = s.askQuestion(
			"Should we create the above binding rule in your Consul cluster? [Y/n]",
		)
	} else {
		createBindingRule = true
	}

	if createBindingRule {
		err = s.createBindingRules(&api.ACLBindingRule{
			Description: "binding rule for Nomad workload identities (WI)",
			AuthMethod:  authMethodServices,
			BindType:    "service",
			BindName:    "${value.nomad_namespace}-${value.nomad_service}",
		})
		if err != nil {
			s.Ui.Error(err.Error())
			return 1
		}
	}

	s.Ui.Output(`
We now need to create a JWT auth method for Nomad tasks. Here is the auth
method configuration we will create:
`)

	authMethodConf, err = s.renderAuthMethodConf(authMethodTasks)
	if err != nil {
		s.Ui.Error(err.Error())
		return 1
	}

	jsConf, _ = json.MarshalIndent(authMethodConf, "", "\t")
	s.Ui.Output(string(jsConf))

	if !s.autoYes {
		createAuthMethod = s.askQuestion(
			fmt.Sprintf(
				"Should we create the %s auth method in your Consul cluster? [Y/n]",
				authMethodTasks,
			))
	} else {
		createAuthMethod = true
	}

	if createAuthMethod {
		err = s.createAuthMethod(authMethodTasks, authMethodConf)
		if err != nil {
			s.Ui.Error(err.Error())
			return 1
		}
	}

	s.Ui.Output(`
In order to map claims between Nomad's JWTs and Consul ACL, we need to create
the following binding rule:
{
	"Description": "binding rule for Nomad templates w/ (WI)",
	"AuthMethod": "nomad-tasks",
	"BindType": "role",
	"BindName": "nomad-${value.nomad_namespace}-templates"
}
`)

	if !s.autoYes {
		createBindingRule = s.askQuestion(
			"Should we create the above binding rule in your Consul cluster? [Y/n]",
		)
	} else {
		createBindingRule = true
	}

	if createBindingRule {
		err = s.createBindingRules(&api.ACLBindingRule{
			Description: "binding rule for Nomad templates w/ (WI)",
			AuthMethod:  authMethodTasks,
			BindType:    "role",
			BindName:    "nomad-${value.nomad_namespace}-templates",
		})
		if err != nil {
			s.Ui.Error(err.Error())
			return 1
		}
	}

	s.Ui.Output(`
Nomad tasks require a Consul ACL policy and Role. Below is the body of the policy
we need to create:
`)
	s.Ui.Output(string(policyBody))

	var createPolicy bool
	if !s.autoYes {
		createPolicy = s.askQuestion(
			"Should we create the above policy in your Consul cluster? [Y/n]",
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

	s.Ui.Output(`
Finally, we need to create a role role-nomad-tasks associated with the policy
above.`)

	var createRole bool
	if !s.autoYes {
		createRole = s.askQuestion(
			"Should we create the above role in your Consul cluster? [Y/n]",
		)
	} else {
		createRole = true
	}

	if createRole {
		err = s.createRoleForTemplate()
		if err != nil {
			s.Ui.Error(err.Error())
			return 1
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

  # Consul mTLS configuration.
  # ssl       = true
  # ca_file   = "/var/ssl/bundle/ca.bundle"
  # cert_file = "/etc/ssl/consul.crt"
  # key_file  = "/etc/ssl/consul.key"

  service_auth_method = "nomad-services"
  task_auth_method    = "nomad-tasks"
}

and the configuration of your Nomad servers as follows:

consul {
  enabled = true
  address = "<Consul address>"

  # Nomad agents still need a Consul token in order to register themselves
  # for automated clustering. It is recommended to set the token using the
  # CONSUL_HTTP_TOKEN environment variable instead of writing it in the
  # configuration file.

  # Consul mTLS configuration.
  # ssl       = true
  # ca_file   = "/var/ssl/bundle/ca.bundle"
  # cert_file = "/etc/ssl/consul.crt"
  # key_file  = "/etc/ssl/consul.key"

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

func (s *SetupConsulCommand) renderAuthMethodConf(authMethodName string) (map[string]any, error) {
	authConfig := map[string]any{}
	err := json.Unmarshal(authConfigBody, &authConfig)
	if err != nil {
		return authConfig, fmt.Errorf("default auth config text could not be deserialized: %v", err)
	}

	authConfig["JWKSURL"] = s.jwksURL
	authConfig["BoundAudiences"] = []string{aud}
	authConfig["JWTSupportedAlgs"] = []string{"RS256"}

	return authConfig, nil
}

func (s *SetupConsulCommand) createAuthMethod(authMethodName string, authMethodConf map[string]any) error {
	_, _, err := s.client.ACL().AuthMethodCreate(&api.ACLAuthMethod{
		Name:          authMethodName,
		Type:          "jwt",
		DisplayName:   authMethodName,
		Description:   "login method for Nomad workload identities (WI)",
		MaxTokenTTL:   defaultTTL,
		TokenLocality: "local",
		Config:        authMethodConf,
		NamespaceRules: []*api.ACLAuthMethodNamespaceRule{{
			Selector:      "",
			BindNamespace: "${value.nomad_namespace}",
		}},
	}, nil)

	if err != nil {
		return fmt.Errorf("[✘] could not create Consul auth method: %w", err)
	}

	s.Ui.Info(fmt.Sprintf("[✔] Created auth method %s", authMethodName))
	return nil
}

func (s *SetupConsulCommand) createBindingRules(rule *api.ACLBindingRule) error {
	_, _, err := s.client.ACL().BindingRuleCreate(rule, nil)
	if err != nil {
		return fmt.Errorf("[✘] could not create Consul binding rule: %w", err)
	}

	s.Ui.Info(fmt.Sprintf("[✔] Created binding rule for auth method %s", rule.AuthMethod))
	return nil
}

func (s *SetupConsulCommand) createRoleForTemplate() error {
	_, _, err := s.client.ACL().RoleCreate(&api.ACLRole{
		Name:        roleTasks,
		Description: "role for Nomad templates w/ workload identities (WI)",
		Policies:    []*api.ACLLink{{Name: policyName}},
	}, nil)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			s.Ui.Warn(fmt.Sprintf("[ ] role %s already exists", roleTasks))
			return nil
		}
		return fmt.Errorf("[✘] could not create Consul role: %w", err)
	}

	s.Ui.Info(fmt.Sprintf("[✔] Created role %s\n", roleTasks))
	return nil
}

func (s *SetupConsulCommand) createPolicy() error {
	_, _, err := s.client.ACL().PolicyCreate(&api.ACLPolicy{
		Name:  policyName,
		Rules: string(policyBody),
	}, nil)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			s.Ui.Warn(fmt.Sprintf("[ ] policy %s already exists", policyName))
		}
		return fmt.Errorf("[✘] could not create Consul policy: %w", err)
	}

	s.Ui.Info(fmt.Sprintf("[✔] Created policy %s", policyName))

	return nil
}

// askQuestion asks question to user until they provide a valid response.
func (s *SetupConsulCommand) askQuestion(question string) bool {
	for {
		answer, err := s.Ui.Ask(s.Colorize().Color(fmt.Sprintf("[?] %s", question)))
		if err != nil {
			if err.Error() != "interrupted" {
				s.Ui.Output(err.Error())
			}
			return false
		}

		switch strings.TrimSpace(strings.ToLower(answer)) {
		case "", "n", "no":
			return false
		case "y", "yes":
			return true
		default:
			s.Ui.Output(fmt.Sprintf("%s is not a valid response, please answer \"yes\" or \"no\".", answer))
			continue
		}
	}
}
