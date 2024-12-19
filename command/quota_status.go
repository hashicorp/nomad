// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"encoding/base64"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type QuotaStatusCommand struct {
	Meta
}

func (c *QuotaStatusCommand) Help() string {
	helpText := `
Usage: nomad quota status [options] <quota>

  Status is used to view the status of a particular quota specification.

  If ACLs are enabled, this command requires a token with the 'quota:read'
  capability and access to any namespaces that the quota is applied to.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Status Specific Options:

  -json
    Output the latest quota status information in a JSON format.

  -t
    Format and display quota status information using a Go template.
`

	return strings.TrimSpace(helpText)
}

func (c *QuotaStatusCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json": complete.PredictNothing,
			"-t":    complete.PredictAnything,
		})
}

func (c *QuotaStatusCommand) AutocompleteArgs() complete.Predictor {
	return QuotaPredictor(c.Meta.Client)
}

func (c *QuotaStatusCommand) Synopsis() string {
	return "Display a quota's status and current usage"
}

func (c *QuotaStatusCommand) Name() string { return "quota status" }

func (c *QuotaStatusCommand) Run(args []string) int {
	var json bool
	var tmpl string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&tmpl, "t", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got one arguments
	args = flags.Args()
	if l := len(args); l != 1 {
		c.Ui.Error("This command takes one argument: <quota>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	name := args[0]

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	quotas := client.Quotas()
	spec, possible, err := getQuotaByPrefix(quotas, name)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error retrieving quota: %s", err))
		return 1
	}
	if len(possible) != 0 {
		c.Ui.Error(fmt.Sprintf("Prefix matched multiple quotas\n\n%s", formatQuotaSpecs(possible)))
		return 1
	}

	if json || len(tmpl) > 0 {
		out, err := Format(json, tmpl, spec)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}

		c.Ui.Output(out)
		return 0
	}

	// Format the basics
	c.Ui.Output(formatQuotaSpecBasics(spec))

	// Get the quota usages
	usages, failures := quotaUsages(spec, quotas)

	// Format the limits
	c.Ui.Output(c.Colorize().Color("\n[bold]Quota Limits[reset]"))
	c.Ui.Output(formatQuotaLimits(spec, usages))

	// If quota has limits on devices, format them separately
	if slices.ContainsFunc(spec.Limits, func(l *api.QuotaLimit) bool { return l.RegionLimit.Devices != nil }) {
		c.Ui.Output(c.Colorize().Color("\n[bold]Quota Device Limits[reset]"))
		c.Ui.Output(formatQuotaDevices(spec, usages))
	}

	// Display any failures
	if len(failures) != 0 {
		c.Ui.Error(c.Colorize().Color("\n[bold][red]Lookup Failures[reset]"))
		for region, failure := range failures {
			c.Ui.Error(fmt.Sprintf("  * Failed to retrieve quota usage for region %q: %v", region, failure))
			return 1
		}
	}

	return 0
}

// quotaUsages returns the quota usages for the limits described by the spec. It
// will make a request to each referenced Nomad region. If the region couldn't
// be contacted, the error will be stored in the failures map
func quotaUsages(spec *api.QuotaSpec, client *api.Quotas) (usages map[string]*api.QuotaUsage, failures map[string]error) {
	// Determine the regions we have limits for
	regions := make(map[string]struct{})
	for _, limit := range spec.Limits {
		regions[limit.Region] = struct{}{}
	}

	usages = make(map[string]*api.QuotaUsage, len(regions))
	failures = make(map[string]error)
	q := api.QueryOptions{}

	// Retrieve the usage per region
	for region := range regions {
		q.Region = region
		usage, _, err := client.Usage(spec.Name, &q)
		if err != nil {
			failures[region] = err
			continue
		}

		usages[region] = usage
	}

	return usages, failures
}

// formatQuotaSpecBasics formats the basic information of the quota
// specification.
func formatQuotaSpecBasics(spec *api.QuotaSpec) string {
	basic := []string{
		fmt.Sprintf("Name|%s", spec.Name),
		fmt.Sprintf("Description|%s", spec.Description),
		fmt.Sprintf("Limits|%d", len(spec.Limits)),
	}

	return formatKV(basic)
}

// lookupUsage returns the regions quota usage for the limit
func lookupUsage(usages map[string]*api.QuotaUsage, specLimit *api.QuotaLimit) (*api.QuotaLimit, bool) {
	usage, ok := usages[specLimit.Region]
	if !ok {
		return nil, false
	}

	used, ok := usage.Used[base64.StdEncoding.EncodeToString(specLimit.Hash)]
	return used, ok
}

