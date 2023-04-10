// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !ent
// +build !ent

package command

import "github.com/mitchellh/cli"

func EntCommands(metaPtr *Meta, agentUi cli.Ui) map[string]cli.CommandFactory {
	return map[string]cli.CommandFactory{}
}
