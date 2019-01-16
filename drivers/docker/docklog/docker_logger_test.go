package docklog

import (
	"bytes"
	"fmt"
	"runtime"
	"testing"

	docker "github.com/fsouza/go-dockerclient"
	ctu "github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
)

func testContainerDetails() (image string, imageName string, imageTag string) {
	if runtime.GOOS == "windows" {
		return "dantoml/busybox-windows:08012019",
			"dantoml/busybox-windows",
			"08012019"
	}

	return "busybox:1", "busybox", "1"
}

func TestDockerLogger(t *testing.T) {
	ctu.DockerCompatible(t)

	t.Parallel()
	require := require.New(t)

	containerImage, containerImageName, containerImageTag := testContainerDetails()

	client, err := docker.NewClientFromEnv()
	if err != nil {
		t.Skip("docker unavailable:", err)
	}

	if img, err := client.InspectImage(containerImage); err != nil || img == nil {
		t.Log("image not found locally, downloading...")
		err = client.PullImage(docker.PullImageOptions{
			Repository: containerImageName,
			Tag:        containerImageTag,
		}, docker.AuthConfiguration{})
		if err != nil {
			t.Fatalf("failed to pull image: %v", err)
		}
	}

	containerConf := docker.CreateContainerOptions{
		Config: &docker.Config{
			Cmd: []string{
				"sh", "-c", "touch ~/docklog; tail -f ~/docklog",
			},
			Image: containerImage,
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

	testutil.WaitForResult(func() (bool, error) {
		container, err = client.InspectContainer(container.ID)
		if err != nil {
			return false, err
		}
		if !container.State.Running {
			return false, fmt.Errorf("container not running")
		}
		return true, nil
	}, func(err error) {
		require.NoError(err)
	})

	stdout := &noopCloser{bytes.NewBuffer(nil)}
	stderr := &noopCloser{bytes.NewBuffer(nil)}

	dl := NewDockerLogger(testlog.HCLogger(t)).(*dockerLogger)
	dl.stdout = stdout
	dl.stderr = stderr
	require.NoError(dl.Start(&StartOpts{
		ContainerID: container.ID,
	}))

	echoToContainer(t, client, container.ID, "abc")
	echoToContainer(t, client, container.ID, "123")

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
			"ash", "-c",
			fmt.Sprintf("echo %s >>~/docklog", line),
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
