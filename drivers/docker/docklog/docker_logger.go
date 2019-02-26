package docklog

import (
	"fmt"
	"io"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	hclog "github.com/hashicorp/go-hclog"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/lib/fifo"
	"golang.org/x/net/context"
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
	return &dockerLogger{logger: logger}
}

// dockerLogger implements the DockerLogger interface
type dockerLogger struct {
	logger hclog.Logger

	stdout    io.WriteCloser
	stderr    io.WriteCloser
	cancelCtx context.CancelFunc
}

// Start log monitoring
func (d *dockerLogger) Start(opts *StartOpts) error {
	client, err := d.getDockerClient(opts)
	if err != nil {
		return fmt.Errorf("failed to open docker client: %v", err)
	}

	if d.stdout == nil {
		stdout, err := fifo.Open(opts.Stdout)
		if err != nil {
			return fmt.Errorf("failed to open fifo for path %s: %v", opts.Stdout, err)
		}
		d.stdout = stdout
	}
	if d.stderr == nil {
		stderr, err := fifo.Open(opts.Stderr)
		if err != nil {
			return fmt.Errorf("failed to open fifo for path %s: %v", opts.Stdout, err)
		}
		d.stderr = stderr
	}
	ctx, cancel := context.WithCancel(context.Background())
	d.cancelCtx = cancel

	go func() {
		sinceTime := time.Unix(opts.StartTime, 0)

		for {
			logOpts := docker.LogsOptions{
				Context:      ctx,
				Container:    opts.ContainerID,
				OutputStream: d.stdout,
				ErrorStream:  d.stderr,
				Since:        sinceTime.Unix(),
				Follow:       true,
				Stdout:       true,
				Stderr:       true,
			}

			err := client.Logs(logOpts)
			if ctx.Err() != nil {
				// If context is terminated then we can safely break the loop
				return
			} else if err != nil {
				d.logger.Error("Log streaming ended with error", "error", err)
			}

			sinceTime = time.Now()

			container, err := client.InspectContainer(opts.ContainerID)
			if err != nil {
				_, notFoundOk := err.(*docker.NoSuchContainer)
				if !notFoundOk {
					return
				}
			} else if !container.State.Running {
				return
			}
		}
	}()
	return nil

}

// Stop log monitoring
func (d *dockerLogger) Stop() error {
	if d.cancelCtx != nil {
		d.cancelCtx()
	}
	if d.stdout != nil {
		d.stdout.Close()
	}
	if d.stderr != nil {
		d.stderr.Close()
	}
	return nil
}

func (d *dockerLogger) getDockerClient(opts *StartOpts) (*docker.Client, error) {
	var err error
	var merr multierror.Error
	var newClient *docker.Client

	// Default to using whatever is configured in docker.endpoint. If this is
	// not specified we'll fall back on NewClientFromEnv which reads config from
	// the DOCKER_* environment variables DOCKER_HOST, DOCKER_TLS_VERIFY, and
	// DOCKER_CERT_PATH. This allows us to lock down the config in production
	// but also accept the standard ENV configs for dev and test.
	if opts.Endpoint != "" {
		if opts.TLSCert+opts.TLSKey+opts.TLSCA != "" {
			d.logger.Debug("using TLS client connection to docker", "endpoint", opts.Endpoint)
			newClient, err = docker.NewTLSClient(opts.Endpoint, opts.TLSCert, opts.TLSKey, opts.TLSCA)
			if err != nil {
				merr.Errors = append(merr.Errors, err)
			}
		} else {
			d.logger.Debug("using plaintext client connection to docker", "endpoint", opts.Endpoint)
			newClient, err = docker.NewClient(opts.Endpoint)
			if err != nil {
				merr.Errors = append(merr.Errors, err)
			}
		}
	} else {
		d.logger.Debug("using client connection initialized from environment")
		newClient, err = docker.NewClientFromEnv()
		if err != nil {
			merr.Errors = append(merr.Errors, err)
		}
	}

	return newClient, merr.ErrorOrNil()
}
