// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"encoding/base64"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
)

func TestGossipKeyringGenerateCommand(t *testing.T) {
	ci.Parallel(t)

	ui := cli.NewMockUi()
	c := &OperatorGossipKeyringGenerateCommand{Meta: Meta{Ui: ui}}
	code := c.Run(nil)
	if code != 0 {
		t.Fatalf("bad: %d", code)
	}

	output := ui.OutputWriter.String()
	result, err := base64.StdEncoding.DecodeString(output)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if len(result) != 32 {
		t.Fatalf("bad: %#v", result)
	}
}
