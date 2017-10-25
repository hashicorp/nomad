package e2e

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/hashicorp/nomad/testutil"
)

func isSuccess(execCmd *exec.Cmd, retries int, keyword string) (string, error) {
	var successOut string
	var err error

	testutil.WaitForResultRetries(2000, func() (bool, error) {
		var out bytes.Buffer
		cmd := *execCmd
		cmd.Stdout = &out
		err := cmd.Run()

		if err != nil {
			return false, err
		}

		success := (out.String() != "" && !strings.Contains(out.String(), keyword))
		if !success {
			out.Reset()
			return false, err
		}

		successOut = out.String()
		return true, nil
	}, func(cmd_err error) {
		err = cmd_err
	})

	return successOut, err
}

func allNomadNodesAreReady(retries int) (string, error) {
	cmd := exec.Command("nomad", "node-status")
	return isSuccess(cmd, retries, "initializing")
}

func jobIsReady(retries int, jobName string) (string, error) {
	cmd := exec.Command("nomad", "job", "status", jobName)
	return isSuccess(cmd, retries, "pending")
}

// requires nomad executable on the path
func startCluster(clusterConfig []string) (func(), error) {
	cmds := make([]*exec.Cmd, 0)

	for _, agentConfig := range clusterConfig {
		cmd := exec.Command("nomad", "agent", "-config", agentConfig)
		err := cmd.Start()

		if err != nil {
			return func() {}, err
		}

		cmds = append(cmds, cmd)
	}

	f := func() {
		for _, cmd := range cmds {
			cmd.Process.Kill()
		}
	}

	return f, nil
}

func TestJobMigrations(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	clusterConfig := []string{"server.hcl", "client1.hcl", "client2.hcl"}
	stopCluster, err := startCluster(clusterConfig)
	assert.Nil(err)
	defer stopCluster()

	_, err = allNomadNodesAreReady(10)
	assert.Nil(err)

	fh, err := ioutil.TempFile("", "nomad-sleep-1")
	assert.Nil(err)

	defer os.Remove(fh.Name())
	_, err = fh.WriteString(`
	job "sleep" {
		type = "batch"
		datacenters = ["dc1"]
		constraint {
			attribute = "${meta.secondary}"
			value     = 1
		}
		group "group1" {
			restart {
				mode = "fail"
			}
			count = 1
			ephemeral_disk {
				migrate = true
				sticky = true
			}
			task "sleep" {
				template {
					data = "hello world"
					destination = "/local/hello-world"
				}
				driver = "exec"
				config {
					command = "/bin/sleep"
					args = [ "infinity" ]
				}
			}
		}
	}`)

	assert.Nil(err)

	jobCmd := exec.Command("nomad", "run", fh.Name())
	err = jobCmd.Run()
	assert.Nil(err)

	firstJoboutput, err := jobIsReady(20, "sleep")
	assert.Nil(err)
	assert.NotContains(firstJoboutput, "failed")

	fh2, err := ioutil.TempFile("", "nomad-sleep-2")
	assert.Nil(err)

	defer os.Remove(fh2.Name())
	_, err = fh2.WriteString(`
	job "sleep" {
		type = "batch"
		datacenters = ["dc1"]
		constraint {
			attribute = "${meta.secondary}"
			value     = 1
		}
		group "group1" {
			restart {
				mode     = "fail"
			}
			count = 1
			ephemeral_disk {
				migrate = true
				sticky = true
			}
			task "sleep" {
				driver = "exec"

				config {
					command = "test"
					args = [ "-f", "/local/hello-world" ]
				}
			}
		}
	}`)

	assert.Nil(err)

	secondJobCmd := exec.Command("nomad", "run", fh2.Name())
	err = secondJobCmd.Run()
	assert.Nil(err)

	jobOutput, err := jobIsReady(20, "sleep")
	assert.Nil(err)
	assert.NotContains(jobOutput, "failed")
	assert.Contains(jobOutput, "complete")
}
