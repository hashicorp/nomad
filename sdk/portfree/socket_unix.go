//go:build !windows

package portfree

import (
	"fmt"
	"net"
	"syscall"
)

func setSocketOpt(l *net.TCPListener) error {
	f, fileErr := l.File()
	if fileErr != nil {
		return fmt.Errorf("failed to open socket file: %w", fileErr)
	}

	h := int(f.Fd())
	setErr := syscall.SetsockoptLinger(h, syscall.SOL_SOCKET, syscall.SO_LINGER, &syscall.Linger{Onoff: 0, Linger: 0})
	if setErr != nil {
		return fmt.Errorf("failed to set linger option: %w", setErr)
	}

	closeErr := f.Close()
	if closeErr != nil {
		return fmt.Errorf("failed to close socket file: %w", closeErr)
	}

	return nil
}
