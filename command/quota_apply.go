// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	humanize "github.com/dustin/go-humanize"
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
			limit.RegionLimit = new(api.QuotaResources)
			if err := parseQuotaResource(limit.RegionLimit, o); err != nil {
				return multierror.Prefix(err, "region_limit ->")
			}
		}

		*result = append(*result, &limit)
	}

	return nil
}

// parseQuotaResource parses the region_limit resources
func parseQuotaResource(result *api.QuotaResources, list *ast.ObjectList) error {
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
		"cores",
		"cpu",
		"memory",
		"memory_max",
		"device",
		"storage",
	}
	if err := helper.CheckHCLKeys(listVal, valid); err != nil {
		return multierror.Prefix(err, "resources ->")
	}

	var m map[string]interface{}
	if err := hcl.DecodeObject(&m, o.Val); err != nil {
		return err
	}

	// Manually parse
	delete(m, "device")
	delete(m, "storage")

	if err := mapstructure.WeakDecode(m, result); err != nil {
		return err
	}

	// Parse devices
	if o := listVal.Filter("device"); len(o.Items) > 0 {
		result.Devices = make([]*api.RequestedDevice, 0)
		if err := parseDeviceResource(&result.Devices, o); err != nil {
			return multierror.Prefix(err, "devices ->")
		}
	}

	// Parse storage block
	storageBlocks := listVal.Filter("storage")
	storage, err := parseStorageResource(storageBlocks)
	if err != nil {
		return multierror.Prefix(err, "storage ->")
	}
	result.Storage = storage

	return nil
}

func parseStorageResource(storageBlocks *ast.ObjectList) (*api.QuotaStorageResources, error) {
	switch len(storageBlocks.Items) {
	case 0:
		return nil, nil
	case 1:
	default:
		return nil, errors.New("only one storage block is allowed")
	}
	block := storageBlocks.Items[0]
	valid := []string{"variables", "host_volumes"}
	if err := helper.CheckHCLKeys(block.Val, valid); err != nil {
		return nil, err
	}

	var m map[string]any
	if err := hcl.DecodeObject(&m, block.Val); err != nil {
		return nil, err
	}

	variablesLimit, err := parseQuotaMegabytes(m["variables"])
	if err != nil {
		return nil, fmt.Errorf("invalid variables limit: %v", err)
	}
	hostVolumesLimit, err := parseQuotaMegabytes(m["host_volumes"])
	if err != nil {
		return nil, fmt.Errorf("invalid host_volumes limit: %v", err)
	}

	return &api.QuotaStorageResources{
		VariablesMB:   variablesLimit,
		HostVolumesMB: hostVolumesLimit,
	}, nil
}

func parseQuotaMegabytes(raw any) (int, error) {
	switch val := raw.(type) {
	case string:
		b, err := humanize.ParseBytes(val)
		if err != nil {
			return 0, fmt.Errorf("could not parse value as bytes: %v", err)
		}
		return int(b >> 20), nil
	case int:
		return val, nil
	case nil:
		return 0, nil
	default:
		return 0, fmt.Errorf("invalid type %T", raw)
	}
}

func parseDeviceResource(result *[]*api.RequestedDevice, list *ast.ObjectList) error {
	for idx, o := range list.Items {
		if l := len(o.Keys); l == 0 {
			return multierror.Prefix(fmt.Errorf("missing device name"), fmt.Sprintf("resources, device[%d]->", idx))
		} else if l > 1 {
			return multierror.Prefix(fmt.Errorf("only one name may be specified"), fmt.Sprintf("resources, device[%d]->", idx))
		}

		name := o.Keys[0].Token.Value().(string)

		// Check for invalid keys
		valid := []string{
			"name",
			"count",
		}
		if err := helper.CheckHCLKeys(o.Val, valid); err != nil {
			return err
		}

		// Set the name
		var device api.RequestedDevice
		device.Name = name

		var m map[string]interface{}
		if err := hcl.DecodeObject(&m, o.Val); err != nil {
			return err
		}

		if err := mapstructure.WeakDecode(m, &device); err != nil {
			return err
		}

		*result = append(*result, &device)
	}
	return nil
}
