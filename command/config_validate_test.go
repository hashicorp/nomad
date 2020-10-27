package command

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/mitchellh/cli"
)

func TestConfigValidateCommand_FailWithEmptyDir(t *testing.T) {
	t.Parallel()
	fh, err := ioutil.TempDir("", "nomad")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(fh)

	ui := cli.NewMockUi()
	cmd := &ConfigValidateCommand{Meta: Meta{Ui: ui}}
	args := []string{fh}

	code := cmd.Run(args)
	if code != 1 {
		t.Fatalf("expected exit 1, actual: %d", code)
	}
}

func TestConfigValidateCommand_SucceedWithMinimalConfigFile(t *testing.T) {
	t.Parallel()
	fh, err := ioutil.TempDir("", "nomad")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(fh)

	fp := filepath.Join(fh, "config.hcl")
	err = ioutil.WriteFile(fp, []byte(`data_dir="/"
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
	t.Parallel()
	fh, err := ioutil.TempDir("", "nomad")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(fh)

	fp := filepath.Join(fh, "config.hcl")
	err = ioutil.WriteFile(fp, []byte(`a: b`), 0644)
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
	t.Parallel()
	fh, err := ioutil.TempDir("", "nomad")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(fh)

	fp := filepath.Join(fh, "config.hcl")
	err = ioutil.WriteFile(fp, []byte(`data_dir="../"
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
