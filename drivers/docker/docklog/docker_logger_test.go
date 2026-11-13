// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package docklog

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	containerapi "github.com/moby/moby/api/types/container"
	mclient "github.com/moby/moby/client"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/ci"
	ctu "github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/testutil"
)

func testRemoteContainerImage() string {
	if runtime.GOOS == "windows" {
		return "hashicorpdev/busybox-windows:server2016-0.1"
	}

	if testutil.IsCI() {
		// use our mirror to avoid rate-limiting in CI
		return "docker.mirror.hashicorp.services/busybox:1"
	}
	return "docker.io/busybox:1"
}

func TestDockerLogger_Success(t *testing.T) {
	ci.Parallel(t)
	ctu.DockerCompatible(t)
	ctx := context.Background()

	containerImage := testRemoteContainerImage()

	client, err := mclient.New(mclient.FromEnv)
	if err != nil {
		t.Skip("docker unavailable:", err)
	}

	if img, err := client.ImageInspect(ctx, containerImage); err != nil || img.ID == "" {
		t.Log("image not found locally, downloading...")
		out, err := client.ImagePull(ctx, containerImage, mclient.ImagePullOptions{})
		must.NoError(t, err, must.Sprint("failed to pull image"))
		defer out.Close()
		io.Copy(os.Stdout, out)
	}

	container, err := client.ContainerCreate(
		ctx,
		mclient.ContainerCreateOptions{
			Config: &containerapi.Config{
				Cmd:   []string{"sh", "-c", "touch ~/docklog; tail -f ~/docklog"},
				Image: containerImage,
			},
		},
	)
	must.NoError(t, err)

	cleanup := func() { client.ContainerRemove(ctx, container.ID, mclient.ContainerRemoveOptions{Force: true}) }
	t.Cleanup(cleanup)

	_, err = client.ContainerStart(ctx, container.ID, mclient.ContainerStartOptions{})
	must.NoError(t, err)

	testutil.WaitForResult(func() (bool, error) {
		container, err := client.ContainerInspect(ctx, container.ID, mclient.ContainerInspectOptions{})
		if err != nil {
			return false, err
		}
		if !container.Container.State.Running {
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

	containerImage := testRemoteContainerImage()

	client, err := mclient.New(mclient.FromEnv)
	if err != nil {
		t.Skip("docker unavailable:", err)
	}

	if img, err := client.ImageInspect(ctx, containerImage); err != nil || img.ID == "" {
		t.Log("image not found locally, downloading...")
		_, err = client.ImagePull(ctx, containerImage, mclient.ImagePullOptions{})
		must.NoError(t, err, must.Sprint("failed to pull image"))
	}

	container, err := client.ContainerCreate(
		ctx,
		mclient.ContainerCreateOptions{
			Config: &containerapi.Config{
				Cmd: []string{
					"sh", "-c", "touch ~/docklog; tail -f ~/docklog",
				},
				Image: containerImage,
				Tty:   true,
			},
		},
	)
	must.NoError(t, err)

	defer client.ContainerRemove(ctx, container.ID, mclient.ContainerRemoveOptions{Force: true})

	_, err = client.ContainerStart(ctx, container.ID, mclient.ContainerStartOptions{})
	must.NoError(t, err)

	testutil.WaitForResult(func() (bool, error) {
		container, err := client.ContainerInspect(ctx, container.ID, mclient.ContainerInspectOptions{})
		if err != nil {
			return false, err
		}
		if !container.Container.State.Running {
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

func echoToContainer(t *testing.T, client *mclient.Client, id string, line string) {
	ctx := context.Background()
	op := mclient.ExecCreateOptions{
		Cmd: []string{
			"/bin/sh", "-c",
			fmt.Sprintf("echo %s >>~/docklog", line),
		},
	}

	exec, err := client.ExecCreate(ctx, id, op)
	must.NoError(t, err)
	_, err = client.ExecStart(ctx, exec.ID, mclient.ExecStartOptions{Detach: true})
	must.NoError(t, err)
}

func TestDockerLogger_LoggingNotSupported(t *testing.T) {
	ci.Parallel(t)
	ctu.DockerCompatible(t)
	ctx := context.Background()

	containerImage := testRemoteContainerImage()

	client, err := mclient.NewClientWithOpts(mclient.FromEnv, mclient.WithAPIVersionNegotiation())
	if err != nil {
		t.Skip("docker unavailable:", err)
	}

	if img, err := client.ImageInspect(ctx, containerImage); err != nil || img.ID == "" {
		t.Log("image not found locally, downloading...")
		_, err = client.ImagePull(ctx, containerImage, mclient.ImagePullOptions{})
		require.NoError(t, err, "failed to pull image")
	}

	container, err := client.ContainerCreate(
		ctx,
		mclient.ContainerCreateOptions{
			Config: &containerapi.Config{
				Cmd: []string{
					"sh", "-c", "touch ~/docklog; tail -f ~/docklog",
				},
				Image: containerImage,
			},
			HostConfig: &containerapi.HostConfig{
				LogConfig: containerapi.LogConfig{
					Type:   "none",
					Config: map[string]string{},
				},
			},
		},
	)
	must.NoError(t, err)

	defer client.ContainerRemove(ctx, container.ID, mclient.ContainerRemoveOptions{Force: true})

	_, err = client.ContainerStart(ctx, container.ID, mclient.ContainerStartOptions{})
	must.NoError(t, err)

	testutil.WaitForResult(func() (bool, error) {
		container, err := client.ContainerInspect(ctx, container.ID, mclient.ContainerInspectOptions{})
		if err != nil {
			return false, err
		}
		if !container.Container.State.Running {
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

type fakeDockerLogsAPI struct {
	stream  func(w http.ResponseWriter, gone <-chan struct{})
	running bool

	// failLogsCalls fails the first n log requests.
	failLogsCalls int32
	logsCalls     atomic.Int32
	// gone makes inspect return 404.
	gone bool
}

func (f *fakeDockerLogsAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.HasSuffix(r.URL.Path, "/logs"):
		if f.logsCalls.Add(1) <= f.failLogsCalls {
			http.Error(w, `{"message":"daemon hiccup"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.docker.multiplexed-stream")
		w.WriteHeader(http.StatusOK)
		f.stream(w, r.Context().Done())
	case strings.HasSuffix(r.URL.Path, "/json"):
		if f.gone {
			http.Error(w, `{"message":"no such container"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"Id":"c1","State":{"Running":%t}}`, f.running)
	case strings.HasSuffix(r.URL.Path, "/_ping"):
		w.WriteHeader(http.StatusOK)
	default:
		http.NotFound(w, r)
	}
}

func writeStderrFrames(w http.ResponseWriter, lines ...string) {
	for _, line := range lines {
		payload := []byte(line + "\n")
		header := []byte{2, 0, 0, 0, 0, 0, 0, 0}
		binary.BigEndian.PutUint32(header[4:], uint32(len(payload)))
		w.Write(header)
		w.Write(payload)
	}
	w.(http.Flusher).Flush()
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (*lockedBuffer) Close() error { return nil }

func startFakeLogger(t *testing.T, api *fakeDockerLogsAPI) (*dockerLogger, *lockedBuffer) {
	t.Helper()
	srv := httptest.NewServer(api)
	t.Cleanup(srv.Close)

	stderr := &lockedBuffer{}
	dl := NewDockerLogger(testlog.HCLogger(t)).(*dockerLogger)
	dl.stdout = &lockedBuffer{}
	dl.stderr = stderr
	must.NoError(t, dl.Start(&StartOpts{
		Endpoint:    strings.Replace(srv.URL, "http://", "tcp://", 1),
		ContainerID: "c1",
	}))
	return dl, stderr
}

func TestDockerLogger_StopReadsTheLastLines(t *testing.T) {
	ci.Parallel(t)

	release := make(chan struct{})
	api := &fakeDockerLogsAPI{running: false}
	api.stream = func(w http.ResponseWriter, gone <-chan struct{}) {
		writeStderrFrames(w, "line 1", "line 2", "line 3", "line 4")
		select {
		case <-release:
			writeStderrFrames(w, "line 5", "line 6", "line 7", "line 8")
		case <-gone:
		}
	}
	dl, stderr := startFakeLogger(t, api)

	testutil.WaitForResult(func() (bool, error) {
		if n := strings.Count(stderr.String(), "\n"); n < 4 {
			return false, fmt.Errorf("want the first 4 lines before stopping, got %d", n)
		}
		return true, nil
	}, func(err error) { must.NoError(t, err) })

	stopped := make(chan error, 1)
	go func() { stopped <- dl.Stop() }()
	close(release)

	select {
	case err := <-stopped:
		must.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not return")
	}
	must.Eq(t, 8, strings.Count(stderr.String(), "\n"), must.Sprintf("stderr:\n%s", stderr.String()))
}

func TestDockerLogger_StopDoesNotWaitForARunningContainer(t *testing.T) {
	ci.Parallel(t)

	api := &fakeDockerLogsAPI{running: true}
	api.stream = func(w http.ResponseWriter, gone <-chan struct{}) {
		writeStderrFrames(w, "still running")
		<-gone
	}
	dl, stderr := startFakeLogger(t, api)
	dl.stopIdleTimeout = 200 * time.Millisecond

	testutil.WaitForResult(func() (bool, error) {
		return strings.Contains(stderr.String(), "still running"), fmt.Errorf("no line yet")
	}, func(err error) { must.NoError(t, err) })

	start := time.Now()
	must.NoError(t, dl.Stop())
	elapsed := time.Since(start)
	must.GreaterEq(t, 200*time.Millisecond, elapsed, must.Sprint("Stop returned before the idle bound"))
	must.Less(t, 2*time.Second, elapsed, must.Sprint("Stop waited past the idle bound"))

	select {
	case <-dl.doneCh:
	case <-time.After(5 * time.Second):
		t.Fatal("the stream goroutine did not end after Stop cancelled it")
	}
}

func TestDockerLogger_LogsErrorIsRetried(t *testing.T) {
	ci.Parallel(t)

	api := &fakeDockerLogsAPI{running: false, failLogsCalls: 1}
	api.stream = func(w http.ResponseWriter, _ <-chan struct{}) {
		writeStderrFrames(w, "after retry")
	}
	dl, stderr := startFakeLogger(t, api)

	testutil.WaitForResultUntil(5*time.Second, func() (bool, error) {
		return strings.Contains(stderr.String(), "after retry"), fmt.Errorf("no line after the retry, %d log requests", api.logsCalls.Load())
	}, func(err error) { must.NoError(t, err) })

	must.NoError(t, dl.Stop())
	must.Eq(t, int32(2), api.logsCalls.Load())
}

func TestDockerLogger_StopsWhenTheContainerIsGone(t *testing.T) {
	ci.Parallel(t)

	api := &fakeDockerLogsAPI{gone: true}
	api.stream = func(w http.ResponseWriter, _ <-chan struct{}) {
		writeStderrFrames(w, "last line")
	}
	dl, stderr := startFakeLogger(t, api)

	select {
	case <-dl.doneCh:
	case <-time.After(5 * time.Second):
		t.Fatal("the logger kept running after the container was gone")
	}

	start := time.Now()
	must.NoError(t, dl.Stop())
	must.Less(t, time.Second, time.Since(start))
	must.StrContains(t, stderr.String(), "last line")
	must.Eq(t, int32(1), api.logsCalls.Load())
}

func TestDockerLogger_StopWaitsWhileTheStreamDelivers(t *testing.T) {
	ci.Parallel(t)

	const lines = 8
	api := &fakeDockerLogsAPI{running: false}
	api.stream = func(w http.ResponseWriter, gone <-chan struct{}) {
		for i := 1; i <= lines; i++ {
			writeStderrFrames(w, fmt.Sprintf("line %d", i))
			select {
			case <-time.After(150 * time.Millisecond):
			case <-gone:
				return
			}
		}
	}
	dl, stderr := startFakeLogger(t, api)
	dl.stopIdleTimeout = 400 * time.Millisecond

	testutil.WaitForResult(func() (bool, error) {
		return strings.Contains(stderr.String(), "line 1"), fmt.Errorf("no line yet")
	}, func(err error) { must.NoError(t, err) })

	must.NoError(t, dl.Stop())
	must.Eq(t, lines, strings.Count(stderr.String(), "\n"), must.Sprintf("stderr:\n%s", stderr.String()))
}
