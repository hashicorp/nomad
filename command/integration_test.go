package command_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"testing"
)

func TestIntegration_Command_NomadInit(t *testing.T) {
	t.Parallel()
	tmpDir, err := ioutil.TempDir("", "nomadtest-rootsecretdir")
	if err != nil {
		t.Fatalf("unable to create tempdir for test: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	{
		cmd := exec.Command("nomad", "init")
		cmd.Dir = tmpDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("error running init: %v", err)
		}
	}

	{
		cmd := exec.Command("nomad", "validate", "example.nomad")
		cmd.Dir = tmpDir
		cmd.Env = []string{`NOMAD_ADDR=http://127.0.0.1:0`}
		if err := cmd.Run(); err != nil {
			t.Fatalf("error validating example.nomad: %v", err)
		}
	}
}
