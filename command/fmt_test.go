// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFmtCommand(t *testing.T) {
	ci.Parallel(t)

	const inSuffix = ".in.hcl"
	const expectedSuffix = ".out.hcl"

	tests := []struct {
		name        string
		testFile    string
		flags       []string
		expectWrite bool
		expectCode  int
	}{
		{
			name:       "config with check",
			testFile:   "nomad",
			flags:      []string{"-check"},
			expectCode: 1,
		},
		{
			name:        "config without check",
			testFile:    "nomad",
			flags:       []string{},
			expectWrite: true,
			expectCode:  0,
		},
		{
			name:       "job with check",
			testFile:   "job",
			flags:      []string{"-check"},
			expectCode: 1,
		},
		{
			name:        "job without check",
			testFile:    "job",
			flags:       []string{},
			expectWrite: true,
			expectCode:  0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc := tc
			ci.Parallel(t)

			tmpDir := t.TempDir()
			inFile := filepath.Join("testdata", "fmt", tc.testFile+inSuffix)
			expectedFile := filepath.Join("testdata", "fmt", tc.testFile+expectedSuffix)
			fmtFile := filepath.Join(tmpDir, tc.testFile+".hcl")

			expected, err := os.ReadFile(expectedFile)
			must.NoError(t, err)

			// copy the input file to the test tempdir so that we don't
			// overwrite the test input in source control
			input, err := os.ReadFile(inFile)
			must.NoError(t, err)
			must.NoError(t, os.WriteFile(fmtFile, input, 0644))

			ui := cli.NewMockUi()
			cmd := &FormatCommand{
				Meta: Meta{Ui: ui},
			}

			flags := append(tc.flags, fmtFile)

			code := cmd.Run(flags)
			must.Eq(t, tc.expectCode, code)

			// compare the maybe-overwritten file contents
			actual, err := os.ReadFile(fmtFile)
			must.NoError(t, err)

			if tc.expectWrite {
				must.Eq(t, string(expected), string(actual))
			} else {
				must.Eq(t, string(input), string(actual))
			}
		})
	}
}

func TestFmtCommand_FromStdin(t *testing.T) {
	ci.Parallel(t)

	tests := []struct {
		name       string
		flags      []string
		expectCode int
	}{
		{
			name:       "with check",
			flags:      []string{"-check", "-"},
			expectCode: 1,
		},
		{
			name:       "without check",
			flags:      []string{"-"},
			expectCode: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			stdinFake := bytes.NewBuffer(fmtFixture.input)
			ui := cli.NewMockUi()
			cmd := &FormatCommand{
				Meta:  Meta{Ui: ui},
				stdin: stdinFake,
			}

			code := cmd.Run(tc.flags)
			must.Eq(t, tc.expectCode, code)
			must.StrContains(t, string(fmtFixture.golden), ui.OutputWriter.String())
		})
	}
}

func TestFmtCommand_FromWorkingDirectory(t *testing.T) {
	tmpDir := fmtFixtureWriteDir(t)

	cwd, err := os.Getwd()
	require.NoError(t, err)

	err = os.Chdir(tmpDir)
	require.NoError(t, err)
	defer os.Chdir(cwd)

	tests := []struct {
		name       string
		flags      []string
		expectCode int
	}{
		{
			name:       "with check",
			flags:      []string{"-check"},
			expectCode: 1,
		},
		{
			name:       "without check",
			flags:      []string{},
			expectCode: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			cmd := &FormatCommand{Meta: Meta{Ui: ui}}
			code := cmd.Run(tc.flags)
			must.Eq(t, tc.expectCode, code)
			must.Eq(t, fmt.Sprintf("%s\n", fmtFixture.filename), ui.OutputWriter.String())
		})
	}
}

func TestFmtCommand_FromDirectoryArgument(t *testing.T) {
	tmpDir := fmtFixtureWriteDir(t)

	tests := []struct {
		name       string
		flags      []string
		expectCode int
	}{
		{
			name:       "with check",
			flags:      []string{"-check", tmpDir},
			expectCode: 1,
		},
		{
			name:       "without check",
			flags:      []string{tmpDir},
			expectCode: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			cmd := &FormatCommand{Meta: Meta{Ui: ui}}

			code := cmd.Run(tc.flags)
			must.Eq(t, tc.expectCode, code)
			must.Eq(t,
				fmt.Sprintf("%s\n", filepath.Join(tmpDir, fmtFixture.filename)),
				ui.OutputWriter.String())
		})
	}
}

func TestFmtCommand_FromFileArgument(t *testing.T) {
	tmpDir := fmtFixtureWriteDir(t)
	path := filepath.Join(tmpDir, fmtFixture.filename)

	tests := []struct {
		name       string
		flags      []string
		expectCode int
	}{
		{
			name:       "with check",
			flags:      []string{"-check", path},
			expectCode: 1,
		},
		{
			name:       "without check",
			flags:      []string{path},
			expectCode: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			cmd := &FormatCommand{Meta: Meta{Ui: ui}}

			code := cmd.Run(tc.flags)
			must.Eq(t, tc.expectCode, code)
			must.Eq(t, fmt.Sprintf("%s\n", path), ui.OutputWriter.String())
		})
	}
}

func TestFmtCommand_FileDoesNotExist(t *testing.T) {
	ci.Parallel(t)

	ui := cli.NewMockUi()
	cmd := &FormatCommand{
		Meta: Meta{Ui: ui},
	}

	code := cmd.Run([]string{"file/does/not/exist.hcl"})
	assert.Equal(t, 1, code)
}

func TestFmtCommand_InvalidSyntax(t *testing.T) {
	ci.Parallel(t)

	stdinFake := bytes.NewBufferString(`client {enabled true }`)

	ui := cli.NewMockUi()
	cmd := &FormatCommand{
		Meta:  Meta{Ui: ui},
		stdin: stdinFake,
	}

	code := cmd.Run([]string{"-"})
	assert.Equal(t, 1, code)
}

func fmtFixtureWriteDir(t *testing.T) string {
	dir := t.TempDir()

	err := os.WriteFile(filepath.Join(dir, fmtFixture.filename), fmtFixture.input, 0644)
	require.NoError(t, err)

	return dir
}

var fmtFixture = struct {
	filename string
	input    []byte
	golden   []byte
}{
	filename: "nomad.hcl",
	input:    []byte("client   {enabled = true}"),
	golden:   []byte("client { enabled = true }\n\n"),
}
