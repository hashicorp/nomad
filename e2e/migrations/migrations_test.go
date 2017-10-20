package e2e

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// requires nomad executable on the path
func startCluster() (func(), error) {
	serverCmd := exec.Command("nomad", "agent", "-config", "server.hcl")
	err := serverCmd.Start()
	if err != nil {
		return func() {}, err
	}

	time.Sleep(10 * time.Second)

	clientCmd := exec.Command("nomad", "agent", "-config", "client1.hcl")
	err = clientCmd.Start()
	if err != nil {
		return func() {}, err
	}

	time.Sleep(10 * time.Second)

	secondClientCmd := exec.Command("nomad", "agent", "-config", "client2.hcl")
	err = secondClientCmd.Start()
	if err != nil {
		return func() {}, err
	}

	time.Sleep(10 * time.Second)

	f := func() {
		clientCmd.Process.Kill()
		secondClientCmd.Process.Kill()
		serverCmd.Process.Kill()
	}

	return f, nil
}

func TestJobMigrations(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	stopCluster, err := startCluster()
	assert.Nil(err)
	defer stopCluster()

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

	jobCmd := exec.Command("nomad", "run", fh.Name())
	err = jobCmd.Run()
	assert.Nil(err)

	time.Sleep(20 * time.Second)

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

	secondJobCmd := exec.Command("nomad", "run", fh2.Name())
	err = secondJobCmd.Run()
	assert.Nil(err)

	time.Sleep(20 * time.Second)

	jobStatusCmd := exec.Command("nomad", "job", "status", "sleep")
	var jobStatusOut bytes.Buffer
	jobStatusCmd.Stdout = &jobStatusOut

	err = jobStatusCmd.Run()
	assert.Nil(err)

	jobOutput := jobStatusOut.String()
	assert.NotContains(jobOutput, "failed")
	assert.Contains(jobOutput, "complete")
}
