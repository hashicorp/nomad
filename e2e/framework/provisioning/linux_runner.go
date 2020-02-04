package provisioning

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/pkg/errors"
)

// LinuxRunner is a ProvisioningRunner that runs on the executing host only.
// The Nomad configurations used with this runner will need to avoid port
// conflicts!
//
// Must call Open before other methods.
type LinuxRunner struct {
	// populated on Open.
	t *testing.T
}

// Open sets up the LinuxRunner to run using t as a logging mechanism.
func (runner *LinuxRunner) Open(t *testing.T) error {
	runner.t = t
	return nil
}

func parseCommand(command string) (string, []string) {
	fields := strings.Fields(strings.TrimSpace(command))
	if len(fields) == 1 {
		return fields[0], nil
	}
	return fields[0], fields[1:]
}

// Run the script (including any arguments)
func (runner *LinuxRunner) Run(script string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	commands := strings.Split(script, "\n")
	for _, command := range commands {
		executable, args := parseCommand(command)
		response, err := exec.CommandContext(ctx, executable, args...).CombinedOutput()

		// Nothing fancy around separating stdin from stdout, or failed vs
		// successful commands for now.
		runner.LogOutput(string(response))

		if err != nil {
			return errors.Wrapf(err, "failed to execute command %q", command)
		}
	}
	return nil
}

func (runner *LinuxRunner) Copy(local, remote string) error {
	cmd := exec.Command("cp", "-rf", local, remote)
	return cmd.Run()
}

func (runner *LinuxRunner) Close() {}

func (runner *LinuxRunner) Logf(format string, args ...interface{}) {
	if runner.t == nil {
		log.Fatal("no t.Testing configured for LinuxRunner")
	}
	if testing.Verbose() {
		fmt.Printf("[local] "+format+"\n", args...)
	} else {
		runner.t.Logf("[local] "+format, args...)
	}
}

func (runner *LinuxRunner) LogOutput(output string) {
	if testing.Verbose() {
		fmt.Println("\033[32m" + output + "\033[0m")
	} else {
		runner.t.Log(output)
	}
}
