// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hashicorp/nomad/helper/raftutil"
	"github.com/posener/complete"
)

type OperatorSnapshotRedactCommand struct {
	Meta
}

func (c *OperatorSnapshotRedactCommand) Help() string {
	helpText := `
Usage: nomad operator snapshot redact [options] <file>

  Removes key material from an existing snapshot file created by the operator
  snapshot save command, when using the AEAD keyring provider. When using a KMS
  keyring provider, no cleartext key material is stored in snapshots and this
  command is not necessary. Note that this command requires loading the entire
  snapshot into memory locally and overwrites the existing snapshot.

  This is useful for situations where you need to transmit a snapshot without
  exposing key material.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace)

	return strings.TrimSpace(helpText)
}

func (c *OperatorSnapshotRedactCommand) AutocompleteFlags() complete.Flags {
	return complete.Flags{}
}

func (c *OperatorSnapshotRedactCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictFiles("*")
}

func (c *OperatorSnapshotRedactCommand) Synopsis() string {
	return "Redacts an existing snapshot of Nomad server state"
}

func (c *OperatorSnapshotRedactCommand) Name() string { return "operator snapshot redact" }

func (c *OperatorSnapshotRedactCommand) Run(args []string) int {
	if len(args) != 1 {
		c.Ui.Error("This command takes one argument: <file>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	path := args[0]
	f, err := os.Open(path)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error opening snapshot file: %s", err))
		return 1
	}
	defer f.Close()

	tmpFile, err := os.Create(path + ".tmp")
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to create temporary file: %v", err))
		return 1
	}

	_, err = io.Copy(tmpFile, f)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to copy snapshot to temporary file: %v", err))
		return 1
	}

	err = raftutil.RedactSnapshot(tmpFile)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to redact snapshot: %v", err))
		return 1
	}

	err = os.Rename(tmpFile.Name(), path)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to finalize snapshot file: %v", err))
		return 1
	}

	c.Ui.Output("Snapshot redacted")
	return 0
}
