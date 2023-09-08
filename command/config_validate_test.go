// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
)

func TestConfigValidateCommand_FailWithEmptyDir(t *testing.T) {
	ci.Parallel(t)
	fh := t.TempDir()

	ui := cli.NewMockUi()
	cmd := &ConfigValidateCommand{Meta: Meta{Ui: ui}}
	args := []string{fh}

	code := cmd.Run(args)
	must.One(t, code)
}

func TestConfigValidateCommand_SucceedWithMinimalConfigFile(t *testing.T) {
	ci.Parallel(t)
	fh := t.TempDir()

	fp := filepath.Join(fh, "config.hcl")
	err := os.WriteFile(fp, []byte(`data_dir="/"
	client {
		enabled = true
	}`), 0644)
	must.NoError(t, err)

	ui := cli.NewMockUi()
	cmd := &ConfigValidateCommand{Meta: Meta{Ui: ui}}
	args := []string{fh}

	code := cmd.Run(args)
	must.Zero(t, code)
}

func TestConfigValidateCommand_FailOnParseBadConfigFile(t *testing.T) {
	ci.Parallel(t)
	fh := t.TempDir()

	fp := filepath.Join(fh, "config.hcl")
	err := os.WriteFile(fp, []byte(`a: b`), 0644)
	must.NoError(t, err)

	ui := cli.NewMockUi()
	cmd := &ConfigValidateCommand{Meta: Meta{Ui: ui}}
	args := []string{fh}

	code := cmd.Run(args)
	must.One(t, code)
}

func TestConfigValidateCommand_FailOnValidateParsableConfigFile(t *testing.T) {
	ci.Parallel(t)
	fh := t.TempDir()

	fp := filepath.Join(fh, "config.hcl")
	err := os.WriteFile(fp, []byte(`data_dir="../" 
	client {
		enabled = true 
	}`), 0644)
	must.NoError(t, err)

	ui := cli.NewMockUi()
	cmd := &ConfigValidateCommand{Meta: Meta{Ui: ui}}
	args := []string{fh}

	code := cmd.Run(args)
	must.One(t, code)
}
