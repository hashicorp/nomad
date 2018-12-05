package structs

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"

	"github.com/hashicorp/nomad/client/lib/fifo"
	cstructs "github.com/hashicorp/nomad/client/structs"
)

// Executor is the interface which allows a driver to launch and supervise
// a process
type Executor interface {
	// Launch a user process configured by the given ExecCommand
	Launch(launchCmd *ExecCommand) (*ProcessState, error)

	// Wait blocks until the process exits or an error occures
	Wait(ctx context.Context) (*ProcessState, error)

	// Shutdown will shutdown the executor by stopping the user process,
	// cleaning up and resources created by the executor. The shutdown sequence
	// will first send the given signal to the process. This defaults to "SIGINT"
	// if not specified. The executor will then wait for the process to exit
	// before cleaning up other resources. If the executor waits longer than the
	// given grace period, the process is forcefully killed.
	//
	// To force kill the user process, gracePeriod can be set to 0.
	Shutdown(signal string, gracePeriod time.Duration) error

	// UpdateResources updates any resource isolation enforcement with new
	// constraints if supported.
	UpdateResources(*Resources) error

	// Version returns the executor API version
	Version() (*ExecutorVersion, error)

	// Stats fetchs process usage stats for the executor and each pid if available
	Stats() (*cstructs.TaskResourceUsage, error)

	// Signal sends the given signal to the user process
	Signal(os.Signal) error

	// Exec executes the given command and args inside the executor context
	// and returns the output and exit code.
	Exec(deadline time.Time, cmd string, args []string) ([]byte, int, error)
}

// Resources describes the resource isolation required
type Resources struct {
	CPU      int
	MemoryMB int
	DiskMB   int
	IOPS     int
}

// ExecCommand holds the user command, args, and other isolation related
// settings.
type ExecCommand struct {
	// Cmd is the command that the user wants to run.
	Cmd string

	// Args is the args of the command that the user wants to run.
	Args []string

	// Resources defined by the task
	Resources *Resources

	// StdoutPath is the path the process stdout should be written to
	StdoutPath string
	stdout     io.WriteCloser

	// StderrPath is the path the process stderr should be written to
	StderrPath string
	stderr     io.WriteCloser

	// Env is the list of KEY=val pairs of environment variables to be set
	Env []string

	// User is the user which the executor uses to run the command.
	User string

	// TaskDir is the directory path on the host where for the task
	TaskDir string

	// ResourceLimits determines whether resource limits are enforced by the
	// executor.
	ResourceLimits bool

	// Cgroup marks whether we put the process in a cgroup. Setting this field
	// doesn't enforce resource limits. To enforce limits, set ResourceLimits.
	// Using the cgroup does allow more precise cleanup of processes.
	BasicProcessCgroup bool
}

// SetWriters sets the writer for the process stdout and stderr. This should
// not be used if writing to a file path such as a fifo file. SetStdoutWriter
// is mainly used for unit testing purposes.
func (c *ExecCommand) SetWriters(out io.WriteCloser, err io.WriteCloser) {
	c.stdout = out
	c.stderr = err
}

// GetWriters returns the unexported io.WriteCloser for the stdout and stderr
// handles. This is mainly used for unit testing purposes.
func (c *ExecCommand) GetWriters() (stdout io.WriteCloser, stderr io.WriteCloser) {
	return c.stdout, c.stderr
}

type nopCloser struct {
	io.Writer
}

func (nopCloser) Close() error { return nil }

// Stdout returns a writer for the configured file descriptor
func (c *ExecCommand) Stdout() (io.WriteCloser, error) {
	if c.stdout == nil {
		if c.StdoutPath != "" {
			f, err := fifo.Open(c.StdoutPath)
			if err != nil {
				return nil, fmt.Errorf("failed to create stdout: %v", err)
			}
			c.stdout = f
		} else {
			c.stdout = nopCloser{ioutil.Discard}
		}
	}
	return c.stdout, nil
}

// Stderr returns a writer for the configured file descriptor
func (c *ExecCommand) Stderr() (io.WriteCloser, error) {
	if c.stderr == nil {
		if c.StderrPath != "" {
			f, err := fifo.Open(c.StderrPath)
			if err != nil {
				return nil, fmt.Errorf("failed to create stderr: %v", err)
			}
			c.stderr = f
		} else {
			c.stderr = nopCloser{ioutil.Discard}
		}
	}
	return c.stderr, nil
}

func (c *ExecCommand) Close() {
	stdout, err := c.Stdout()
	if err == nil {
		stdout.Close()
	}
	stderr, err := c.Stderr()
	if err == nil {
		stderr.Close()
	}
}

// ProcessState holds information about the state of a user process.
type ProcessState struct {
	Pid      int
	ExitCode int
	Signal   int
	Time     time.Time
}

// ExecutorVersion is the version of the executor
type ExecutorVersion struct {
	Version string
}

func (v *ExecutorVersion) GoString() string {
	return v.Version
}
