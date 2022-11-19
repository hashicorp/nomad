package subproc

import (
	"fmt"
	"os"
)

var (
	// executable is the executable of this process
	executable string
)

func init() {
	s, err := os.Executable()
	if err != nil {
		panic(fmt.Sprintf("failed to detect executable: %v", err))
	}
	executable = s
}

// Self returns the path to the executable of this process.
func Self() string {
	return executable
}
