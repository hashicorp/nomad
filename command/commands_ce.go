// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

package command

import "github.com/hashicorp/cli"

func EntCommands(metaPtr *Meta, agentUi cli.Ui) map[string]cli.CommandFactory {
	return map[string]cli.CommandFactory{}
}
