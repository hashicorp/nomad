// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package docklog

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"sync"
	"time"

	containerapi "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/nomad/client/lib/fifo"
)

// DockerLogger is a small utility to forward logs from a docker container to a target
// destination
type DockerLogger interface {
	Start(*StartOpts) error
	Stop() error
}

// StartOpts are the options needed to start docker log monitoring
type StartOpts struct {
	// Endpoint sets the docker client endpoint, defaults to environment if not set
	Endpoint string

	// ContainerID of the container to monitor logs for
	ContainerID string
	TTY         bool

	// Stdout path to fifo
	Stdout string
	//Stderr path to fifo
	Stderr string

	// StartTime is the Unix time that the docker logger should fetch logs beginning
	// from
	StartTime int64

	// TLS settings for docker client
	TLSCert string
	TLSKey  string
	TLSCA   string
}

// NewDockerLogger returns an implementation of the DockerLogger interface
func NewDockerLogger(logger hclog.Logger) DockerLogger {
	return &dockerLogger{
		logger: logger,
		doneCh: make(chan interface{}),
	}
}

// dockerLogger implements the DockerLogger interface
type dockerLogger struct {
	logger hclog.Logger

	stdout  io.WriteCloser
	stderr  io.WriteCloser
	stdLock sync.Mutex

	cancelCtx context.CancelFunc
	doneCh    chan interface{}
}

// Start log monitoring
func (d *dockerLogger) Start(opts *StartOpts) error {
	client, err := d.getDockerClient(opts)
	if err != nil {
		return fmt.Errorf("failed to open docker client: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	d.cancelCtx = cancel

	go func() {
		defer close(d.doneCh)

		stdout, stderr, err := d.openStreams(ctx, opts)
		if err != nil {
			d.logger.Error("log streaming ended with terminal error", "error", err)
			return
		}

		sinceTime := time.Unix(opts.StartTime, 0)
		backoff := 0.0

		for {
			logOpts := containerapi.LogsOptions{
				Since:      sinceTime.Format(time.RFC3339),
				Follow:     true,
				ShowStdout: true,
				ShowStderr: true,
			}

			logs, err := client.ContainerLogs(ctx, opts.ContainerID, logOpts)
			if ctx.Err() != nil {
				// If context is terminated then we can safely break the loop
				return
			} else if err == nil {
				backoff = 0.0
			} else if isLoggingTerminalError(err) {
				d.logger.Error("log streaming ended with terminal error", "error", err)
				return
			} else if err != nil {
				backoff = nextBackoff(backoff)
				d.logger.Error("log streaming ended with error", "error", err, "retry_in", backoff)

				time.Sleep(time.Duration(backoff) * time.Second)
			}
			defer logs.Close()

			// attempt to check if the container uses a TTY. if it does, there is no
			// multiplexing or headers in the log stream
			if opts.TTY {
				_, err = io.Copy(stdout, logs)
			} else {
				_, err = stdcopy.StdCopy(stdout, stderr, logs)
			}
			if err != nil && err != io.EOF {
				d.logger.Error("log streaming ended with error", "error", err)
				return
			}

			sinceTime = time.Now()

			container, err := client.ContainerInspect(ctx, opts.ContainerID)
			if err != nil {
				if !errdefs.IsNotFound(err) {
					return
				}
			} else if !container.State.Running {
				return
			}
		}
	}()
	return nil

}

// openStreams open logger stdout/stderr; should be called in a background goroutine to avoid locking up
// process to avoid locking goroutine process
func (d *dockerLogger) openStreams(ctx context.Context, opts *StartOpts) (stdout, stderr io.WriteCloser, err error) {
	d.stdLock.Lock()
	stdoutF, stderrF := d.stdout, d.stderr
	d.stdLock.Unlock()

	if stdoutF != nil && stderrF != nil {
		return stdoutF, stderrF, nil
	}

	// opening a fifo may block indefinitely until a reader end opens, so
	// we preform open() without holding the stdLock, so Stop and interleave.
	// This a defensive measure - logmon (the reader end) should be up and
	// started before dockerLogger is started
	if stdoutF == nil {
		stdoutF, err = fifo.OpenWriter(opts.Stdout)
		if err != nil {
			return nil, nil, err
		}
	}

	if stderrF == nil {
		stderrF, err = fifo.OpenWriter(opts.Stderr)
		if err != nil {
			return nil, nil, err
		}
	}

	if ctx.Err() != nil {
		// Stop was called and don't need files anymore
		stdoutF.Close()
		stderrF.Close()
		return nil, nil, ctx.Err()
	}

	d.stdLock.Lock()
	d.stdout, d.stderr = stdoutF, stderrF
	d.stdLock.Unlock()

	return stdoutF, stderrF, nil
}

// Stop log monitoring
func (d *dockerLogger) Stop() error {
	if d.cancelCtx != nil {
		d.cancelCtx()
	}

	d.stdLock.Lock()
	stdout, stderr := d.stdout, d.stderr
	d.stdLock.Unlock()

	if stdout != nil {
		stdout.Close()
	}
	if stderr != nil {
		stderr.Close()
	}
	return nil
}

func (d *dockerLogger) getDockerClient(opts *StartOpts) (*client.Client, error) {
	var err error
	var merr multierror.Error
	var newClient *client.Client

	// Default to using whatever is configured in docker.endpoint. If this is
	// not specified we'll fall back on NewClientFromEnv which reads config from
	// the DOCKER_* environment variables DOCKER_HOST, DOCKER_TLS_VERIFY, and
	// DOCKER_CERT_PATH. This allows us to lock down the config in production
	// but also accept the standard ENV configs for dev and test.
	if opts.Endpoint != "" {
		if opts.TLSCert+opts.TLSKey+opts.TLSCA != "" {
			d.logger.Debug("using TLS client connection to docker", "endpoint", opts.Endpoint)
			newClient, err = client.NewClientWithOpts(
				client.WithHost(opts.Endpoint),
				client.WithTLSClientConfig(opts.TLSCA, opts.TLSCert, opts.TLSKey),
				client.WithAPIVersionNegotiation(),
			)
			if err != nil {
				merr.Errors = append(merr.Errors, err)
			}
		} else {
			d.logger.Debug("using plaintext client connection to docker", "endpoint", opts.Endpoint)
			newClient, err = client.NewClientWithOpts(
				client.WithHost(opts.Endpoint),
				client.WithAPIVersionNegotiation(),
			)
			if err != nil {
				merr.Errors = append(merr.Errors, err)
			}
		}
	} else {
		d.logger.Debug("using client connection initialized from environment")
		newClient, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			merr.Errors = append(merr.Errors, err)
		}
	}

	return newClient, merr.ErrorOrNil()
}

func isLoggingTerminalError(err error) bool {
	if err == nil {
		return false
	}

	terminals := []string{
		"configured logging driver does not support reading",
		"not implemented",
	}

	for _, c := range terminals {
		if strings.Contains(strings.ToLower(err.Error()), c) {
			return true
		}
	}

	return false
}

// nextBackoff returns the next backoff period in seconds given current backoff
func nextBackoff(backoff float64) float64 {
	if backoff < 0.5 {
		backoff = 0.5
	}

	backoff = backoff * 1.15 * (1.0 + rand.Float64())
	if backoff > 120 {
		backoff = 120
	} else if backoff < 0.5 {
		backoff = 0.5
	}

	return backoff
}
