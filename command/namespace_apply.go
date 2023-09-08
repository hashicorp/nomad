// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/ast"
	"github.com/hashicorp/nomad/api"
	flaghelper "github.com/hashicorp/nomad/helper/flags"
	"github.com/mitchellh/mapstructure"
	"github.com/posener/complete"
)

type NamespaceApplyCommand struct {
	Meta
}

func (c *NamespaceApplyCommand) Help() string {
	helpText := `
Usage: nomad namespace apply [options] <input>

  Apply is used to create or update a namespace. The specification file
  will be read from stdin by specifying "-", otherwise a path to the file is
  expected.

  Instead of a file, you may instead pass the namespace name to create
  or update as the only argument.

  If ACLs are enabled, this command requires a management ACL token. In
  federated clusters, the namespace will be created in the authoritative region
  and will be replicated to all federated regions.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Apply Options:

  -quota
    The quota to attach to the namespace.

  -description
    An optional description for the namespace.

  -json
    Parse the input as a JSON namespace specification.
`
	return strings.TrimSpace(helpText)
}

func (c *NamespaceApplyCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-description": complete.PredictAnything,
			"-quota":       QuotaPredictor(c.Meta.Client),
			"-json":        complete.PredictNothing,
		})
}

func (c *NamespaceApplyCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictOr(
		NamespacePredictor(c.Meta.Client, nil),
		complete.PredictFiles("*.hcl"),
		complete.PredictFiles("*.json"),
	)
}

func (c *NamespaceApplyCommand) Synopsis() string {
	return "Create or update a namespace"
}

func (c *NamespaceApplyCommand) Name() string { return "namespace apply" }

func (c *NamespaceApplyCommand) Run(args []string) int {
	var jsonInput bool
	var description, quota *string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.Var((flaghelper.FuncVar)(func(s string) error {
		description = &s
		return nil
	}), "description", "")
	flags.Var((flaghelper.FuncVar)(func(s string) error {
		quota = &s
		return nil
	}), "quota", "")
	flags.BoolVar(&jsonInput, "json", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we get exactly one argument
	args = flags.Args()
	if l := len(args); l != 1 {
		c.Ui.Error("This command takes one argument: <input>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	file := args[0]
	var rawNamespace []byte
	var err error
	var namespace *api.Namespace

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	if fi, err := os.Stat(file); (file == "-" || err == nil) && !fi.IsDir() {
		if quota != nil || description != nil {
			c.Ui.Warn("Flags are ignored when a file is specified!")
		}

		if file == "-" {
			rawNamespace, err = io.ReadAll(os.Stdin)
			if err != nil {
				c.Ui.Error(fmt.Sprintf("Failed to read stdin: %v", err))
				return 1
			}
		} else {
			rawNamespace, err = os.ReadFile(file)
			if err != nil {
				c.Ui.Error(fmt.Sprintf("Failed to read file: %v", err))
				return 1
			}
		}
		if jsonInput {
			var jsonSpec api.Namespace
			dec := json.NewDecoder(bytes.NewBuffer(rawNamespace))
			if err := dec.Decode(&jsonSpec); err != nil {
				c.Ui.Error(fmt.Sprintf("Failed to parse quota: %v", err))
				return 1
			}
			namespace = &jsonSpec
		} else {
			hclSpec, err := parseNamespaceSpec(rawNamespace)
			if err != nil {
				c.Ui.Error(fmt.Sprintf("Error parsing quota specification: %s", err))
				return 1
			}

			namespace = hclSpec
		}
	} else {
		name := args[0]

		// Validate we have at-least a name
		if name == "" {
			c.Ui.Error("Namespace name required")
			return 1
		}

		// Lookup the given namespace
		namespace, _, err = client.Namespaces().Info(name, nil)
		if err != nil && !strings.Contains(err.Error(), "404") {
			c.Ui.Error(fmt.Sprintf("Error looking up namespace: %s", err))
			return 1
		}

		if namespace == nil {
			namespace = &api.Namespace{
				Name: name,
			}
		}

		// Add what is set
		if description != nil {
			namespace.Description = *description
		}
		if quota != nil {
			namespace.Quota = *quota
		}
	}
	_, err = client.Namespaces().Register(namespace, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error applying namespace: %s", err))
		return 1
	}

	c.Ui.Output(fmt.Sprintf("Successfully applied namespace %q!", namespace.Name))

	return 0
}

// parseNamespaceSpec is used to parse the namespace specification from HCL
func parseNamespaceSpec(input []byte) (*api.Namespace, error) {
	root, err := hcl.ParseBytes(input)
	if err != nil {
		return nil, err
	}

	// Top-level item should be a list
	list, ok := root.Node.(*ast.ObjectList)
	if !ok {
		return nil, fmt.Errorf("error parsing: root should be an object")
	}

	var spec api.Namespace
	if err := parseNamespaceSpecImpl(&spec, list); err != nil {
		return nil, err
	}

	return &spec, nil
}

// parseNamespaceSpec parses the quota namespace taking as input the AST tree
func parseNamespaceSpecImpl(result *api.Namespace, list *ast.ObjectList) error {
	// Decode the full thing into a map[string]interface for ease
	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, list); err != nil {
		return err
	}

	delete(m, "capabilities")
	delete(m, "meta")
	delete(m, "node_pool_config")

	// Decode the rest
	if err := mapstructure.WeakDecode(m, result); err != nil {
		return err
	}

	cObj := list.Filter("capabilities")
	if len(cObj.Items) > 0 {
		for _, o := range cObj.Elem().Items {
			ot, ok := o.Val.(*ast.ObjectType)
			if !ok {
				break
			}
			var opts *api.NamespaceCapabilities
			if err := hcl.DecodeObject(&opts, ot.List); err != nil {
				return err
			}
			result.Capabilities = opts
			break
		}
	}

	npObj := list.Filter("node_pool_config")
	if len(npObj.Items) > 0 {
		for _, o := range npObj.Elem().Items {
			ot, ok := o.Val.(*ast.ObjectType)
			if !ok {
				break
			}
			var npConfig *api.NamespaceNodePoolConfiguration
			if err := hcl.DecodeObject(&npConfig, ot.List); err != nil {
				return err
			}
			result.NodePoolConfiguration = npConfig
			break
		}
	}

	if metaO := list.Filter("meta"); len(metaO.Items) > 0 {
		for _, o := range metaO.Elem().Items {
			var m map[string]interface{}
			if err := hcl.DecodeObject(&m, o.Val); err != nil {
				return err
			}
			if err := mapstructure.WeakDecode(m, &result.Meta); err != nil {
				return err
			}
		}
	}

	return nil
}
