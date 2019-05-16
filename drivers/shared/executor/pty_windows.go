// +build windows

package executor

import (
	"fmt"
	"io"
	"os"
	"syscall"
)

func sessionCmdAttr(tty *os.File) *syscall.SysProcAttr {
	return &syscall.SysProcAttr{}
}

func setTTYSize(w io.Writer, height, width int32) error {
	return fmt.Errorf("unsupported")

}

func isUnixEIOErr(err error) bool {
	return false
}
