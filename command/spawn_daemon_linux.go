package command

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// Configuration for the command to start as a daemon.
type DaemonConfig struct {
	exec.Cmd

	// The paths, if not /dev/null, must be either in the tasks root directory
	// or in the shared alloc directory.
	StdoutFile string
	StdinFile  string
	StderrFile string

	Chroot string
}

// Whether to start the user command or abort.
type TaskStart bool

func (c *SpawnDaemonCommand) Run(args []string) int {
	flags := c.Meta.FlagSet("spawn-daemon", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got json input.
	args = flags.Args()
	if len(args) != 1 {
		c.Ui.Error(c.Help())
		return 1
	}
	jsonInput, err := strconv.Unquote(args[0])
	if err != nil {
		return c.outputStartStatus(fmt.Errorf("Failed to unquote json input: %v", err), 1)
	}

	// De-serialize the passed command.
	var cmd DaemonConfig
	dec := json.NewDecoder(strings.NewReader(jsonInput))
	if err := dec.Decode(&cmd); err != nil {
		return c.outputStartStatus(err, 1)
	}

	// Isolate the user process.
	if _, err := syscall.Setsid(); err != nil {
		return c.outputStartStatus(fmt.Errorf("Failed setting sid: %v", err), 1)
	}

	syscall.Umask(0)

	// Redirect logs.
	stdo, err := os.OpenFile(cmd.StdoutFile, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		return c.outputStartStatus(fmt.Errorf("Error opening file to redirect Stdout: %v", err), 1)
	}

	stde, err := os.OpenFile(cmd.StderrFile, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		return c.outputStartStatus(fmt.Errorf("Error opening file to redirect Stderr: %v", err), 1)
	}

	stdi, err := os.OpenFile(cmd.StdinFile, os.O_CREATE|os.O_RDONLY, 0666)
	if err != nil {
		return c.outputStartStatus(fmt.Errorf("Error opening file to redirect Stdin: %v", err), 1)
	}

	cmd.Cmd.Stdout = stdo
	cmd.Cmd.Stderr = stde
	cmd.Cmd.Stdin = stdi

	// Chroot jail the process and set its working directory.
	if cmd.Cmd.SysProcAttr == nil {
		cmd.Cmd.SysProcAttr = &syscall.SysProcAttr{}
	}

	cmd.Cmd.SysProcAttr.Chroot = cmd.Chroot
	cmd.Cmd.Dir = "/"

	// Wait to get the start command.
	var start TaskStart
	dec = json.NewDecoder(os.Stdin)
	if err := dec.Decode(&start); err != nil {
		return c.outputStartStatus(err, 1)
	}

	if !start {
		return 0
	}

	// Spawn the user process.
	if err := cmd.Cmd.Start(); err != nil {
		return c.outputStartStatus(fmt.Errorf("Error starting user command: %v", err), 1)
	}

	// Indicate that the command was started successfully.
	c.outputStartStatus(nil, 0)

	// Wait and then output the exit status.
	if err := cmd.Wait(); err != nil {
		return 1
	}

	return 0
}
