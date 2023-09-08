// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestOperatorClientStateCommand(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &OperatorClientStateCommand{Meta: Meta{Ui: ui}}

	failedCode := cmd.Run([]string{"some", "bad", "args"})
	require.Equal(t, 1, failedCode)
	if out := ui.ErrorWriter.String(); !strings.Contains(out, commandErrorText(cmd)) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	dir := t.TempDir()
	code := cmd.Run([]string{dir})

	require.Equal(t, 0, code)
	require.Contains(t, ui.OutputWriter.String(), "{}")
}
