package docklog

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
)

func TestDocklog(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	client, err := docker.NewClientFromEnv()
	if err != nil {
		t.Skip("docker unavailable:", err)
	}

	containerConf := docker.CreateContainerOptions{
		Config: &docker.Config{
			Cmd: []string{
				"/bin/ash", "-c", "touch /tmp/docklog; tail -f /tmp/docklog",
			},
			Image: "alpine",
		},
		Context: context.Background(),
	}

	container, err := client.CreateContainer(containerConf)
	require.NoError(err)

	defer client.RemoveContainer(docker.RemoveContainerOptions{
		ID:    container.ID,
		Force: true,
	})

	err = client.StartContainer(container.ID, nil)
	require.NoError(err)

	var count int
	for !container.State.Running {
		if count > 10 {
			t.Fatal("timeout waiting for container to start")
		}
		time.Sleep(100 * time.Millisecond)
		container, err = client.InspectContainer(container.ID)
		count++
	}

	stdout := &noopCloser{bytes.NewBufferString("")}
	stderr := &noopCloser{bytes.NewBufferString("")}

	dl := NewDocklog(testlog.HCLogger(t)).(*dockerLogger)
	dl.stdout = stdout
	dl.stderr = stderr
	require.NoError(dl.Start(&StartOpts{
		ContainerID: container.ID,
	}))

	echoToContainer(t, client, container.ID, "abc")
	echoToContainer(t, client, container.ID, "123")

	time.Sleep(2000 * time.Millisecond)
	testutil.WaitForResult(func() (bool, error) {
		act := stdout.String()
		if "abc\n123\n" != act {
			return false, fmt.Errorf("expected abc\\n123\\n for stdout but got %s", act)
		}

		return true, nil
	}, func(err error) {
		require.NoError(err)
	})
}

func echoToContainer(t *testing.T, client *docker.Client, id string, line string) {
	op := docker.CreateExecOptions{
		Container: id,
		Cmd: []string{
			"/bin/ash", "-c",
			fmt.Sprintf("echo %s >>/tmp/docklog", line),
		},
	}

	exec, err := client.CreateExec(op)
	require.NoError(t, err)
	require.NoError(t, client.StartExec(exec.ID, docker.StartExecOptions{Detach: true}))
}

type noopCloser struct {
	*bytes.Buffer
}

func (*noopCloser) Close() error {
	return nil
}
