// +build !windows
package fifo

import (
	"io"
	"os"
	"syscall"
)

func New(path string) (io.ReadWriteCloser, error) {
	err := syscall.Mkfifo(path, 0600)
	if err != nil {
		return nil, err
	}

	return Open(path)
}

func Open(path string) (io.ReadWriteCloser, error) {
	f, err := os.OpenFile(path, os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	return f, nil
}
