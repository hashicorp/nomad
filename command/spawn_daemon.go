package command

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

type SpawnDaemonCommand struct {
	Meta
	config   *DaemonConfig
	exitFile io.WriteCloser
}

func (c *SpawnDaemonCommand) Help() string {
	helpText := `
Usage: nomad spawn-daemon [options] <daemon_config>

  INTERNAL ONLY

  Spawns a daemon process by double forking. The required daemon_config is a
  json encoding of the DaemonConfig struct containing the isolation
  configuration and command to run. SpawnStartStatus is json serialized to
  stdout upon running the user command or if any error prevents its execution.
  If there is no error, the process waits on the users command. Once the user
  command exits, the exit code is written to a file specified in the
  daemon_config and this process exits with the same exit status as the user
  command.
  `

	return strings.TrimSpace(helpText)
}

func (c *SpawnDaemonCommand) Synopsis() string {
	return "Spawn a daemon command with configurable isolation."
}

// Status of executing the user's command.
type SpawnStartStatus struct {
	// The PID of the user's command.
	UserPID int

	// ErrorMsg will be empty if the user command was started successfully.
	// Otherwise it will have an error message.
	ErrorMsg string
}

// Exit status of the user's command.
type SpawnExitStatus struct {
	// The exit code of the user's command.
	ExitCode int
}

// Configuration for the command to start as a daemon.
type DaemonConfig struct {
	exec.Cmd

	// The filepath to write the exit status to.
	ExitStatusFile string

	// The paths, if not /dev/null, must be either in the tasks root directory
	// or in the shared alloc directory.
	StdoutFile string
	StdinFile  string
	StderrFile string

	// An optional path specifying the directory to chroot the process in.
	Chroot string
}

// Whether to start the user command or abort.
type TaskStart bool

// parseConfig reads the DaemonConfig from the passed arguments. If not
// successful, an error is returned.
func (c *SpawnDaemonCommand) parseConfig(args []string) (*DaemonConfig, error) {
	flags := c.Meta.FlagSet("spawn-daemon", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	if err := flags.Parse(args); err != nil {
		return nil, fmt.Errorf("failed to parse args: %v", err)
	}

	// Check that we got json input.
	args = flags.Args()
	if len(args) != 1 {
		return nil, fmt.Errorf("incorrect number of args; got %v; want 1", len(args))
	}
	jsonInput, err := strconv.Unquote(args[0])
	if err != nil {
		return nil, fmt.Errorf("Failed to unquote json input: %v", err)
	}

	// De-serialize the passed command.
	var config DaemonConfig
	dec := json.NewDecoder(strings.NewReader(jsonInput))
	if err := dec.Decode(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// configureLogs creates the log files and redirects the process
// stdin/stderr/stdout to them. If unsuccessful, an error is returned.
func (c *SpawnDaemonCommand) configureLogs() error {
	stdo, err := os.OpenFile(c.config.StdoutFile, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("Error opening file to redirect stdout: %v", err)
	}

	stde, err := os.OpenFile(c.config.StderrFile, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("Error opening file to redirect stderr: %v", err)
	}

	stdi, err := os.OpenFile(c.config.StdinFile, os.O_CREATE|os.O_RDONLY, 0666)
	if err != nil {
		return fmt.Errorf("Error opening file to redirect stdin: %v", err)
	}

	c.config.Cmd.Stdout = stdo
	c.config.Cmd.Stderr = stde
	c.config.Cmd.Stdin = stdi
	return nil
}

func (c *SpawnDaemonCommand) Run(args []string) int {
	var err error
	c.config, err = c.parseConfig(args)
	if err != nil {
		return c.outputStartStatus(err, 1)
	}

	// Open the file we will be using to write exit codes to. We do this early
	// to ensure that we don't start the user process when we can't capture its
	// exit status.
	c.exitFile, err = os.OpenFile(c.config.ExitStatusFile, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return c.outputStartStatus(fmt.Errorf("Error opening file to store exit status: %v", err), 1)
	}

	// Isolate the user process.
	if err := c.isolateCmd(); err != nil {
		return c.outputStartStatus(err, 1)
	}

	// Redirect logs.
	if err := c.configureLogs(); err != nil {
		return c.outputStartStatus(err, 1)
	}

	// Chroot jail the process and set its working directory.
	c.configureChroot()

	// Wait to get the start command.
	var start TaskStart
	dec := json.NewDecoder(os.Stdin)
	if err := dec.Decode(&start); err != nil {
		return c.outputStartStatus(err, 1)
	}

	// Aborted by Nomad process.
	if !start {
		return 0
	}

	// Spawn the user process.
	if err := c.config.Cmd.Start(); err != nil {
		return c.outputStartStatus(fmt.Errorf("Error starting user command: %v", err), 1)
	}

	// Indicate that the command was started successfully.
	c.outputStartStatus(nil, 0)

	// Wait and then output the exit status.
	return c.writeExitStatus(c.config.Cmd.Wait())
}

// outputStartStatus is a helper function that outputs a SpawnStartStatus to
// Stdout with the passed error, which may be nil to indicate no error. It
// returns the passed status.
func (c *SpawnDaemonCommand) outputStartStatus(err error, status int) int {
	startStatus := &SpawnStartStatus{}
	enc := json.NewEncoder(os.Stdout)

	if err != nil {
		startStatus.ErrorMsg = err.Error()
	}

	if c.config != nil && c.config.Process != nil {
		startStatus.UserPID = c.config.Process.Pid
	}

	enc.Encode(startStatus)
	return status
}

// writeExitStatus takes in the error result from calling wait and writes out
// the exit status to a file. It returns the same exit status as the user
// command.
func (c *SpawnDaemonCommand) writeExitStatus(exit error) int {
	// Parse the exit code.
	exitStatus := &SpawnExitStatus{}
	if exit != nil {
		// Default to exit code 1 if we can not get the actual exit code.
		exitStatus.ExitCode = 1

		if exiterr, ok := exit.(*exec.ExitError); ok {
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				exitStatus.ExitCode = status.ExitStatus()
			}
		}
	}

	if c.exitFile != nil {
		enc := json.NewEncoder(c.exitFile)
		enc.Encode(exitStatus)
		c.exitFile.Close()
	}

	return exitStatus.ExitCode
}
