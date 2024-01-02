// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
)

// OperatorGossipKeyringGenerateCommand is a Command implementation that
// generates an encryption key for use in `nomad agent`.
type OperatorGossipKeyringGenerateCommand struct {
	Meta
}

func (c *OperatorGossipKeyringGenerateCommand) Synopsis() string {
	return "Generates a new encryption key"
}

func (c *OperatorGossipKeyringGenerateCommand) Help() string {
	helpText := `
Usage: nomad operator gossip keying generate

  Generates a new 32-byte encryption key that can be used to configure the
  agent to encrypt traffic. The output of this command is already
  in the proper format that the agent expects.
`
	return strings.TrimSpace(helpText)
}

func (c *OperatorGossipKeyringGenerateCommand) Name() string {
	return "operator gossip keyring generate"
}

func (c *OperatorGossipKeyringGenerateCommand) Run(_ []string) int {
	key := make([]byte, 32)
	n, err := rand.Reader.Read(key)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error reading random data: %s", err))
		return 1
	}
	if n != 32 {
		c.Ui.Error("Couldn't read enough entropy. Generate more entropy!")
		return 1
	}

	c.Ui.Output(base64.StdEncoding.EncodeToString(key))
	return 0
}
