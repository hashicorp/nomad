package command

import (
	"encoding/base64"
	"fmt"
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

General Options:

  ` + generalOptionsUsage()

	return strings.TrimSpace(helpText)
}

func (c *QuotaStatusCommand) AutocompleteFlags() complete.Flags {
	return c.Meta.AutocompleteFlags(FlagSetClient)
}

func (c *QuotaStatusCommand) AutocompleteArgs() complete.Predictor {
	return QuotaPredictor(c.Meta.Client)
}

func (c *QuotaStatusCommand) Synopsis() string {
	return "Display a quota's status and current usage"
}

func (c *QuotaStatusCommand) Name() string { return "quota status" }

func (c *QuotaStatusCommand) Run(args []string) int {
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

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

	// Do a prefix lookup
	quotas := client.Quotas()
	spec, possible, err := getQuota(quotas, name)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error retrieving quota: %s", err))
		return 1
	}

	if len(possible) != 0 {
		c.Ui.Error(fmt.Sprintf("Prefix matched multiple quotas\n\n%s", formatQuotaSpecs(possible)))
		return 1
	}

	// Format the basics
	c.Ui.Output(formatQuotaSpecBasics(spec))

	// Get the quota usages
	usages, failures := quotaUsages(spec, quotas)

	// Format the limits
	c.Ui.Output(c.Colorize().Color("\n[bold]Quota Limits[reset]"))
	c.Ui.Output(formatQuotaLimits(spec, usages))

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
	limits[0] = "Region|CPU Usage|Memory Usage"
	i := 0
	for _, specLimit := range spec.Limits {
		i++

		// lookupUsage returns the regions quota usage for the limit
		lookupUsage := func() (*api.QuotaLimit, bool) {
			usage, ok := usages[specLimit.Region]
			if !ok {
				return nil, false
			}

			used, ok := usage.Used[base64.StdEncoding.EncodeToString(specLimit.Hash)]
			return used, ok
		}

		used, ok := lookupUsage()
		if !ok {
			cpu := fmt.Sprintf("- / %s", formatQuotaLimitInt(specLimit.RegionLimit.CPU))
			memory := fmt.Sprintf("- / %s", formatQuotaLimitInt(specLimit.RegionLimit.MemoryMB))
			limits[i] = fmt.Sprintf("%s|%s|%s", specLimit.Region, cpu, memory)
			continue
		}

		cpu := fmt.Sprintf("%d / %s", *used.RegionLimit.CPU, formatQuotaLimitInt(specLimit.RegionLimit.CPU))
		memory := fmt.Sprintf("%d / %s", *used.RegionLimit.MemoryMB, formatQuotaLimitInt(specLimit.RegionLimit.MemoryMB))
		limits[i] = fmt.Sprintf("%s|%s|%s", specLimit.Region, cpu, memory)
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

func getQuota(client *api.Quotas, quota string) (match *api.QuotaSpec, possible []*api.QuotaSpec, err error) {
	// Do a prefix lookup
	quotas, _, err := client.PrefixList(quota, nil)
	if err != nil {
		return nil, nil, err
	}

	l := len(quotas)
	switch {
	case l == 0:
		return nil, nil, fmt.Errorf("Quota %q matched no quotas", quota)
	case l == 1:
		return quotas[0], nil, nil
	default:
		return nil, quotas, nil
	}
}
