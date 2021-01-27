package e2eutil

import (
	"os/exec"
)

// Command sends a command line argument to Nomad and returns the unbuffered
// stdout as a string (or, if there's an error, the stderr)
func Command(cmd string, args ...string) (string, error) {
	out, err := exec.Command(cmd, args...).CombinedOutput()
	return string(out), err
}
