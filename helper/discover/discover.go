package discover

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kardianos/osext"
)

const (
	nomadExe = "nomad"
)

// Checks the current executable, then $GOPATH/bin, and finally the CWD, in that
// order. If it can't be found, an error is returned.
func NomadExecutable() (string, error) {
	// Check the current executable.
	bin, err := osext.Executable()
	if err != nil {
		return "", fmt.Errorf("Failed to determine the nomad executable: %v", err)
	}

	if filepath.Base(bin) == nomadExe {
		return bin, nil
	}

	// Check the $GOPATH.
	bin = filepath.Join(os.Getenv("GOPATH"), "bin", nomadExe)
	if _, err := os.Stat(bin); err == nil {
		return bin, nil
	}

	// Check the CWD.
	bin = filepath.Join(os.Getenv("GOPATH"), "bin", nomadExe)
	if _, err := os.Stat(bin); err == nil {
		return bin, nil
	}

	pwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("Could not find Nomad executable (%v): %v", err)
	}

	bin = filepath.Join(pwd, nomadExe)
	if _, err := os.Stat(bin); err == nil {
		return bin, nil
	}

	return "", fmt.Errorf("Could not find Nomad executable (%v)", nomadExe)
}
