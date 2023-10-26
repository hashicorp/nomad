// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"golang.org/x/exp/maps"
)

// Ensure SetupConsulCommand satisfies the cli.Command interface.
var _ cli.Command = &SetupConsulCommand{}

//go:embed asset/consul-wi-default-auth-method-config.json
var defaultAuthConfigText []byte

//go:embed asset/consul-wi-default-policy.hcl
var defaultPolicyText []byte

type SetupConsulCommand struct {
	Meta

	// client is the Consul API client shared by all functions in the command to
	// reuse the same connection.
	client *api.Client

	methodNameServices   string
	methodNameTasks      string
	roleTasks            string
	policyTemplatesPaths stringSetFlags
	aud                  stringSetFlags
	jwksURL              string
	ttl                  string

	json bool
	tmpl string
}

// Help satisfies the cli.Command Help function.
func (s *SetupConsulCommand) Help() string {
	helpText := `
Usage: nomad setup consul [options]

  This command sets up Consul for allowing Nomad workloads to authenticate
  themselves using Workload Identity.

  Setup Consul for Nomad:

      $ nomad setup consul -auth-method-name="nomad-workloads"

  Please see the individual subcommand help for detailed usage information.
`
	return strings.TrimSpace(helpText)
}

func (s *SetupConsulCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(s.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-method-name-services":   complete.PredictAnything,
			"-method-name-tasks":      complete.PredictAnything,
			"-role-tasks":             complete.PredictAnything,
			"-policy-templates-paths": complete.PredictFiles("*.hcl"),
			"-aud":                    complete.PredictSet("consul.io"),
			"-jwks-url":               complete.PredictAnything,
			"-json":                   complete.PredictNothing,
			"-t":                      complete.PredictAnything,
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
	flags.StringVar(&s.methodNameServices, "method-name-services", "nomad-workloads", "")
	flags.StringVar(&s.methodNameTasks, "method-name-tasks", "nomad-tasks", "")
	flags.StringVar(&s.roleTasks, "role-tasks", "", "")
	flags.Var(&s.policyTemplatesPaths, "template-policy", "Path to a policy file used for the template role (accepts multiple)")
	flags.Var(&s.aud, "aud", "consul.io")
	flags.StringVar(&s.jwksURL, "jwks-url", "http://localhost:4646/.well-known/jwks.json", "")
	flags.StringVar(&s.ttl, "ttl", "8h", "Maximum token TTL")
	flags.BoolVar(&s.json, "json", false, "")
	flags.StringVar(&s.tmpl, "t", "", "")
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
		os.Exit(1)
	}

	err = s.createAuthMethod(s.methodNameServices)
	if err != nil {
		s.Ui.Error(err.Error())
		os.Exit(1)
	}

	err = s.createBindingRules(&api.ACLBindingRule{
		Description: "binding rule for Nomad workload identities (WI)",
		AuthMethod:  s.methodNameServices,
		BindType:    "service",
		BindName:    "${value.nomad_namespace}-${value.nomad_service}",
		Namespace:   "", // TODO
		Partition:   "", // TOOD
	})
	if err != nil {
		s.Ui.Error(err.Error())
		os.Exit(1)
	}

	err = s.createAuthMethod(s.methodNameTasks)
	if err != nil {
		s.Ui.Error(err.Error())
		os.Exit(1)
	}

	err = s.createBindingRules(&api.ACLBindingRule{
		Description: "binding rule for Nomad templates w/ (WI)",
		AuthMethod:  s.methodNameTasks,
		BindType:    "role",
		BindName:    "nomad-${value.nomad_namespace}-templates",
	})
	if err != nil {
		s.Ui.Error(err.Error())
		os.Exit(1)
	}

	policies, err := s.readPolicies(s.policyTemplatesPaths.Values())
	if err != nil {
		s.Ui.Error(err.Error())
		os.Exit(1)
	}

	err = s.createPolicies(policies)
	if err != nil {
		s.Ui.Error(err.Error())
		os.Exit(1)
	}

	err = s.createRoleForTemplate(maps.Keys(policies))
	if err != nil {
		s.Ui.Error(err.Error())
		os.Exit(1)
	}

	return 0
}

