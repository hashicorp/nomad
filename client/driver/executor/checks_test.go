package executor

import (
	"log"
	"os"
	"strings"
	"testing"
	"time"

	docker "github.com/fsouza/go-dockerclient"

	dstructs "github.com/hashicorp/nomad/client/driver/structs"
	"github.com/hashicorp/nomad/client/testutil"
)

func TestExecScriptCheckNoIsolation(t *testing.T) {
	check := &ExecScriptCheck{
		id:          "foo",
		cmd:         "/bin/echo",
		args:        []string{"hello", "world"},
		taskDir:     "/tmp",
		FSIsolation: false,
	}

	res := check.Run()
	expectedOutput := "hello world"
	expectedExitCode := 0
	if res.Err != nil {
		t.Fatalf("err: %v", res.Err)
	}
	if strings.TrimSpace(res.Output) != expectedOutput {
		t.Fatalf("output expected: %v, actual: %v", expectedOutput, res.Output)
	}

	if res.ExitCode != expectedExitCode {
		t.Fatalf("exitcode expected: %v, actual: %v", expectedExitCode, res.ExitCode)
	}
}

func TestExecScriptCheckWithIsolation(t *testing.T) {
	testutil.ExecCompatible(t)

	execCmd := ExecCommand{Cmd: "/bin/echo", Args: []string{"hello world"}}
	ctx, allocDir := testExecutorContextWithChroot(t)
	defer allocDir.Destroy()

	execCmd.FSIsolation = true
	execCmd.ResourceLimits = true
	execCmd.User = dstructs.DefaultUnpriviledgedUser

	executor := NewExecutor(log.New(os.Stdout, "", log.LstdFlags))

	if err := executor.SetContext(ctx); err != nil {
		t.Fatalf("Unexpected error")
	}

	_, err := executor.LaunchCmd(&execCmd)
	if err != nil {
		t.Fatalf("error in launching command: %v", err)
	}

	check := &ExecScriptCheck{
		id:          "foo",
		cmd:         "/bin/echo",
		args:        []string{"hello", "world"},
		taskDir:     ctx.TaskDir,
		FSIsolation: true,
	}

	res := check.Run()
	expectedOutput := "hello world"
	expectedExitCode := 0
	if res.Err != nil {
		t.Fatalf("err: %v", res.Err)
	}
	if strings.TrimSpace(res.Output) != expectedOutput {
		t.Fatalf("output expected: %v, actual: %v", expectedOutput, res.Output)
	}

	if res.ExitCode != expectedExitCode {
		t.Fatalf("exitcode expected: %v, actual: %v", expectedExitCode, res.ExitCode)
	}
}

func TestDockerScriptCheck(t *testing.T) {
	if !testutil.DockerIsConnected(t) {
		return
	}
	client, err := docker.NewClientFromEnv()
	if err != nil {
		t.Fatalf("error creating docker client: %v", err)
	}

	if err := client.PullImage(docker.PullImageOptions{Repository: "busybox", Tag: "latest"},
		docker.AuthConfiguration{}); err != nil {
		t.Fatalf("error pulling redis: %v", err)
	}

	container, err := client.CreateContainer(docker.CreateContainerOptions{
		Config: &docker.Config{
			Image: "busybox",
			Cmd:   []string{"/bin/sleep", "1000"},
		},
	})
	if err != nil {
		t.Fatalf("error creating container: %v", err)
	}
	defer removeContainer(client, container.ID)

	if err := client.StartContainer(container.ID, container.HostConfig); err != nil {
		t.Fatalf("error starting container", err)
	}

	check := &DockerScriptCheck{
		id:          "1",
		interval:    5 * time.Second,
		containerID: container.ID,
		logger:      log.New(os.Stdout, "", log.LstdFlags),
		cmd:         "/bin/echo",
		args:        []string{"hello", "world"},
	}

	res := check.Run()
	expectedOutput := "hello world"
	expectedExitCode := 0
	if res.Err != nil {
		t.Fatalf("err: %v", res.Err)
	}
	if strings.TrimSpace(res.Output) != expectedOutput {
		t.Fatalf("output expected: %v, actual: %v", expectedOutput, res.Output)
	}

	if res.ExitCode != expectedExitCode {
		t.Fatalf("exitcode expected: %v, actual: %v", expectedExitCode, res.ExitCode)
	}
}

// removeContainer kills and removes a container
func removeContainer(client *docker.Client, containerID string) {
	client.KillContainer(docker.KillContainerOptions{ID: containerID})
	client.RemoveContainer(docker.RemoveContainerOptions{ID: containerID, RemoveVolumes: true, Force: true})
}
