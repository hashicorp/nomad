// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/asset"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

func TestInitCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &JobInitCommand{}
}

func TestInitCommand_Run(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &JobInitCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("expect exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, commandErrorText(cmd)) {
		t.Fatalf("expect help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Ensure we change the cwd back
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Chdir(origDir)

	// Create a temp dir and change into it
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Works if the file doesn't exist
	if code := cmd.Run([]string{}); code != 0 {
		t.Fatalf("expect exit code 0, got: %d", code)
	}
	content, err := os.ReadFile(DefaultInitName)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defaultJob := asset.JobExample
	if string(content) != string(defaultJob) {
		t.Fatalf("unexpected file content\n\n%s", string(content))
	}

	// Works with -short flag
	os.Remove(DefaultInitName)
	if code := cmd.Run([]string{"-short"}); code != 0 {
		require.Zero(t, code, "unexpected exit code: %d", code)
	}
	content, err = os.ReadFile(DefaultInitName)
	require.NoError(t, err)
	shortJob := asset.JobExampleShort
	require.Equal(t, string(content), string(shortJob))

	// Fails if the file exists
	if code := cmd.Run([]string{}); code != 1 {
		t.Fatalf("expect exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "exists") {
		t.Fatalf("expect file exists error, got: %s", out)
	}
}

func TestInitCommand_defaultJob(t *testing.T) {
	ci.Parallel(t)
	// Ensure the job file is always written with spaces instead of tabs. Since
	// the default job file is embedded in the go file, it's easy for tabs to
	// slip in.
	defaultJob := asset.JobExample
	if strings.Contains(string(defaultJob), "\t") {
		t.Error("default job contains tab character - please convert to spaces")
	}
}

func TestInitCommand_listTemplates(t *testing.T) {
	ci.Parallel(t)
	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()

	jobCmd := &JobInitCommand{Meta: Meta{Ui: ui}}
	jobCmd.Run([]string{"-address=" + url, "-list-templates"})
	expectedOutput := "No variables in nomad/job-templates\n"
	must.StrContains(t, ui.ErrorWriter.String(), expectedOutput)

	varCmd := &VarPutCommand{Meta: Meta{Ui: ui}}
	// Set up 3 job template variables
	for i := 1; i <= 3; i++ {
		templateName := fmt.Sprintf("template-%d", i)
		must.Eq(t, 0, varCmd.Run([]string{"-address=" + url, "-out=json", "nomad/job-templates/" + templateName, "k1=v1", "k2=v2", "k3=v3"}))
	}
	ui.ErrorWriter.Reset()
	ui.OutputWriter.Reset()

	jobCmd = &JobInitCommand{Meta: Meta{Ui: ui}}
	must.Eq(t, 0, jobCmd.Run([]string{"-address=" + url, "-list-templates"}))
	expectedOutput = "Use nomad job init -template=<template> with any of the following:\n  template-1\n  template-2\n  template-3\n"
	must.StrContains(t, ui.OutputWriter.String(), expectedOutput)
}

func TestInitCommand_fromJobTemplate(t *testing.T) {
	ci.Parallel(t)
	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	varCmd := &VarPutCommand{Meta: Meta{Ui: ui}}

	tinyJob := `job "tiny" {
		group "foo" {
			task "bar" {
			}
		}
	}`

	// Set up job template variables
	varCmd.Run([]string{"-address=" + url, "-out=json", "nomad/job-templates/invalid-template", "k1=v1"})
	varCmd.Run([]string{"-address=" + url, "-out=json", "nomad/job-templates/valid-template", "template=" + tinyJob})
	ui.ErrorWriter.Reset()
	ui.OutputWriter.Reset()

	// Ensure we change the cwd back
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Chdir(origDir)

	// Create a temp dir and change into it
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("err: %s", err)
	}

	jobCmd := &JobInitCommand{Meta: Meta{Ui: ui}}

	// Doesn't work if our var lacks a template key
	must.Eq(t, 1, jobCmd.Run([]string{"-address=" + url, "-template=invalid-template"}))

	// Works if the file doesn't exist
	must.Eq(t, 0, jobCmd.Run([]string{"-address=" + url, "-template=valid-template"}))

	content, err := os.ReadFile(DefaultInitName)
	must.NoError(t, err)
	must.Eq(t, string(content), string(tinyJob))

	ui.ErrorWriter.Reset()
	expectedOutput := "Initializing a job template from valid-template\nExample job file written to example.nomad.hcl\n"
	must.StrContains(t, ui.OutputWriter.String(), expectedOutput)
}

func TestInitCommand_customFilename(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &JobInitCommand{Meta: Meta{Ui: ui}}
	filename := "custom.nomad"

	// Ensure we change the cwd back
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Chdir(origDir)

	// Create a temp dir and change into it
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Works if the file doesn't exist
	if code := cmd.Run([]string{filename}); code != 0 {
		t.Fatalf("expect exit code 0, got: %d", code)
	}
	content, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defaultJob := asset.JobExample
	if string(content) != string(defaultJob) {
		t.Fatalf("unexpected file content\n\n%s", string(content))
	}

	// Works with -short flag
	os.Remove(filename)
	if code := cmd.Run([]string{"-short", filename}); code != 0 {
		require.Zero(t, code, "unexpected exit code: %d", code)
	}
	content, err = os.ReadFile(filename)
	require.NoError(t, err)
	shortJob := asset.JobExampleShort
	require.Equal(t, string(content), string(shortJob))

	// Fails if the file exists
	if code := cmd.Run([]string{filename}); code != 1 {
		t.Fatalf("expect exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "exists") {
		t.Fatalf("expect file exists error, got: %s", out)
	}
}
