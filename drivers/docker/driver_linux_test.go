package docker

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/freeport"
	tu "github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func TestDockerDriver_authFromHelper(t *testing.T) {
	dir, err := ioutil.TempDir("", "test-docker-driver_authfromhelper")
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	helperPayload := "{\"Username\":\"hashi\",\"Secret\":\"nomad\"}"
	helperContent := []byte(fmt.Sprintf("#!/bin/sh\ncat > %s/helper-$1.out;echo '%s'", dir, helperPayload))

	helperFile := filepath.Join(dir, "docker-credential-testnomad")
	err = ioutil.WriteFile(helperFile, helperContent, 0777)
	require.NoError(t, err)

	path := os.Getenv("PATH")
	os.Setenv("PATH", fmt.Sprintf("%s:%s", path, dir))
	defer os.Setenv("PATH", path)

	helper := authFromHelper("testnomad")
	creds, err := helper("registry.local:5000/repo/image")
	require.NoError(t, err)
	require.NotNil(t, creds)
	require.Equal(t, "hashi", creds.Username)
	require.Equal(t, "nomad", creds.Password)

	if _, err := os.Stat(filepath.Join(dir, "helper-get.out")); os.IsNotExist(err) {
		t.Fatalf("Expected helper-get.out to exist")
	}
	content, err := ioutil.ReadFile(filepath.Join(dir, "helper-get.out"))
	require.NoError(t, err)
	require.Equal(t, "registry.local:5000", string(content))
}

func TestDockerDriver_PluginConfig_PidsLimit(t *testing.T) {
	if !tu.IsCI() {
		t.Parallel()
	}

	dh := dockerDriverHarness(t, nil)
	driver := dh.Impl().(*Driver)
	driver.config.PidsLimit = 5

	task, cfg, ports := dockerTask(t)
	defer freeport.Return(ports)
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	cfg.PidsLimit = 7
	_, err := driver.createContainerConfig(task, cfg, "org/repo:0.1")
	require.Error(t, err)
	require.Contains(t, err.Error(), `pids_limit cannot be greater than nomad plugin config pids_limit`)

	// Task PidsLimit should override plugin PidsLimit.
	cfg.PidsLimit = 3
	opts, err := driver.createContainerConfig(task, cfg, "org/repo:0.1")
	require.NoError(t, err)
	require.Equal(t, helper.Int64ToPtr(3), opts.HostConfig.PidsLimit)
}

func TestDockerDriver_PidsLimit(t *testing.T) {
	if !tu.IsCI() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)
	require := require.New(t)

	task, cfg, ports := dockerTask(t)
	defer freeport.Return(ports)
	cfg.PidsLimit = 1
	cfg.Command = "/bin/sh"
	cfg.Args = []string{"-c", "sleep 5 & sleep 5 & sleep 5"}
	require.NoError(task.EncodeConcreteDriverConfig(cfg))

	_, driver, _, cleanup := dockerSetup(t, task, nil)
	defer cleanup()

	driver.WaitUntilStarted(task.ID, time.Duration(tu.TestMultiplier()*5)*time.Second)

	// XXX Logging doesn't work on OSX so just test on Linux
	// Check that data was written to the directory.
	outputFile := filepath.Join(task.TaskDir().LogDir, "redis-demo.stderr.0")
	exp := "can't fork"
	tu.WaitForResult(func() (bool, error) {
		act, err := ioutil.ReadFile(outputFile)
		if err != nil {
			return false, err
		}
		if !strings.Contains(string(act), exp) {
			return false, fmt.Errorf("Expected %q in output %q", exp, string(act))
		}
		return true, nil
	}, func(err error) {
		require.NoError(err)
	})
}
