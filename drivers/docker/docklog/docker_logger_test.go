// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package docklog

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/ci"
	ctu "github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/testutil"
)

func testContainerDetails() (image string, imageName string, imageTag string) {
	image = testutil.TestBusyboxImage()
	parts := strings.Split(image, ":")
	imageName = parts[0]
	imageTag = parts[1]

	return image, imageName, imageTag
}

func TestDockerLogger_Success(t *testing.T) {
	ci.Parallel(t)
	ctu.DockerCompatible(t)

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
		require.NoError(err, "failed to pull image")
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

func TestDockerLogger_Success_TTY(t *testing.T) {
	ci.Parallel(t)
	ctu.DockerCompatible(t)

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
		require.NoError(err, "failed to pull image")
	}

	containerConf := docker.CreateContainerOptions{
		Config: &docker.Config{
			Cmd: []string{
				"sh", "-c", "touch ~/docklog; tail -f ~/docklog",
			},
			Image: containerImage,
			Tty:   true,
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
		TTY:         true,
	}))

	echoToContainer(t, client, container.ID, "abc")
	echoToContainer(t, client, container.ID, "123")

	testutil.WaitForResult(func() (bool, error) {
		act := stdout.String()
		if "abc\r\n123\r\n" != act {
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
			"/bin/sh", "-c",
			fmt.Sprintf("echo %s >>~/docklog", line),
		},
	}

	exec, err := client.CreateExec(op)
	require.NoError(t, err)
	require.NoError(t, client.StartExec(exec.ID, docker.StartExecOptions{Detach: true}))
}

func TestDockerLogger_LoggingNotSupported(t *testing.T) {
	ci.Parallel(t)
	ctu.DockerCompatible(t)

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
		require.NoError(t, err, "failed to pull image")
	}

	containerConf := docker.CreateContainerOptions{
		Config: &docker.Config{
			Cmd: []string{
				"sh", "-c", "touch ~/docklog; tail -f ~/docklog",
			},
			Image: containerImage,
		},
		HostConfig: &docker.HostConfig{
			LogConfig: docker.LogConfig{
				Type:   "none",
				Config: map[string]string{},
			},
		},
		Context: context.Background(),
	}

	container, err := client.CreateContainer(containerConf)
	require.NoError(t, err)

	defer client.RemoveContainer(docker.RemoveContainerOptions{
		ID:    container.ID,
		Force: true,
	})

	err = client.StartContainer(container.ID, nil)
	require.NoError(t, err)

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
		require.NoError(t, err)
	})

	stdout := &noopCloser{bytes.NewBuffer(nil)}
	stderr := &noopCloser{bytes.NewBuffer(nil)}

	dl := NewDockerLogger(testlog.HCLogger(t)).(*dockerLogger)
	dl.stdout = stdout
	dl.stderr = stderr
	require.NoError(t, dl.Start(&StartOpts{
		ContainerID: container.ID,
	}))

	select {
	case <-dl.doneCh:
	case <-time.After(10 * time.Second):
		require.Fail(t, "timeout while waiting for docker_logging to terminate")
	}
}

type noopCloser struct {
	*bytes.Buffer
}

func (*noopCloser) Close() error {
	return nil
}

func TestNextBackoff(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		currentBackoff float64
		min            float64
		max            float64
	}{
		{0.0, 0.5, 1.15},
		{5.0, 5.0, 16},
		{120, 120, 120},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("case %v", c.currentBackoff), func(t *testing.T) {
			next := nextBackoff(c.currentBackoff)
			t.Logf("computed backoff(%v) = %v", c.currentBackoff, next)

			require.True(t, next >= c.min, "next backoff is smaller than expected")
			require.True(t, next <= c.max, "next backoff is larger than expected")
		})
	}
}

func TestIsLoggingTerminalError(t *testing.T) {
	ci.Parallel(t)

	terminalErrs := []error{
		errors.New("docker returned: configured logging driver does not support reading"),
		&docker.Error{
			Status:  501,
			Message: "configured logging driver does not support reading",
		},
		&docker.Error{
			Status:  501,
			Message: "not implemented",
		},
	}

	for _, err := range terminalErrs {
		require.Truef(t, isLoggingTerminalError(err), "error should be terminal: %v", err)
	}

	nonTerminalErrs := []error{
		errors.New("not expected"),
		&docker.Error{
			Status:  503,
			Message: "Service Unavailable",
		},
	}

	for _, err := range nonTerminalErrs {
		require.Falsef(t, isLoggingTerminalError(err), "error should be terminal: %v", err)
	}
}
