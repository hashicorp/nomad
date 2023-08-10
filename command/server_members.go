// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"net"
	"sort"
	"strings"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
	"github.com/ryanuber/columnize"
)

type ServerMembersCommand struct {
	Meta
}

func (c *ServerMembersCommand) Help() string {
	helpText := `
Usage: nomad server members [options]

  Display a list of the known servers and their status. Only Nomad servers are
  able to service this command.

  If ACLs are enabled, this option requires a token with the 'node:read'
  capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Server Members Options:

  -verbose
    Show detailed information about each member. This dumps a raw set of tags
    which shows more information than the default output format.

 -json
    Output the latest information about each member in a JSON format.

  -t
    Format and display latest information about each member using a Go template.
`
	return strings.TrimSpace(helpText)
}

func (c *ServerMembersCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-detailed": complete.PredictNothing,
			"-verbose":  complete.PredictNothing,
			"-json":     complete.PredictNothing,
			"-t":        complete.PredictAnything,
		})
}

func (c *ServerMembersCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *ServerMembersCommand) Synopsis() string {
	return "Display a list of known servers and their status"
}

func (c *ServerMembersCommand) Name() string { return "server members" }

func (c *ServerMembersCommand) Run(args []string) int {
	var detailed, verbose, json bool
	var tmpl string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&detailed, "detailed", false, "Show detailed output")
	flags.BoolVar(&verbose, "verbose", false, "Show detailed output")
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&tmpl, "t", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check for extra arguments
	args = flags.Args()
	if len(args) != 0 {
		c.Ui.Error("This command takes no arguments")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	// Keep support for previous flag name
	if detailed {
		verbose = true
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Query the members
	srvMembers, err := client.Agent().Members()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying servers: %s", err))
		return 1
	}

	if srvMembers == nil {
		c.Ui.Error("Agent doesn't know about server members")
		return 0
	}

	// Sort the members
	sort.Sort(api.AgentMembersNameSort(srvMembers.Members))

	// Determine the leaders per region.
	leaders, leaderErr := regionLeaders(client, srvMembers.Members)

	if json || len(tmpl) > 0 {
		for _, member := range srvMembers.Members {
			member.Tags["Leader"] = fmt.Sprintf("%t", isLeader(member, leaders))
		}
		out, err := Format(json, tmpl, srvMembers.Members)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}

		c.Ui.Output(out)
		return 0
	}

	// Format the list
	var out []string
	if verbose {
		out = verboseOutput(srvMembers.Members, leaders)
	} else {
		out = standardOutput(srvMembers.Members, leaders)
	}

	// Dump the list
	c.Ui.Output(columnize.SimpleFormat(out))

	// If there were leader errors display a warning
	if leaderErr != nil {
		c.Ui.Output("")
		c.Ui.Warn(fmt.Sprintf("Error determining leaders: %s", leaderErr))
		return 1
	}

	return 0
}

func standardOutput(mem []*api.AgentMember, leaders map[string]string) []string {
	// Format the members list
	members := make([]string, len(mem)+1)
	members[0] = "Name|Address|Port|Status|Leader|Raft Version|Build|Datacenter|Region"
	for i, member := range mem {
		members[i+1] = fmt.Sprintf("%s|%s|%d|%s|%t|%s|%s|%s|%s",
			member.Name,
			member.Addr,
			member.Port,
			member.Status,
			isLeader(member, leaders),
			member.Tags["raft_vsn"],
			member.Tags["build"],
			member.Tags["dc"],
			member.Tags["region"])
	}
	return members
}

func verboseOutput(mem []*api.AgentMember, leaders map[string]string) []string {
	// Format the members list
	members := make([]string, len(mem)+1)
	members[0] = "Name|Address|Port|Status|Leader|Protocol|Raft Version|Build|Datacenter|Region|Tags"
	for i, member := range mem {
		// Format the tags
		tagPairs := make([]string, 0, len(member.Tags))
		for k, v := range member.Tags {
			tagPairs = append(tagPairs, fmt.Sprintf("%s=%s", k, v))
		}
		tags := strings.Join(tagPairs, ",")

		members[i+1] = fmt.Sprintf("%s|%s|%d|%s|%t|%d|%s|%s|%s|%s|%s",
			member.Name,
			member.Addr,
			member.Port,
			member.Status,
			isLeader(member, leaders),
			member.ProtocolCur,
			member.Tags["raft_vsn"],
			member.Tags["build"],
			member.Tags["dc"],
			member.Tags["region"],
			tags,
		)
	}
	return members
}

// regionLeaders returns a map of regions to the IP of the member that is the
// leader.
func regionLeaders(client *api.Client, mem []*api.AgentMember) (map[string]string, error) {
	// Determine the unique regions.
	leaders := make(map[string]string)
	regions := make(map[string]struct{})
	for _, m := range mem {
		// Ignore left members
		// This prevents querying for leader status on regions where all members have left
		if m.Status == "left" {
			continue
		}

		regions[m.Tags["region"]] = struct{}{}
	}

	if len(regions) == 0 {
		return leaders, nil
	}

	var mErr multierror.Error
	status := client.Status()
	for reg := range regions {
		l, err := status.RegionLeader(reg)
		if err != nil {
			_ = multierror.Append(&mErr, fmt.Errorf("Region %q: %v", reg, err))
			continue
		}

		leaders[reg] = l
	}

	return leaders, mErr.ErrorOrNil()
}

func isLeader(member *api.AgentMember, leaders map[string]string) bool {
	addr := net.JoinHostPort(member.Addr, member.Tags["port"])
	reg := member.Tags["region"]
	regLeader, ok := leaders[reg]
	return ok && regLeader == addr
}
