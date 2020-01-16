package provisioning

import (
	"os/exec"
	"strings"
	"testing"
)

// LinuxRunner is a ProvisioningRunner that runs on the executing host only.
// The Nomad configurations used with this runner will need to avoid port
// conflicts!
type LinuxRunner struct{}

func (runner *LinuxRunner) Open(_ *testing.T) error { return nil }

func (runner *LinuxRunner) Run(script string) error {
	commands := strings.Split(script, "\n")
	for _, command := range commands {
		cmd := exec.Command(strings.TrimSpace(command))
		err := cmd.Run()
		if err != nil {
			return err
		}
	}
	return nil
}

func (runner *LinuxRunner) Copy(local, remote string) error {
	cmd := exec.Command("cp", "-rf", local, remote)
	return cmd.Run()
}

func (runner *LinuxRunner) Close() {}
