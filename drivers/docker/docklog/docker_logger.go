package docklog

import (
	"fmt"
	"io"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
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
	TTY         bool

	// Stdout path to fifo
	Stdout string
	//Stderr path to fifo
	Stderr string

	// StartTime is the Unix time that the docker logger should fetch logs beginning
	// from
	StartTime int64

	// GracePeriod is the time in which we can attempt to collect logs from a stopped
	// container, if none have been read yet.
	GracePeriod time.Duration

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
	logger      hclog.Logger
	containerID string

	stdout  io.WriteCloser
	stderr  io.WriteCloser
	stdLock sync.Mutex

	// containerDoneCtx is called when the container dies, indicating that there will be no
	// more logs to be read.
	containerDoneCtx context.CancelFunc

	// read indicates whether we have read anything from the logs.  This is manipulated
	// using the sync package via multiple goroutines.
	read int64

	doneCh chan interface{}
	// readDelay is used in testing to delay reads, simulating race conditions between
	// container exits and reading
	readDelay *time.Duration
}

// Start log monitoring
func (d *dockerLogger) Start(opts *StartOpts) error {
	d.containerID = opts.ContainerID

	client, err := d.getDockerClient(opts)
	if err != nil {
		return fmt.Errorf("failed to open docker client: %v", err)
	}

	// Set up a ctx which is called when the container quits.
	containerDoneCtx, cancel := context.WithCancel(context.Background())
	d.containerDoneCtx = cancel

	// Set up a ctx which will be cancelled when we stop reading logs.  This
	// grace period allows us to collect logs from stopped containers if none
	// have yet been read.
	ctx, cancelStreams := context.WithCancel(context.Background())
	go func() {
		<-containerDoneCtx.Done()

		// Wait until we've read from the logs to exit.
		timeout := time.After(opts.GracePeriod)
		for {
			select {
			case <-time.After(time.Second):
				if d.read > 0 {
					cancelStreams()
					return
				}
			case <-timeout:
				cancelStreams()
				return
			}
		}

	}()

	go func() {
		defer close(d.doneCh)
		defer d.cleanup()

		if d.readDelay != nil {
			// Allows us to simulate reading from stopped containers in testing.
			<-time.After(*d.readDelay)
		}

		stdout, stderr, err := d.openStreams(ctx, opts)
		if err != nil {
			d.logger.Error("log streaming ended with terminal error", "error", err)
			return
		}

		sinceTime := time.Unix(opts.StartTime, 0)
		backoff := 0.0

		for {
			logOpts := docker.LogsOptions{
				Context:      ctx,
				Container:    opts.ContainerID,
				OutputStream: stdout,
				ErrorStream:  stderr,
				Since:        sinceTime.Unix(),
				Follow:       true,
				Stdout:       true,
				Stderr:       true,

				// When running in TTY, we must use a raw terminal.
				// If not, we set RawTerminal to false to allow docker client
				// to interpret special stdout/stderr messages
				RawTerminal: opts.TTY,
			}

			err := client.Logs(logOpts)
			// If we've been reading logs and the container is done we can safely break
			// the loop
			if containerDoneCtx.Err() != nil && d.read != 0 {
				return
			} else if ctx.Err() != nil {
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

			sinceTime = time.Now()

			_, err = client.InspectContainerWithOptions(docker.InspectContainerOptions{
				ID: opts.ContainerID,
			})
			if err != nil {
				_, notFoundOk := err.(*docker.NoSuchContainer)
				if !notFoundOk {
					return
				}
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

	d.stdLock.Lock()
	d.stdout, d.stderr = stdoutF, stderrF
	d.stdLock.Unlock()

	return d.streamCopier(stdoutF), d.streamCopier(stderrF), nil
}

// streamCopier copies into the given writer and sets a flag indicating that
// we have read some logs.
func (d *dockerLogger) streamCopier(to io.WriteCloser) io.WriteCloser {
	return &copier{read: &d.read, writer: to}
}

type copier struct {
	read   *int64
	writer io.WriteCloser
}

func (c *copier) Write(p []byte) (n int, err error) {
	if *c.read == 0 {
		atomic.AddInt64(c.read, int64(len(p)))
	}
	return c.writer.Write(p)
}

func (c *copier) Close() error {
	return c.writer.Close()
}

// Stop log monitoring
func (d *dockerLogger) Stop() error {
	if d.containerDoneCtx != nil {
		d.containerDoneCtx()
	}
	return nil
}

func (d *dockerLogger) cleanup() error {
	d.logger.Debug("cleaning up", "container_id", d.containerID)

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

func isLoggingTerminalError(err error) bool {
	if err == nil {
		return false
	}

	if apiErr, ok := err.(*docker.Error); ok {
		switch apiErr.Status {
		case 501:
			return true
		}
	}

	terminals := []string{
		"configured logging driver does not support reading",
	}

	for _, c := range terminals {
		if strings.Contains(err.Error(), c) {
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
