package command

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFmtCommand(t *testing.T) {
	ci.Parallel(t)

	const inSuffix = ".in.hcl"
	const expectedSuffix = ".out.hcl"
	tests := [][]string{
		{"nomad", "-check"},
		{"nomad", ""},
		{"job", "-check"},
		{"job", ""},
	}
	tmpDir := t.TempDir()

	for _, test := range tests {
		t.Run(test[0]+test[1], func(t *testing.T) {
			inFile := filepath.Join("testdata", "fmt", test[0]+inSuffix)
			expectedFile := filepath.Join("testdata", "fmt", test[0]+expectedSuffix)
			fmtFile := filepath.Join(tmpDir, test[0]+".hcl")

			input, err := os.ReadFile(inFile)
			require.NoError(t, err)

			expected, err := os.ReadFile(expectedFile)
			require.NoError(t, err)

			require.NoError(t, os.WriteFile(fmtFile, input, 0644))

			ui := cli.NewMockUi()
			cmd := &FormatCommand{
				Meta: Meta{Ui: ui},
			}

			var code int
			var expectedCode int
			if test[1] == "-check" {
				code = cmd.Run([]string{test[1], fmtFile})
				expectedCode = 1
			} else {
				code = cmd.Run([]string{fmtFile})
				expectedCode = 0
			}
			assert.Equal(t, expectedCode, code)

			actual, err := os.ReadFile(fmtFile)
			require.NoError(t, err)

			assert.Equal(t, string(expected), string(actual))
		})
	}
}

func TestFmtCommand_FromStdin(t *testing.T) {
	ci.Parallel(t)

	stdinFake := bytes.NewBuffer(fmtFixture.input)

	ui := cli.NewMockUi()
	cmd := &FormatCommand{
		Meta:  Meta{Ui: ui},
		stdin: stdinFake,
	}

	tests := []string{"-check", ""}
	for _, checkFlag := range tests {
		t.Run(checkFlag, func(t *testing.T) {
			var code int
			var expectedCode int

			if checkFlag == "-check" {
				code = cmd.Run([]string{checkFlag, "-"})
				expectedCode = 1
			} else {
				code = cmd.Run([]string{"-"})
				expectedCode = 0

			}

			assert.Equal(t, expectedCode, code)
			assert.Contains(t, ui.OutputWriter.String(), string(fmtFixture.golden))
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

	ui := cli.NewMockUi()
	cmd := &FormatCommand{
		Meta: Meta{Ui: ui},
	}

	tests := []string{"-check", ""}
	for _, checkFlag := range tests {
		t.Run(checkFlag, func(t *testing.T) {
			var code int
			var expectedCode int

			if checkFlag == "-check" {
				code = cmd.Run([]string{checkFlag})
				expectedCode = 1
			} else {
				code = cmd.Run([]string{})
				expectedCode = 0

			}

			assert.Equal(t, expectedCode, code)
			assert.Equal(t, fmt.Sprintf("%s\n", fmtFixture.filename), ui.OutputWriter.String())
		})
	}
}

func TestFmtCommand_FromDirectoryArgument(t *testing.T) {
	tmpDir := fmtFixtureWriteDir(t)

	ui := cli.NewMockUi()
	cmd := &FormatCommand{
		Meta: Meta{Ui: ui},
	}

	tests := []string{"-check", ""}
	for _, checkFlag := range tests {
		t.Run(checkFlag, func(t *testing.T) {
			var code int
			var expectedCode int

			if checkFlag == "-check" {
				code = cmd.Run([]string{checkFlag, tmpDir})
				expectedCode = 1
			} else {
				code = cmd.Run([]string{tmpDir})
				expectedCode = 0

			}

			assert.Equal(t, expectedCode, code)
			assert.Equal(t, fmt.Sprintf("%s\n", filepath.Join(tmpDir, fmtFixture.filename)), ui.OutputWriter.String())
		})
	}
}

func TestFmtCommand_FromFileArgument(t *testing.T) {
	tmpDir := fmtFixtureWriteDir(t)

	ui := cli.NewMockUi()
	cmd := &FormatCommand{
		Meta: Meta{Ui: ui},
	}
	path := filepath.Join(tmpDir, fmtFixture.filename)

	tests := []string{"-check", ""}
	for _, checkFlag := range tests {
		t.Run(checkFlag, func(t *testing.T) {
			var code int
			var expectedCode int

			if checkFlag == "-check" {
				code = cmd.Run([]string{checkFlag, path})
				expectedCode = 1
			} else {
				code = cmd.Run([]string{path})
				expectedCode = 0

			}
			assert.Equal(t, expectedCode, code)
			assert.Equal(t, fmt.Sprintf("%s\n", path), ui.OutputWriter.String())
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
	input:    []byte(`client   {enabled = true}`),
	golden:   []byte(`client { enabled = true }`),
}
