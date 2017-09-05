package command

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
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

func TestIntegration_Command_RoundTripJob(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	tmpDir, err := ioutil.TempDir("", "nomadtest-rootsecretdir")
	assert.Nil(err)
	defer os.RemoveAll(tmpDir)

	// Start in dev mode so we get a node registration
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	{
		cmd := exec.Command("nomad", "init")
		cmd.Dir = tmpDir
		assert.Nil(cmd.Run())
	}

	{
		cmd := exec.Command("nomad", "run", "example.nomad")
		cmd.Dir = tmpDir
		cmd.Env = []string{fmt.Sprintf("NOMAD_ADDR=%s", url)}
		err := cmd.Run()
		if err != nil && !strings.Contains(err.Error(), "exit status 2") {
			t.Fatalf("error running example.nomad: %v", err)
		}
	}

	{
		cmd := exec.Command("nomad", "inspect", "example")
		cmd.Dir = tmpDir
		cmd.Env = []string{fmt.Sprintf("NOMAD_ADDR=%s", url)}
		out, err := cmd.Output()
		assert.Nil(err)

		var req api.JobRegisterRequest
		dec := json.NewDecoder(bytes.NewReader(out))
		assert.Nil(dec.Decode(&req))

		var resp api.JobRegisterResponse
		_, err = client.Raw().Write("/v1/jobs", req, &resp, nil)
		assert.Nil(err)
		assert.NotZero(resp.EvalID)
	}
}
