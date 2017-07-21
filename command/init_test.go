package command

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/mitchellh/cli"
)

func TestInitCommand_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &InitCommand{}
}

func TestInitCommand_Run(t *testing.T) {
	t.Parallel()
	ui := new(cli.MockUi)
	cmd := &InitCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("expect exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, cmd.Help()) {
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
	content, err := ioutil.ReadFile(DefaultInitName)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if string(content) != defaultJob {
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

func TestInitCommand_defaultJob(t *testing.T) {
	t.Parallel()
	// Ensure the job file is always written with spaces instead of tabs. Since
	// the default job file is embedded in the go file, it's easy for tabs to
	// slip in.
	if strings.Contains("\t", defaultJob) {
		t.Error("default job contains tab character - please convert to spaces")
	}
}
