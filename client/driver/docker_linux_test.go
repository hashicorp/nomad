package driver

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	task.Config["args"] = []string{"-c", "sleep 2 & sleep 2"}

	ctx := testDockerDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewDockerDriver(ctx.DriverCtx)

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
	defer resp.Handle.Kill()

	select {
	case res := <-resp.Handle.WaitCh():
		if res.Successful() {
			t.Fatalf("expected error, but container exited successful")
		}
	case <-time.After(time.Duration(tu.TestMultiplier()*5) * time.Second):
		t.Fatalf("timeout")
	}

	// XXX Logging doesn't work on OSX so just test on Linux
	// Check that data was written to the directory.
	outputFile := filepath.Join(ctx.ExecCtx.TaskDir.LogDir, "redis-demo.stderr.0")
	act, err := ioutil.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Couldn't read expected output: %v", err)
	}

	exp := "can't fork"
	if !strings.Contains(string(act), exp) {
		t.Fatalf("Expected failed fork: %q", act)
	}

}
