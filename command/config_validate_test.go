package command

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
)

func TestConfigValidateCommand_FailWithEmptyDir(t *testing.T) {
	ci.Parallel(t)
	fh := t.TempDir()

	ui := cli.NewMockUi()
	cmd := &ConfigValidateCommand{Meta: Meta{Ui: ui}}
	args := []string{fh}

	code := cmd.Run(args)
	if code != 1 {
		t.Fatalf("expected exit 1, actual: %d", code)
	}
}

func TestConfigValidateCommand_SucceedWithMinimalConfigFile(t *testing.T) {
	ci.Parallel(t)
	fh := t.TempDir()

	fp := filepath.Join(fh, "config.hcl")
	err := ioutil.WriteFile(fp, []byte(`data_dir="/"
	client {
		enabled = true
	}`), 0644)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	ui := cli.NewMockUi()
	cmd := &ConfigValidateCommand{Meta: Meta{Ui: ui}}
	args := []string{fh}

	code := cmd.Run(args)
	if code != 0 {
		t.Fatalf("expected exit 0, actual: %d", code)
	}
}

func TestConfigValidateCommand_FailOnParseBadConfigFile(t *testing.T) {
	ci.Parallel(t)
	fh := t.TempDir()

	fp := filepath.Join(fh, "config.hcl")
	err := ioutil.WriteFile(fp, []byte(`a: b`), 0644)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	ui := cli.NewMockUi()
	cmd := &ConfigValidateCommand{Meta: Meta{Ui: ui}}
	args := []string{fh}

	code := cmd.Run(args)
	if code != 1 {
		t.Fatalf("expected exit 1, actual: %d", code)
	}
}

func TestConfigValidateCommand_FailOnValidateParsableConfigFile(t *testing.T) {
	ci.Parallel(t)
	fh := t.TempDir()

	fp := filepath.Join(fh, "config.hcl")
	err := ioutil.WriteFile(fp, []byte(`data_dir="../"
	client {
		enabled = true
	}`), 0644)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	ui := cli.NewMockUi()
	cmd := &ConfigValidateCommand{Meta: Meta{Ui: ui}}
	args := []string{fh}

	code := cmd.Run(args)
	if code != 1 {
		t.Fatalf("expected exit 1, actual: %d", code)
	}
}
