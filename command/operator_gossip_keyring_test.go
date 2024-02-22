// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"encoding/base64"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
)

func TestGossipKeyringGenerateCommand(t *testing.T) {
	ci.Parallel(t)

	ui := cli.NewMockUi()
	c := &OperatorGossipKeyringGenerateCommand{Meta: Meta{Ui: ui}}
	code := c.Run(nil)
	must.Zero(t, code)

	output := ui.OutputWriter.String()
	result, err := base64.StdEncoding.DecodeString(output)
	must.NoError(t, err)
	must.Len(t, 32, result)

}
