package command

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/mitchellh/cli"
)

func TestValidateCommand_Implements(t *testing.T) {
	var _ cli.Command = &ValidateCommand{}
}

func TestValidateCommand(t *testing.T) {
	ui := new(cli.MockUi)
	cmd := &ValidateCommand{Meta: Meta{Ui: ui}}

	fh, err := ioutil.TempFile("", "nomad")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(fh.Name())
	_, err = fh.WriteString(`
job "job1" {
	type = "service"
	datacenters = [ "dc1" ]
	group "group1" {
		count = 1
		task "task1" {
			driver = "exec"
			resources = {
				cpu = 1000
				disk = 150
				memory = 512
			}
		}
	}
}`)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if code := cmd.Run([]string{fh.Name()}); code != 0 {
		t.Fatalf("expect exit 0, got: %d: %s", code, ui.ErrorWriter.String())
	}
}

func TestValidateCommand_Fails(t *testing.T) {
	ui := new(cli.MockUi)
	cmd := &ValidateCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, cmd.Help()) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails when specified file does not exist
	if code := cmd.Run([]string{"/unicorns/leprechauns"}); code != 1 {
		t.Fatalf("expect exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error opening") {
		t.Fatalf("expect parsing error, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails on invalid HCL
	fh1, err := ioutil.TempFile("", "nomad")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(fh1.Name())
	if _, err := fh1.WriteString("nope"); err != nil {
		t.Fatalf("err: %s", err)
	}
	if code := cmd.Run([]string{fh1.Name()}); code != 1 {
		t.Fatalf("expect exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error parsing") {
		t.Fatalf("expect parsing error, got: %s", err)
	}
	ui.ErrorWriter.Reset()

	// Fails on invalid job spec
	fh2, err := ioutil.TempFile("", "nomad")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(fh2.Name())
	if _, err := fh2.WriteString(`job "job1" {}`); err != nil {
		t.Fatalf("err: %s", err)
	}
	if code := cmd.Run([]string{fh2.Name()}); code != 1 {
		t.Fatalf("expect exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error validating") {
		t.Fatalf("expect validation error, got: %s", out)
	}
	ui.ErrorWriter.Reset()
}

func TestValidateCommand_From_STDIN(t *testing.T) {
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	ui := new(cli.MockUi)
	cmd := &ValidateCommand{
		Meta:   Meta{Ui: ui},
		Helper: Helper{testStdin: stdinR},
	}

	go func() {
		stdinW.WriteString(`
job "job1" {
  type = "service"
  datacenters = [ "dc1" ]
  group "group1" {
                count = 1
                task "task1" {
                        driver = "exec"
                        resources = {
                                cpu = 1000
                                disk = 150
                                memory = 512
                        }
                }
        }
}`)
		stdinW.Close()
	}()

	args := []string{"-"}
	if code := cmd.Run(args); code != 0 {
		t.Fatalf("expected exit code 0, got %d: %q", code, ui.ErrorWriter.String())
	}
	ui.ErrorWriter.Reset()
}

func TestValidateCommand_From_URL(t *testing.T) {
	ui := new(cli.MockUi)
	cmd := &RunCommand{
		Meta: Meta{Ui: ui},
	}

	args := []string{"https://example.com/foo/bar"}
	if code := cmd.Run(args); code != 1 {
		t.Fatalf("expected exit code 1, got %d: %q", code, ui.ErrorWriter.String())
	}

	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error reading URL") {
		t.Fatalf("expected runtime error, got: %s", out)
	}
}
