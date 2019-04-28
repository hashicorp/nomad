// +build darwin dragonfly freebsd linux netbsd openbsd solaris

package executor

import (
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"github.com/kr/pty"
	"golang.org/x/sys/unix"
)

func sessionCmdAttr(tty *os.File) *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setsid:  true,
		Setctty: true,
		Ctty:    int(tty.Fd()),
	}
}

func setTTYSize(w io.Writer, height, width int32) error {
	f, ok := w.(*os.File)
	if !ok {
		return fmt.Errorf("attempted to resize a non-tty session")
	}

	return pty.Setsize(f, &pty.Winsize{
		Rows: uint16(height),
		Cols: uint16(width),
	})

}

func isUnixEIOErr(err error) bool {
	if err == nil {
		return false
	}

	return strings.Contains(err.Error(), unix.EIO.Error())
}
