// +build darwin dragonfly freebsd linux netbsd openbsd solaris

package command

import (
	"os"
	"os/signal"

	"golang.org/x/sys/unix"
)

func setupWindowNotification(ch chan<- os.Signal) {
	signal.Notify(ch, unix.SIGWINCH)
}