func (s *SetupConsulCommand) createAuthMethod(authMethodName string) error {

	authConfig := map[string]any{}
	err := json.Unmarshal(defaultAuthConfigText, &authConfig)
	if err != nil {
		panic("default auth config text could not be deserialized")
	}

	ttlDur, err := time.ParseDuration(s.ttl)
	if err != nil {
		return fmt.Errorf("could not parse ttl %q: %w", s.ttl, err)
	}

	authConfig["JWKSURL"] = s.jwksURL
	authConfig["BoundAudiences"] = s.aud
	authConfig["JWTSupportedAlgs"] = []string{"RS256"}

	_, _, err = s.client.ACL().AuthMethodCreate(&api.ACLAuthMethod{
		Name:          authMethodName,
		Type:          "jwt",
		DisplayName:   authMethodName,
		Description:   "login method for Nomad workload identities (WI)",
		MaxTokenTTL:   ttlDur,
		TokenLocality: "local",
		Config:        authConfig,
		NamespaceRules: []*api.ACLAuthMethodNamespaceRule{{
			Selector:      "",
			BindNamespace: "${value.nomad_namespace}",
		}},
	}, nil)

	if err != nil {
		return fmt.Errorf("could not create Consul auth method: %w", err)
	}

	s.Ui.Info(fmt.Sprintf("[✔] Created auth method %s\n", authMethodName))
	return nil
}

func (s *SetupConsulCommand) createBindingRules(rule *api.ACLBindingRule) error {
	_, _, err := s.client.ACL().BindingRuleCreate(rule, nil)
	if err != nil {
		return fmt.Errorf("could not create Consul binding rule: %w", err)
	}

	s.Ui.Info(fmt.Sprintf("[✔] Created binding rule for auth method %s\n", rule.AuthMethod))
	return nil
}

func (s *SetupConsulCommand) readPolicies(policyPaths []string) (map[string][]byte, error) {
	if len(policyPaths) == 0 {
		return map[string][]byte{"nomad-workloads": defaultPolicyText}, nil
	}

	policies := make(map[string][]byte, len(policyPaths))
	for _, policyPath := range policyPaths {
		policyText, err := os.ReadFile(policyPath)
		if err != nil {
			return nil, fmt.Errorf("could not read policy file %q: %w", policyPath, err)
		}

		policyName := policyPathToName(policyPath)
		policies[policyName] = policyText
	}

	return policies, nil
}

// policyPathToName converts a path like:
// "/home/example/dir/workload_policy.hcl" to "workload-policy"
func policyPathToName(policyPath string) string {
	return strings.ReplaceAll(
		strings.Split(filepath.Base(policyPath), ".")[0], "_", "-")
}

func (s *SetupConsulCommand) createRoleForTemplate(policyNames []string) error {

	policies := []*api.ACLLink{}
	for _, policyName := range policyNames {
		policies = append(policies, &api.ACLLink{Name: policyName})
	}

	_, _, err := s.client.ACL().RoleCreate(&api.ACLRole{
		ID:          "",
		Name:        s.roleTasks,
		Description: "role for Nomad templates w/ workload identities (WI)",
		Policies:    policies,
		Namespace:   "", // TODO
		Partition:   "", // TODO
	}, nil)
	if err != nil {
		return fmt.Errorf("could not create Consul role: %w", err)
	}

	s.Ui.Info(fmt.Sprintf("[✔] Created role %s\n", s.roleTasks))
	return nil
}

func (s *SetupConsulCommand) createPolicies(policies map[string][]byte) error {

	for policyName, policyText := range policies {
		_, _, err := s.client.ACL().PolicyCreate(&api.ACLPolicy{
			Name:  policyName,
			Rules: string(policyText),
		}, nil)
		if err != nil {
			return fmt.Errorf("could not create Consul policy: %w", err)
		}

		s.Ui.Info(fmt.Sprintf("[✔] Created policy %s\n", policyName))
	}

	return nil
}

type stringSetFlags []string

func (set *stringSetFlags) String() string {
	out := ""
	for _, value := range *set {
		out = out + value + ","
	}
	return out
}

func (set *stringSetFlags) Values() []string {
	out := []string{}
	for _, value := range *set {
		out = append(out, value)
	}
	return out
}

func (set *stringSetFlags) Set(value string) error {
	*set = append(*set, value)
	return nil
}
