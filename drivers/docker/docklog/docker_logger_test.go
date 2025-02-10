// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package docklog

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	containerapi "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/shoenig/test/must"
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
	ctx := context.Background()

	containerImage, containerImageName, containerImageTag := testContainerDetails()

	client, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Skip("docker unavailable:", err)
	}

	if img, _, err := client.ImageInspectWithRaw(ctx, containerImage); err != nil || img.ID == "" {
		t.Log("image not found locally, downloading...")
		out, err := client.ImagePull(ctx, fmt.Sprintf("%s:%s", containerImageName, containerImageTag), image.PullOptions{})
		must.NoError(t, err, must.Sprint("failed to pull image"))
		defer out.Close()
		io.Copy(os.Stdout, out)
	}

	container, err := client.ContainerCreate(ctx, &containerapi.Config{
		Cmd: []string{
			"sh", "-c", "touch ~/docklog; tail -f ~/docklog",
		},
		Image: containerImage,
	}, nil, nil, nil, "")
	must.NoError(t, err)

	cleanup := func() { client.ContainerRemove(ctx, container.ID, containerapi.RemoveOptions{Force: true}) }
	t.Cleanup(cleanup)

	err = client.ContainerStart(ctx, container.ID, containerapi.StartOptions{})
	must.NoError(t, err)

	testutil.WaitForResult(func() (bool, error) {
		container, err := client.ContainerInspect(ctx, container.ID)
		if err != nil {
			return false, err
		}
		if !container.State.Running {
			return false, fmt.Errorf("container not running")
		}
		return true, nil
	}, func(err error) {
		must.NoError(t, err)
	})

	stdout := &noopCloser{bytes.NewBuffer(nil)}
	stderr := &noopCloser{bytes.NewBuffer(nil)}

	dl := NewDockerLogger(testlog.HCLogger(t)).(*dockerLogger)
	dl.stdout = stdout
	dl.stderr = stderr
	must.NoError(t, dl.Start(&StartOpts{
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
		must.NoError(t, err)
	})
}

func TestDockerLogger_Success_TTY(t *testing.T) {
	ci.Parallel(t)
	ctu.DockerCompatible(t)
	ctx := context.Background()

	containerImage, containerImageName, containerImageTag := testContainerDetails()

	client, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Skip("docker unavailable:", err)
	}

	if img, _, err := client.ImageInspectWithRaw(ctx, containerImage); err != nil || img.ID == "" {
		t.Log("image not found locally, downloading...")
		_, err = client.ImagePull(ctx, fmt.Sprintf("%s:%s", containerImageName, containerImageTag), image.PullOptions{})
		must.NoError(t, err, must.Sprint("failed to pull image"))
	}

	container, err := client.ContainerCreate(ctx, &containerapi.Config{
		Cmd: []string{
			"sh", "-c", "touch ~/docklog; tail -f ~/docklog",
		},
		Image: containerImage,
		Tty:   true,
	}, nil, nil, nil, "")
	must.NoError(t, err)

	defer client.ContainerRemove(ctx, container.ID, containerapi.RemoveOptions{Force: true})

	err = client.ContainerStart(ctx, container.ID, containerapi.StartOptions{})
	must.NoError(t, err)

	testutil.WaitForResult(func() (bool, error) {
		container, err := client.ContainerInspect(ctx, container.ID)
		if err != nil {
			return false, err
		}
		if !container.State.Running {
			return false, fmt.Errorf("container not running")
		}
		return true, nil
	}, func(err error) {
		must.NoError(t, err)
	})

	stdout := &noopCloser{bytes.NewBuffer(nil)}
	stderr := &noopCloser{bytes.NewBuffer(nil)}

	dl := NewDockerLogger(testlog.HCLogger(t)).(*dockerLogger)
	dl.stdout = stdout
	dl.stderr = stderr
	must.NoError(t, dl.Start(&StartOpts{
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
		must.NoError(t, err)
	})
}

func echoToContainer(t *testing.T, client *client.Client, id string, line string) {
	ctx := context.Background()
	op := containerapi.ExecOptions{
		Cmd: []string{
			"/bin/sh", "-c",
			fmt.Sprintf("echo %s >>~/docklog", line),
		},
	}

	exec, err := client.ContainerExecCreate(ctx, id, op)
	must.NoError(t, err)
	must.NoError(t, client.ContainerExecStart(ctx, exec.ID, containerapi.ExecStartOptions{Detach: true}))
}

func TestDockerLogger_LoggingNotSupported(t *testing.T) {
	ci.Parallel(t)
	ctu.DockerCompatible(t)
	ctx := context.Background()

	containerImage, containerImageName, containerImageTag := testContainerDetails()

	client, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Skip("docker unavailable:", err)
	}

	if img, _, err := client.ImageInspectWithRaw(ctx, containerImage); err != nil || img.ID == "" {
		t.Log("image not found locally, downloading...")
		_, err = client.ImagePull(ctx, fmt.Sprintf("%s:%s", containerImageName, containerImageTag), image.PullOptions{})
		require.NoError(t, err, "failed to pull image")
	}

	container, err := client.ContainerCreate(ctx,
		&containerapi.Config{
			Cmd: []string{
				"sh", "-c", "touch ~/docklog; tail -f ~/docklog",
			},
			Image: containerImage,
		},
		&containerapi.HostConfig{
			LogConfig: containerapi.LogConfig{
				Type:   "none",
				Config: map[string]string{},
			},
		}, nil, nil, "")
	must.NoError(t, err)

	defer client.ContainerRemove(ctx, container.ID, containerapi.RemoveOptions{Force: true})

	err = client.ContainerStart(ctx, container.ID, containerapi.StartOptions{})
	must.NoError(t, err)

	testutil.WaitForResult(func() (bool, error) {
		container, err := client.ContainerInspect(ctx, container.ID)
		if err != nil {
			return false, err
		}
		if !container.State.Running {
			return false, fmt.Errorf("container not running")
		}
		return true, nil
	}, func(err error) {
		must.NoError(t, err)
	})

	stdout := &noopCloser{bytes.NewBuffer(nil)}
	stderr := &noopCloser{bytes.NewBuffer(nil)}

	dl := NewDockerLogger(testlog.HCLogger(t)).(*dockerLogger)
	dl.stdout = stdout
	dl.stderr = stderr
	must.NoError(t, dl.Start(&StartOpts{
		ContainerID: container.ID,
	}))

	select {
	case <-dl.doneCh:
	case <-time.After(10 * time.Second):
		t.Fatal("timeout while waiting for docker_logging to terminate")
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
		errors.New("configured logging driver does not support reading"),
		errors.New("not implemented"),
	}

	for _, err := range terminalErrs {
		must.True(t, isLoggingTerminalError(err), must.Sprintf("error should be terminal: %v", err))
	}

	nonTerminalErrs := []error{
		errors.New("not expected"),
		errors.New("Service unavailable"),
	}

	for _, err := range nonTerminalErrs {
		must.False(t, isLoggingTerminalError(err), must.Sprintf("error should be terminal: %v", err))
	}
}
