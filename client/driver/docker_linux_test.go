package driver

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/nomad/client/testutil"
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
	require.Equal(t, []byte("https://registry.local:5000"), content)
}

func TestDockerDriver_PidsLimit(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	task, _, _ := dockerTask(t)
	task.Config["pids_limit"] = "1"
	task.Config["command"] = "/bin/sh"
	task.Config["args"] = []string{"-c", "sleep 1000"}

	ctx := testDockerDriverContexts(t, task)
	defer ctx.Destroy()
	d := NewDockerDriver(ctx.DriverCtx)

	// TODO: current log capture of docker driver is broken
	// so we must fetch logs from docker daemon directly
	// which works in Linux as well as Mac
	d.(*DockerDriver).DriverContext.config.Options[dockerCleanupContainerConfigOption] = "false"

	// Copy the image into the task's directory
	copyImage(t, ctx.ExecCtx.TaskDir, "busybox.tar")

	_, err := d.Prestart(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("error in prestart: %v", err)
	}
	resp, err := d.Start(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	h := resp.Handle.(*DockerHandle)
	defer h.client.RemoveContainer(docker.RemoveContainerOptions{
		ID:            h.containerID,
		RemoveVolumes: true,
		Force:         true,
	})

	defer resp.Handle.Kill()

	select {
	case res := <-resp.Handle.WaitCh():
		if res.Successful() {
			t.Fatalf("expected error, but container exited successful")
		}

		// /bin/sh exits with 2
		if res.ExitCode != 2 {
			t.Fatalf("expected exit code of 2 but found %v", res.ExitCode)
		}
	case <-time.After(time.Duration(tu.TestMultiplier()*5) * time.Second):
		t.Fatalf("timeout")
	}

	// XXX Logging doesn't work on OSX so just test on Linux
	// Check that data was written to the directory.
	var act bytes.Buffer
	err = h.client.Logs(docker.LogsOptions{
		Container:   h.containerID,
		Stderr:      true,
		ErrorStream: &act,
	})
	if err != nil {
		t.Fatalf("error in fetching logs: %v", err)

	}

	exp := "can't fork"
	if !strings.Contains(act.String(), exp) {
		t.Fatalf("Expected failed fork: %q", act)
	}

}
