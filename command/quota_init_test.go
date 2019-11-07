package command

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/mitchellh/cli"
)

func TestQuotaInitCommand_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &QuotaInitCommand{}
}

func TestQuotaInitCommand_Run_HCL(t *testing.T) {
	t.Parallel()
	ui := new(cli.MockUi)
	cmd := &QuotaInitCommand{Meta: Meta{Ui: ui}}

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
	dir, err := ioutil.TempDir("", "nomad")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.RemoveAll(dir)
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Works if the file doesn't exist
	if code := cmd.Run([]string{}); code != 0 {
		t.Fatalf("expect exit code 0, got: %d", code)
	}
	content, err := ioutil.ReadFile(DefaultHclQuotaInitName)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if string(content) != defaultHclQuotaSpec {
		t.Fatalf("unexpected file content\n\n%s", string(content))
	}

	// Fails if the file exists
	if code := cmd.Run([]string{}); code != 1 {
		t.Fatalf("expect exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "exists") {
		t.Fatalf("expect file exists error, got: %s", out)
	}
}

func TestQuotaInitCommand_Run_JSON(t *testing.T) {
	t.Parallel()
	ui := new(cli.MockUi)
	cmd := &QuotaInitCommand{Meta: Meta{Ui: ui}}

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
	dir, err := ioutil.TempDir("", "nomad")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.RemoveAll(dir)
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Works if the file doesn't exist
	if code := cmd.Run([]string{"-json"}); code != 0 {
		t.Fatalf("expect exit code 0, got: %d", code)
	}
	content, err := ioutil.ReadFile(DefaultJsonQuotaInitName)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if string(content) != defaultJsonQuotaSpec {
		t.Fatalf("unexpected file content\n\n%s", string(content))
	}

	// Fails if the file exists
	if code := cmd.Run([]string{"-json"}); code != 1 {
		t.Fatalf("expect exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "exists") {
		t.Fatalf("expect file exists error, got: %s", out)
	}
}
