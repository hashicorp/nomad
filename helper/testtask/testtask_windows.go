// +build windows

package testtask

import (
	"fmt"
	"os"
)

func executeProcessGroup(gid string) {
	fmt.Fprintf(os.Stderr, "process groups are not supported on windows\n")
	os.Exit(1)
}
