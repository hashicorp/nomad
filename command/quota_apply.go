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

	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/ast"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper"
	"github.com/mitchellh/mapstructure"
	"github.com/posener/complete"
)

type QuotaApplyCommand struct {
	Meta
}

func (c *QuotaApplyCommand) Help() string {
	helpText := `
Usage: nomad quota apply [options] <input>

  Apply is used to create or update a quota specification. The specification file
  will be read from stdin by specifying "-", otherwise a path to the file is
  expected.

  If ACLs are enabled, this command requires a token with the 'quota:write'
  capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Apply Options:

  -json
    Parse the input as a JSON quota specification.
`

	return strings.TrimSpace(helpText)
}

func (c *QuotaApplyCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json": complete.PredictNothing,
		})
}

func (c *QuotaApplyCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictFiles("*")
}

func (c *QuotaApplyCommand) Synopsis() string {
	return "Create or update a quota specification"
}

func (c *QuotaApplyCommand) Name() string { return "quota apply" }

func (c *QuotaApplyCommand) Run(args []string) int {
	var jsonInput bool
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
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

	// Read the file contents
	file := args[0]
	var rawQuota []byte
	var err error
	if file == "-" {
		rawQuota, err = io.ReadAll(os.Stdin)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to read stdin: %v", err))
			return 1
		}
	} else {
		rawQuota, err = os.ReadFile(file)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to read file: %v", err))
			return 1
		}
	}

	var spec *api.QuotaSpec
	if jsonInput {
		var jsonSpec api.QuotaSpec
		dec := json.NewDecoder(bytes.NewBuffer(rawQuota))
		if err := dec.Decode(&jsonSpec); err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to parse quota: %v", err))
			return 1
		}
		spec = &jsonSpec
	} else {
		hclSpec, err := parseQuotaSpec(rawQuota)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error parsing quota specification: %s", err))
			return 1
		}

		spec = hclSpec
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	_, err = client.Quotas().Register(spec, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error applying quota specification: %s", err))
		return 1
	}

	c.Ui.Output(fmt.Sprintf("Successfully applied quota specification %q!", spec.Name))
	return 0
}

// parseQuotaSpec is used to parse the quota specification from HCL
func parseQuotaSpec(input []byte) (*api.QuotaSpec, error) {
	root, err := hcl.ParseBytes(input)
	if err != nil {
		return nil, err
	}

	// Top-level item should be a list
	list, ok := root.Node.(*ast.ObjectList)
	if !ok {
		return nil, fmt.Errorf("error parsing: root should be an object")
	}

	var spec api.QuotaSpec
	if err := parseQuotaSpecImpl(&spec, list); err != nil {
		return nil, err
	}

	return &spec, nil
}

// parseQuotaSpecImpl parses the quota spec taking as input the AST tree
func parseQuotaSpecImpl(result *api.QuotaSpec, list *ast.ObjectList) error {
	// Check for invalid keys
	valid := []string{
		"name",
		"description",
		"limit",
	}
	if err := helper.CheckHCLKeys(list, valid); err != nil {
		return err
	}

	// Decode the full thing into a map[string]interface for ease
	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, list); err != nil {
		return err
	}

	// Manually parse
	delete(m, "limit")

	// Decode the rest
	if err := mapstructure.WeakDecode(m, result); err != nil {
		return err
	}

	// Parse limits
	if o := list.Filter("limit"); len(o.Items) > 0 {
		if err := parseQuotaLimits(&result.Limits, o); err != nil {
			return multierror.Prefix(err, "limit ->")
		}
	}

	return nil
}

// parseQuotaLimits parses the quota limits
func parseQuotaLimits(result *[]*api.QuotaLimit, list *ast.ObjectList) error {
	for _, o := range list.Elem().Items {
		// Check for invalid keys
		valid := []string{
			"region",
			"region_limit",
			"variables_limit",
		}
		if err := helper.CheckHCLKeys(o.Val, valid); err != nil {
			return err
		}

		var m map[string]interface{}
		if err := hcl.DecodeObject(&m, o.Val); err != nil {
			return err
		}

		// Manually parse
		delete(m, "region_limit")

		// Decode the rest
		var limit api.QuotaLimit
		if err := mapstructure.WeakDecode(m, &limit); err != nil {
			return err
		}

		// We need this later
		var listVal *ast.ObjectList
		if ot, ok := o.Val.(*ast.ObjectType); ok {
			listVal = ot.List
		} else {
			return fmt.Errorf("limit should be an object")
		}

		// Parse limits
		if o := listVal.Filter("region_limit"); len(o.Items) > 0 {
			limit.RegionLimit = new(api.Resources)
			if err := parseQuotaResource(limit.RegionLimit, o); err != nil {
				return multierror.Prefix(err, "region_limit ->")
			}
		}

		*result = append(*result, &limit)
	}

	return nil
}

// parseQuotaResource parses the region_limit resources
func parseQuotaResource(result *api.Resources, list *ast.ObjectList) error {
	list = list.Elem()
	if len(list.Items) == 0 {
		return nil
	}
	if len(list.Items) > 1 {
		return fmt.Errorf("only one 'region_limit' block allowed per limit")
	}

	// Get our resource object
	o := list.Items[0]

	// We need this later
	var listVal *ast.ObjectList
	if ot, ok := o.Val.(*ast.ObjectType); ok {
		listVal = ot.List
	} else {
		return fmt.Errorf("resource: should be an object")
	}

	// Check for invalid keys
	valid := []string{
		"cpu",
		"memory",
		"memory_max",
	}
	if err := helper.CheckHCLKeys(listVal, valid); err != nil {
		return multierror.Prefix(err, "resources ->")
	}

	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, o.Val); err != nil {
		return err
	}

	if err := mapstructure.WeakDecode(m, result); err != nil {
		return err
	}

	return nil
}
