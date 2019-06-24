// +build !windows

package windows

import (
	"fmt"
	"runtime"
)

func TerminateProcess(pid int) error {
	return fmt.Errorf("unimplemented on platform: %s", runtime.GOOS)
}
