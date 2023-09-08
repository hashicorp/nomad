// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/mitchellh/cli"
)

// Ensure ScalingPolicyCommand satisfies the cli.Command interface.
var _ cli.Command = &ScalingPolicyCommand{}

// ScalingPolicyCommand implements cli.Command.
type ScalingPolicyCommand struct {
	Meta
}

// Help satisfies the cli.Command Help function.
func (s *ScalingPolicyCommand) Help() string {
	helpText := `
Usage: nomad scaling policy <subcommand> [options]

  This command groups subcommands for interacting with scaling policies. Scaling
  policies can be used by an external autoscaler to perform scaling actions on
  Nomad targets.

  List policies:

      $ nomad scaling policy list

  Detail an individual scaling policy:

      $ nomad scaling policy info <policy_id>

  Please see the individual subcommand help for detailed usage information.
`
	return strings.TrimSpace(helpText)
}

// Synopsis satisfies the cli.Command Synopsis function.
func (s *ScalingPolicyCommand) Synopsis() string {
	return "Interact with Nomad scaling policies"
}

// Name returns the name of this command.
func (s *ScalingPolicyCommand) Name() string { return "scaling policy" }

// Run satisfies the cli.Command Run function.
func (s *ScalingPolicyCommand) Run(_ []string) int { return cli.RunResultHelp }

// formatScalingPolicyTarget is a command helper that correctly formats a
// scaling policy target map into a command string output.
func formatScalingPolicyTarget(t map[string]string) string {
	var ns, j, g string
	var other []string

	for k, v := range t {

		s := fmt.Sprintf("%s:%s", k, v)

		switch strings.ToLower(k) {
		case "namespace":
			ns = s
		case "job":
			j = s
		case "group":
			g = s
		default:
			other = append(other, s)
		}
	}

	out := []string{ns, j, g}

	if len(other) > 0 {
		out = append(out, other...)
	}
	return strings.Trim(strings.Join(out, ","), ",")
}
