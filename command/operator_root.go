// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

type OperatorRootCommand struct {
	Meta
}

func (*OperatorRootCommand) Help() string {
	helpText := `
Usage: nomad operator root <subcommand> [options] [args]

  This command is accessed by using one of the subcommands below.
`

	return strings.TrimSpace(helpText)
}

func (*OperatorRootCommand) Synopsis() string {
	return "Provides access to root encryption keys"
}

func (f *OperatorRootCommand) Name() string { return "operator root" }

func (f *OperatorRootCommand) Run(_ []string) int {
	return cli.RunResultHelp
}