// formatQuotaLimits formats the limits to display the quota usage versus the
// limit per quota limit. It takes as input the specification as well as quota
// usage by region. The formatter handles missing usages.
func formatQuotaLimits(spec *api.QuotaSpec, usages map[string]*api.QuotaUsage) string {
	if len(spec.Limits) == 0 {
		return "No quota limits defined"
	}

	// Sort the limits
	sort.Sort(api.QuotaLimitSort(spec.Limits))

	limits := make([]string, len(spec.Limits)+1)
	limits[0] = "Region|CPU Usage|Core Usage|Memory Usage|Memory Max Usage|Variables Usage"
	i := 0
	for _, specLimit := range spec.Limits {
		i++

		used, ok := lookupUsage(usages, specLimit)
		if !ok {
			cores := fmt.Sprintf("- / %s", formatQuotaLimitInt(specLimit.RegionLimit.Cores))
			cpu := fmt.Sprintf("- / %s", formatQuotaLimitInt(specLimit.RegionLimit.CPU))
			memory := fmt.Sprintf("- / %s", formatQuotaLimitInt(specLimit.RegionLimit.MemoryMB))
			memoryMax := fmt.Sprintf("- / %s", formatQuotaLimitInt(specLimit.RegionLimit.MemoryMaxMB))
			vars := fmt.Sprintf("- / %s", formatQuotaLimitInt(specLimit.VariablesLimit))
			limits[i] = fmt.Sprintf("%s|%s|%s|%s|%s|%s", specLimit.Region, cpu, cores, memory, memoryMax, vars)
			continue
		}

		orZero := func(v *int) int {
			if v == nil {
				return 0
			}
			return *v
		}

		cores := fmt.Sprintf("%d / %s", orZero(used.RegionLimit.Cores), formatQuotaLimitInt(specLimit.RegionLimit.Cores))
		cpu := fmt.Sprintf("%d / %s", orZero(used.RegionLimit.CPU), formatQuotaLimitInt(specLimit.RegionLimit.CPU))
		memory := fmt.Sprintf("%d / %s", orZero(used.RegionLimit.MemoryMB), formatQuotaLimitInt(specLimit.RegionLimit.MemoryMB))
		memoryMax := fmt.Sprintf("%d / %s", orZero(used.RegionLimit.MemoryMaxMB), formatQuotaLimitInt(specLimit.RegionLimit.MemoryMaxMB))

		vars := fmt.Sprintf("%d / %s", orZero(used.VariablesLimit), formatQuotaLimitInt(specLimit.VariablesLimit))
		limits[i] = fmt.Sprintf("%s|%s|%s|%s|%s|%s", specLimit.Region, cpu, cores, memory, memoryMax, vars)
	}

	return formatList(limits)
}

// formatQuotaLimitInt takes a integer resource value and returns the
// appropriate string for output.
func formatQuotaLimitInt(value *int) string {
	if value == nil {
		return "-"
	}

	v := *value
	if v < 0 {
		return "0"
	} else if v == 0 {
		return "inf"
	}

	return strconv.Itoa(v)
}

func formatQuotaDevices(spec *api.QuotaSpec, usages map[string]*api.QuotaUsage) string {
	devices := []string{"Region|Device Name|Device Usage"}
	i := 0

	for _, specLimit := range spec.Limits {
		i++

		usage := "-"
		used, ok := lookupUsage(usages, specLimit)
		if !ok {
			for _, d := range specLimit.RegionLimit.Devices {
				devices = append(devices, fmt.Sprintf("%s|%s|%s / %d", specLimit.Region, d.Name, usage, *d.Count))
			}
			continue
		}

		for _, d := range specLimit.RegionLimit.Devices {
			idx := slices.IndexFunc(used.RegionLimit.Devices, func(dd *api.RequestedDevice) bool { return dd.Name == d.Name })
			if idx >= 0 {
				usage = fmt.Sprintf("%d", int(*used.RegionLimit.Devices[idx].Count))
			}

			devices = append(devices, fmt.Sprintf("%s|%s|%s / %d", specLimit.Region, d.Name, usage, *d.Count))
		}
	}
	return formatList(devices)
}

func getQuotaByPrefix(client *api.Quotas, quota string) (match *api.QuotaSpec, possible []*api.QuotaSpec, err error) {
	// Do a prefix lookup
	quotas, _, err := client.PrefixList(quota, nil)
	if err != nil {
		return nil, nil, err
	}

	switch len(quotas) {
	case 0:
		return nil, nil, fmt.Errorf("Quota %q matched no quotas", quota)
	case 1:
		return quotas[0], nil, nil
	default:
		// find exact match if possible
		for _, q := range quotas {
			if q.Name == quota {
				return q, nil, nil
			}
		}
		return nil, quotas, nil
	}
}
