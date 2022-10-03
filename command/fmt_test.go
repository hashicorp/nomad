package command

import (
	"os"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFmtNomadJob(t *testing.T) {
	ci.Parallel(t)

	ui := cli.NewMockUi()

	cmd := &FormatCommand{
		Meta: Meta{Ui: ui},
	}

	if code := cmd.Run([]string{"testdata/example-basic.nomad", "testdata/example-vault.nomad"}); code != 0 {
		t.Fatalf("expected code 0, got %d", code)
	}
}

func TestFmtNomadJobDontOverwrite(t *testing.T) {
	ci.Parallel(t)

	ui := cli.NewMockUi()
	cmd := &FormatCommand{
		Meta: Meta{Ui: ui},
	}

	assert.Equal(t, 0, cmd.Run([]string{"-overwrite=false", "testdata/example-basic.nomad"}))

	fileBytes, err := os.ReadFile("testdata/example-basic.nomad")
	require.NoError(t, err)

	assert.Contains(t, ui.OutputWriter.String(), string(fileBytes))
}

func TestFmtFileDoesNotExist(t *testing.T) {
	ci.Parallel(t)

	ui := cli.NewMockUi()
	cmd := &FormatCommand{
		Meta: Meta{Ui: ui},
	}

	assert.Equal(t, 1, cmd.Run([]string{"file/does/not/exist.hcl"}))
}

func TestFmtWrongHCLFile(t *testing.T) {
	ci.Parallel(t)

	ui := cli.NewMockUi()

	cmd := &FormatCommand{
		Meta: Meta{Ui: ui},
	}

	bytes, err := os.ReadFile("testdata/fmt/bad-nomad.hcl")
	require.NoError(t, err)

	f, err := os.CreateTemp("", "nomad-fmt-")
	require.NoError(t, err)

	_, err = f.Write(bytes)
	require.NoError(t, err)

	assert.Equal(t, 0, cmd.Run([]string{f.Name()}))

	expectedBytes, err := os.ReadFile("testdata/fmt/bad-nomad-after-fmt.hcl")
	require.NoError(t, err)

	bytesAfterFmt, err := os.ReadFile(f.Name())
	require.NoError(t, err)

	assert.Equal(t, expectedBytes, bytesAfterFmt, "HCL file was not formatted, but it should be")
}

func TestFmtWrongHCLFileWithCheck(t *testing.T) {
	ci.Parallel(t)

	ui := cli.NewMockUi()
	cmd := &FormatCommand{
		Meta: Meta{Ui: ui},
	}

	bytes, err := os.ReadFile("testdata/fmt/bad-nomad.hcl")
	require.NoError(t, err)

	f, err := os.CreateTemp("", "nomad-fmt-")
	require.NoError(t, err)

	_, err = f.Write(bytes)
	require.NoError(t, err)

	assert.Equal(t, 1, cmd.Run([]string{"-check", f.Name()}))

	bytesAfterFmt, err := os.ReadFile(f.Name())
	require.NoError(t, err)

	assert.Equal(t, bytes, bytesAfterFmt, "HCL file was formatted, but it should not be")
}
