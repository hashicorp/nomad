package e2e

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func isSuccess(execCmd *exec.Cmd, retries int, keyword string) (bool, string) {
	numAttempts := retries
	success := false
	var out bytes.Buffer

	for numAttempts > 0 {
		time.Sleep(2 * time.Second)

		cmd := *execCmd
		cmd.Stdout = &out

		err := cmd.Run()
		if err != nil {
			return false, ""
		}

		success = (out.String() != "" && !strings.Contains(out.String(), keyword))
		if success {
			return success, out.String()
		}

		out.Reset()
		numAttempts -= 1
	}

	return success, out.String()
}

func allNomadNodesAreReady(retries int) (bool, string) {
	cmd := exec.Command("nomad", "node-status")
	return isSuccess(cmd, retries, "initializing")
}

func jobIsReady(retries int, jobName string) (bool, string) {
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

	isReady, _ := allNomadNodesAreReady(10)
	assert.True(isReady)

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

	isFirstJobReady, firstJoboutput := jobIsReady(20, "sleep")
	assert.True(isFirstJobReady)
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

	isReady, jobOutput := jobIsReady(20, "sleep")
	assert.True(isReady)
	assert.NotContains(jobOutput, "failed")
	assert.Contains(jobOutput, "complete")
}
